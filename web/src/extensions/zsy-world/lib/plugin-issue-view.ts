/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { EntitlementRow, IssueResult, PluginTemplateView } from '../api'

/**
 * 这一屏的**判断**（"该说什么"），与**画法**分开。
 *
 * # 为什么值得单独一个文件
 *
 * 这一屏上最容易写错的不是布局，而是**几句话**：
 *
 * | 说错的话 | 后果 |
 * |---|---|
 * | 把"签发好了"说成"开通了" | ★ 运营以为点一下就给了，用户导入之后什么都用不了 |
 * | 有问题的模板不显示原因 | 运营看到的是"我放的文件不见了"，以为后台坏了 |
 * | 不提醒"还差哪个能力" | 只授了一个能力，用户点「解析」被拒，而运营不知道自己漏了一步 |
 *
 * 它们是纯函数，于是可以逐条钉住，不必靠眼睛在界面上找。
 */

/** 模板列表分两堆：能用的与不能用的（**都要显示**）。 */
export interface TemplateBuckets {
  usable: PluginTemplateView[]
  broken: PluginTemplateView[]
}

/**
 * ★ 坏模板**排在最前面**，而且带上它自己的那句话。
 *
 * 与 `pluginsPage`（客户端那一屏）同一条纪律：坏掉的东西不该藏在末尾 ——
 * 它是运营唯一需要动手去修的东西。
 */
export function bucketTemplates(rows: PluginTemplateView[]): TemplateBuckets {
  const usable: PluginTemplateView[] = []
  const broken: PluginTemplateView[] = []
  for (const row of rows || []) {
    if (row.problem) broken.push(row)
    else usable.push(row)
  }
  return { usable, broken }
}

/** 模板目录为空时那句能照做的话。 */
export function emptyTemplatesHint(directory: string, knownSources: string[]): string {
  const dir = directory || '（服务端没报目录）'
  const sources = knownSources?.length ? knownSources.join(' / ') : '（服务端没报）'
  return (
    `模板目录 ${dir} 里还没有一份能用的模板。` +
    `把一份插件 JSON 放进去（文件名就是插件 id，例如 world-ip.json），` +
    `再点「重新读取」。它要写的 source 只能是这几个人：${sources}。`
  )
}

/**
 * ★★ 写好的文件**不等于**他能用 —— 这一段是这一屏最重要的一句话。
 *
 * # 为什么必须说出来
 *
 * 「签发」与「授予」是两件事（服务端那边也是两件事）：
 *
 *   · 签发只产出一个文件；没被授予能力的账号照样签得出来；
 *   · 而那份文件对他**一点用都没有** —— op 会回 `E_ENTITLEMENT`。
 *
 * 所以当文件上写着的能力多于他**此刻真的持有**的，就要明确说"还差这一步"，
 * 否则运营会把文件发出去、然后收到"你给我的东西用不了"。
 *
 * @returns 还没生效的能力名（空数组 = 全都生效了）
 */
export function missingCapabilities(
  result: Pick<IssueResult, 'capabilities' | 'granted'>
): string[] {
  return capabilitiesNotGranted(result.capabilities, result.granted)
}

/**
 * 纯判据：`want` 里哪些不在 `have` 里。
 *
 * ⚠ 参数顺序刻意是 `(want, have)`：写成 `(have, want)` 时两边都是 `string[]`，
 * 传反了**不会报错**，只会把"还差什么"说成"已经有什么" —— 一句听着完全合理的错话。
 */
export function capabilitiesNotGranted(
  want: string[] | undefined,
  have: string[] | undefined
): string[] {
  const held = new Set(have || [])
  return (want || []).filter((c) => !held.has(c))
}

/** 签发成功之后那一整句提示（含"还差一步"那半句，如果有的话）。 */
export function issueSuccessLine(
  result: IssueResult,
  template: Pick<PluginTemplateView, 'capabilities'> | undefined
): string {
  const canUse = result.granted || []
  const want = template?.capabilities || []
  const missing = capabilitiesNotGranted(want, canUse)

  /*
   * ★ 带上**版本**：运营手上会有好几份看着一样的插件文件（上个月发给 A 的、刚发给 A 的），
   * 而"这一份是哪一版"是那一刻唯一能分辨它们的东西 —— 文件名里也有（见服务端的
   * `pluginFileName`），但这句话会被复制粘贴到聊天记录里，文件名不会。
   */
  const version = result.pluginVersion ? ` v${result.pluginVersion}` : ''
  const head =
    `已经签好「${result.pluginId}」${version} 给账号 ${result.username}（ID ${result.userId}）：` +
    `文件名 ${result.fileName}。`

  if (!missing.length) {
    return `${head}这个账号现在持有 ${canUse.join('、')}，把文件发给他导入即可。`
  }
  /*
   * ★ 这一句是"签发 ≠ 授权"的落点。它同时给出下一步（去授予）与**为什么**，
   * 因为运营最可能的理解是"我都生成文件了，怎么还不能用"。
   */
  return (
    `${head}⚠ 但这个账号现在只有 ${canUse.length ? canUse.join('、') : '（一个都没有）'}，` +
    `还差 ${missing.join('、')} —— **文件装得上，可是打开世界那一屏会被服务端拒绝**。` +
    `请在下面把缺的能力授予他。`
  )
}

/**
 * 一条能力行的状态（**现算**，不从"有没有 expiresAt"推）。
 *
 * ★ 与客户端 `pluginSources` 那条"四条判据各自独立"同一个道理：撤销与过期是
 * 两件不同的事，合成一个"不生效"会让运营看不出他到底是**被撤销了**还是**到期了**
 * —— 而这两种的下一步动作完全不同（一个是问为什么，一个是续期）。
 */
export type EntitlementState = 'active' | 'revoked' | 'expired'

export function entitlementState(
  row: Pick<EntitlementRow, 'expiresAt' | 'revokedAt'>,
  nowSeconds: number
): EntitlementState {
  if (row.revokedAt != null) return 'revoked'
  if (row.expiresAt != null && row.expiresAt <= nowSeconds) return 'expired'
  return 'active'
}

export const ENTITLEMENT_STATE_LABEL: Record<EntitlementState, string> = {
  active: '生效中',
  revoked: '已撤销',
  expired: '已过期',
}

/**
 * 建议的账号输入值。
 *
 * ⚠ 只接受正整数：`Number('')` 是 0、`Number('abc')` 是 NaN，而两者都会让
 * "签发"这一下发出一个 userId=0 的请求 —— 服务端会拒（那句提示是"需要 user_id"），
 * 但用户看到的是自己明明填了东西。所以判据放在这里，并给出能照做的话。
 */
export function parseUserId(raw: string): { value: number; problem: string } {
  const text = String(raw ?? '').trim()
  if (!text) return { value: 0, problem: '请填账号 ID（站点后台的「用户」列表里有）。' }
  if (!/^\d+$/.test(text)) {
    return { value: 0, problem: `账号 ID 是一串数字，而这里写的是「${text}」。` }
  }
  const value = Number(text)
  if (!Number.isSafeInteger(value) || value <= 0) {
    return { value: 0, problem: `账号 ID 要是正整数，而这里算出来是 ${value}。` }
  }
  return { value, problem: '' }
}
