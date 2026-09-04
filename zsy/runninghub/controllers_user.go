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
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
)

// rhPlatform is the stable TaskPlatform name used to route to the RunningHub
// adaptor. It matches ChannelTypeRunningHub (61) cast as a platform string
// which is what relay.GetTaskAdaptor consults for plugin registrations.
var rhPlatform = constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeRunningHub))

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
	// Per-second billing reads the seconds/duration schema parameter value and
	// carries it in metadata so the adaptor's EstimateBilling can apply it as
	// the "seconds" billing multiplier. The value is already validated and
	// bounded by coerceValueByType (MaxTaskDurationSeconds); default to 1 when
	// the app declares no seconds parameter.
	if app.PerSecondBilling {
		seconds := resolveSecondsParam(schema, payload.Values)
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
	// When the caller selected an API key for this run, bind the token's
	// billing context (id/key/unlimited/group/quota) onto the request before
	// GenRelayInfo snapshots it into RelayInfo. The host's dashboard
	// UserAuth path never sets token_* keys, so without this the task would
	// bill against TokenId=0.
	if payload.TokenId > 0 {
		if err := applySelectedToken(c, payload.TokenId); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
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
	//   - PerCallBilling → the per-call price is kept in sync with the table by
	//     syncAppBillingPrice, so UsePrice=true and the pre-charge is the fixed
	//     quota (task PerCallBilling skips diff settlement on completion).
	//   - dynamic billing → EstimateBilling contributes the "app_rate_ratio"
	//     OtherRatio (ModelBaseRateRatio) scaling the model's base price.
	c.Set("rh_app", app)

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

	// The site selector routes every attempt through a channel of the matching
	// type (RunningHub 61 / RunningHub Intl 62 / Liblib 63), exactly like a pin
	// but without naming a specific channel. It overrides the legacy
	// model->channel selection below; a pinned channel still takes priority.
	var pinnedChannel *model.Channel
	if app.ChannelID > 0 {
		pc, pcErr := model.GetChannelById(int(app.ChannelID), false)
		if pcErr != nil {
			common.ApiErrorMsg(c, "应用绑定渠道无效: "+pcErr.Error())
			return
		}
		pinnedChannel = pc
	}
	wantSiteType := siteToChannelType(app.Site)
	if app.Site != "" && wantSiteType == 0 {
		common.ApiErrorMsg(c, "非法的站点选择: "+app.Site)
		return
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel
		if pinnedChannel != nil {
			channel = pinnedChannel
			if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
				taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_pinned_channel_failed", http.StatusInternalServerError)
				break
			}
		} else if wantSiteType != 0 {
			// Site-scoped selection: pick a random enabled channel of the site's
			// type that advertises this app as a model. Kept local (no
			// request-path filtering) so the existing relay plumbing stays the
			// selection source of truth.
			siteChannel, selectErr := selectChannelBySiteType(c, app, wantSiteType, retryParam)
			if selectErr != nil {
				taskErr = service.TaskErrorWrapperLocal(selectErr.Err, "get_channel_failed", http.StatusBadRequest)
				break
			}
			channel = siteChannel
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
		}
		controller.AddUsedChannel(c, channel.Id)
		// Body storage must be refreshed per attempt because adaptors can
		// drain it.
		if bodyErr := replaceRequestBody(c, data); bodyErr != nil {
			taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			break
		}

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}
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
		if taskErr.StatusCode == http.StatusTooManyRequests {
			taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
		}
		c.JSON(taskErr.StatusCode, taskErr)
		return
	}
	publicTaskID := relayInfo.PublicTaskID
	if result != nil && publicTaskID != "" {
		userId, _ := c.Get("id")
		userIdInt, _ := userId.(int)
		task := &model.Task{
			TaskID:     publicTaskID,
			UserId:     userIdInt,
			Platform:   rhPlatform,
			Quota:      quotaFromResult(result),
			Action:     relayInfo.Action,
			Status:     model.TaskStatusInProgress,
			Data:       dataFromResult(result),
			ChannelId:  c.GetInt("channel_id"),
			Properties: model.Properties{OriginModelName: relayInfo.OriginModelName},
			PrivateData: model.TaskPrivateData{
				UpstreamTaskID: upstreamFromResult(result),
				TokenId:        relayInfo.TokenId,
				NodeName:       common.NodeName,
				BillingContext: &model.TaskBillingContext{
					ModelPrice:      relayInfo.PriceData.ModelPrice,
					GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
					ModelRatio:      relayInfo.PriceData.ModelRatio,
					OtherRatios:     relayInfo.PriceData.OtherRatios(),
					OriginModelName: relayInfo.OriginModelName,
					PerCallBilling: (common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice) && !app.PerSecondBilling,
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

// listMyRhTasks returns the current user's RunningHub tasks (paginated). It
// reuses the host task query filtered to the RH platform so the "generation
// records" panel renders the same TaskDto envelope as the submit path.
func listMyRhTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")

	status := strings.TrimSpace(c.Query("status"))
	params := model.SyncTaskQueryParams{Platform: rhPlatform, Status: status}

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
// that advertises the app's UpstreamID as a model for the request's group,
// using the host's weighted random pick so site-scoped apps participate in the
// normal group/weight distribution instead of bypassing it.
func selectChannelBySiteType(c *gin.Context, app *AppView, channelType int, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if retryParam != nil {
		ch, err := model.GetRandomSatisfiedChannel(retryParam.TokenGroup, app.UpstreamID, 0, retryParam.RequestPath)
		if err == nil && ch != nil && ch.Type == channelType {
			if setupErr := middleware.SetupContextForSelectedChannel(c, ch, app.UpstreamID); setupErr != nil {
				return nil, setupErr
			}
			return ch, nil
		}
	}
	// Fallback: any enabled channel of the site's type that advertises the app
	// as a model, so site-scoped apps keep working even when the caller's group
	// is not wired to a channel of that type. Exact model match is checked in Go
	// (the models column is a comma-separated list; a LIKE could over-match).
	var cands []model.Channel
	if err := db().
		Where("type = ? AND status = ?", channelType, common.ChannelStatusEnabled).
		Order("weight desc").
		Limit(100).
		Find(&cands).Error; err == nil {
		for i := range cands {
			ch := &cands[i]
			for _, m := range ch.GetModels() {
				if m == app.UpstreamID {
					if setupErr := middleware.SetupContextForSelectedChannel(c, ch, app.UpstreamID); setupErr != nil {
						return nil, setupErr
					}
					return ch, nil
				}
			}
		}
	}
	return nil, types.NewError(
		fmt.Errorf("站点 %s 下没有可用渠道 (type=%d, model=%s)", siteName(app.Site), channelType, app.UpstreamID),
		types.ErrorCodeGetChannelFailed,
		types.ErrOptionWithSkipRetry(),
	)
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
