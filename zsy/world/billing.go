package world

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// =========================================================================
// 计费：按 token，与生成视频同一套换算（docs/23 §6.5）
//
// # 这条规矩的全部内容
//
// **换算是宿主的，本插件一条比率都不发明。** 用户看到的是 new-api 已有的那本账
// （`quota` / 积分）：同一个 `QuotaPerUnit`、同一张 `ModelRatio` 表、同一个
// `GroupRatio`。世界上多一个"世界积分"就等于多一本账，用户对不上，运营也退不了款。
//
// 换算公式与宿主的中继结算**逐字相同**（`service/text_quota.go` 的同一条）：
//
//	额度 = (prompt_tokens × 1 + completion_tokens × ModelRatio) / 1e6
//	       × GroupRatio × QuotaPerUnit
//
// ⚠ 为什么 prompt 不带倍率而 completion 带：那是宿主对"输入/输出不同价"的建模
// （`ModelRatio` 是输出对输入的倍率）。照抄它，才叫"同一套换算"；自己改成
// "两个都乘"会让这里的数字与用户在别处看到的对不上。
//
// # 为什么不用 relay 的预扣-结算那套（PreConsumedQuota）
//
// 那套是为**流式、可按 token 预估**的中继请求设计的：请求前按 prompt token 预扣，
// 结算时补差。世界的抽取是**一次批处理**——事前唯一能估的是"上界"（章数 × 类型数），
// 而按上界预扣会把用户卡在"钱不够但实际花不了那么多"上。
//
// 故选了"**先查后结**"：跑之前确认余额够这个下界，跑完按**实际 token** 结算。
// ⚠ 代价要说清：引擎真的花了钱之后才发现余额不足时，这次已经发生。
// 所以 `estimateQuotaForIngest` 的下界要保守（宁可高估），而不是精确。
// =========================================================================

// errModelUnpriced marks "the operator never priced this model".
//
// ★ It is a distinct sentinel because the *audience* differs: every other charge
// failure is a server fault the user cannot act on, while this one is fixed in the
// dashboard by whoever runs the deployment. The caller forwards this one's message
// verbatim and summarises the rest (see op_ingest.go).
var errModelUnpriced = errors.New("world: model has no configured ratio")

// tokenQuota is the host's token→quota conversion, in one place.
//
// Value returns a float; callers convert with `common.QuotaFromFloat`, which is the
// convention the sibling plugins use (zsy/runninghub does exactly this for its
// per-call prices). Keeping the conversion here — rather than inline at the call
// site — is what makes "同一套换算" checkable by reading one function.
func tokenQuota(promptTokens, completionTokens int, model string, group string) (float64, error) {
	ratio, known, _ := ratio_setting.GetModelRatio(model)
	if !known {
		// ⚠ GetModelRatio answers a DEFAULT ratio (37.5) for an unknown model, and
		// reports `known=false`. Billing a user at a default ratio for a model the
		// operator never priced is exactly the kind of silent overcharge that is
		// impossible to explain later, so it is refused instead.
		return 0, fmt.Errorf(
			"%w：型号 %q 没有配置倍率，无法计费。请在「运营设置 → 模型倍率」里配置它（或让调用方改用别的型号）",
			errModelUnpriced, model)
	}
	if ratio < 0 {
		ratio = 0
	}
	groupRatio := ratio_setting.GetGroupRatio(group)
	if groupRatio < 0 {
		groupRatio = 0
	}

	const perMillion = 1_000_000.0
	prompt := float64(promptTokens) / perMillion
	completion := float64(completionTokens) * ratio / perMillion
	return (prompt + completion) * groupRatio * common.QuotaPerUnit, nil
}

// usageTotals sums the engine's per-stage usage.
//
// ★ The engine reports usage *per stage* (题材识别 / 类型发现 / 逐章抽取 / …) because
// that is what answers "钱花在哪了" (docs/23 §6.5). For billing the total is what
// matters, but the per-stage breakdown is kept in the response so an operator can
// still explain the bill.
func usageTotals(stages []stageUsage) (calls int, promptTokens int, completionTokens int) {
	for _, s := range stages {
		calls += s.Calls
		promptTokens += s.PromptTokens
		completionTokens += s.CompletionTokens
	}
	return calls, promptTokens, completionTokens
}

// requiresQuota checks that the account can afford one ingest before it runs, and
// is where `E_QUOTA` comes from.
//
// ★ `E_QUOTA` is deliberately NOT `E_ENTITLEMENT` (docs/23 §6.5): the capability
// answers "may this account use world-ip-ai at all", the quota answers "is there
// enough left". A client draws "去购买" for the first and "积分不足（还差 N）" for the
// second.
func requireQuota(userID int, needed int) error {
	if needed <= 0 {
		return nil
	}
	quota, err := model.GetUserQuota(userID, true)
	if err != nil {
		return fmt.Errorf("world: read quota of user %d: %w", userID, err)
	}
	if quota < needed {
		return &opFailure{
			Code: CodeQuota,
			Message: fmt.Sprintf(
				"积分不足：本次解析预计至少需要 %d，当前余额 %d，还差 %d。请充值后再试。",
				needed, quota, needed-quota),
		}
	}
	return nil
}

// chargeIngest debits the account for what the engine actually spent.
//
// It runs **after** the engine call, because the true cost is only known then
// (see the file header). It is called only on the success path — a failed
// extraction whose usage is non-zero still consumed model calls, and that case is
// handled by the caller reporting the usage even when it refuses the result.
func chargeIngest(userID int, modelName string, group string, stages []stageUsage) (int, error) {
	_, promptTokens, completionTokens := usageTotals(stages)
	if promptTokens == 0 && completionTokens == 0 {
		return 0, nil
	}
	value, err := tokenQuota(promptTokens, completionTokens, modelName, group)
	if err != nil {
		return 0, err
	}
	quota := common.QuotaFromFloat(value)
	if quota <= 0 {
		return 0, nil
	}
	if err := model.DecreaseUserQuota(userID, quota, false); err != nil {
		return 0, fmt.Errorf("world: charge user %d %d quota: %w", userID, quota, err)
	}
	model.UpdateUserUsedQuotaAndRequestCount(userID, quota)
	model.RecordLog(userID, model.LogTypeConsume, fmt.Sprintf(
		"世界解析：%s，prompt %d / completion %d tokens，扣 %s",
		modelName, promptTokens, completionTokens, logger.LogQuota(quota)))
	return quota, nil
}
