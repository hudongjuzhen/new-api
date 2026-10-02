package types

type FileType string

const (
	FileTypeImage FileType = "image" // Image file type
	FileTypeAudio FileType = "audio" // Audio file type
	FileTypeVideo FileType = "video" // Video file type
	FileTypeFile  FileType = "file"  // Generic file type
)

// AudioMinutesRatioKey 是按生成音频时长计费时使用的计费维度键名。
//
// 模型（如 seed-audio-1.0）在价格表里配的是**每分钟**单价，而这个键的值是对
// ModelPrice 的乘数，所以它必须是**分钟数**：预扣额度随之等于
// 单价 × 分组倍率 × 预估分钟数。把秒数当乘数会让 2 分钟的预估变成 120 倍单价。
//
// 放在 relaykit 是因为两条音频链路都要用它——同步音频生成（请求的
// GetTokenCountMeta 把它放进 BillingRatios）与音频任务（adaptor 把它放进
// OtherRatios）——而 relaykit 不能反向依赖主模块。
const AudioMinutesRatioKey = "audio_minutes"

// 估算音频产出时长时使用的常量。
//
// 时长既是预扣额度也是结算基准，因此宁可略高也不能低：低估会让真实产出超出预扣，
// 高估只是把额度多冻结一会儿。
const (
	// AudioCharsPerSecond 是没有语速线索时使用的默认语速（字符/秒）。
	AudioCharsPerSecond = 4.0
	// AudioEstimateSafetyFactor 是安全系数，覆盖停顿、语气与音效段落。
	AudioEstimateSafetyFactor = 1.2
	// AudioMaxSeconds 是单次音频生成的产出上限（上游 2 分钟）。
	AudioMaxSeconds = 120
)

// EstimateAudioSeconds 估算一次音频生成会产生多少秒音频。
//
// 依据按优先级取：请求显式声明的目标时长、文本长度与语速、单次产出上限。
// 结果收口在 [1, AudioMaxSeconds]：0 会让预扣变成 0（结算随之失真），超过上限
// 则会多冻结用户额度。speech_rate 采用上游量纲：100 为 2 倍速，-50 为 0.5 倍速，
// 0 表示未设置。
func EstimateAudioSeconds(reqSeconds int, textChars int, speechRate int) float64 {
	if reqSeconds > 0 {
		return ClampAudioSeconds(float64(reqSeconds))
	}
	if textChars <= 0 {
		return AudioMaxSeconds
	}
	speed := 1.0
	if speechRate != 0 {
		speed = 1.0 + float64(speechRate)/100.0
		if speed < 0.5 {
			speed = 0.5
		}
	}
	return ClampAudioSeconds(float64(textChars) / AudioCharsPerSecond / speed * AudioEstimateSafetyFactor)
}

// ClampAudioSeconds 把时长收口到 [1, AudioMaxSeconds]。
func ClampAudioSeconds(seconds float64) float64 {
	if !(seconds > 1) {
		return 1
	}
	if seconds > AudioMaxSeconds {
		return AudioMaxSeconds
	}
	return seconds
}

type TokenType string

const (
	TokenTypeTextNumber TokenType = "text_number" // Text or number tokens
	TokenTypeTokenizer  TokenType = "tokenizer"   // Tokenizer tokens
	TokenTypeImage      TokenType = "image"       // Image tokens
)

type TokenCountMeta struct {
	TokenType     TokenType   `json:"token_type,omitempty"`     // Type of tokens used in the request
	CombineText   string      `json:"combine_text,omitempty"`   // Combined text from all messages
	ToolsCount    int         `json:"tools_count,omitempty"`    // Number of tools used
	NameCount     int         `json:"name_count,omitempty"`     // Number of names in the request
	MessagesCount int         `json:"messages_count,omitempty"` // Number of messages in the request
	Files         []*FileMeta `json:"files,omitempty"`          // List of files, each with type and content
	MaxTokens     int         `json:"max_tokens,omitempty"`     // Maximum tokens allowed in the request

	ImagePriceRatio float64            `json:"image_ratio,omitempty"`    // Ratio for image size, if applicable
	BillingRatios   map[string]float64 `json:"billing_ratios,omitempty"` // Validated request multipliers used by pre-consume billing
	//IsStreaming   bool        `json:"is_streaming,omitempty"`   // Indicates if the request is streaming
}

type FileMeta struct {
	FileType
	Source FileSource // 统一的文件来源（URL 或 base64）
	Detail string     // 图片细节级别（low/high/auto）
}

// NewFileMeta 创建新的 FileMeta
func NewFileMeta(fileType FileType, source FileSource) *FileMeta {
	return &FileMeta{
		FileType: fileType,
		Source:   source,
	}
}

// NewImageFileMeta 创建图片类型的 FileMeta
func NewImageFileMeta(source FileSource, detail string) *FileMeta {
	return &FileMeta{
		FileType: FileTypeImage,
		Source:   source,
		Detail:   detail,
	}
}

// GetIdentifier 获取文件标识符（用于日志）
func (f *FileMeta) GetIdentifier() string {
	if f.Source != nil {
		return f.Source.GetIdentifier()
	}
	return "unknown"
}

// IsURL 判断是否是 URL 来源
func (f *FileMeta) IsURL() bool {
	return f.Source != nil && f.Source.IsURL()
}

// GetRawData 获取原始数据（兼容旧代码）
// Deprecated: 请使用 Source.GetRawData()
func (f *FileMeta) GetRawData() string {
	if f.Source != nil {
		return f.Source.GetRawData()
	}
	return ""
}

type RequestMeta struct {
	OriginalModelName string `json:"original_model_name"`
	UserUsingGroup    string `json:"user_using_group"`
	PromptTokens      int    `json:"prompt_tokens"`
	PreConsumedQuota  int    `json:"pre_consumed_quota"`
}
