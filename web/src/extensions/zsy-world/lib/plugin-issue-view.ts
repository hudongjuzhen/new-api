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
import type { TFunction } from 'i18next'

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
 * | 有问题的模板不显示原因 | 运营看到的是"我放进去的文件不见了"，以为后台坏了 |
 * | 不提醒"还差哪个能力" | 只开了一个能力，用户点「解析」被拒，而运营不知道自己漏了一步 |
 *
 * 它们是纯函数，于是可以逐条钉住，不必靠眼睛在界面上找。
 *
 * # ⚠★★ 这里每一句话都**过 `t()`**，而 `t` 是**参数**（用户 2026-… 报的那件事）
 *
 * 用户的原话：
 *
 * > "页面要适配国际化，我看现在很多就是英文，我希望适配上其他语言"
 *
 * ⚠★ 他看到的"很多是英文"里，有一部分其实是**反过来的**：这一组曾经把中文
 * **写死在代码里**（`'公共 · 一键可装'`、`'已开通'`…）—— 于是在中文界面上看着正常，
 * 而换任何一种语言都还是中文；同时页面上那些走 `t()` 的字跟着语言变了。
 * 一块屏幕上两种语言并存，谁看都觉得"国际化没做"。
 *
 * 所以这里一律**只留 key，不留话**：文案在七个语言包里，函数拿 `t` 现翻。
 * 那些"看起来只是常量"的标签（可见性、状态）也**必须**是函数 —— 常量在模块加载
 * 那一刻就定死了，而那时既不知道用户选的是哪种语言、也拿不到 `t`。
 */
export type Translate = TFunction

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
export function emptyTemplatesHint(
  t: Translate,
  directory: string,
  knownSources: string[]
): string {
  const dir = directory || t('The server did not report a directory')
  const sources = knownSources?.length
    ? knownSources.join(' / ')
    : t('The server did not report')
  return t(
    'The template directory {{dir}} holds no usable template yet. Put a plugin JSON into it (the file name is the plugin id, for example world-ip.json), then click Reload. Its source may only be one of these: {{sources}}.',
    { dir, sources }
  )
}

/**
 * ★★ 写好的文件**不等于**他能用 —— 这一段是这一屏最重要的一句话。
 *
 * # 为什么必须说出来
 *
 * 「签发」与「开通」是两件事（服务端那边也是两件事）：
 *
 *   · 签发只产出一个文件；没被开通能力的账号照样签得出来；
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

/**
 * ★★ 一张能力卡上"这个账号开通了没有"的那一格（用户 2026-… 点名要的）。
 *
 * # 为什么它单独一个判据，而不是在 JSX 里现算
 *
 * 那一格有三个分支，而其中**两个都长得像"未开通"**（读不到 / 真的没有）——
 * 把它们合成一个的代价是运营**对着一个其实已经有能力的人再开一次**
 * （服务端会多插一行，界面上看不出错）。所以三支各有各的字，而这里是唯一的判据。
 *
 * | 取值 | 什么时候 | 界面上说 |
 * |---|---|---|
 * | `unknown` | 账号 ID 还没填对、或那一份能力**没读到** | ★ "状态读不到"，按钮禁用 |
 * | `open`    | 此刻真的生效 | "已开通"，按钮 = 取消 |
 * | `closed`  | 读到了，而他没有 | "未开通"，按钮 = 开通 |
 */
export type AccountCapabilityState = 'unknown' | 'open' | 'closed'

export function accountCapabilityState(input: {
  /** 那一份"这个账号手上有什么"读回来了吗（读失败 / 还没回来都是 false）。 */
  readable: boolean
  /** 此刻真的生效的能力。 */
  active: readonly string[]
  capability: string
}): AccountCapabilityState {
  if (!input.readable) return 'unknown'
  return input.active.includes(input.capability) ? 'open' : 'closed'
}

export function accountCapabilityStateLabel(t: Translate, state: AccountCapabilityState): string {
  if (state === 'unknown') return t('Status unreadable')
  if (state === 'open') return t('Opened')
  return t('Not opened')
}

/** 签发成功之后那一整句提示（含"还差一步"那半句，如果有的话）。 */
export function issueSuccessLine(t: Translate, result: IssueResult): string {
  const canUse = result.granted || []
  const missing = capabilitiesNotGranted(result.capabilities, canUse)

  /*
   * ★ 带上**版本**：运营手上会有好几份看着一样的插件文件（上个月发给 A 的、刚发给 A 的），
   * 而"这一份是哪一版"是那一刻唯一能分辨它们的东西 —— 文件名里也有（见服务端的
   * `pluginFileName`），但这句话会被复制粘贴到聊天记录里，文件名不会。
   */
  const version = result.pluginVersion ? ` v${result.pluginVersion}` : ''
  const head = t(
    'Signed “{{id}}”{{version}} for account {{username}} (ID {{userId}}): file {{fileName}}.',
    {
      id: result.pluginId,
      version,
      username: result.username,
      userId: result.userId,
      fileName: result.fileName,
    }
  )

  if (!missing.length) {
    return `${head}${t('This account now holds {{held}} — send the file over for import.', {
      held: canUse.join(t(', ')),
    })}`
  }
  /*
   * ★ 这一句是"签发 ≠ 开通"的落点。它同时给出下一步（去开通）与**为什么**，
   * 因为运营最可能的理解是"我都生成文件了，怎么还不能用"。
   *
   * ⚠ 这一屏是**按能力**开通的（用户 2026-… 那一版把动作搬到了账号那一边），
   * 所以这句话指的方向是"下面那一块"，不是某一个插件。
   */
  return `${head}${t(
    '⚠ But this account only holds {{held}}, and {{missing}} are still missing — the file installs fine, yet opening the world screen is refused by the server. Open those capabilities for it below.',
    {
      held: canUse.length ? canUse.join(t(', ')) : t('(none at all)'),
      missing: missing.join(t(', ')),
    }
  )}`
}

/**
 * ★★ 这一屏要列的那几个能力（用户 2026-…："**一次填 ID，然后把这个人的
 * 所有权限都摆出来**"）。
 *
 * # 为什么名册来自**模板**而不是"那几个字符串常量"
 *
 * 能力是模板声明的（`x-capabilities`），所以"有哪些能力可开通"这件事的**唯一权威**
 * 就是磁盘上那几份模板 —— 把名字在这里再写一遍，就会多出第二份名单，
 * 而它迟早与模板分叉（新加一份插件时没人记得改这里）。
 *
 * ⚠ 排序是**刻意**的：不排的话顺序跟着模板文件名的顺序走，而运营两次刷新
 * 看到的名册顺序可能不同 —— 那会让人以为中间发生了什么。
 */
export function capabilityHints(rows: PluginTemplateView[]): string[] {
  const all = new Set<string>()
  for (const row of rows || []) {
    for (const capability of row.capabilities || []) {
      const name = String(capability ?? '').trim()
      if (name) all.add(name)
    }
  }
  return [...all].sort((a, b) => a.localeCompare(b))
}

/**
 * 每个能力是**哪几份插件**要的 —— 卡片上那一句"Required by …"。
 *
 * ★ 它回答的是运营的下一个问题："开通它之后那个人能打开什么、我该签哪一份给他？"
 * 能力与插件是**两件事**（一份插件要好几个能力，而一个能力也可能被好几份插件要），
 * 而签发是按插件做的 —— 所以这一句必须把两个名字连起来。
 *
 * ⚠ 一份插件都没有时报的是**空串**，由调用方决定说什么（这里不编一句
 * "没有"塞进去：那句话的形状属于界面）。
 */
export function templatesOfCapabilities(
  t: Translate,
  rows: PluginTemplateView[]
): Record<string, string> {
  const names: Record<string, string[]> = {}
  for (const row of rows || []) {
    const label = (row.name || row.id || '').trim()
    if (!label) continue
    for (const capability of row.capabilities || []) {
      const name = String(capability ?? '').trim()
      if (!name) continue
      const list = names[name] || []
      if (!list.includes(label)) list.push(label)
      names[name] = list
    }
  }

  const out: Record<string, string> = {}
  for (const [capability, list] of Object.entries(names)) {
    out[capability] = list.join(t(', '))
  }
  return out
}

/**
 * ★★ 那一句里"一份插件都没要它"时填什么。
 *
 * ⚠★ **它由这里给，不由调用方现编**：`t('Required by {{plugins}}')` 这一句
 * 拼出来的每个字都应当是**这一屏的文案**（`plugin-issue-view.ts` 存在的理由），
 * 而"没模板"那句话要是写在 JSX 里，它就会成为界面上唯一一句没人钉过的字
 * （"哪几份插件要它"这一格在名字打错时**看起来完全正常**，只有这一句会露出来）。
 */
export function capabilityHintFallback(t: Translate): string {
  return t('No plugin asks for it')
}

/**
 * 一条能力行的状态（**现算**，不从"有没有 expiresAt"推）。
 *
 * ★ 与客户端 `pluginSources` 那条"四条判据各自独立"同一个道理：取消与过期是
 * 两件不同的事，合成一个"不生效"会让运营看不出他到底是**被取消了**还是**到期了**
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

export function entitlementStateLabel(t: Translate, state: EntitlementState): string {
  if (state === 'revoked') return t('Already cancelled')
  if (state === 'expired') return t('Expired')
  return t('In effect')
}

/**
 * ★★ 这份插件**怎么发出去**（`x-visibility`）—— 模板卡片上那一个标签。
 *
 * # 为什么它必须显示出来（这一格是这一屏最容易出错的一格）
 *
 * 同一份模板，写 `public` 与写 `private` 的后果**完全不同**：
 *
 * | 取值 | 后果 |
 * |---|---|
 * | `public` | ★ 它出现在**所有**用户的插件页里，任何人点一下就装上了 |
 * | `private` | 只有运营按账号签发的那一份文件能装 |
 *
 * 而这两个值在磁盘上只差一个词（`"x-visibility": "public"`），文件本身长得一模一样
 * —— 所以"我到底把它设成公有了没有"必须在这一屏上看得见，不能靠去翻那个文件。
 */
export function visibilityLabel(t: Translate, raw: string | undefined): string {
  const key = String(raw ?? '').trim()
  if (key === 'public') return t('Public plugin — one-click install')
  return t('Private plugin — signed per account')
}

/**
 * 建议的账号输入值。
 *
 * ⚠ 只接受正整数：`Number('')` 是 0、`Number('abc')` 是 NaN，而两者都会让
 * "签发"这一下发出一个 userId=0 的请求 —— 服务端会拒（那句提示是"需要 user_id"），
 * 但用户看到的是自己明明填了东西。所以判据放在这里，并给出能照做的话。
 *
 * ⚠ 它与 `zsy-mode` 的 `parseUserId` 是同一条判据的第二份（见那边的说明）。
 */
export function parseUserId(t: Translate, raw: string): { value: number; problem: string } {
  const text = String(raw ?? '').trim()
  if (!text) {
    return {
      value: 0,
      problem: t('Fill in an account ID (it is in the users list of the site’s admin panel).'),
    }
  }
  if (!/^\d+$/.test(text)) {
    return {
      value: 0,
      problem: t('The account ID is a run of digits, but this says “{{text}}”.', { text }),
    }
  }
  const value = Number(text)
  if (!Number.isSafeInteger(value) || value <= 0) {
    return {
      value: 0,
      problem: t('The account ID has to be a positive integer, but this works out to {{value}}.', {
        value,
      }),
    }
  }
  return { value, problem: '' }
}
