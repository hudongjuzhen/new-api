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
 * zsy-world frontend registration (zsy/world on the Go side).
 *
 * 这一屏（插件管理）与「公共数据」那一组是两件事：
 *
 *	· 「公共数据」里那三个广场是**给别人用的目录**（应用 / 音色 / 形象）；
 *	· 这一屏是**运营的动作**：给某个账号签发一份插件文件、并授予 / 撤销能力。
 *
 * 所以它自己占一个顶层入口，而不是挤进「公共数据」—— 混进去会让运营在
 * "维护目录"与"给某个用户开通"之间找不到东西。
 *
 * 与 `zsy-tone` 一样，标题写的是**英文 i18n key**：宿主的侧边栏用 `t()` 渲染
 * 扩展菜单的标题，所以这个模块**绝不能**调用 `useTranslation()`。
 */
import { Globe2 } from 'lucide-react'

import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { ROLE } from '@/lib/roles'

const WORLD_PLUGINS_ITEM = {
  title: 'World IP · Plugins',
  url: '/world-plugins',
  icon: Globe2,
  /*
   * 一屏只有管理员能看：它列出的是**别人的账号**与他们的能力。
   */
  requiredRole: ROLE.ADMIN,
}

EXT_MENU_GROUPS.push({
  id: 'zsy-world',
  title: 'World IP',
  items: [WORLD_PLUGINS_ITEM],
})
