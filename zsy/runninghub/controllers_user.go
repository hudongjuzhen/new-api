package runninghub

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
)

// rhPlatform is the stable TaskPlatform name used to route to the RunningHub
// adaptor. It matches ChannelTypeRunningHub (61) cast as a platform string
// which is what relay.GetTaskAdaptor consults for plugin registrations.
var rhPlatform = constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeRunningHub))

// channelTypePlatform maps a RunningHub-family channel type to the platform
// string the task is recorded under. Each site (61 国内站 / 62 国际站 / 63
// LiblibAI) is its own independent TaskPlatform — tasks never collapse to a
// single "61" regardless of the actual channel they ran on. The poller's
// per-platform rules (timeout exemption, etc.) read this exact value, so it
// must be the channel's own type.
func channelTypePlatform(channelType int) constant.TaskPlatform {
	return constant.TaskPlatform(strconv.Itoa(channelType))
}

// ---------------------------------------------------------------------------
// Public listing
// ---------------------------------------------------------------------------

// listPublicApps returns the published, non-admin-only apps visible to any
// unauthenticated or authenticated caller. It mirrors the admin list result
// envelope but with reduced fields (admin-only fields are stripped).
func listPublicApps(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	kind := strings.TrimSpace(c.Query("kind"))
	p, _ := strconv.Atoi(strings.TrimSpace(c.Query("p")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("page_size")))
	if p < 1 {
		p = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	published := true
	adminOnly := false
	sortBy := strings.ToLower(strings.TrimSpace(c.Query("sort_by")))
	sortOrder := strings.ToLower(strings.TrimSpace(c.Query("sort_order")))
	if sortBy == "" {
		sortBy = "id"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}
	res, err := AppSearch(AppListQuery{
		Keyword:   keyword,
		Kind:      kind,
		Published: &published,
		AdminOnly: &adminOnly,
		Page:      p,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, res)
}

// getPublicAppDetail returns the detail shape used by the dynamic form
// renderer on the user side. Published + non-admin-only guard keeps admin
// drafts invisible.
func getPublicAppDetail(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := AppGetByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if view == nil || !view.Published || view.AdminOnly {
		common.ApiErrorMsg(c, "应用不存在")
		return
	}
	common.ApiSuccess(c, view)
}

// ---------------------------------------------------------------------------
// Schema-driven submit
// ---------------------------------------------------------------------------

// AppRunPayload is the user-side submit body. Field values keyed by the
// same (nodeId/fieldName) identifier as ParamSchema.
type AppRunPayload struct {
	// Values is the flattened form state keyed by the field's identifier.
	Values map[string]any `json:"values"`
	// InstanceType overrides the app default (default / plus) when set.
	InstanceType string `json:"instanceType"`
	// WebhookURL optionally receives RH completion hooks.
	WebhookURL string `json:"webhookUrl"`
	// TokenId optionally pins the API key (token) this task bills against.
	// Zero means the host picks the user's default token, which is the
	// standard path for requests that carry their own Authorization header.
	TokenId int64 `json:"tokenId,omitempty"`
}

// submitAppRun validates the form payload against the published app's
// ParamSchema then composes a host TaskSubmitReq with the RH metadata
// signature required by the adaptor, and runs the full RelayTaskSubmit
// pipeline (channel select → pre-consume → submit → settle → insert task).
func submitAppRun(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	app, err := AppGetByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app == nil || !app.Published {
		common.ApiErrorMsg(c, "应用不存在")
		return
	}
	if app.AdminOnly {
		common.ApiErrorMsg(c, "该应用仅管理员可见")
		return
	}

	var payload AppRunPayload
	if err := common.DecodeJson(c.Request.Body, &payload); err != nil {
		common.ApiErrorMsg(c, "无效的提交内容: "+err.Error())
		return
	}

	schema := app.ParamSchema
	nodeInfos, valErr := validateAndBuildNodeInfoList(schema, payload.Values)
	if valErr != nil {
		common.ApiErrorMsg(c, valErr.Error())
		return
	}

	instanceType := strings.TrimSpace(payload.InstanceType)
	if instanceType == "" {
		instanceType = InstanceDefault
	}
	switch instanceType {
	case InstanceDefault, InstancePlus:
	default:
		common.ApiErrorMsg(c, fmt.Sprintf("非法 instanceType: %q (可选 %s / %s)", instanceType, InstanceDefault, InstancePlus))
		return
	}

	metadata := map[string]any{
		"rh": map[string]any{
			"kind":         string(app.Kind),
			"upstreamId":   app.UpstreamID,
			"nodes":        nodeInfos,
			"instanceType": instanceType,
		},
	}
	if strings.TrimSpace(payload.WebhookURL) != "" {
		metadata["rh"].(map[string]any)["webhookUrl"] = strings.TrimSpace(payload.WebhookURL)
	}
	// Per-second billing needs the run length before the task is submitted. The
	// app's configured seconds expression (e.g. "229-212", "nodeId=212") wins;
	// apps without one fall back to a duration/seconds-typed parameter. Values
	// are already bounded by coerceValueByType / secondsFromExpr.
	if app.PerSecondBilling {
		seconds, secondsErr := resolveAppSeconds(app, schema, payload.Values)
		if secondsErr != nil {
			common.ApiErrorMsg(c, secondsErr.Error())
			return
		}
		metadata["rh"].(map[string]any)["seconds"] = seconds
	}
	// The host's task validation (ValidateBasicTaskRequest) requires a
	// non-empty prompt even for schema-driven apps. Derive it from the first
	// non-empty string field value; the upstream payload itself is carried by
	// metadata.rh.nodes, so this prompt is bookkeeping only.
	prompt := ""
	for _, p := range schema {
		v, ok := payload.Values[schemaFieldKey(p.NodeID, p.FieldName)]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			prompt = s
			break
		}
	}
	if prompt == "" {
		prompt = app.Name
	}
	req := relaycommon.TaskSubmitReq{
		Model:    app.UpstreamID,
		Prompt:   prompt,
		Metadata: metadata,
	}
	data, err := common.Marshal(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Reset body so ValidateBasicTaskRequest / relay adaptors can read it
	// using the host's standard UnmarshalBodyReusable helper.
	if err := replaceRequestBody(c, data); err != nil {
		common.ApiError(c, err)
		return
	}
	// Token binding. A dashboard caller may pick which of its keys pays for the
	// run; an API-key caller already is a key, so its own token context (set by
	// middleware.TokenAuth) is the only acceptable one — asking for a different
	// key is rejected rather than silently ignored, because the key that pays is
	// also the key that may later read or cancel the task.
	if callerUsesAPIKey(c) {
		callerTokenID := c.GetInt(string(constant.ContextKeyTokenId))
		if payload.TokenId > 0 && int(payload.TokenId) != callerTokenID {
			apiError(c, http.StatusBadRequest, ErrorCodeTokenSelectionNotAllowed,
				"使用 API Key 调用时不能指定其他密钥，本次调用只能由该 Key 计费")
			return
		}
	} else if payload.TokenId > 0 {
		// The host's dashboard UserAuth path never sets token_* keys, so without
		// this the task would bill against TokenId=0.
		if err := applySelectedToken(c, payload.TokenId); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	}
	// Token model limits live in middleware.Distribute on the /v1/* relay path;
	// this handler picks its own channel from the site pool, so the allow-list
	// has to be enforced here or a key limited to a model list could run any app.
	if !tokenAllowsApp(c, app.UpstreamID) {
		apiError(c, http.StatusForbidden, ErrorCodeTokenModelForbidden,
			fmt.Sprintf("当前密钥未被授权使用模型 %s", app.UpstreamID))
		return
	}
	c.Set("platform", rhPlatform)

	relayInfo, genErr := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if genErr != nil {
		common.ApiErrorMsg(c, "构建提交上下文失败: "+genErr.Error())
		return
	}
	relayInfo.OriginModelName = app.UpstreamID

	// Billing is derived inside RelayTaskSubmit, which rebuilds PriceData from
	// the host model price table (ModelPriceHelperPerCall). The app is stashed
	// for the adaptor:
	//   - per-call → the per-call price is kept in sync with the table by
	//     syncAppBillingPrice, so UsePrice=true and the pre-charge is the fixed
	//     quota.
	//   - per-second → EstimateBilling contributes the "seconds" OtherRatio
	//     (QuotaPerSecond × seconds, bounded by MaxTaskDurationSeconds).
	//   - dynamic billing → EstimateBilling contributes the "app_rate_ratio"
	//     OtherRatio (ModelBaseRateRatio) scaling the model's base price.
	// All three are fixed-price: their tasks are recorded with PerCallBilling so
	// the completion poll keeps the pre-charge instead of replacing it with RH's
	// coin usage (see taskChargeIsFinal).
	c.Set("rh_app", app)
	// This handler answers with the dashboard envelope (taskId + status), so the
	// task adaptor must not write a second body onto the same response.
	c.Set(contextKeyControllerResponds, true)

	// The plugin mounts its routes outside the host's Distribute middleware,
	// so — unlike the core task controllers — there is no pre-selected channel
	// in the gin context. Mark the relay info with an empty (non-nil)
	// ChannelMeta so GetChannelForRelay takes the real selection branch
	// (CacheGetRandomSatisfiedChannel + SetupContextForSelectedChannel);
	// RelayTaskSubmit rebuilds the meta from the selected channel's context
	// keys. Without this the first attempt would silently run on a stub
	// channel (id=0, no base URL, no key).
	if relayInfo.ChannelMeta == nil {
		relayInfo.ChannelMeta = &relaycommon.ChannelMeta{}
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	// The site path holds a concurrency slot from channel selection until the
	// task row is persisted. This defer is the safety net for every early return;
	// the retry path releases explicitly because it acquires a new slot per
	// attempt.
	var activeSlot *channelSlot
	defer func() {
		if activeSlot != nil {
			activeSlot.release()
		}
	}()
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}

	// The site field (site=cn | intl) is the authoritative routing input: every
	// attempt goes through an enabled channel of the matching site's channel
	// type (RunningHub 61 for cn / RunningHub 国际站 62 for intl) drawn from
	// the site's channel pool (weighted random, host distribution semantics).
	// Apps no longer bind a channel; there is no pin to honour.
	wantSiteType := siteToChannelType(app.Site)
	if app.Site != "" && wantSiteType == 0 {
		common.ApiErrorMsg(c, "非法的站点选择: "+app.Site)
		return
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel
		var slot *channelSlot
		if wantSiteType != 0 {
			// Site-scoped selection: pick an enabled channel of the site's type that
			// still has a free concurrency slot, queueing until one of them frees
			// one. Kept local (no request-path filtering) so the existing relay
			// plumbing stays the selection source of truth.
			siteChannel, siteSlot, selectErr := selectChannelBySiteType(c, app, wantSiteType, retryParam)
			if selectErr != nil {
				// Every channel of the site is at its cap: accept the task and let
				// the dispatcher run it as soon as a slot frees (queue.go). The
				// caller gets a QUEUED record instead of a wait or an error.
				if errors.Is(selectErr.Err, errChannelSaturated) {
					queuedTaskID, queueErr := enqueueAppRun(c, app, relayInfo)
					if queueErr != nil {
						taskErr = queueErr
						break
					}
					common.ApiSuccess(c, gin.H{
						"taskId": queuedTaskID,
						"status": string(model.TaskStatusQueued),
						"queued": true,
					})
					return
				}
				taskErr = taskErrorFromSelection(selectErr.Err)
				break
			}
			channel, slot = siteChannel, siteSlot
		} else if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = controller.GetChannelForRelay(c, relayInfo, retryParam)
			if channelErr != nil {
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
			// The concurrency cap also covers site-less apps. There is no candidate
			// pool to queue against on this path, so a saturated channel is reported
			// as busy instead of waiting for it.
			var slotErr error
			slot, slotErr = reserveChannelSlot(channel)
			if slotErr != nil {
				taskErr = service.TaskErrorWrapperLocal(slotErr, "channel_concurrency_check_failed", http.StatusInternalServerError)
				break
			}
			if slot == nil {
				taskErr = taskErrorFromSelection(saturatedError(app.Site))
				break
			}
		}
		activeSlot = slot
		controller.AddUsedChannel(c, channel.Id)
		// Body storage must be refreshed per attempt because adaptors can
		// drain it.
		if bodyErr := replaceRequestBody(c, data); bodyErr != nil {
			taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			break
		}

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			// The slot stays reserved: after the task row below is persisted the
			// database (non-terminal task count) takes the accounting over.
			break
		}
		slot.release()
		activeSlot = nil
		if !taskErr.LocalError {
			controller.ProcessChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					"", channel.GetAutoBan()),
				types.NewError(fmt.Errorf("%s: %s", taskErr.Code, taskErr.Message),
					types.ErrorCodeDoRequestFailed,
					types.ErrOptionWithStatusCode(taskErr.StatusCode)))
		}
		if !controller.ShouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes) {
			break
		}
	}

	if taskErr != nil {
		// A queue timeout already carries a queue-specific message; other 429s
		// (upstream rate limits) keep the generic wording.
		if taskErr.StatusCode == http.StatusTooManyRequests && taskErr.Code != errorCodeChannelSaturated {
			taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
		}
		c.JSON(taskErr.StatusCode, taskErr)
		return
	}
	publicTaskID := relayInfo.PublicTaskID
	if result != nil && publicTaskID != "" {
		userId, _ := c.Get("id")
		userIdInt, _ := userId.(int)
		// The task is recorded under the platform string of the channel it
		// actually ran on (61 国内站 / 62 国际站 / 63 LiblibAI), not a shared
		// "61". The poller's per-platform rules and the tasks list both key
		// off this value.
		taskPlatform := rhPlatform
		if channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType); channelType > 0 {
			taskPlatform = channelTypePlatform(channelType)
		}
		task := &model.Task{
			TaskID:    publicTaskID,
			UserId:    userIdInt,
			Platform:  taskPlatform,
			Quota:     quotaFromResult(result),
			Action:    relayInfo.Action,
			Status:    model.TaskStatusInProgress,
			Data:      dataFromResult(result),
			ChannelId: c.GetInt("channel_id"),
			// submit_time is what the timeout sweep's cutoff compares against;
			// without it (0) the task is immediately past every cutoff and would
			// be a timeout candidate on the very next sweep. It's also the
			// "任务提交时间" shown in the UI.
			SubmitTime: time.Now().Unix(),
			Properties: model.Properties{OriginModelName: relayInfo.OriginModelName},
			PrivateData: model.TaskPrivateData{
				UpstreamTaskID: upstreamFromResult(result),
				// Persist which funding source paid for this run so the
				// generation-records DTO can show per-record deduction source.
				BillingSource:  relayInfo.BillingSource,
				SubscriptionId: relayInfo.SubscriptionId,
				TokenId:        relayInfo.TokenId,
				NodeName:       common.NodeName,
				BillingContext: &model.TaskBillingContext{
					ModelPrice:      relayInfo.PriceData.ModelPrice,
					GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
					ModelRatio:      relayInfo.PriceData.ModelRatio,
					OtherRatios:     relayInfo.PriceData.OtherRatios(),
					OriginModelName: relayInfo.OriginModelName,
					PerCallBilling:  taskChargeIsFinal(relayInfo, app),
				},
			},
		}
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}
	common.ApiSuccess(c, gin.H{
		"taskId":         publicTaskID,
		"status":         string(model.TaskStatusInProgress),
		"upstreamTaskId": upstreamFromResult(result),
		"raw":            rawFromResult(result),
	})
}

// rhFamilyPlatforms is the set of TaskPlatform strings the RunningHub family
// can record tasks under (61 国内站 / 62 国际站 / 63 LiblibAI). The user-facing
// "generation records" list filters by this family so it sees tasks from every
// site regardless of which channel type they ran on.
var rhFamilyPlatforms = []constant.TaskPlatform{
	channelTypePlatform(constant.ChannelTypeRunningHub),     // "61"
	channelTypePlatform(constant.ChannelTypeRunningHubIntl), // "62"
	channelTypePlatform(constant.ChannelTypeLiblib),         // "63"
}

// listMyRhTasks returns the current account's RunningHub tasks (paginated). It
// reuses the host task query filtered to the RunningHub family (`"61"`, `"62"`,
// `"63"`) so the "generation records" panel renders tasks submitted through
// any of the three sites, each recorded under its own platform string.
//
// API-key callers are refused: the host task table indexes nothing by key (the
// paying key lives in the private_data JSON blob), so a key-scoped page cannot
// be filtered without a schema change. Answering the account-wide page instead
// would leak every other key's runs to the caller, and post-filtering a page is
// not pagination. A key polls the task ids it created.
func listMyRhTasks(c *gin.Context) {
	if callerUsesAPIKey(c) {
		apiError(c, http.StatusForbidden, ErrorCodeKeyScopedListUnsupported,
			"API Key 调用不支持列出历史任务，请使用 task_id 查询单个任务")
		return
	}
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")

	status := strings.TrimSpace(c.Query("status"))
	// Platform filter: only the RunningHub family platforms, never "all".
	params := model.SyncTaskQueryParams{Platforms: rhFamilyPlatforms, Status: status}

	items := model.TaskGetAllUserTask(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), params)
	total := model.TaskCountAllUserTask(userId, params)

	dtos := make([]*dto.TaskDto, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, relay.TaskModel2Dto(it))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(dtos)
	common.ApiSuccess(c, pageInfo)
}

// getAppTaskResult returns the user's task record by public task_id. Outputs
// a unified TaskDto envelope plus the RH result array when the task is done.
func getAppTaskResult(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		common.ApiErrorMsg(c, "task_id 不能为空")
		return
	}
	userId := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userId, taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiErrorMsg(c, "任务不存在或无权访问")
		return
	}
	if denied := tokenScopeDenied(c, task); denied != "" {
		apiError(c, http.StatusForbidden, ErrorCodeTaskNotOwnedByKey, denied)
		return
	}
	common.ApiSuccess(c, relay.TaskModel2Dto(task))
}

// ---------------------------------------------------------------------------
// Helpers (private)
// ---------------------------------------------------------------------------

// validateAndBuildNodeInfoList runs the typed schema validator and converts
// every matched field into a SubmitNodeInfo entry. Missing required fields,
// unknown keys, or out-of-bound numeric/select values cause a detailed error.
func validateAndBuildNodeInfoList(schema []rhparser.SchemaParam, values map[string]any) ([]SubmitNodeInfo, error) {
	if len(schema) == 0 {
		return nil, fmt.Errorf("当前应用未配置参数表单，请先在管理端导入 curl")
	}
	byKey := make(map[string]*rhparser.SchemaParam, len(schema))
	for i := range schema {
		p := &schema[i]
		key := schemaFieldKey(p.NodeID, p.FieldName)
		byKey[key] = p
	}

	// Missing required?
	var missing []string
	for key, p := range byKey {
		if !p.Required {
			continue
		}
		if _, ok := values[key]; !ok {
			missing = append(missing, missingFieldLabel(p, key))
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("缺少必填参数: %s", strings.Join(missing, ", "))
	}

	result := make([]SubmitNodeInfo, 0, len(values))
	for key, raw := range values {
		p, ok := byKey[key]
		if !ok {
			continue
		}
		coerced, err := coerceValueByType(p, raw)
		if err != nil {
			return nil, err
		}
		result = append(result, SubmitNodeInfo{
			NodeID:     p.NodeID,
			FieldName:  p.FieldName,
			Field:      p.FieldName,
			FieldValue: coerced,
		})
	}
	return result, nil
}

func missingFieldLabel(p *rhparser.SchemaParam, fallbackKey string) string {
	if strings.TrimSpace(p.Label) != "" {
		return p.Label
	}
	return fallbackKey
}

// applySelectedToken binds a caller-chosen API key onto the request so the
// task bills against that token (id/key/unlimited/group/quota) instead of the
// default TokenId=0 that the dashboard UserAuth path leaves behind. The token
// must belong to the logged-in user and be enabled; otherwise the request is
// rejected before any billing happens.
func applySelectedToken(c *gin.Context, tokenID int64) error {
	if tokenID <= 0 {
		return fmt.Errorf("非法的令牌 ID")
	}
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(int(tokenID), userId)
	if err != nil || token == nil {
		return fmt.Errorf("令牌不存在或不属于当前用户")
	}
	if token.Status != common.TokenStatusEnabled {
		return fmt.Errorf("所选令牌不可用（状态非启用）")
	}
	c.Set(string(constant.ContextKeyTokenId), token.Id)
	c.Set(string(constant.ContextKeyTokenKey), token.Key)
	c.Set(string(constant.ContextKeyTokenUnlimited), token.UnlimitedQuota)
	if !token.UnlimitedQuota {
		c.Set("token_quota", token.RemainQuota)
	}
	if token.Group != "" {
		c.Set(string(constant.ContextKeyTokenGroup), token.Group)
	}
	if token.ModelLimitsEnabled {
		c.Set(string(constant.ContextKeyTokenModelLimitEnabled), true)
		c.Set(string(constant.ContextKeyTokenModelLimit), token.GetModelLimitsMap())
	}
	return nil
}

// schemaFieldKey returns the stable identifier used as the Values map key.
// NodeID+FieldName mirrors RH's node info contract and avoids collisions
// when the same fieldName appears across nodes.
func schemaFieldKey(nodeID, fieldName string) string {
	return strings.TrimSpace(nodeID) + "." + strings.TrimSpace(fieldName)
}

// resolveAppSeconds returns the run length used for per-second billing. When
// the app configures a seconds expression (App.SecondsExpr) that expression is
// evaluated against the submitted values; otherwise the legacy scan for a
// duration/seconds-typed parameter applies. Both results are already bounded to
// [1, MaxTaskDurationSeconds] so the value can never become an unbounded quota
// multiplier.
func resolveAppSeconds(app *AppView, schema []rhparser.SchemaParam, values map[string]any) (float64, error) {
	if expr := strings.TrimSpace(app.SecondsExpr); expr != "" {
		return secondsFromExpr(expr, schema, values)
	}
	return resolveSecondsParam(schema, values), nil
}

// resolveSecondsParam returns the seconds/duration parameter value for
// per-second billing, bounded to [1, MaxTaskDurationSeconds] so the value can
// never grow into a quota multiplier that overflows (the same bound
// coerceValueByType applies during node validation). Apps whose schema has no
// seconds parameter fall back to 1 second.
func resolveSecondsParam(schema []rhparser.SchemaParam, values map[string]any) float64 {
	for _, p := range schema {
		switch strings.ToLower(strings.TrimSpace(p.Type)) {
		case "seconds", "duration":
		default:
			continue
		}
		v, ok := values[schemaFieldKey(p.NodeID, p.FieldName)]
		if !ok {
			continue
		}
		n, ok := asNumber(v)
		if !ok || n <= 0 {
			continue
		}
		if n > float64(relaycommon.MaxTaskDurationSeconds) {
			n = float64(relaycommon.MaxTaskDurationSeconds)
		}
		return n
	}
	return 1
}

// coerceValueByType enforces type bounds and returns the stringified value
// RH expects inside nodeInfoList[].fieldValue. Number bounds are clamped to
// relaycommon.MaxTaskDurationSeconds for video/audio durations; otherwise we
// use int64's usual 32-bit safety range (see AGENTS.md billing-safety rule).
func coerceValueByType(p *rhparser.SchemaParam, raw any) (string, error) {
	label := p.Label
	if label == "" {
		label = p.FieldName
	}
	switch strings.ToLower(strings.TrimSpace(p.Type)) {
	case "", "text", "textarea", "string", "password":
		s, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf("%s 必须为字符串", label)
		}
		if p.Max != nil && *p.Max > 0 && int64(len(s)) > int64(*p.Max) {
			return "", fmt.Errorf("%s 长度超过上限 %v 字符", label, *p.Max)
		}
		if p.Min != nil && *p.Min > 0 && int64(len(s)) < int64(*p.Min) {
			return "", fmt.Errorf("%s 长度不足下限 %v 字符", label, *p.Min)
		}
		return s, nil
	case "number", "int", "integer", "float", "duration", "seconds":
		n, ok := asNumber(raw)
		if !ok {
			return "", fmt.Errorf("%s 必须为数字", label)
		}
		lo, hi := billingBoundsFor(p)
		if n < lo {
			return "", fmt.Errorf("%s 不能小于 %v", label, lo)
		}
		if n > hi {
			return "", fmt.Errorf("%s 不能大于 %v", label, hi)
		}
		return numberToString(n), nil
	case "select", "radio", "enum":
		s, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf("%s 必须为字符串选项", label)
		}
		if len(p.Options) > 0 {
			found := false
			for _, o := range p.Options {
				if (o.Value != "" && o.Value == s) || o.Label == s {
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("%s 非法选项值: %q", label, s)
			}
		}
		return s, nil
	case "boolean", "bool", "checkbox", "toggle":
		b, ok := asBool(raw)
		if !ok {
			return "", fmt.Errorf("%s 必须为布尔值", label)
		}
		return strconv.FormatBool(b), nil
	case "image", "audio", "video", "file":
		s, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf("%s 必须为可访问的 URL 字符串", label)
		}
		if strings.TrimSpace(s) == "" && p.Required {
			return "", fmt.Errorf("%s 不能为空", label)
		}
		return s, nil
	default:
		// Unknown type: stringify best-effort to avoid breaking the submit
		// pipeline when admin added a custom type without frontend support.
		return fmt.Sprintf("%v", raw), nil
	}
}

func billingBoundsFor(p *rhparser.SchemaParam) (float64, float64) {
	var lo, hi float64
	if p.Min != nil {
		lo = *p.Min
	}
	if p.Max != nil {
		hi = *p.Max
	}
	switch strings.ToLower(strings.TrimSpace(p.Type)) {
	case "duration", "seconds":
		if hi == 0 || hi > float64(relaycommon.MaxTaskDurationSeconds) {
			hi = float64(relaycommon.MaxTaskDurationSeconds)
		}
	}
	// Top-level numeric ceiling: int32-ish so quota math saturates in range.
	const int32ishMax = 2_000_000_000
	if hi == 0 || hi > int32ishMax {
		hi = int32ishMax
	}
	return lo, hi
}

func numberToString(n float64) string {
	if n == float64(int64(n)) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func asNumber(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func asBool(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, false
		}
		return b, true
	}
	return false, false
}

func replaceRequestBody(c *gin.Context, body []byte) error {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	// Re-stash reusable-body cache so UnmarshalBodyReusable returns the same
	// bytes as the new body (helper stashes once on first call).
	c.Set("body_storage", body)
	return nil
}

// selectChannelBySiteType returns an enabled channel of the given channel type
// that still has a free concurrency slot, together with that granted slot.
//
// Channels are drawn from the site's pool with the host's priority/weight
// semantics, but only from those below their cap (ChannelSettings.MaxConcurrency,
// 0 = unlimited). When every channel of the site is at its cap the call fails
// with errChannelSaturated — the submit handler turns that into a QUEUED task
// record (queue.go) instead of making the caller wait.
//
// Channel models are intentionally NOT matched: a RunningHub channel is a site
// endpoint (one key, one base URL) and any app of the matching site runs
// through it; the upstream validates the app id itself. The call never returns
// a channel whose type differs from `channelType`.
func selectChannelBySiteType(c *gin.Context, app *AppView, channelType int, retryParam *service.RetryParam) (*model.Channel, *channelSlot, *types.NewAPIError) {
	if channelType == 0 {
		return nil, nil, types.NewError(
			fmt.Errorf("站点 %s 未配置 (channelType=0)", siteName(app.Site)),
			types.ErrorCodeGetChannelFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}
	group := ""
	startTier := 0
	if retryParam != nil {
		group = retryParam.TokenGroup
		startTier = retryParam.GetRetry()
	}

	candidates, err := siteCandidates(channelType, group)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if len(candidates) == 0 {
		return nil, nil, types.NewError(
			fmt.Errorf("站点 %s 下没有可用渠道 (type=%d)", siteName(app.Site), channelType),
			types.ErrorCodeGetChannelFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}

	channel, slot, reserveErr := reserveSlot(candidates, startTier)
	if reserveErr != nil {
		return nil, nil, types.NewError(reserveErr, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, nil, types.NewError(saturatedError(app.Site), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, app.UpstreamID); setupErr != nil {
		slot.release()
		return nil, nil, setupErr
	}
	return channel, slot, nil
}

// taskErrorFromSelection maps a channel-selection failure onto the error the
// submit API returns. A queue timeout (or a saturated channel on the site-less
// path) is a 429 with its own code so a client can tell "the upstream is busy,
// try again" apart from "no channel is configured at all".
func taskErrorFromSelection(selectionErr error) *dto.TaskError {
	if errors.Is(selectionErr, errChannelSaturated) {
		return service.TaskErrorWrapperLocal(selectionErr, errorCodeChannelSaturated, http.StatusTooManyRequests)
	}
	return service.TaskErrorWrapperLocal(selectionErr, "get_channel_failed", http.StatusBadRequest)
}

func quotaFromResult(r *relay.TaskSubmitResult) int {
	if r == nil {
		return 0
	}
	return r.Quota
}
func dataFromResult(r *relay.TaskSubmitResult) []byte {
	if r == nil {
		return nil
	}
	return r.TaskData
}
func upstreamFromResult(r *relay.TaskSubmitResult) string {
	if r == nil {
		return ""
	}
	return r.UpstreamTaskID
}
func rawFromResult(r *relay.TaskSubmitResult) any {
	if r == nil {
		return nil
	}
	var out any
	_ = common.Unmarshal(r.TaskData, &out)
	return out
}

// taskChargeIsFinal reports whether the charge decided at submit time is the
// final one for this run.
//
// Every plugin billing mode qualifies: per-call charges FixedQuotaPerCall,
// per-second charges QuotaPerSecond × seconds, and dynamic charges the channel's
// base model price × ModelBaseRateRatio. All three are computed from configured
// values when the run is submitted, so the completion poll must keep them and the
// host skips its diff settlement for a task flagged PerCallBilling.
//
// The settlement this suppresses re-priced the run from RH's usage.consumeCoins
// read as raw quota (1 coin = 1 quota in the adaptor's old ParseTaskResult) —
// 500000 coins per dollar, which undercharged a real 8-second run from $0.24 to
// $0.000098. Enabling pass-through again requires a real coin→quota rate.
func taskChargeIsFinal(info *relaycommon.RelayInfo, app *AppView) bool {
	if app != nil {
		return true
	}
	// Defensive fallback for a caller without an app in context: mirror the
	// host's own rule for a flat model price.
	return common.StringsContains(constant.TaskPricePatches, info.OriginModelName) ||
		info.PriceData.UsePrice
}

// ---------------------------------------------------------------------------
// Queued runs
// ---------------------------------------------------------------------------

// enqueueAppRun accepts a run whose site is at its concurrency cap: it prices
// and pre-charges the run, records the task as QUEUED ("排队中") and stores the
// built upstream request for the dispatcher. It returns the public task id.
//
// The record is created with progress="100%" and no channel on purpose — the
// core poller only picks up tasks whose progress is not 100%, and a task that
// has not reached the upstream has nothing to poll. Dispatch (queue.go) flips it
// to IN_PROGRESS with a real channel id.
func enqueueAppRun(c *gin.Context, app *AppView, info *relaycommon.RelayInfo) (string, *dto.TaskError) {
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequestAndSetAction(c, info); taskErr != nil {
		return "", taskErr
	}
	if taskErr := priceAndPreConsumeQueuedRun(c, info); taskErr != nil {
		return "", taskErr
	}
	bodyReader, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		return "", service.TaskErrorWrapper(err, "build_request_failed", http.StatusInternalServerError)
	}
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return "", service.TaskErrorWrapper(err, "read_request_body_failed", http.StatusInternalServerError)
	}
	if info.PublicTaskID == "" {
		info.PublicTaskID = model.GenerateTaskID()
	}

	siteChannelType := siteToChannelType(app.Site)
	submitPath, pathAbsolute := submitPathFor(app.Kind, app.UpstreamID)
	quota := info.PriceData.Quota
	task := &model.Task{
		TaskID:     info.PublicTaskID,
		UserId:     info.UserId,
		Platform:   channelTypePlatform(siteChannelType),
		Quota:      quota,
		Action:     info.Action,
		Status:     model.TaskStatusQueued,
		Progress:   "100%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{OriginModelName: info.OriginModelName},
		PrivateData: model.TaskPrivateData{
			BillingSource:  info.BillingSource,
			SubscriptionId: info.SubscriptionId,
			TokenId:        info.TokenId,
			NodeName:       common.NodeName,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      info.PriceData.ModelPrice,
				GroupRatio:      info.PriceData.GroupRatioInfo.GroupRatio,
				ModelRatio:      info.PriceData.ModelRatio,
				OtherRatios:     info.PriceData.OtherRatios(),
				OriginModelName: info.OriginModelName,
				PerCallBilling:  taskChargeIsFinal(info, app),
			},
		},
	}
	if err := task.Insert(); err != nil {
		return "", service.TaskErrorWrapper(err, "insert_task_failed", http.StatusInternalServerError)
	}

	entry := &RhQueuedTask{
		TaskID:       info.PublicTaskID,
		UserID:       info.UserId,
		Platform:     string(task.Platform),
		Site:         app.Site,
		SubmitPath:   submitPath,
		PathAbsolute: pathAbsolute,
		Body:         string(body),
		Group:        info.TokenGroup,
		CreatedAt:    time.Now().Unix(),
	}
	if err := enqueueTask(entry); err != nil {
		// Without a queue entry the task could never run, and the pre-charge is
		// already taken: fail the record and refund it.
		failQueuedTask(c.Request.Context(), task, "排队入队失败: "+err.Error())
		return "", service.TaskErrorWrapper(err, "enqueue_failed", http.StatusInternalServerError)
	}
	common.SysLog(fmt.Sprintf(
		"runninghub queue: task %s queued (site=%s, app=%s, quota=%d)",
		info.PublicTaskID, app.Site, app.Name, quota,
	))
	return info.PublicTaskID, nil
}

// priceAndPreConsumeQueuedRun mirrors the pricing + pre-consume steps of
// relay.RelayTaskSubmit so a queued task is charged exactly like a directly
// submitted one: base price → adaptor ratios → saturation-checked quota →
// pre-consume. Keep in sync with relay/relay_task.go steps 4-7.
func priceAndPreConsumeQueuedRun(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	priceData, err := helper.ModelPriceHelperPerCall(c, info)
	if err != nil {
		return service.TaskErrorWrapper(err, "model_price_error", http.StatusBadRequest)
	}
	info.PriceData = priceData

	adaptor := &TaskAdaptor{}
	for ratio, value := range adaptor.EstimateBilling(c, info) {
		info.PriceData.AddOtherRatio(ratio, value)
	}
	if !common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		quota, clamp := common.QuotaFromFloatChecked(
			info.PriceData.ApplyOtherRatiosToFloat(float64(info.PriceData.Quota)),
		)
		info.PriceData.Quota = quota
		// A saturated quota must fail the pre-consume below instead of being
		// silently charged; PreConsumeBilling rejects a non-nil clamp.
		if clamp != nil && info.QuotaClamp == nil {
			info.QuotaClamp = clamp
		}
	}
	if info.PriceData.FreeModel {
		return nil
	}
	info.ForcePreConsume = true
	if apiErr := service.PreConsumeBilling(c, info.PriceData.Quota, info); apiErr != nil {
		return service.TaskErrorFromAPIError(apiErr)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Cancel
// ---------------------------------------------------------------------------

// cancelAppTask cancels one of the caller's RunningHub tasks.
//
// A task that is still queued never reached the upstream, so cancelling it is
// purely local (fail the record and refund the pre-charge). A dispatched task is
// stopped upstream through the legacy cancel endpoint first; the local record is
// only closed once the upstream confirms, so a refused cancel cannot hand out a
// refund for a run that keeps burning upstream quota.
func cancelAppTask(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		common.ApiErrorMsg(c, "缺少 task_id")
		return
	}
	userId := c.GetInt("id")
	task, exist, err := model.GetByTaskId(userId, taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exist || task == nil {
		common.ApiErrorMsg(c, "任务不存在")
		return
	}
	if denied := tokenScopeDenied(c, task); denied != "" {
		apiError(c, http.StatusForbidden, ErrorCodeTaskNotOwnedByKey, denied)
		return
	}
	if !isRhFamilyPlatform(task.Platform) {
		common.ApiErrorMsg(c, "该任务不是 RunningHub 任务，无法在此取消")
		return
	}
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		common.ApiErrorMsg(c, "任务已结束，无法取消")
		return
	}

	ctx := c.Request.Context()
	// Still waiting for a slot: nothing reached the upstream yet.
	entry, entryErr := queuedTaskByTaskID(taskID)
	if entryErr == nil && entry != nil {
		if failQueuedTask(ctx, task, "用户取消") {
			deleteQueuedTask(taskID)
			common.ApiSuccess(c, gin.H{
				"taskId":   taskID,
				"status":   string(model.TaskStatusFailure),
				"refunded": true,
			})
			return
		}
		// The dispatcher claimed the task while we were deciding: fall through and
		// cancel it upstream instead.
		refreshed, exists, loadErr := model.GetByTaskId(userId, taskID)
		if loadErr == nil && exists && refreshed != nil {
			task = refreshed
		}
	}

	if task.ChannelId <= 0 {
		common.ApiErrorMsg(c, "任务尚未派发到渠道，请稍后重试")
		return
	}
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		common.ApiErrorMsg(c, "渠道不可用，无法取消: "+err.Error())
		return
	}
	baseURL, key := channelBaseAndKey(channel)
	if strings.TrimSpace(task.PrivateData.Key) != "" {
		key = task.PrivateData.Key
	}
	httpClient, err := service.GetHttpClientWithProxySettings(channel.GetSetting().Proxy, channel.GetSetting())
	if err != nil {
		common.ApiErrorMsg(c, "构建上游请求失败: "+err.Error())
		return
	}
	outcome, err := NewClientForType(channel.Type, baseURL, key, httpClient).
		CancelTask(task.GetUpstreamTaskID())
	if err != nil {
		common.ApiErrorMsg(c, "上游取消失败: "+err.Error())
		return
	}
	if outcome.NotAllowed {
		common.ApiErrorMsg(c, "上游不允许取消（任务可能已完成或已进入不可中断阶段）: "+outcome.Message)
		return
	}

	previous := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = "用户取消"
	won, err := task.UpdateWithStatus(previous)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !won {
		common.ApiErrorMsg(c, "任务状态已被其他操作更新，请刷新后重试")
		return
	}
	quota := task.Quota
	service.RefundTaskQuota(ctx, task, "用户取消")
	common.ApiSuccess(c, gin.H{
		"taskId":   taskID,
		"status":   string(model.TaskStatusFailure),
		"refunded": quota > 0,
		"quota":    quota,
	})
}

// isRhFamilyPlatform reports whether a task platform string belongs to the
// RunningHub family (61 国内站 / 62 国际站 / 63 LiblibAI).
func isRhFamilyPlatform(platform constant.TaskPlatform) bool {
	for _, candidate := range rhFamilyPlatforms {
		if platform == candidate {
			return true
		}
	}
	return false
}
