package world

import (
	"fmt"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// =========================================================================
// 计费：**一本账** —— 中继扣的那一笔就是这一跑的账（2026-10-08 改；docs/23 §12.13）
//
// # 这条规矩的全部内容
//
// 世界解析的模型调用**打回本机中继**、用的是**发起者自己的令牌**（`gateway.go`
// 从请求的 Host 推导地址，推不出来就拒绝这次 op，绝不悄悄回落到运营的账上）。
// 也就是说中继已经按它自己的价目表逐次调用把钱从那个用户的额度里扣掉了。
//
// **那就是账。本插件不再自己扣第二笔。**
//
// # 为什么把原来那一笔删了（实测，不是口味问题）
//
// 原来这里有一个 `chargeIngest`：跑完按引擎回报的 token、用**本机的 `ModelRatio`
// 表**算一个数，再 `DecreaseUserQuota` 扣一次。它和中继那一笔同时发生，
// 于是同一次解析被收两次钱：
//
//	2026-10-08 那次真跑（41 分钟、39 次模型调用、校验没过所以没有落库）
//	  中继侧（`logs` 表里那 39 行，用户在「积分记录」里看得到的）：111,577 quota = 2.23154 积分
//	  插件侧（本文件算出来的，同一跑）：                             757,166 quota = 15.14 积分
//	  → 合计 17.37 积分，而设计文档 §12.13 写的是"模型费与 quota 合成一笔"
//
// 差额不是舍入误差，是**两张价目表**：本部署的中继走 `usage_billing_path: upstream`
// （按上游真实价：实测约 $0.069/M 输入、$0.384/M 输出），而本机 `ModelRatio` 表把
// 这个型号算成"输入 $1/M"——对"以整本书为上下文"的抽取来说贵约 6.8 倍。
// 两个价目表永远不会自动对上，所以"插件自己算一笔更准的账"这条路从根上不成立。
//
// # 那 `requireQuota` 为什么留着
//
// 它是**准入检查**，不是记账：跑之前确认余额还够一个保守下界，免得跑到一半没钱了
// 才炸在模型调用上。它用一个固定的每章下限、**不查倍率表**——所以就算运营把倍率配错，
// 也不会把一个还有钱的账号挡在门外（这是老设计里唯一值得留的那半条）。
//
// ⚠ 代价说清：模型调用真的发生之后才发现余额不足时，这次已经花掉了——中继那笔是
// 扣定了的。"先查"就是为了让这种情况尽量别发生。
//
// ⚠ `chargeIngest` 曾经在本文件里（跑完再按本机倍率表扣一次）。**不要再把它加回来**：
// 界面上"这次花了多少"的答案来自中继的账，用户在「积分记录」里逐笔看得到。
// =========================================================================

// requireQuota checks that the account can afford one ingest before it runs, and
// is where `E_QUOTA` comes from.
//
// ★ `E_QUOTA` is deliberately NOT `E_ENTITLEMENT` (docs/23 §6.5): the capability
// answers "may this account use world-ip-ai at all", the quota answers "is there
// enough left". A client draws "去购买" for the first and "额度不足（还差 N）" for the
// second.
//
// ★ 金额一律经**宿主自己的格式化器**（`logger.LogQuota`）输出，绝不打印裸 `quota`。
// 裸 quota 是产品里哪一屏都看不到的数：把它写成"积分"会差一个 `QuotaPerUnit`
// （默认 500,000）——2026-10-08 那句"本次已消耗 757166 积分"就是这么来的，
// 而用户当时看到的是 2.23154 积分。一条对不上账的提示比没有提示更糟。
//
// ⚠ 措辞里**不出现"积分"，也不出现"额度"**：单位由 `logger.LogQuota` 自己带
// （`＄1.514332` / `¥…`，随站点的额度显示设置），这里再说一遍只会跟它打架。
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
				"余额不足：本次解析预计至少需要 %s，当前余额 %s，还差 %s。请充值后再试。",
				logger.LogQuota(needed), logger.LogQuota(quota), logger.LogQuota(needed-quota)),
		}
	}
	return nil
}

// usageTotals sums the engine's per-stage usage.
//
// ★ The engine reports usage *per stage* (题材识别 / 类型发现 / 逐章抽取 / …) because
// that is what answers "钱花在哪了" (docs/23 §6.5). The totals are **reported**, not
// charged: the charge itself is the gateway's, and its per-call amount is the thing
// the user can reconcile against 「积分记录」.
func usageTotals(stages []stageUsage) (calls int, promptTokens int, completionTokens int) {
	for _, s := range stages {
		calls += s.Calls
		promptTokens += s.PromptTokens
		completionTokens += s.CompletionTokens
	}
	return calls, promptTokens, completionTokens
}
