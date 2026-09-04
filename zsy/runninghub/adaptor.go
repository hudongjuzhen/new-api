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
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// TaskAdaptor implements channel.TaskAdaptor for RunningHub.
// Billing methods are delegated to taskcommon.BaseBilling — which means the
// default pricing is derived straight from `info.PriceData` (UsePrice /
// OtherRatios / ModelRatio set at the host level). RunningHub v1 uses
// per-call billing, which is expressed by the task caller setting
// PriceData.UsePrice=true when building the app's submit.
//
// Per-call tasks are settled at the pre-charged amount (the host skips the
// diff settlement for BillingContext.PerCallBilling). For dynamic-billing
// tasks the actual quota comes from RH's usage.consumeCoins: ParseTaskResult
// converts it into TaskInfo.CompletionTokens (1 coin = 1 quota, the v1
// semantic; per-coin pricing can later be layered on the model price config),
// and AdjustBillingOnCompleteChecked feeds it into the host's diff settlement
// while surfacing any quota saturation event as a *common.QuotaClamp.
type TaskAdaptor struct {
	taskcommon.BaseBilling

	// channelID/key/baseURL are captured from the RelayInfo during Init.
	ChannelType int
	apiKey      string
	baseURL     string
	userAgent   string

	// quotaClamp holds the saturation event (if any) captured while converting
	// usage.consumeCoins in ParseTaskResult, so the settle phase can audit it.
	quotaClamp *common.QuotaClamp
}

// Compile-time check: TaskAdaptor satisfies channel.TaskAdaptor. Kept here so
// a missing method fails the package build rather than the relay layer's
// runtime assertion.
var _ channel.TaskAdaptor = (*TaskAdaptor)(nil)

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

// GetModelList exposes the AI apps / workflows as selectable "models" in the
// UI / API. The host routing maps originModelName → appID in
// ValidateRequestAndSetAction by reading the metadata.
func (a *TaskAdaptor) GetModelList() []string {
	// TODO: query runninghub.App table and populate when wiring the management
	// API. For the skeleton we return a placeholder list so the channel
	// settings UI is not empty.
	return []string{
		"runninghub:ai-app",
		"runninghub:workflow",
		"runninghub:model",
	}
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

// BuildRequestURL composes the submit URL using info.Action to decide which
// RH path to POST to. The app id is resolved from info.OriginModelName and
// stored by the router's metadata hook. Skeleton does not yet decode the app
// id from model; it passes model as <id> directly so unit tests can verify
// the URL without touching a DB.
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
		return base + PathSubmitAICApp + model, nil
	case "rh_workflow":
		if model == "" {
			return "", fmt.Errorf("runninghub: origin model (workflow id) is empty")
		}
		return base + PathSubmitWorkflow + model, nil
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
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse parses the V2 submit response, returns (upstream taskId, raw
// bytes). The host already writes the response body to the client via the
// surrounding relay handler; this only extracts task bookkeeping data.
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
	// RH can return 200 OK with a flat protocol-level error code when the
	// submit itself failed (e.g. 1014 enterprise-shared key required). Those
	// are surfaced as task errors so the relay layer can refund the user.
	if rhResp.ErrorCode != "" && (rhResp.TaskID == "" || rhResp.Status == StatusFailed || common.JsonRawMessageToString(rhResp.FailedReason) != "") {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("code=%s msg=%s", rhResp.ErrorCode, rhResp.ErrorMessage),
			mapErrorCodeToTaskCode(rhResp.ErrorCode, rhResp.ErrorMessage),
			http.StatusBadRequest,
		)
		taskErr.Data = rhResp
		return rhResp.TaskID, raw, taskErr
	}
	return rhResp.TaskID, raw, nil
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
// SUCCESS; FAILED/CANCELED → FAILURE. Charges from usage.consumeCoins are
// fed back as CompletionTokens so the billing chain can settle.
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	if len(respBody) == 0 {
		return nil, fmt.Errorf("runninghub empty query response")
	}
	rh := &QueryResp{}
	if err := common.Unmarshal(respBody, rh); err != nil {
		return nil, fmt.Errorf("runninghub parse query: %w", err)
	}
	out := &relaycommon.TaskInfo{
		TaskID: rh.TaskID,
		Status: mapRHStatus(rh.Status),
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
	if rh.Usage != nil {
		if coins, err := parseFloatRelaxed(rh.Usage.ConsumeCoins); err == nil {
			// Coins → quota conversion is saturation-checked per the billing
			// invariants in AGENTS.md. A clamp is stashed so the settle-time
			// AdjustBillingOnCompleteChecked can surface it into the task
			// billing log's admin_info.quota_saturation.
			quota, clamp := common.QuotaFromFloatChecked(coins)
			out.CompletionTokens = quota
			a.quotaClamp = clamp
		}
	}
	return out, nil
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
//     QuotaPerSecond × N × groupRatio; the completion poll then diff-settles
//     against RH usage.consumeCoins (dynamic-billing semantics).
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

// AdjustBillingOnComplete returns the actual quota derived from RH's reported
// usage (usage.consumeCoins, converted in ParseTaskResult). Returning 0 keeps
// the pre-charged amount (or defers to the token-based recalculation).
func (a *TaskAdaptor) AdjustBillingOnComplete(_ *model.Task, taskResult *relaycommon.TaskInfo) int {
	if taskResult == nil || taskResult.CompletionTokens <= 0 {
		return 0
	}
	return taskResult.CompletionTokens
}

// AdjustBillingOnCompleteChecked is the clamp-audited variant used by the
// host's diff settlement: alongside the actual quota it returns the
// *common.QuotaClamp captured during the coins→quota conversion in
// ParseTaskResult, so the saturation event lands on the billing log's
// admin_info. QuotaClamp is nil for in-range conversions.
func (a *TaskAdaptor) AdjustBillingOnCompleteChecked(_ *model.Task, taskResult *relaycommon.TaskInfo) (int, *common.QuotaClamp) {
	if taskResult == nil || taskResult.CompletionTokens <= 0 {
		return 0, nil
	}
	return taskResult.CompletionTokens, a.quotaClamp
}

// mapRHStatus translates RH upstream status to the host task status set.
func mapRHStatus(s string) string {
	switch s {
	case StatusQueued:
		return model.TaskStatusQueued
	case StatusRunning:
		return model.TaskStatusInProgress
	case StatusSuccess:
		return model.TaskStatusSuccess
	case StatusFailed, StatusCanceled:
		return model.TaskStatusFailure
	}
	// Unknown status string → treat as pending so the poller keeps trying.
	return model.TaskStatusQueued
}

func pickFailureReason(rh *QueryResp) string {
	switch {
	case common.JsonRawMessageToString(rh.FailedReason) != "":
		return common.JsonRawMessageToString(rh.FailedReason)
	case rh.ErrorMessage != "":
		return fmt.Sprintf("[%s] %s", rh.ErrorCode, rh.ErrorMessage)
	}
	return "runninghub task failed"
}

// parseFloatRelaxed parses a string or string-shaped number, tolerating null
// and empty strings (upstream usage fields are optional).
func parseFloatRelaxed(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseFloat(s, 64)
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
