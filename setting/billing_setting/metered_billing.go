package billing_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/samber/lo"
)

// BillingModeMetered 标记"按实际产出量计费"的模型。
//
// 与 BillingModeTieredExpr 并列，属于"定价口径"而不是"计费机制"：真正的计费由
// 任务适配器的 channel.TaskMeteredBilling / TaskMeteredUsageReporter 完成（提交
// 时冻结计价基础，任务成功后按上游实际产出量做差额结算）。该标记不改动任何计费
// 金额，只让模型定价页、日志页能把它和"按次固定价"区分开——早期它显示成
// "按次计费"，而实际是按时长计价。
const BillingModeMetered = "metered"

// meteredModels 是按时长计费的模型注册表：模型名 → 计价单位（用于展示，如 "分钟"）。
//
// 单位的含义是"ModelPrice 按什么单位计价"，必须与该模型在 ModelPrice 表里的配置
// 一致：seed-audio-1.0 配的是每分钟价格，定价页就显示"$0.375 / 分钟"。计费时由
// 适配器把上游产出的秒数折算成分钟（见 relay/channel/task/doubao）。
//
// 判定必须覆盖且仅覆盖真正按量计费的模型：它既是定价页的展示依据，也是适配器的
// 分流依据。按前缀猜测（例如所有 seed-audio* 都按量）会让定价页把一个按次固定价
// 的模型标成按量，也让适配器去结算一个从未冻结计价基础的任务，因此这里只做精确
// 匹配（大小写不敏感），新增按量计费模型必须显式登记。
var meteredModels = map[string]string{
	"seed-audio-1.0": MeteredUnitMinute,
}

// 计价单位取值。
const (
	MeteredUnitSecond = "second"
	MeteredUnitMinute = "minute"
)

// AudioMinutesRatioKey 是按生成音频时长计费时使用的计费维度键名。
//
// 定义在 relaykit/types：同步音频生成接口（豆包语音）与音频任务适配器都要用它，
// 而 relaykit 不能反向依赖主模块。这里保留别名，让主模块代码从同一个包读到
// "计量口径"相关的全部定义。
const AudioMinutesRatioKey = types.AudioMinutesRatioKey

// AudioMaxSeconds 是单次音频生成的产出上限（上游 2 分钟）。
const AudioMaxSeconds = types.AudioMaxSeconds

// EstimateAudioSeconds 估算一次音频生成会产生多少秒音频。
//
// 具体启发式定义在 relaykit/types，同步音频生成接口与音频任务适配器共用同一份
// 实现，避免两条音频链路对同一段文本给出不同的预扣量级。
func EstimateAudioSeconds(reqSeconds int, textChars int, speechRate int) float64 {
	return types.EstimateAudioSeconds(reqSeconds, textChars, speechRate)
}

// ClampAudioSeconds 把时长收口到 [1, AudioMaxSeconds]。
func ClampAudioSeconds(seconds float64) float64 {
	return types.ClampAudioSeconds(seconds)
}

// IsMeteredBillingModel 判断模型是否按实际产出量（时长）计费。
func IsMeteredBillingModel(model string) bool {
	return GetMeteredUnit(model) != ""
}

// GetMeteredUnit 返回模型的计价单位，供定价页展示（例如"$0.375 / 分钟"）。
// 未按量计费的模型返回空字符串。
func GetMeteredUnit(model string) string {
	return meteredModels[strings.ToLower(strings.TrimSpace(model))]
}

// GetMeteredModels 返回按量计费模型的副本（模型名 → 计价单位）。
func GetMeteredModels() map[string]string {
	return lo.Assign(meteredModels)
}
