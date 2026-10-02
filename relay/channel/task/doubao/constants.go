package doubao

import (
	"strconv"
	"strings"
)

var ModelList = []string{
	// 音频生成：/api/v3/contents/generations/tasks（Seed Audio 系列）
	// 计费方式为"按生成音频分钟数"，见 audioBillingKey。
	"seed-audio-1.0",
	// 视频生成：/api/v3/contents/generations/tasks（Seedance 系列）
	"doubao-seedance-2-5-260628",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-mini-260615",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-pro-fast-251015",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
}

var ChannelName = "doubao-video"

// audioModelPrefixes 是按"生成音频时长"计费的模型前缀。方舟把音频生成任务放在
// 与视频生成相同的任务接口上，请求/轮询结构一致，只有请求参数与计费口径不同，
// 因此复用同一个 adaptor，用模型名分流。
var audioModelPrefixes = []string{
	"seed-audio",
	"doubao-seed-audio",
}

// audioBillingKey 是"按生成音频时长计费"这一计量维度的键名。
//
// 同一个键贯穿全链路：提交时它是一个 OtherRatio，值等于预估产出秒数（因此
// 预扣额度 = 模型单价 × 分组倍率 × 预估秒数）；随任务固化后，轮询结算阶段用
// "该维度预扣额度 ÷ 预估秒数"反推出有效单价，再乘以上游实际产出的秒数。
const audioBillingKey = "audio_seconds"

// maxAudioSeconds 是单次音频生成请求的产出上限（上游 2 分钟）。
// 提交时的预扣估值以此为上限：预扣即使用户预估不足也不会低估上游真实产出。
const maxAudioSeconds = 120

// audioCharsPerSecond 是提交时估算音频时长的默认语速（字符/秒），用于在没有任何
// 时长线索时得到一个可预扣的量级。
const audioCharsPerSecond = 4.0

// audioEstimateSafetyFactor 是提交估算的安全系数：预估比理论朗读时长留出余量，
// 覆盖停顿、语气与音效段落，避免真实产出超出预估。
const audioEstimateSafetyFactor = 1.2

// IsAudioModel 判断模型是否按生成音频时长计费。
func IsAudioModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	for _, prefix := range audioModelPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// EstimateAudioSeconds 在提交时估算一次生成会产生多少秒音频。
//
// 这是一个刻意的保守估值：预估时长既是预扣额度，也是结算阶段退款的上限，
// 因此宁可略高也不能低。估算依据按优先级取：
//  1. 请求显式声明的目标时长（顶层 duration / seconds，协议已做上界校验）；
//  2. 文本长度与用户设置的语速；
//  3. 都拿不到时取单次产出上限。
func EstimateAudioSeconds(reqSeconds int, textChars int, speechRate int) float64 {
	if reqSeconds > 0 {
		return clampAudioSeconds(float64(reqSeconds))
	}
	if textChars <= 0 {
		return maxAudioSeconds
	}
	// speech_rate 100 表示 2 倍速，-50 表示 0.5 倍速（上游量纲）；0 表示未设置。
	speed := 1.0
	if speechRate != 0 {
		speed = 1.0 + float64(speechRate)/100.0
		if speed < 0.5 {
			speed = 0.5
		}
	}
	return clampAudioSeconds(float64(textChars) / audioCharsPerSecond / speed * audioEstimateSafetyFactor)
}

// clampAudioSeconds 把估算值收口到 [1, maxAudioSeconds]。
func clampAudioSeconds(seconds float64) float64 {
	if !(seconds > 1) {
		return 1
	}
	if seconds > maxAudioSeconds {
		return maxAudioSeconds
	}
	return seconds
}

// ParseSpeechRate 读取 metadata.speech_rate（可能是数字或字符串）。
// 超出 [-50, 100] 的值按上游行为收口；无法解析时返回 0（按正常语速处理）。
func ParseSpeechRate(value any) int {
	rate, ok := parseFloatAny(value)
	if !ok {
		return 0
	}
	if rate > 100 {
		return 100
	}
	if rate < -50 {
		return -50
	}
	return int(rate)
}

func parseFloatAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// videoPriceKey 价格表的键：输出分辨率档（is1080p/is4k 均为 false 即 480p/720p 基准档）、输入是否含视频。
type videoPriceKey struct {
	is1080p  bool
	is4k     bool
	hasVideo bool
}

// videoPriceTable 各模型在不同 (输出分辨率档, 是否含视频输入) 下的单价（元/百万 token）。
// 其中零值键 {480p/720p, 不含视频} 为基准价，等于管理员应配置的 ModelRatio；
// 计费时取 实际单价/基准价 作为 OtherRatio。
var videoPriceTable = map[string]map[videoPriceKey]float64{
	"doubao-seedance-2-0-260128": {
		{hasVideo: false}:                46.0,
		{hasVideo: true}:                 28.0,
		{is1080p: true, hasVideo: false}: 51.0,
		{is1080p: true, hasVideo: true}:  31.0,
		{is4k: true, hasVideo: false}:    26.0,
		{is4k: true, hasVideo: true}:     16.0,
	},
	"doubao-seedance-2-0-fast-260128": {
		{hasVideo: false}: 37.0,
		{hasVideo: true}:  22.0,
	},
}

// GetVideoInputRatio 返回指定模型在给定输出分辨率/是否含视频输入下，相对基准价的计费倍率。
// 第二个返回值表示该模型是否配置了价格表；倍率为 1.0 时调用方可忽略该 OtherRatio。
func GetVideoInputRatio(modelName, resolution string, hasVideo bool) (float64, bool) {
	prices, ok := videoPriceTable[modelName]
	base := prices[videoPriceKey{}] // 零值键 = {480p/720p, 不含视频} 基准价
	if !ok || base <= 0 {
		return 0, false
	}
	res := strings.ToLower(strings.TrimSpace(resolution))
	price, ok := prices[videoPriceKey{is1080p: res == "1080p", is4k: res == "4k", hasVideo: hasVideo}]
	if !ok {
		// 未配置的组合（如 fast 无 1080p/4k，上游会自行报错）按基准价计费即可。
		return 1.0, true
	}
	return price / base, true
}
