package doubao

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	SafetyIdentifier string         `json:"safety_identifier,omitempty"`
	Priority         *dto.IntValue  `json:"priority,omitempty"`
	Resolution       string         `json:"resolution,omitempty"`
	Ratio            string         `json:"ratio,omitempty"`
	Duration         *dto.IntValue  `json:"duration,omitempty"`
	Frames           *dto.IntValue  `json:"frames,omitempty"`
	Seed             *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed      *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark        *dto.BoolValue `json:"watermark,omitempty"`

	// ── 音频生成（Seed Audio）专用参数 ──────────────────────────────
	// 这些字段对视频模型无效，方舟会忽略；放在同一结构体里是为了让音频任务复用
	// 完全相同的任务接口与轮询链路。
	Format       string        `json:"format,omitempty"`
	SampleRate   *dto.IntValue `json:"sample_rate,omitempty"`
	SpeechRate   *dto.IntValue `json:"speech_rate,omitempty"`
	PitchRate    *dto.IntValue `json:"pitch_rate,omitempty"`
	LoudnessRate *dto.IntValue `json:"loudness_rate,omitempty"`
	References   []any         `json:"references,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseContent struct {
	VideoURL string  `json:"video_url"`
	AudioURL string  `json:"audio_url"`
	Duration float64 `json:"duration"`
}

type responseTask struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Status  string          `json:"status"`
	Content responseContent `json:"content"`
	Seed    int             `json:"seed"`
	// Duration 是任务级时长字段（部分模型在顶层返回生成时长）。
	Duration        float64 `json:"duration"`
	Resolution      string  `json:"resolution"`
	Ratio           string  `json:"ratio"`
	FramesPerSecond int     `json:"framespersecond"`
	ServiceTier     string  `json:"service_tier"`
	Tools           []struct {
		Type string `json:"type"`
	} `json:"tools"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		// AudioSeconds 是音频生成任务上报的产出时长（秒）。字段名按上游可能出现的
		// 形式并列声明，取第一个解析出正数的值。
		AudioSeconds float64 `json:"audio_seconds"`
		Duration     float64 `json:"duration"`
		ToolUsage    struct {
			WebSearch int `json:"web_search"`
		} `json:"tool_usage"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	// isAudioModel 标记本次请求是否按生成音频时长计费，Init 时按模型名分流。
	isAudioModel bool
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	if meta := info.ChannelMeta; meta != nil {
		a.ChannelType = meta.ChannelType
		a.baseURL = meta.ChannelBaseUrl
		a.apiKey = meta.ApiKey
	}
	a.isAudioModel = IsAudioModel(info.OriginModelName)
}

// audioModel 判断当前请求是否按生成音频时长计费。
//
// Init 记录的标记服务于请求构建阶段（那时任务记录还不存在）；轮询结算阶段没有
// RelayInfo，则从任务固化的模型名判断，两次判断依据的是同一个模型名。
func (a *TaskAdaptor) audioModel(task *model.Task) bool {
	if task != nil {
		return IsAudioModel(taskModelName(task))
	}
	return a.isAudioModel
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *taskdto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling 返回相对基准价的计费 OtherRatio。
//
// 视频任务：按请求 metadata 中的输出分辨率与是否包含视频输入计费。
//
// 音频任务（Seed Audio）：按"生成音频分钟数"计费，而真实时长只有任务成功后才
// 知道，因此这里返回的是保守预估秒数（见 EstimateAudioSeconds）。预估秒数既是
// 预扣额度，也是结算阶段退款的上限：任务成功后 MeteredUsage 用上游实际产出时长
// 重算，短了退款，长了以上限为准。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	if IsAudioModel(info.OriginModelName) {
		seconds := EstimateAudioSeconds(req.Duration, audioTextChars(req), ParseSpeechRate(req.Metadata["speech_rate"]))
		return map[string]float64{audioBillingKey: seconds}
	}
	hasVideo := hasVideoInMetadata(req.Metadata)
	resolution, _ := req.Metadata["resolution"].(string)
	ratio, ok := GetVideoInputRatio(info.OriginModelName, resolution, hasVideo)
	if !ok || ratio == 1.0 {
		return nil
	}
	return map[string]float64{"video_input": ratio}
}

// MeteredPreChargeQuota 记录音频任务在提交时按预估时长实际预扣的额度，随任务
// 固化。轮询结算阶段用它与预估秒数反推出有效单价，再乘以上游实际产出时长。
//
// 配置侧的模型单价是"每分钟"价格，而计量维度 audio_seconds 的单位是秒，因此这里
// 必须先折算再回传：预扣额度 = 单价 × QuotaPerUnit × 分组倍率 × 预估秒数 / 60。
func (a *TaskAdaptor) MeteredPreChargeQuota(info *relaycommon.RelayInfo) map[string]int {
	if info == nil || !IsAudioModel(info.OriginModelName) {
		return nil
	}
	base := info.PriceData.ModelPrice
	if !info.PriceData.UsePrice {
		base = info.PriceData.ModelRatio
	}
	seconds := info.PriceData.OtherRatios()[audioBillingKey]
	if !(base > 0) || !(seconds > 0) {
		return nil
	}
	quota, _ := common.QuotaFromFloatChecked(
		base * common.QuotaPerUnit * info.PriceData.GroupRatioInfo.GroupRatio * seconds / 60.0)
	if quota <= 0 {
		return nil
	}
	return map[string]int{audioBillingKey: quota}
}

// MeteredUsage 报告一次音频生成的结果产物需要重新计费的秒数。
//
// 上游结果里直接上报了产出时长时，ParseTaskResult 已经把它记进
// taskResult.MeteredUsage，这里返回 nil 交由核心结算；只有上游没上报时，才由
// 本方法读回产物音频文件实测时长作为兜底。
//
// 无论走哪条路径，返回值都以"提交时固化的预估秒数"为上界：预估即退款上限，
// 上游产出超长时不会向用户补扣。
func (a *TaskAdaptor) MeteredUsage(task *model.Task, taskResult *relaycommon.TaskInfo) map[string]float64 {
	if !a.audioModel(task) {
		return nil
	}
	if taskResult != nil && taskResult.MeteredUsage[audioBillingKey] > 0 {
		// 上游已上报产出时长，无需再下载音频文件。
		return nil
	}
	billingContext := task.PrivateData.BillingContext
	if billingContext == nil {
		return nil
	}
	maxSeconds := billingContext.OtherRatios[audioBillingKey]
	if !(maxSeconds > 0) || taskResult == nil || taskResult.Url == "" {
		return nil
	}
	measured, err := channel.MeasureAudioDuration(taskResult.Url)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("任务 %s 音频实测失败，保持预扣额度：%s", task.TaskID, err.Error()))
		return nil
	}
	seconds := relaycommon.ClampMeteredSeconds(measured, 1)
	if seconds > maxSeconds {
		seconds = maxSeconds
	}
	if seconds <= 0 {
		return nil
	}
	return map[string]float64{audioBillingKey: seconds}
}

// taskModelName 读取任务记录里的模型名（优先计费上下文，其次任务属性）。
func taskModelName(task *model.Task) string {
	if billingContext := task.PrivateData.BillingContext; billingContext != nil && billingContext.OriginModelName != "" {
		return billingContext.OriginModelName
	}
	return task.Properties.OriginModelName
}

// audioTextChars 统计本次音频生成要合成的文本长度（字符数），用于估算时长。
// 参考音频/参考图不参与估时：它们影响音色与风格，不按字符计费。
func audioTextChars(req relaycommon.TaskSubmitReq) int {
	chars := len([]rune(strings.TrimSpace(req.Prompt)))
	if chars > 0 {
		return chars
	}
	if text, ok := req.Metadata["text"].(string); ok {
		return len([]rune(strings.TrimSpace(text)))
	}
	return 0
}

// hasVideoInMetadata 直接检查 metadata 的 content 数组是否包含 video_url 条目，
// 避免构建完整的上游 requestPayload。
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] == "video_url" {
			return true
		}
		if _, has := itemMap["video_url"]; has {
			return true
		}
	}
	return false
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	// Add images if present
	if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
			})
		}
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	if sec, _ := strconv.Atoi(req.Seconds); sec > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(sec))
	}

	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	// 音频生成任务的请求体：文本走 content.text，其余为音频专用参数。
	// 方舟的音频任务与视频任务共用同一接口，因此这里只是把参数补全，不改协议。
	if IsAudioModel(req.Model) {
		if r.Format == "" {
			r.Format = "mp3"
		}
		if r.SampleRate == nil {
			r.SampleRate = lo.ToPtr(dto.IntValue(24000))
		}
	}

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		if taskResult.Url == "" {
			taskResult.Url = resTask.Content.AudioURL
		}
		// 解析 usage 信息用于按倍率计费
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
		// 音频生成任务：上游直接上报了产出时长时，它就是本次计费的计量基础，
		// 差额结算据此返还未生成的时长；拿不到则由 MeteredUsage 读回音频实测。
		// 只对按音频时长计费的模型记录：视频任务的时长来自请求参数，不是这里。
		if a.isAudioModel {
			taskResult.AddMeteredUsage(audioBillingKey, reportedAudioSeconds(&resTask), 1)
		}
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Error.Message
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", firstNonEmpty(dResp.Content.VideoURL, dResp.Content.AudioURL))
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: dResp.Error.Message,
			Code:    dResp.Error.Code,
		}
	}

	return common.Marshal(openAIVideo)
}

// reportedAudioSeconds 读取上游结果中上报的产出音频时长（秒）。
// 上游不同版本可能把时长放在 usage.audio_seconds、usage.duration、
// content.duration 或顶层 duration，取第一个解析出正数的值。
func reportedAudioSeconds(resTask *responseTask) float64 {
	if resTask == nil {
		return 0
	}
	for _, candidate := range []float64{
		resTask.Usage.AudioSeconds,
		resTask.Usage.Duration,
		resTask.Content.Duration,
		resTask.Duration,
	} {
		if candidate > 0 {
			return candidate
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
