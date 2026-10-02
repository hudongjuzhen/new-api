package doubao

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/billing_setting"
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

// audioBillingKey 是"按生成音频时长计费"这一计量维度的键名。
//
// 键名本身定义在 setting/billing_setting：同步音频生成接口（豆包语音）用同一个
// 键把预扣乘数交给 ModelPriceHelper，两处必须是同一个字符串，否则预扣与结算会
// 按不同维度记账。这里保留别名，让本包的算式读起来贴近音频语义。
//
// 值必须与 ModelPrice 的单位一致：配置侧单价的含义是"每个生成分钟的价格"，
// 而乘数就是**预估产出分钟数**（预估秒数 / 60），预扣额度随之自然等于
// 单价 × 分组倍率 × 预估分钟数。
//
// 这个键随任务冻结在 MeteredBasis 中，轮询结算阶段用 min(实际上报分钟数,
// 预估分钟数) × 每分钟额度 重算最终额度。单位一旦写错就会成倍放大预扣费：把
// 秒数直接当作乘数，2 分钟的预估会变成 120 倍单价。
//
// 注意：该键只能是"计费乘数"，任何记账数据（例如原始预估秒数）都不得写进
// OtherRatios——它会被 ApplyOtherRatiosToFloat 当成倍率再乘一遍。
const audioBillingKey = billing_setting.AudioMinutesRatioKey

// secondsPerMinute 把产出秒数与按分钟计价的基础单价对齐。
const secondsPerMinute = 60.0

// maxAudioSeconds 是单次音频生成请求的产出上限（上游 2 分钟）。
const maxAudioSeconds = billing_setting.AudioMaxSeconds

// IsAudioModel 判断模型是否按生成音频时长计费。
//
// 判定来源是 setting/billing_setting 的按量计费注册表：同一份判定既驱动这里的
// 请求/结算分流，也驱动定价页的"按量计费"标注，避免两处各维护一份模型名列表。
//
// 注意：这个判定只回答"怎么计费"，不回答"打哪个接口"。方舟的
// /api/v3/contents/generations/tasks 上没有音频模型——请求会被上游拒绝，所以在
// 这里发起新请求之前必须先拦下音频模型（见 ValidateRequestAndSetAction），
// 让它走豆包语音的同步接口（relay/channel/doubaoaudio）。
//
// 保留该判定是因为历史任务记录里存在音频任务：它们的结算仍要按音频时长处理。
func IsAudioModel(modelName string) bool {
	return billing_setting.IsMeteredBillingModel(modelName)
}

// EstimateAudioSeconds 在提交时估算一次生成会产生多少秒音频。
//
// 具体启发式定义在 setting/billing_setting，与同步音频生成接口共用同一份实现，
// 避免两条音频链路对同一段文本给出不同的预扣量级。
func EstimateAudioSeconds(reqSeconds int, textChars int, speechRate int) float64 {
	return billing_setting.EstimateAudioSeconds(reqSeconds, textChars, speechRate)
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
