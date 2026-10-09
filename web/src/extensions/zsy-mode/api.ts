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
 * ★★ **新增一档模式**（用户 2026-…："右上角增加一个添加模式的功能"）。
 *
 * 它把新那份模式写进**服务端的模式目录**（`<模式目录>/<id>.json`）——
 * 服务端那份 = 客户端将来会拿到的那一份，所以这个动作就是"把这个模式做出来"。
 *
 * ⚠ 请求体是**下划线**（宿主 dashboard 那一套）：`ai` 那一格是可选的 ——
 * 不带它时正文由调用方给（`body`），带了它就由服务端按那几个输入去生成。
 */
export interface ModeCreateInput {
  /** 模式名称（界面上的 `label`）。 */
  label: string
  /** `video` / `audio` / `text` —— 客户端按它分类。 */
  medium: string
  /** 一句话说明（写进 `x-summary`，广场卡片上那一句）。 */
  summary: string
  /** 模式 id（文件名）。留空则由后端按 label 推一个。 */
  id?: string
  /** 公开 / 私有。 */
  visibility?: string
  /** ★ AI 生成那一路：给模型的需求（用户写的那段话）。 */
  request?: string
  /** ★ AI 生成用哪个模型（默认由服务端配置决定）。 */
  model?: string
  /** ★ 生成用的密钥（用户在弹窗里选的那一把）。 */
  api_key?: string
  /** 不走 AI 时的正文（一整份模式文件）。 */
  body?: Record<string, unknown>
}

export async function createMode(input: ModeCreateInput): Promise<ModeView> {
  const res = await api.post<{ success: boolean; message?: string; data: ModeView }>(
    `${ADMIN_BASE}/create`,
    input
  )
  return ok(res.data)
}

/** 读一档模式的**完整正文**（编辑弹窗里那份表单与原始 JSON 都要它）。 */
export async function getModeContent(modeId: string): Promise<Record<string, unknown>> {
  const res = await api.get<{
    success: boolean
    message?: string
    data: { modeId: string; content: Record<string, unknown> }
  }>(`${ADMIN_BASE}/${encodeURIComponent(modeId)}/content`)
  return ok(res.data).content
}

/**
 * ★★ **写回一档模式的完整正文**（编辑弹窗里的表单 / 原始 JSON 都走它）。
 *
 * ⚠★ 它与 `updateModeMeta`（只改 `x-visibility` / `x-summary`）**是两件事**：
 * 那一条改的是**分发策略**，这一条改的是**模式本身**。
 * 服务端保存前会跑一遍与读盘同源的校验（`format` / `id` / `label` / `medium`…），
 * 校验没过时磁盘**一个字节都不动**。
 */
export async function saveModeContent(
  modeId: string,
  content: Record<string, unknown>
): Promise<ModeView> {
  const res = await api.post<{ success: boolean; message?: string; data: ModeView }>(
    `${ADMIN_BASE}/${encodeURIComponent(modeId)}/content`,
    { content }
  )
  return ok(res.data)
}

/** 生成模式时能选的那把密钥（服务端只回前缀，绝不回明文）。 */
export interface ModeAiKey {
  id: number
  name: string
  /** ★ 只用于显示 —— 真正的密钥在这一条**不在**响应里。 */
  keyPrefix: string
  status: number
  /** 这个密钥是否被允许调用生成用的那个模型（服务端判的）。 */
  usable: boolean
  /** 不可用的原因（能照做的一句话）。 */
  problem: string
}

/**
 * 生成模式时那两格：能选的密钥 + 服务端这一趟会用的模型。
 *
 * ⚠★ 模型名**不是给运营改的**（它由服务端的 `ZSY_MODE_AI_MODEL` 决定，
 * 默认 `glm-5.3-flash`）：界面上再放一格只会多一个与渠道配置分叉的地方。
 * 回它是为了让界面**念出来**——运营一定会问"它到底调了哪个模型"。
 */
export interface ModeAiSetup {
  items: ModeAiKey[]
  model: string
}

export async function listModeAiKeys(): Promise<ModeAiSetup> {
  const res = await api.get<{ success: boolean; message?: string; data: ModeAiSetup }>(
    `${ADMIN_BASE}/ai/keys`
  )
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
