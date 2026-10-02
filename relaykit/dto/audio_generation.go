package dto

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// MaxAudioGenerationPromptChars 是 text_prompt 的长度上限（上游文档约束）。
// 校验与适配器共用同一个常量，避免两处各写一个数字。
const MaxAudioGenerationPromptChars = 3000

// AudioGenerationRequest 是音频创作接口（豆包语音 Seed Audio）的请求体。
//
// 字段名沿用上游文档：文本放在 text_prompt，音频参数放在 audio_config，
// 参考资源放在 references。之所以不套用 OpenAI 的 TTS 请求（model/input/voice），
// 是因为两者语义不同——TTS 是"把这段文字读出来"，音频创作是"按这段描述生成一段
// 音频作品"，还支持参考音频/图片与时间轴控制。
type AudioGenerationRequest struct {
	Model string `json:"model"`
	// TextPrompt 是待合成的文本或描述性提示词。
	TextPrompt string `json:"text_prompt"`
	// References 是参考资源列表，元素为 audio_url / image_url 等（音频与图片互斥）。
	References []AudioGenerationReference `json:"references,omitempty"`
	// AudioConfig 是输出音频配置，省略时由上游用默认值。
	AudioConfig *AudioGenerationConfig `json:"audio_config,omitempty"`
	// Watermark 是水印配置。
	Watermark *AudioGenerationWatermark `json:"watermark,omitempty"`
}

// AudioGenerationReference 是一条参考资源。上游要求每个元素只设置其中一个字段，
// 且音频参考（audio_url / audio_data）与图片参考（image_url / image_data）不能混用。
type AudioGenerationReference struct {
	AudioURL  string `json:"audio_url,omitempty"`
	AudioData string `json:"audio_data,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
	ImageData string `json:"image_data,omitempty"`
}

// AudioGenerationConfig 对应上游的 audio_config。
type AudioGenerationConfig struct {
	Format       string `json:"format,omitempty"`
	SampleRate   *int   `json:"sample_rate,omitempty"`
	SpeechRate   *int   `json:"speech_rate,omitempty"`
	LoudnessRate *int   `json:"loudness_rate,omitempty"`
	PitchRate    *int   `json:"pitch_rate,omitempty"`
}

// AudioGenerationWatermark 对应上游的 watermark。
type AudioGenerationWatermark struct {
	AIGCWatermark *bool `json:"aigc_watermark,omitempty"`
}

// AudioGenerationResponse 是上游同步返回的结果。
//
// 上游把音频以 base64 放在 audio，同时给一个 2 小时有效的 url；original_duration
// 是模型产出的原始时长，也是计费依据（上限 120 秒）。
type AudioGenerationResponse struct {
	Code             int     `json:"code"`
	Message          string  `json:"message"`
	Audio            string  `json:"audio"`
	Duration         float64 `json:"duration"`
	OriginalDuration float64 `json:"original_duration"`
	URL              string  `json:"url"`
}

// GetTokenCountMeta 提供预扣所需的提示文本与**预估产出分钟数**。
//
// 这里返回的 BillingRatios 是 ModelPriceHelper 在按量计费模型上的预扣乘数：
// 配置侧单价是"每分钟价格"，所以乘数必须是分钟数，预扣额度随之等于
// 单价 × 分组倍率 × 预估分钟数。真实时长只有响应里才有，结算阶段按
// original_duration 重算（见 relay/audio_generation_handler.go）。
//
// 一旦把秒数当乘数写进来，2 分钟的预扣会变成 120 倍单价。
func (r *AudioGenerationRequest) GetTokenCountMeta() *types.TokenCountMeta {
	textChars := len([]rune(strings.TrimSpace(r.TextPrompt)))
	speechRate := 0
	if r.AudioConfig != nil && r.AudioConfig.SpeechRate != nil {
		speechRate = *r.AudioConfig.SpeechRate
	}
	minutes := types.EstimateAudioSeconds(0, textChars, speechRate) / 60.0

	return &types.TokenCountMeta{
		CombineText:   r.TextPrompt,
		TokenType:     types.TokenTypeTextNumber,
		BillingRatios: map[string]float64{types.AudioMinutesRatioKey: minutes},
	}
}

func (r *AudioGenerationRequest) IsStream(_ *http.Request) bool {
	return false
}

func (r *AudioGenerationRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}
