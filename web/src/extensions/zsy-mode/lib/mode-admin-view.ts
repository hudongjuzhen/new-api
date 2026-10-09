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

import type { ModeEntitlementRow, ModeView } from '../api'

/**
 * 模式管理那一屏的**判断**（"该说什么"），与**画法**分开。
 *
 * ⚠ 这一组与 `zsy-world` 的 `plugin-issue-view.ts` 是**同一类东西**，但那边
 * 服务的是"签发 + 授权"两步，这边只有"授权"一步 —— 所以这里没有
 * `missingCapabilities` / `issueSuccessLine` 那一套，只有"这一档怎么发出去"
 * 与"这一档给了谁"。
 *
 * # 为什么值得单独一个文件
 *
 * 这一屏上最容易写错的不是布局，而是**几句话**：
 *
 * | 说错的话 | 后果 |
 * |---|---|
 * | 把 `private` 说成"公共" | ★ 运营以为自己设对了，而实际正好相反（付费模式人人可开） |
 * | 不显示坏文件的原因 | 运营看到的是"我放的文件不见了"，以为后台坏了 |
 * | 把"取消"说成"删掉他机器上那份" | ★ 运营以为收回了就没了，而用户手上还开着 |
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
 * **写死在代码里**（`'公共 · 谁都能开通'`、`'已开通'`、那句私有模式说明…）——
 * 于是在中文界面上看着正常，而换任何一种语言都还是中文；
 * 同时**页面上那些走 `t()` 的字**（按钮、标题）跟着语言变了。
 * 一块屏幕上两种语言并存，谁看都觉得"国际化没做"。
 *
 * 所以这里一律**只留 key，不留话**：文案在七个语言包里，函数拿 `t` 现翻。
 * 那些"看起来只是常量"的标签（可见性、状态）也**必须**是函数 —— 常量在模块加载
 * 那一刻就定死了，而那时既不知道用户选的是哪种语言、也拿不到 `t`。
 */
export type Translate = TFunction

/** 模式列表分两堆：能用的与不能用的（**都要显示**）。 */
export interface ModeBuckets {
  usable: ModeView[]
  broken: ModeView[]
}

/**
 * ★ 坏模式**排在最前面**，而且带上它自己的那句话 —— 它是运营唯一需要动手去修的东西。
 *
 * 与 `zsy-world` 的 `bucketTemplates` 同一条纪律（也与客户端 `pluginsPage`
 * 那一屏同一条）：坏掉的东西不该藏在末尾。
 */
export function bucketModes(rows: ModeView[]): ModeBuckets {
  const usable: ModeView[] = []
  const broken: ModeView[] = []
  for (const row of rows || []) {
    if (row.problem) broken.push(row)
    else usable.push(row)
  }
  return { usable, broken }
}

/** 模式目录为空时那句能照做的话。 */
export function emptyModesHint(
  t: Translate,
  directory: string,
  knownMediums: string[]
): string {
  const dir = directory || t('The server did not report a directory')
  const mediums = knownMediums?.length
    ? knownMediums.join(' / ')
    : t('The server did not report')
  return t(
    'The mode directory {{dir}} holds no usable mode yet. Put a mode JSON into it (the file name is the mode id, for example mv.json), then click Reload. Its medium may only be one of these: {{mediums}}.',
    { dir, mediums }
  )
}

/**
 * ★★ 这一档**怎么发出去**（`x-visibility`）—— 列表上那一枚标签。
 *
 * # 为什么它必须显示出来（这一格是这一屏最容易出错的一格）
 *
 * 同一份模式，写 `public` 与写 `private` 的后果**完全不同**：
 *
 * | 取值 | 后果 |
 * |---|---|
 * | `public` | ★ 它出现在**所有**用户（含没登录的）的「模式广场」里，点一下就下到本机 |
 * | `private` | ★ 只有后台给它开通过的账号才看得见、才取得到 |
 *
 * 而这两个值在磁盘上只差一个词（`"x-visibility": "public"`），文件本身长得一模一样
 * —— 所以"我到底把它设成公有了没有"必须在这一屏上看得见，不能靠去翻那个文件。
 */
export function modeVisibilityLabel(t: Translate, raw: string | undefined): string {
  const key = String(raw ?? '').trim()
  if (key === 'public') return t('Public mode — anyone can open it')
  return t('Private mode — opened per account')
}

/** 这一档是公共的吗（判据与服务端 `IsPublic()` 同一格，且同样**默认私有**）。 */
export function isPublicMode(row: Pick<ModeView, 'visibility'>): boolean {
  return String(row?.visibility ?? '').trim() === 'public'
}

/**
 * ★★ "这一档给了几个账号"那一句。
 *
 * # 为什么 private + 0 要说得比其他的重
 *
 * 一个 `private` 而**零开通**的模式，多半意味着"运营忘了给谁开" ——
 * 用户那边看到的是"广场上没这一档"，而运营这边看到的是"我明明放进去了"。
 * 两句话都对，而中间缺的就是这一下。所以那个 0 不能只是"0"，
 * 它要说出**下一步**（在上面填账号去开通）。
 *
 * ⚠ 人数读不到（服务端把它压成了负数）时**不许说 0**：那个 0 是一个结论
 * （"没人有权限"），而读不到是"不知道" —— 见服务端 `liveGrantCountOf` 的说明。
 */
export function grantedLabel(t: Translate, row: Pick<ModeView, 'visibility' | 'granted'>): string {
  const raw = Number(row?.granted)
  if (!Number.isFinite(raw) || raw < 0) return t('That count could not be read')
  const count = raw || 0
  if (isPublicMode(row)) {
    return count > 0
      ? t('Public — plus {{count}} account(s) opened individually', { count })
      : t('Public — no opening needed')
  }
  if (count === 0) return t('Private — nobody holds it yet')
  return t('Private — opened for {{count}} account(s)', { count })
}

/**
 * ★★ 一张模式卡片上"这个账号开通了没有"的那一格（用户 2026-… 点名要的）。
 *
 * # 为什么它单独一个判据，而不是在 JSX 里现算
 *
 * 那一格有三个分支，而其中**两个都长得像"未开通"**（读不到 / 真的没有）——
 * 把它们合成一个的代价是运营**对着一个其实已经开通的人再开一次**
 * （服务端会多插一行，界面上看不出错）。所以三支各有各的字，而这里是唯一的判据。
 *
 * | 取值 | 什么时候 | 界面上说 |
 * |---|---|---|
 * | `unknown` | 账号 ID 还没填对、或那一份授权**没读到** | ★ "状态读不到"，按钮禁用 |
 * | `open`    | 此刻真的生效 | "已开通"，按钮 = 取消 |
 * | `closed`  | 读到了，而他没有 | "未开通"，按钮 = 开通 |
 */
export type AccountModeState = 'unknown' | 'open' | 'closed'

export function accountModeState(input: {
  /** 那一份"这个账号手上有什么"读回来了吗（读失败 / 还没回来都是 false）。 */
  readable: boolean
  /** 此刻真的生效的模式 id。 */
  active: readonly string[]
  modeId: string
}): AccountModeState {
  if (!input.readable) return 'unknown'
  return input.active.includes(input.modeId) ? 'open' : 'closed'
}

export function accountModeStateLabel(t: Translate, state: AccountModeState): string {
  if (state === 'unknown') return t('Status unreadable')
  if (state === 'open') return t('Opened')
  return t('Not opened')
}

/**
 * 一条授权行的状态（**现算**，不从"有没有 expiresAt"推）。
 *
 * ★ 与 `zsy-world` 的 `entitlementState` **逐字同形**：取消与过期是两件不同的事，
 * 合成一个"不生效"会让运营看不出他到底是**被取消了**还是**到期了** ——
 * 而这两种的下一步动作完全不同（一个是问为什么，一个是续期）。
 */
export type EntitlementState = 'active' | 'revoked' | 'expired'

export function entitlementState(
  row: Pick<ModeEntitlementRow, 'expiresAt' | 'revokedAt'>,
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
 * ★★ **没开通的私有模式在广场上是"不显示"，不是"显示但点不动"**。
 *
 * 这句话必须写在这一屏上：运营的第一反应是"我在哪儿能预览一下用户看到的样子"，
 * 而答案是"你换个没开通的账号看，那一档**根本不出现**" —— 与
 * `docs/28` §3b 那条"私有不显示"是同一件事。
 *
 * 附带把**取消之后会怎样**说清：本机那份不会被删掉。
 * ⚠ 那是刻意的（服务端 `revokeModeEntitlement` 的注释里写了为什么），
 * 但它与运营的直觉相反 —— 不说的话他会以为"取消 = 他把东西还回来了"。
 */
export function privateModesNote(t: Translate): string {
  return t(
    'Private modes are hidden, not greyed out. An account that was not opened for one does not see it in the plaza at all — it is not shown greyed out either. After you open it, that user has to refresh that screen for it to appear. And after you cancel it, the copy already downloaded to that machine is not deleted: that is deliberate, because that file is the user’s asset (they may have edited it or built a project on it), and cancelling only means “no new copies will be sent”.'
  )
}

/**
 * 建议的账号输入值。
 *
 * ⚠ 只接受正整数：`Number('')` 是 0、`Number('abc')` 是 NaN，而两者都会让
 * "开通"这一下发出一个 userId=0 的请求 —— 服务端会拒（那句提示是"需要 user_id"），
 * 但运营看到的是自己明明填了东西。所以判据放在这里，并给出能照做的话。
 *
 * ⚠ 它与 `zsy-world` 的 `parseUserId` **是同一条判据的第二份**。没有合并的理由：
 * 两个插件的 `lib/` 各自独立（`zsy-*` 之间不许互相 import —— 那会让"卸掉一个插件"
 * 变成"另一个编译不过"）。⚠ 所以**改一处要同时改另一处**，这条注释就是那句提醒。
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
