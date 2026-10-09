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
/**
 * zsy-mode frontend registration (zsy/mode on the Go side).
 *
 * 这一屏（模式管理）与「公共数据」那一组是两件事：
 *
 *	· 「公共数据」里那几个广场是**给别人用的目录**（应用 / 音色 / 形象 / 文风）；
 *	· 这一屏是**运营的动作**：把某一档私有模式**开通给某个账号**。
 *
 * 所以它自己占一个顶层入口，而不是挤进「公共数据」—— 混进去会让运营在
 * "维护目录"与"给某个用户开通"之间找不到东西。与 `zsy-world`（插件管理）
 * 同一条理：那两屏是一对，一个管能力、一个管模式。
 *
 * 与 `zsy-tone` 一样，标题写的是**英文 i18n key**：宿主的侧边栏用 `t()` 渲染
 * 扩展菜单的标题，所以这个模块**绝不能**调用 `useTranslation()`。
 */
import { Layers } from 'lucide-react'

import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { ROLE } from '@/lib/roles'

const MODE_PLAZA_ITEM = {
  title: 'Mode Plaza',
  url: '/mode-plaza',
  icon: Layers,
  /*
   * 一屏只有管理员能看：它列出的是**别人的账号**与他们的授权。
   */
  requiredRole: ROLE.ADMIN,
}

EXT_MENU_GROUPS.push({
  id: 'zsy-mode',
  title: 'Work Modes',
  items: [MODE_PLAZA_ITEM],
})
