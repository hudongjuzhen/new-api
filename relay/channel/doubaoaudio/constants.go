package doubaoaudio

// DefaultBaseURL 是豆包语音（音频创作）的默认服务地址。
//
// 必须与方舟（ark.cn-beijing.volces.com）区分开：音频生成模型 seed-audio-1.0
// 只在豆包语音上提供，发到方舟的 contents/generations/tasks 会得到
// InvalidEndpointOrModel.NotFound。
const DefaultBaseURL = "https://openspeech.bytedance.com"

// AudioGenerationPath 是音频生成的接口路径（同步返回音频）。
const AudioGenerationPath = "/api/v3/tts/create"

var ModelList = []string{
	"seed-audio-1.0",
}

var ChannelName = "doubao-audio"

// successCode 是上游表示成功的 code。
const successCode = 0

// 参考资源的上游约束。
const MaxAudioReferences = 3

// 输出格式与其允许的采样率。
//
// 逐格式列出而不是共用一张表：ogg_opus 只支持 48000，mp3 不支持 40000，
// 用一张"通用"表就无法在格式切换时给出合法的默认值。
var supportedFormats = []string{"wav", "mp3", "pcm", "ogg_opus"}

var sampleRatesByFormat = map[string][]int{
	"wav":      {8000, 16000, 24000, 32000, 40000, 44100, 48000},
	"pcm":      {8000, 16000, 24000, 32000, 40000, 44100, 48000},
	"mp3":      {8000, 16000, 24000, 32000, 44100, 48000},
	"ogg_opus": {48000},
}

var defaultSampleRates = map[string]int{
	"wav":      40000,
	"pcm":      40000,
	"mp3":      44100,
	"ogg_opus": 48000,
}

// DefaultFormat 与上游默认输出格式保持一致。
const DefaultFormat = "wav"

// 语速/音量/音调的上游取值范围。
const (
	MinSpeechRate   = -50
	MaxSpeechRate   = 100
	MinLoudnessRate = -50
	MaxLoudnessRate = 100
	MinPitchRate    = -12
	MaxPitchRate    = 12
)
