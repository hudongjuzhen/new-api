package runninghub

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// TaskAdaptor implements channel.TaskAdaptor for RunningHub.
// Billing methods are delegated to taskcommon.BaseBilling — which means the
// default pricing is derived straight from `info.PriceData` (UsePrice /
// OtherRatios / ModelRatio set at the host level).
//
// Every plugin billing mode prices the run at submit time, and that charge is
// final: per-call uses FixedQuotaPerCall, per-second uses
// QuotaPerSecond × seconds, dynamic uses the channel base price ×
// ModelBaseRateRatio. The submit controller records such tasks with
// BillingContext.PerCallBilling, so the host skips its completion-time diff
// settlement, and this adaptor contributes no AdjustBillingOnComplete either.
//
// Re-pricing a run from RH's usage.consumeCoins was tried and removed: a RH coin
// has no established quota rate, and the "1 coin = 1 quota" reading (500000
// coins per dollar) turned a real 8-second, $0.24 run into $0.000098.
type TaskAdaptor struct {
	taskcommon.BaseBilling

	// channelID/key/baseURL are captured from the RelayInfo during Init.
	ChannelType int
	apiKey      string
	baseURL     string
	userAgent   string
}

// Compile-time check: TaskAdaptor satisfies channel.TaskAdaptor. Kept here so
// a missing method fails the package build rather than the relay layer's
// runtime assertion.
var _ channel.TaskAdaptor = (*TaskAdaptor)(nil)

// ConvertToOpenAIVideo also makes the adaptor an OpenAIVideoConverter, which is
// what GET /v1/videos/{task_id} requires; without it that endpoint answers
// 501 not_implemented for RunningHub tasks.
var _ channel.OpenAIVideoConverter = (*TaskAdaptor)(nil)

// Init copies the host's channel-level settings into the adaptor. This is the
// only call site where ChannelBaseUrl/ApiKey come from.
func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
	if a.baseURL == "" {
		switch info.ChannelType {
		case constant.ChannelTypeRunningHubIntl:
			a.baseURL = DefaultBaseURLIntl
		default:
			a.baseURL = DefaultBaseURL
		}
	}
	a.userAgent = DefaultUserAgent
}

// GetChannelName returns the short, stable name used in logs.
func (a *TaskAdaptor) GetChannelName() string { return "RunningHub" }

// GetModelList exposes the published apps as channel "models", in the gateway
// form `rh-app-<row id>` — that is the name a client sends to the standard relay
// endpoints and the name this adaptor resolves back to the app (see
// resolveUpstreamIDForModel).
//
// The bare-upstream-id convention still works for models added by hand (and for
// channels synced from the app table), so this list is additive, not a
// replacement. Querying the app table keeps the channel form's "fetch models"
// button honest — the previous placeholder list wrote three unusable names.
func (a *TaskAdaptor) GetModelList() []string {
	apps, err := publishedApps()
	if err != nil {
		common.SysError("runninghub: load app models failed: " + err.Error())
		return nil
	}
	models := make([]string, 0, len(apps)*2)
	for _, app := range apps {
		models = append(models, fmt.Sprintf("%s%d", RhAppModelPrefix, app.ID))
		if upstream := strings.TrimSpace(app.UpstreamID); upstream != "" {
			models = append(models, upstream)
		}
	}
	return models
}

// ValidateRequestAndSetAction decodes the submit body and sets info.Action.
// RunningHub accepts three upstream kinds: ai_app, workflow, and model.
// The `kind` metadata tag in the request body carries which one is used; when
// absent it falls back to ai_app because that's the most common RH submit.
//
// Validation deliberately follows the host's ValidateBasicTaskRequest to stay
// aligned with the standard TaskSubmitReq schema (prompt, model, images,
// metadata). RH-specific parameters are read from metadata.rh.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *taskdto.TaskError) {
	if info == nil || info.TaskRelayInfo == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("nil relay info"), "nil_relay_info", http.StatusInternalServerError)
	}
	taskErr = relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
	if taskErr != nil {
		return taskErr
	}

	// ValidateBasicTaskRequest already saved the parsed TaskSubmitReq into the
	// gin context under "task_request". Re-read it to extract kind.
	v, ok := c.Get("task_request")
	if !ok {
		return nil // default action already applied above.
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil
	}
	kind := extractKindFromMetadata(req.Metadata)
	switch kind {
	case "workflow":
		info.Action = "rh_workflow"
	case "model":
		info.Action = "rh_model"
	case "", "ai_app":
		info.Action = "rh_ai_app"
	default:
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("invalid rh kind: %s", kind),
			"invalid_rh_kind",
			http.StatusBadRequest,
		)
	}
	return nil
}

// extractKindFromMetadata returns the RH submission kind as declared in the
// request metadata. Both `rh.kind` (new) and direct `kind` (legacy) are
// honoured to keep imports curl-compatible.
func extractKindFromMetadata(m map[string]any) string {
	if m == nil {
		return ""
	}
	if rh, ok := m["rh"].(map[string]any); ok {
		if s, ok := rh["kind"].(string); ok && s != "" {
			return strings.ToLower(s)
		}
	}
	if s, ok := m["kind"].(string); ok && s != "" {
		return strings.ToLower(s)
	}
	return ""
}

// --- Request construction ------------------------------------------------

// RhAppModelPrefix is the model-name form that addresses a RunningHub app by its
// row id on this gateway: `rh-app-3` → rh_apps.id = 3.
//
// Why a name form at all: clients (the MV desktop app, and any third-party
// service) pick a *model* on the standard relay endpoints
// (/v1/video/generations, /v1/videos, /suno/submit/:action) — they have no other
// channel to say "run gateway app 3". Resolving it here keeps the client free of
// gateway ids and lets the model name appear in channel model lists / pricing
// like any other model.
//
// The plugin's own submit path (/api/zsy/rh/apps/:id/run) passes the app id in
// the URL instead and is unaffected.
const RhAppModelPrefix = "rh-app-"

// resolveUpstreamIDForModel maps a model name to the upstream app id used in the
// submit URL.
//
//   - `rh-app-<id>` → that app row's UpstreamID (and, when the row is missing,
//     an error so the caller reports a readable cause instead of posting to
//     `/run/ai-app/rh-app-3`, which upstream answers with a generic 901).
//   - anything else → returned unchanged: the historical convention (the bare
//     upstream id is the model name) keeps working, as do hand-written channel
//     entries.
//
// A DB failure is reported to the caller rather than silently falling back: a
// fallback would submit a request upstream cannot resolve, and the resulting
// error would point at the wrong place.
func resolveUpstreamIDForModel(model string) (string, error) {
	model = strings.TrimSpace(model)
	rawID, ok := strings.CutPrefix(model, RhAppModelPrefix)
	if !ok {
		return model, nil
	}
	if rawID == "" {
		return "", fmt.Errorf("模型名 %q 缺少应用 ID（应为 %s<应用行号>）", model, RhAppModelPrefix)
	}
	appID, convErr := strconv.ParseUint(rawID, 10, 32)
	if convErr != nil {
		return "", fmt.Errorf("模型名 %q 里的应用 ID 不是数字", model)
	}
	app, err := AppGetByID(uint(appID))
	if err != nil {
		return "", fmt.Errorf("模型 %q 指向的应用不存在或不合法: %w", model, err)
	}
	if strings.TrimSpace(app.UpstreamID) == "" {
		return "", fmt.Errorf("应用 %s 没有配置上游应用 ID，无法提交", model)
	}
	return app.UpstreamID, nil
}

// BuildRequestURL composes the submit URL using info.Action to decide which
// RH path to POST to. The app id travels in the model name (see
// resolveUpstreamIDForModel): either the bare upstream id (historical
// convention) or the gateway form `rh-app-<row id>`.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("nil relay info")
	}
	base := a.baseURL
	if base == "" {
		if a.ChannelType == constant.ChannelTypeRunningHubIntl {
			base = DefaultBaseURLIntl
		} else {
			base = DefaultBaseURL
		}
	}
	model := strings.TrimSpace(info.OriginModelName)
	switch info.Action {
	case "rh_ai_app":
		if model == "" {
			return "", fmt.Errorf("runninghub: origin model (app id) is empty")
		}
		upstreamID, err := resolveUpstreamIDForModel(model)
		if err != nil {
			return "", err
		}
		return base + PathSubmitAICApp + upstreamID, nil
	case "rh_workflow":
		if model == "" {
			return "", fmt.Errorf("runninghub: origin model (workflow id) is empty")
		}
		upstreamID, err := resolveUpstreamIDForModel(model)
		if err != nil {
			return "", err
		}
		return base + PathSubmitWorkflow + upstreamID, nil
	case "rh_model":
		// Model API path is stored as originModelName; callers prefix the
		// /openapi/v2/ prefix when appropriate.
		if model == "" {
			return "", fmt.Errorf("runninghub: model-API path is empty")
		}
		if strings.HasPrefix(model, "http") {
			return model, nil
		}
		if strings.HasPrefix(model, "/") {
			return base + model, nil
		}
		return base + openAPIV2Prefix + "/" + model, nil
	}
	return "", fmt.Errorf("runninghub: unsupported action %q", info.Action)
}

// BuildRequestHeader sets the RH-standard bearer auth and application JSON.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	if req == nil {
		return fmt.Errorf("nil request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("User-Agent", a.userAgent)
	return nil
}

// BuildRequestBody materialises the upstream V2 submit body.
//
// The function parses the host TaskSubmitReq: prompt → default (node 122,
// field prompt); model → id only used at URL level; images → first image
// → default image node. Anything richer is pulled through metadata.rh.nodes
// (array of {nodeId, fieldName, fieldValue} entries), in which case the
// explicit list takes precedence.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("runninghub: request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("runninghub: task_request wrong type")
	}
	body := SubmitBody{
		InstanceType:   pickInstanceType(extractInstanceTypeFromMetadata(req.Metadata)),
		NodeInfoList:   buildNodeInfoList(req, info.Action),
		WebhookURL:     extractWebhookFromMetadata(req.Metadata),
		AccessPassword: extractAccessPasswordFromMetadata(req.Metadata),
	}
	// Model-API calls pass the metadata as-is (the host body schema does not
	// match a node list anyway). The caller is responsible for crafting the
	// raw body via metadata.rh.rawBody when they need passthrough.
	if info.Action == "rh_model" {
		if raw, ok := extractRawModelBody(req.Metadata); ok {
			return bytes.NewReader(raw), nil
		}
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("runninghub build body: %w", err)
	}
	return bytes.NewReader(data), nil
}

// buildNodeInfoList converts a host TaskSubmitReq + metadata into a list of
// V2 submit nodes.
func buildNodeInfoList(req relaycommon.TaskSubmitReq, action string) []SubmitNodeInfo {
	// Explicit node list in metadata takes priority.
	if nodes, ok := extractExplicitNodes(req.Metadata); ok && len(nodes) > 0 {
		return nodes
	}
	out := make([]SubmitNodeInfo, 0, 2)
	// Default mapping: (node 122, "prompt") for the prompt; first image as
	// (node 121, "image") to match the canonical RH AI 应用 curl template.
	//
	// The actual (nodeId, fieldName) pair for a given upstream app is
	// determined by the admin when they import the curl; for the skeleton we
	// apply a conservative default so submit end-to-end tests can still work.
	if strings.TrimSpace(req.Prompt) != "" {
		out = append(out, SubmitNodeInfo{
			NodeID:     "122",
			FieldName:  "prompt",
			Field:      "prompt",
			FieldValue: req.Prompt,
		})
	}
	if len(req.Images) > 0 {
		out = append(out, SubmitNodeInfo{
			NodeID:     "121",
			FieldName:  "image",
			Field:      "image",
			FieldValue: req.Images[0],
		})
	}
	return out
}

// --- Submit round trip ---------------------------------------------------

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	if action := c.GetString("action"); action != "" && info != nil {
		info.Action = action
	}
	resp, err := channel.DoTaskApiRequest(a, c, info, requestBody)
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		common.SysLog(fmt.Sprintf(
			"[rh-debug] submit called url=%s key=%s body=model=%s",
			resp.Request.URL.String(), debugMaskedKey(info), info.OriginModelName,
		))
	}
	return resp, err
}

// debugMaskedKey exposes the bearer key actually sent to RunningHub for the
// temporary debug run: first 6 + last 4 chars, middle replaced with "…". This
// lets us correlate the exact key to the 90x response without leaking the full
// secret into the log.
func debugMaskedKey(info *relaycommon.RelayInfo) string {
	if info == nil || info.ApiKey == "" {
		return "(no channel key)"
	}
	k := info.ApiKey
	if len(k) <= 12 {
		return k[:2] + "…" + k[len(k)-2:]
	}
	return k[:6] + "…" + k[len(k)-4:]
}

// DoResponse parses the V2 submit response, returns (upstream taskId, raw
// bytes) and — unless the plugin's own run handler owns the response — writes
// the gateway answer to the client.
//
// Writing the body matters for the generic relay endpoints
// (/v1/video/generations, /v1/videos, /suno/submit/:action): the host expects
// the task adaptor to answer, exactly like the built-in task adaptors do
// (see relay/channel/task/sora). Without it an API-key caller received an empty
// 200 and could never learn the task id needed for polling or cancelling.
//
// A failing submit keeps the historical behaviour: the error tuple is returned
// and the host turns it into the error response.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	if resp == nil {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("nil response"), "nil_response", http.StatusBadGateway)
		return
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_rh_response", http.StatusInternalServerError)
		return
	}
	rhResp := &SubmitResp{}
	if err := common.Unmarshal(raw, rhResp); err != nil {
		taskErr = service.TaskErrorWrapper(err, "decode_rh_response", http.StatusInternalServerError)
		return
	}
	if rhResp.ErrorCode != "" && (rhResp.TaskID == "" || rhResp.Status == StatusFailed || common.JsonRawMessageToString(rhResp.FailedReason) != "") {
		common.SysLog(fmt.Sprintf(
			"[rh-debug] submit FAILED url=%s code=%s msg=%s raw=%s",
			debugResolvedURL(resp, info), rhResp.ErrorCode, rhResp.ErrorMessage, string(raw),
		))
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("code=%s msg=%s", rhResp.ErrorCode, rhResp.ErrorMessage),
			mapErrorCodeToTaskCode(rhResp.ErrorCode, rhResp.ErrorMessage),
			http.StatusBadRequest,
		)
		taskErr.Data = rhResp
		return rhResp.TaskID, raw, taskErr
	}

	if !controllerResponds(c) {
		publicTaskID := ""
		if info != nil && info.TaskRelayInfo != nil {
			publicTaskID = info.TaskRelayInfo.PublicTaskID
		}
		c.JSON(http.StatusOK, SubmitResponse{
			TaskID:         publicTaskID,
			TaskIDCompat:   publicTaskID,
			UpstreamTaskID: rhResp.TaskID,
			Status:         rhResp.Status,
			Raw:            raw,
		})
	}
	return rhResp.TaskID, raw, nil
}

// controllerResponds reports whether the plugin's own run handler writes the API
// response for this request, in which case the adaptor must stay silent.
func controllerResponds(c *gin.Context) bool {
	return c != nil && c.GetBool(contextKeyControllerResponds)
}

// debugResolvedURL returns the full URL the submit was dispatched to, or a
// best-effort reconstruction from the relay info when the response's tripped
// URL is missing.
func debugResolvedURL(resp *http.Response, info *relaycommon.RelayInfo) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.String() != "" {
		return resp.Request.URL.String()
	}
	if info == nil {
		return "(no relay info)"
	}
	return fmt.Sprintf("channel_type=%d origin_model=%s base_url=%s", info.ChannelType, info.OriginModelName, info.ChannelBaseUrl)
}

// mapErrorCodeToTaskCode translates RH errorCode strings into stable codes
// suitable for the user-facing API.
func mapErrorCodeToTaskCode(code, msg string) string {
	switch code {
	case ErrCodeAccessDenied:
		if strings.Contains(strings.ToLower(msg), "standard model api") {
			return "rh_standard_model_requires_enterprise_key"
		}
		return "rh_access_denied"
	case ErrCodeParams:
		return "rh_invalid_params"
	case ErrCodeNodeInfoMismatch:
		return "rh_node_mismatch"
	case ErrCodeInvalidURL:
		return "rh_invalid_url"
	}
	return "rh_submit_failed"
}

// --- Polling --------------------------------------------------------------

// FetchTask posts to POST /openapi/v2/query with the upstream body format
// {"taskId":"<id>"} (camelCase, string).
func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("runninghub fetch: missing string task_id")
	}
	// Host stores task.Action on both sides; we ignore it for the query
	// endpoint because it's the same POST regardless of app/workflow kind.
	_ = body["action"]
	if baseURL == "" {
		if a.ChannelType == constant.ChannelTypeRunningHubIntl {
			baseURL = DefaultBaseURLIntl
		} else {
			baseURL = DefaultBaseURL
		}
	}
	fullURL := strings.TrimRight(baseURL, "/") + PathQueryTask
	qbody := QueryBody{TaskID: taskID}
	data, err := common.Marshal(qbody)
	if err != nil {
		return nil, fmt.Errorf("runninghub fetch marshal: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("runninghub fetch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", a.userAgent)

	hc, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("runninghub fetch proxy client: %w", err)
	}
	return hc.Do(req)
}

// ParseTaskResult converts the raw RH query response into the host TaskInfo
// format. Status mapping follows §3.9: QUEUED/RUNNING → PENDING; SUCCESS →
// SUCCESS; FAILED/CANCELED → FAILURE.
//
// usage.consumeCoins is deliberately NOT translated into billing numbers: a RH
// coin has no established quota rate, and the old "1 coin = 1 quota" mapping
// (500000 coins per dollar) undercharged every settled run. The raw usage stays
// visible in the task row, because the poller stores this whole response body in
// task.Data.
//
// A response that carries an upstream error but no usable status
// ("errorCode":"1004", "errorMessage":"Task not found, please check the task
// ID | 任务不存在或已过期，请检查任务ID", "status":"") is a terminal failure: RH
// drops the task record, and since the RunningHub platforms are exempt from the
// local timeout sweep (see service.skipTimeoutForPlatform) nothing else would
// ever move the record out of QUEUED — the pre-charged quota stayed held while
// the panel kept showing "生成中". Mapping it to FAILURE lets the poller's
// failure branch close the record and refund.
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	if len(respBody) == 0 {
		return nil, fmt.Errorf("runninghub empty query response")
	}
	rh := &QueryResp{}
	if err := common.Unmarshal(respBody, rh); err != nil {
		return nil, fmt.Errorf("runninghub parse query: %w", err)
	}
	rawStatus := strings.ToUpper(strings.TrimSpace(rh.Status))
	if rh.hasError() && !isRHStatusKnown(rawStatus) {
		reason := pickFailureReason(rh)
		if isRHRequestError(rh.ErrorCode) {
			// Request-level errors (bad URL / unparseable body) describe our
			// own call, not the task. Failing every in-flight task on a
			// misconfigured base URL would refund live runs, so keep them
			// pending and leave a trace for the operator instead.
			common.SysError(fmt.Sprintf(
				"runninghub query returned request-level error code=%s msg=%s; keeping task %s in-flight",
				rh.ErrorCode, rh.ErrorMessage, rh.TaskID,
			))
		} else {
			return &relaycommon.TaskInfo{
				TaskID:   rh.TaskID,
				Status:   model.TaskStatusFailure,
				Reason:   reason,
				Progress: "100%",
			}, nil
		}
	}
	out := &relaycommon.TaskInfo{
		TaskID: rh.TaskID,
		Status: mapRHStatus(rawStatus),
	}
	switch out.Status {
	case model.TaskStatusSuccess:
		// Pick the first media URL as the primary result; the rest stay on
		// the raw task data so the UI can render them.
		for _, r := range rh.Results {
			if r.URL != "" {
				out.Url = r.URL
				out.RemoteUrl = r.URL
				break
			}
		}
		out.Progress = "100%"
	case model.TaskStatusFailure:
		out.Reason = pickFailureReason(rh)
		out.Progress = "100%"
	case model.TaskStatusQueued:
		out.Progress = "20%"
	case model.TaskStatusInProgress:
		out.Progress = "50%"
	default:
		out.Progress = "10%"
	}
	return out, nil
}

// ConvertToOpenAIVideo renders one stored task as the OpenAI video object that
// GET /v1/videos/{task_id} (and the OpenAI-compatible clients built on it)
// expects.
//
// The base object comes from the host's shared mapper, so status/progress
// translation (QUEUED → queued, IN_PROGRESS → in_progress, …) stays identical
// to every other task platform. RunningHub specifics are layered on top:
//
//   - every result URL of the run is exposed under metadata.results (the first
//     one is also metadata.url), because one RH run can emit several files;
//   - a failed run carries the upstream failure reason in error.message;
//   - the billed run length is exposed as `seconds` when the app is priced per
//     second.
func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	if originTask == nil {
		return nil, fmt.Errorf("runninghub: nil task")
	}
	video := originTask.ToOpenAIVideo()

	var rh QueryResp
	dataParsed := false
	if len(originTask.Data) > 0 && common.Unmarshal(originTask.Data, &rh) == nil {
		dataParsed = true
	}
	urls := rh.resultURLs()
	if len(urls) > 0 {
		video.SetMetadata("results", urls)
		video.SetMetadata("url", urls[0])
	}

	if originTask.Status == model.TaskStatusFailure {
		reason := originTask.FailReason
		if dataParsed {
			if parsed := pickFailureReason(&rh); parsed != "" {
				reason = parsed
			}
		}
		video.Error = &dto.OpenAIVideoError{
			Message: reason,
			Code:    "runninghub_task_failed",
		}
	}

	if billing := originTask.PrivateData.BillingContext; billing != nil {
		if seconds, ok := billing.OtherRatios["seconds"]; ok && seconds > 0 {
			video.Seconds = strconv.FormatFloat(seconds, 'f', -1, 64)
		}
	}

	return common.Marshal(video)
}

// resultURLs returns every media URL a query/submit body carries, in upstream
// order and without duplicates.
func (rh *QueryResp) resultURLs() []string {
	if rh == nil {
		return nil
	}
	urls := make([]string, 0, len(rh.Results))
	seen := make(map[string]bool, len(rh.Results))
	for _, r := range rh.Results {
		if r.URL == "" || seen[r.URL] {
			continue
		}
		seen[r.URL] = true
		urls = append(urls, r.URL)
	}
	return urls
}

// EstimateBilling returns the app-specific OtherRatios that scale the base
// model price:
//
//   - per-call apps      → nil. The flat price lives in the host model price
//     table (kept in sync by syncAppBillingPrice) and flows through
//     PriceData.UsePrice, with diff settlement skipped by the task's
//     PerCallBilling flag.
//   - per-second apps    → {"seconds": N} where N is the customer-chosen
//     seconds/duration parameter (stashed into metadata.rh.seconds by the
//     submit controller, default 1). Pre-charge becomes
//     QuotaPerSecond × N × groupRatio and is final: like per-call, the task is
//     recorded with PerCallBilling so the completion poll keeps it instead of
//     settling against RH usage.consumeCoins.
//   - dynamic apps       → {"app_rate_ratio": ModelBaseRateRatio} (skipped at
//     1.0).
//
// The app is stashed in the gin context by the plugin's submit controller;
// submissions that bypass the plugin controller (raw task API) get the plain
// base-price behaviour, matching BaseBilling.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	if c == nil {
		return nil
	}
	v, ok := c.Get("rh_app")
	if !ok {
		return nil
	}
	app, ok := v.(*AppView)
	if !ok || app == nil {
		return nil
	}
	if app.PerCallBilling {
		return nil
	}
	if app.PerSecondBilling {
		req, err := relaycommon.GetTaskRequest(c)
		if err != nil {
			return map[string]float64{"seconds": 1}
		}
		n := secondsFromMetadata(req.Metadata)
		if n < 1 {
			n = 1
		}
		return map[string]float64{"seconds": n}
	}
	if app.ModelBaseRateRatio == 1.0 {
		return nil
	}
	return map[string]float64{"app_rate_ratio": app.ModelBaseRateRatio}
}

// secondsFromMetadata reads the bounded seconds/duration value the submit
// controller stored under metadata.rh.seconds. Values already outside
// [1, MaxTaskDurationSeconds] are re-clamped here so a raw task-API caller
// (which bypasses the controller) can never feed an unbounded multiplier into
// the billing chain.
func secondsFromMetadata(m map[string]any) float64 {
	if m == nil {
		return 0
	}
	rh, ok := m["rh"].(map[string]any)
	if !ok {
		return 0
	}
	raw, ok := rh["seconds"].(float64)
	if !ok || raw < 1 {
		return 0
	}
	if raw > float64(relaycommon.MaxTaskDurationSeconds) {
		return float64(relaycommon.MaxTaskDurationSeconds)
	}
	return raw
}

// AdjustBillingOnComplete is intentionally NOT overridden: the embedded
// taskcommon.BaseBilling returns 0, which tells the host to keep the pre-charged
// quota. RunningHub runs are priced at submit time from configured values
// (per-call / per-second / dynamic ratio) and the submit controller flags them as
// final, so there is nothing to re-price on completion. Pass-through from RH's
// usage.consumeCoins must not come back until a real coin→quota rate exists —
// see ParseTaskResult.

// mapRHStatus translates RH upstream status to the host task status set.
func mapRHStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case StatusQueued:
		return model.TaskStatusQueued
	case StatusRunning:
		return model.TaskStatusInProgress
	case StatusSuccess:
		return model.TaskStatusSuccess
	case StatusFailed, StatusCanceled, StatusCancelled:
		return model.TaskStatusFailure
	}
	// Unknown status string → treat as pending so the poller keeps trying.
	return model.TaskStatusQueued
}

// isRHStatusKnown reports whether s is one of the task statuses RH is known to
// return for a live task. An empty or unrecognised value means the response
// carries no usable task state.
func isRHStatusKnown(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case StatusQueued, StatusRunning, StatusSuccess, StatusFailed, StatusCanceled, StatusCancelled:
		return true
	}
	return false
}

// isRHRequestError reports whether an error code describes the query call
// itself (bad path, unparseable body) rather than the task's state. Those codes
// are excluded from the "error means the task failed" rule so a broken base URL
// cannot mass-fail and mass-refund live tasks.
func isRHRequestError(code string) bool {
	switch strings.TrimSpace(code) {
	case ErrCodeInvalidURL, ErrCodeParams:
		return true
	}
	return false
}

// hasError reports whether the upstream response carries a task-level error.
func (rh *QueryResp) hasError() bool {
	if rh == nil {
		return false
	}
	return strings.TrimSpace(rh.ErrorCode) != "" || strings.TrimSpace(rh.ErrorMessage) != ""
}

// pickFailureReason prefers the upstream failedReason, then the error message;
// an empty JSON container (RH sends failedReason: {} on error-only responses)
// counts as absent so the reason is never the literal "{}".
func pickFailureReason(rh *QueryResp) string {
	if reason := common.JsonRawMessageToString(rh.FailedReason); !isEmptyJSONContainer(reason) {
		return reason
	}
	if msg := strings.TrimSpace(rh.ErrorMessage); msg != "" {
		return fmt.Sprintf("[%s] %s", rh.ErrorCode, msg)
	}
	return "runninghub task failed"
}

func isEmptyJSONContainer(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || s == "{}" || s == "[]" || s == "null"
}

// ---- tiny metadata accessors --------------------------------------------
//
// They are deliberately thin (and used only inside this package) to keep the
// adaptor readable. Any behaviour that grows more complex than these 3-line
// wrappers should move to a schema module.

func extractInstanceTypeFromMetadata(m map[string]any) string {
	if s, ok := metadataString(m, "rh", "instanceType"); ok {
		return s
	}
	if s, ok := metadataString(m, "", "instanceType"); ok {
		return s
	}
	return ""
}

func extractWebhookFromMetadata(m map[string]any) string {
	if s, ok := metadataString(m, "rh", "webhookUrl"); ok {
		return s
	}
	return ""
}

func extractAccessPasswordFromMetadata(m map[string]any) string {
	if s, ok := metadataString(m, "rh", "accessPassword"); ok {
		return s
	}
	return ""
}

func extractExplicitNodes(m map[string]any) ([]SubmitNodeInfo, bool) {
	raw := metadataValue(m, "rh", "nodes")
	if raw == nil {
		raw = metadataValue(m, "", "nodeInfoList")
	}
	if raw == nil {
		return nil, false
	}
	data, err := common.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var out []SubmitNodeInfo
	if err := common.Unmarshal(data, &out); err != nil {
		return nil, false
	}
	return out, len(out) > 0
}

func extractRawModelBody(m map[string]any) ([]byte, bool) {
	raw := metadataValue(m, "rh", "rawBody")
	if raw == nil {
		return nil, false
	}
	// If the metadata encoded the body as a string, pass it through directly.
	if s, ok := raw.(string); ok {
		return []byte(s), s != ""
	}
	data, err := common.Marshal(raw)
	if err != nil {
		return nil, false
	}
	return data, len(data) > 0
}

func metadataValue(m map[string]any, group, key string) any {
	if m == nil {
		return nil
	}
	if group != "" {
		sub, ok := m[group].(map[string]any)
		if !ok {
			return nil
		}
		return sub[key]
	}
	return m[key]
}

func metadataString(m map[string]any, group, key string) (string, bool) {
	v := metadataValue(m, group, key)
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, t != ""
	}
	return "", false
}
