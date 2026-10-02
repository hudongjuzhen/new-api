package doubaoaudio

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// ContextKeyAudioResult 是适配器把解析后的上游结果交给上层处理函数时使用的上下文键。
//
// 上游是同步返回，响应体里既有音频也有计费用的时长，两者只能在读完 body 之后才知道；
// DoResponse 又是唯一持有 response body 的地方。因此这里解析一次并把结果放进 gin
// 上下文，让处理函数按同一份数据记账，而不是把 body 读两遍。
// 键名与 relay.AudioGenerationResultKey 保持一致。
const ContextKeyAudioResult = "doubao_audio_result"

// Adaptor 是豆包语音音频创作（Seed Audio）的适配器。
//
// 它和方舟（volcengine / DoubaoVideo）是两个独立服务：主机是
// openspeech.bytedance.com，鉴权用 X-Api-Key，模型 seed-audio-1.0 只在这里存在。
// 请求是同步的——直接返回合成好的音频与产出时长，没有任务提交与轮询，因此它走
// /v1/audio/generations 而不是方舟的任务接口。
type Adaptor struct {
}

func (a *Adaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode != relayconstant.RelayModeAudioGenerations {
		return "", fmt.Errorf("unsupported relay mode for doubao audio: %d", info.RelayMode)
	}
	if info.ChannelMeta == nil {
		info.InitChannelMeta(nil)
	}
	baseURL := strings.TrimSuffix(info.ChannelMeta.ChannelBaseUrl, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return baseURL + AudioGenerationPath, nil
}

// SetupRequestHeader 使用豆包语音的单头鉴权：X-Api-Key。
//
// 这里刻意不发 Authorization: Bearer——openspeech 不认这个头，用方舟的鉴权方式
// 打过去只会得到鉴权失败。
func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	if info.ChannelMeta == nil {
		info.InitChannelMeta(c)
	}
	req.Set("Content-Type", "application/json")
	req.Set("X-Api-Key", info.ChannelMeta.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return nil, errors.New("doubao audio does not serve chat completions; use /v1/audio/generations")
}

func (a *Adaptor) ConvertClaudeRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertGeminiRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, _ dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertAudioRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("doubao audio does not serve OpenAI TTS; use /v1/audio/generations with text_prompt")
}

func (a *Adaptor) ConvertImageRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

// ConvertAudioGenerationRequest 把下游请求体转成上游格式。
//
// 请求结构本身就是上游结构，这里只做规范化：补上模型的当前值、按上游约束收口
// 参数。用户可控的量在进入上游之前必须收口——sample_rate 与格式不匹配会让上游
// 直接报错，而语速/音调越界会被上游静默钳制（口径不一致会让人误以为生效了）。
func (a *Adaptor) ConvertAudioGenerationRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.AudioGenerationRequest) (any, error) {
	payload := request
	payload.Model = upstreamModelName(info)
	payload.TextPrompt = strings.TrimSpace(payload.TextPrompt)
	if payload.TextPrompt == "" {
		return nil, errors.New("text_prompt is required")
	}
	if len([]rune(payload.TextPrompt)) > dto.MaxAudioGenerationPromptChars {
		return nil, fmt.Errorf("text_prompt must not exceed %d characters", dto.MaxAudioGenerationPromptChars)
	}
	payload.References = normalizeReferences(payload.References)
	payload.AudioConfig = normalizeAudioConfig(payload.AudioConfig)
	return payload, nil
}

// upstreamModelName 取渠道映射后的模型名，回落到原始模型名。
func upstreamModelName(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	if info.ChannelMeta != nil && info.ChannelMeta.UpstreamModelName != "" {
		return info.ChannelMeta.UpstreamModelName
	}
	return info.OriginModelName
}

// normalizeAudioConfig 收口输出音频配置，并保证格式与采样率相容。
//
// ogg_opus 只支持 48000：保留一个该格式不支持的采样率会让整次请求失败，所以这里
// 直接落到该格式的默认值，而不是把矛盾交给上游报错。
func normalizeAudioConfig(config *dto.AudioGenerationConfig) *dto.AudioGenerationConfig {
	if config == nil {
		return nil
	}
	normalized := *config
	normalized.Format = strings.ToLower(strings.TrimSpace(normalized.Format))
	if normalized.Format != "" && !isSupportedFormat(normalized.Format) {
		normalized.Format = DefaultFormat
	}
	if normalized.Format == "" {
		normalized.Format = DefaultFormat
	}
	if normalized.SampleRate != nil {
		rate := *normalized.SampleRate
		if !isSupportedSampleRate(normalized.Format, rate) {
			rate = defaultSampleRate(normalized.Format)
		}
		normalized.SampleRate = &rate
	}
	normalized.SpeechRate = clampRate(normalized.SpeechRate, MinSpeechRate, MaxSpeechRate)
	normalized.LoudnessRate = clampRate(normalized.LoudnessRate, MinLoudnessRate, MaxLoudnessRate)
	normalized.PitchRate = clampRate(normalized.PitchRate, MinPitchRate, MaxPitchRate)
	return &normalized
}

// normalizeReferences 收口参考资源：丢掉空条目，并按上游的互斥规则只保留一种类型。
//
// 上游要求音频与图片参考不能混用，且每条只设置一个字段。把非法组合发上去只会换来
// 400，所以这里主动收敛：图片参考优先（它是更严格的单条模式）。
func normalizeReferences(references []dto.AudioGenerationReference) []dto.AudioGenerationReference {
	if len(references) == 0 {
		return nil
	}
	var audio []dto.AudioGenerationReference
	var image *dto.AudioGenerationReference
	for _, reference := range references {
		audioURL := strings.TrimSpace(reference.AudioURL)
		audioData := strings.TrimSpace(reference.AudioData)
		imageURL := strings.TrimSpace(reference.ImageURL)
		imageData := strings.TrimSpace(reference.ImageData)

		if imageURL != "" || imageData != "" {
			if image == nil {
				entry := dto.AudioGenerationReference{ImageURL: imageURL, ImageData: imageData}
				image = &entry
			}
			continue
		}
		if audioURL == "" && audioData == "" {
			continue
		}
		if len(audio) >= MaxAudioReferences {
			continue
		}
		audio = append(audio, dto.AudioGenerationReference{AudioURL: audioURL, AudioData: audioData})
	}
	if image != nil {
		return []dto.AudioGenerationReference{*image}
	}
	return audio
}

func clampRate(value *int, minValue int, maxValue int) *int {
	if value == nil {
		return nil
	}
	clamped := *value
	if clamped > maxValue {
		clamped = maxValue
	}
	if clamped < minValue {
		clamped = minValue
	}
	return &clamped
}

func isSupportedFormat(format string) bool {
	for _, candidate := range supportedFormats {
		if candidate == format {
			return true
		}
	}
	return false
}

func isSupportedSampleRate(format string, rate int) bool {
	for _, candidate := range sampleRatesByFormat[format] {
		if candidate == rate {
			return true
		}
	}
	return false
}

func defaultSampleRate(format string) int {
	return defaultSampleRates[format]
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

// DoResponse 解析上游的同步响应，并把结果放进上下文交给处理函数计费。
//
// 上游把业务结果放在 body 的 code 里，而 HTTP 状态码**不一定**反映它：内容被拒
// 可能以 200 返回，网关侧的错误也可能以 4xx/5xx 返回。因此这里以 body 为准——
// 先把 body 当业务结果解析，解析不出来时才回落到状态码。反过来做会把"内容被拒"
// 误报成网络错误，或者把一次失败当成成功。
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeAudioGenerations {
		return nil, types.NewError(fmt.Errorf("unsupported relay mode for doubao audio: %d", info.RelayMode), types.ErrorCodeInvalidRequest)
	}
	if resp == nil {
		return nil, types.NewError(errors.New("empty response from doubao audio"), types.ErrorCodeBadResponse)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var parsed dto.AudioGenerationResponse
	if unmarshalErr := common.Unmarshal(body, &parsed); unmarshalErr != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("doubao audio returned %d: %s", resp.StatusCode, common.LocalLogPreview(string(body))),
				types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
		}
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to parse doubao audio response: %s", common.LocalLogPreview(string(body))),
			types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if parsed.Code != successCode {
		message := parsed.Message
		if message == "" {
			message = "unknown error"
		}
		// 状态码不反映业务结果，所以下游统一用 400 表达"上游拒绝了这次请求"。
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("doubao audio error %d: %s", parsed.Code, message),
			types.ErrorCodeBadResponse, http.StatusBadRequest)
	}

	c.Set(ContextKeyAudioResult, parsed)
	return &dto.Usage{
		PromptTokens:     1,
		CompletionTokens: 1,
		TotalTokens:      2,
	}, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
