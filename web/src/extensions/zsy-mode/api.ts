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
import { api } from '@/lib/api'

/**
 * 模式库 admin API client (zsy/mode on the Go side).
 *
 * 这一屏做两件事：看**模式库里有哪几档**（含私有的与坏掉的），以及
 * **按账号授予 / 撤销**某一档私有模式。
 *
 * ★ 它与 `zsy-world` 的客户端是**一对**，但两屏回答的问题不同：
 *
 *	zsy-world   "这个账号能用哪些**能力**"（世界 IP 那两档）
 *	zsy-mode    "这一档**模式**给了哪些账号"（模式是几十个、运营随时加减）
 *
 * ⚠★ 这里**没有**"签发一份文件"那一步：模式的取件是公开面的事
 * （`GET /api/zsy/mode/:id/file`），客户端自己按权限去取 —— 运营不需要
 * 替谁下载一份文件再发给他。所以这一屏只有"授权"那一个写动作。
 */

const ADMIN_BASE = '/dashboard/zsy/mode'

/**
 * ★★ 这一组接口**出错时也返回 HTTP 200**（`common.ApiErrorMsg` 就是这么写的），
 * 所以 axios 的拦截器**不会**让 promise 失败 —— 它只弹一个 toast，然后把那个
 * `success: false` 的信封原样交回来。
 *
 * ⚠★ 那意味着"读不到"会被读成"读回来的是空"：`listEntitlementsByUser` 失败时
 * 界面会画成"这个账号一档都没有"，而运营照着它去**重复开通**（服务端会多插一行，
 * 哪里都不报错）。所以这里把 `success: false` 当场转成一次**失败**，
 * 让上层那几处 `readError` 与服务端那句原话接住它。
 */
function ok<T>(body: { success?: boolean; message?: string; data?: T }): T {
  if (body?.success === false) {
    throw { response: { data: { message: body.message } } }
  }
  return body.data as T
}

/** One mode under the server's mode directory. */
export interface ModeView {
  /** 模式 id（= 文件名去掉 `.json`）。 */
  id: string
  label: string
  /** `video` / `audio` / `text` —— 客户端按它分堆。 */
  medium: string
  /** `single` / `serial`。 */
  workScale: string
  /** `x-summary`：广场卡片上那句说明。 */
  summary: string
  /** 那份模式文件的字节数（广场卡片上显示它）。 */
  bytes: number
  /**
   * ★★ 这一档**怎么发出去**（模式文件里的 `x-visibility`）。
   *
   * | 取值 | 客户端怎么拿到它 |
   * |---|---|
   * | `public`  | 任何账号（**连没登录的也算**）在「模式广场」里看得见，点「开通」就下到本机 |
   * | `private` | ★ 只有**后台给它授权的账号**才看得见、才取得到 |
   *
   * ⚠ 没写这一格的模式按 `private` 算（服务端那条是刻意的：`public` 意味着
   * "任何人点一下就装上了"，而一份忘写这一格的模式悄悄变成人人可用
   * **没有任何地方会报错**）。
   */
  visibility: string
  /**
   * ★ **现在有几个账号持有它**（private 而 0 是运营多半会想知道的事）。
   */
  granted: number
  /**
   * ★ 非空表示这一份模式有问题，发不出去的原因就写在这里。
   *
   * 有问题的模式**照样会出现在列表里**（绝不静默）：一个不出现的文件会让运营
   * 以为"后台坏了"，而真相是那个文件里有个笔误。
   */
  problem: string
  /** 它来自哪个文件 —— 运营要改的就是它。 */
  source: string
}

export interface ModeList {
  items: ModeView[]
  /** 模式目录（运营往这里放新模式）。 */
  directory: string
  /** 写模式时能用的 `medium` 取值（客户端认可的那几个白名单）。 */
  knownMediums: string[]
  /** 服务端给这一屏的那句说明（存量在哪儿、怎么加一档）。 */
  note: string
}

/** One grant row (including expired / revoked ones). */
export interface ModeEntitlementRow {
  id: number
  userId: number
  modeId: string
  source: string
  createdAt: number
  expiresAt: number | null
  revokedAt: number | null
}

/** 按账号问：他手上有什么。 */
export interface ModeEntitlementByUser {
  userId: number
  items: ModeEntitlementRow[]
  /** ★ 此刻**真的生效**的模式 id（每次现算，不是从上面那些行推的）。 */
  active: string[]
}

/** 按模式问：这一档给了谁（只回生效中的）。 */
export interface ModeEntitlementByMode {
  modeId: string
  items: ModeEntitlementRow[]
}

export async function listModes(): Promise<ModeList> {
  const res = await api.get<{ success: boolean; message?: string; data: ModeList }>(
    `${ADMIN_BASE}/list`
  )
  return ok(res.data)
}

/**
 * ★★ **改一档的分发策略**（用户 2026-…："模式管理 应该是可以编辑的，可以设置
 * 权限是公开还是私有"）。
 *
 * # 只改那两格，正文一个字节都不动
 *
 * `x-visibility`（公开 / 私有）与 `x-summary`（广场卡片上那句说明）——
 * 它们是**分发策略**，住在模式文件里（存量是文件，不是数据库）。
 * ⚠ 服务端的实现是"**逐键原样搬运 + 只换那两格**"：改完之后那份文件与改之前的
 * 差别**只有那两行**（那里有一条逐字节守卫钉着）—— 于是"服务端那份 = 客户端那份、
 * 一眼可验"这条立足点保住了。
 *
 * # ⚠★ `summary` 传 `undefined` 与传 `''` 是**两件事**
 *
 * | 传什么 | 服务端怎么做 |
 * |---|---|
 * | `undefined`（这一格压根不给） | **不动**那一格 |
 * | `''`（给一个空串） | ★ **清掉**那一格 |
 *
 * 用一个必填的 `string` 表示不了这个区别，而它会带来一次很难发现的坏法：
 * 只想改可见性的一次保存**顺手把说明清空**（运营会以为是自己删的）。
 * 所以两格都是可选的，而"给不给"照原样传下去。
 */
export async function updateModeMeta(
  modeId: string,
  patch: { visibility?: string; summary?: string }
): Promise<ModeView> {
  const body: Record<string, unknown> = {}
  if (patch.visibility !== undefined) body.visibility = patch.visibility
  if (patch.summary !== undefined) body.summary = patch.summary
  const res = await api.post<{ success: boolean; message?: string; data: ModeView }>(
    `${ADMIN_BASE}/${encodeURIComponent(modeId)}/meta`,
    body
  )
  return ok(res.data)
}

/**
 * 按账号读他的授权（历史上全部行 + 此刻生效的那几档）。
 *
 * ⚠★ 服务端**拒绝全表列举**（`entitlements.go` 那条"授权表会随账号数线性长大"）：
 * `user_id` 与 `mode_id` 至少要给一个。这里给的是账号那一个 —— 这一屏上操作员
 * 的动作是"给某个人开一档"，所以问题总是从账号出发的。
 */
export async function listEntitlementsByUser(userId: number): Promise<ModeEntitlementByUser> {
  const res = await api.get<{
    success: boolean
    message?: string
    data: ModeEntitlementByUser
  }>(`${ADMIN_BASE}/entitlements`, { params: { user_id: userId } })
  return ok(res.data)
}

/** 按模式读"这一档给了谁"（只回生效中的那几行）。 */
export async function listEntitlementsByMode(modeId: string): Promise<ModeEntitlementByMode> {
  const res = await api.get<{
    success: boolean
    message?: string
    data: ModeEntitlementByMode
  }>(`${ADMIN_BASE}/entitlements`, { params: { mode_id: modeId } })
  return ok(res.data)
}

/**
 * 给一个账号开通一档模式。
 *
 * ⚠★ 请求体是**下划线**（`user_id` / `mode_id` / `expires_at`）—— 宿主 dashboard
 * 那一整套的形状（服务端注释里写了为什么请求与响应两半各按各的来）。
 */
export async function grantMode(
  userId: number,
  modeId: string,
  expiresAt = 0
): Promise<void> {
  const res = await api.post<{ success: boolean; message?: string }>(
    `${ADMIN_BASE}/entitlements/grant`,
    {
      user_id: userId,
      mode_id: modeId,
      source: 'admin',
      expires_at: expiresAt,
    }
  )
  ok(res.data)
}

/**
 * 收回一档模式。
 *
 * ⚠★ **收回不会删掉用户机器上那一份**（那是刻意设计的，见服务端
 * `revokeModeEntitlement` 的注释）：本机那份模式文件是他的资产
 * （他可能已经改过、已经用它建了工程），而"收回权限"能表达的是
 * "以后不再发新的"。这一屏上有一句话讲清楚这件事，因为运营一定会问。
 */
export async function revokeMode(userId: number, modeId: string): Promise<void> {
  const res = await api.post<{ success: boolean; message?: string }>(
    `${ADMIN_BASE}/entitlements/revoke`,
    {
      user_id: userId,
      mode_id: modeId,
    }
  )
  ok(res.data)
}
