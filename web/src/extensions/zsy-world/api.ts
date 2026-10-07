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
 * World IP admin API client (zsy/world on the Go side).
 *
 * 这一屏做三件事：看有哪些插件模板、按账号**签发**一份插件文件、以及授予 / 撤销能力。
 *
 * ★ 签发与授权是**两件事**，这个客户端把它们分得清清楚楚：
 *   · `issuePluginFile` 只产出一个文件 —— 它**不改变**任何授权；
 *   · `grantCapability` / `revokeCapability` 才是改授权的。
 * 混在一起（"生成文件就顺手授予"）会让"点了生成"变成一次不可见的授权动作，
 * 而撤销与审计都无从下手。
 */

const ADMIN_BASE = '/dashboard/zsy/world'

/** One template file under the server's plugin-templates directory. */
export interface PluginTemplateView {
  /** 插件的 id（= 模板文件名去掉 .json）。 */
  id: string
  name: string
  /** 这份插件要配哪些能力（模板里的 `x-capabilities`）。 */
  capabilities: string[]
  /** 它带来的每一屏的标题。 */
  screens: string[]
  /**
   * ★ 非空表示这份模板有问题，装不上的原因就写在这里。
   *
   * 有问题的模板**照样会出现在列表里**（绝不静默）：一个不出现的文件会让运营
   * 以为"后台坏了"，而真相是那个文件里有个笔误。
   */
  problem: string
  /** 它来自哪个文件 —— 运营要改的就是它。 */
  source: string
}

export interface PluginTemplateList {
  items: PluginTemplateView[]
  /** 模板目录（运营往这里放新模板）。 */
  directory: string
  /** 写模板时能用的取值（客户端认可的那两个白名单）。 */
  knownKinds: string[]
  knownSources: string[]
}

/** One capability row of an account (including expired / revoked ones). */
export interface EntitlementRow {
  id: number
  userId: number
  capability: string
  source: string
  createdAt: number
  expiresAt: number | null
  revokedAt: number | null
}

export interface EntitlementList {
  userId: number
  items: EntitlementRow[]
  /** ★ 此刻**真的生效**的能力（每次现算，不是从上面那些行推的）。 */
  active: string[]
}

export interface IssueResult {
  /** 建议的下载文件名（带用户名与 id —— 运营的硬盘上会有好几份同名插件）。 */
  fileName: string
  /** 那份文件**本身**（文本）。 */
  file: string
  pluginId: string
  /** ★ 模板里那个版本（可能为空 —— 旧模板没写）。界面用它显示"这一份是哪一版"。 */
  pluginVersion: string
  userId: number
  username: string
  site: string
  check: string
  /** 文件上写着的能力（= 模板的 `x-capabilities`，或签发时显式指定那一份）。 */
  capabilities: string[]
  /** ★ 这个账号**此刻真的持有**的那几个能力（可能比文件上写的少）。 */
  granted: string[]
}

export async function listPluginTemplates(): Promise<PluginTemplateList> {
  const res = await api.get<{ success: boolean; data: PluginTemplateList }>(
    `${ADMIN_BASE}/plugins`
  )
  return res.data.data
}

/**
 * Signs one plugin file for one account.
 *
 * ⚠ 它**不授权**：一个没被授予能力的账号照样签得出来，而那份文件对他没用
 * （op 会回 E_ENTITLEMENT）。返回里的 `granted` 说明现在生效的有哪些，
 * 界面据此提示"还差一步"。
 */
export async function issuePluginFile(
  userId: number,
  pluginId: string,
  capabilities?: string[]
): Promise<IssueResult> {
  const res = await api.post<{ success: boolean; data: IssueResult }>(
    `${ADMIN_BASE}/plugins/issue`,
    {
      user_id: userId,
      plugin_id: pluginId,
      ...(capabilities?.length ? { capabilities } : {}),
    }
  )
  return res.data.data
}

export async function listEntitlements(userId: number): Promise<EntitlementList> {
  const res = await api.get<{ success: boolean; data: EntitlementList }>(
    `${ADMIN_BASE}/entitlements`,
    { params: { user_id: userId } }
  )
  return res.data.data
}

export async function grantCapability(
  userId: number,
  capability: string,
  expiresAt = 0
): Promise<void> {
  await api.post(`${ADMIN_BASE}/entitlements/grant`, {
    user_id: userId,
    capability,
    source: 'admin',
    expires_at: expiresAt,
  })
}

export async function revokeCapability(
  userId: number,
  capability: string
): Promise<void> {
  await api.post(`${ADMIN_BASE}/entitlements/revoke`, {
    user_id: userId,
    capability,
  })
}
