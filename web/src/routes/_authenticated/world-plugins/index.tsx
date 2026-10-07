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
import { createFileRoute, redirect } from '@tanstack/react-router'

import { WorldPluginsPage } from '@/extensions/zsy-world/pages/world-plugins-page'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

/**
 * 这一屏**只管授权与签发**，所以是管理员专页。
 *
 * ⚠ 与 `zsy-tone` 那条路由同一个形状（连 `beforeLoad` 的写法一起抄）：
 * 前端这道门禁是 UX，不是安全边界 —— 真正说了算的是服务端那一层的
 * `middleware.AdminAuth()`，而这个页面上每一个请求都走它。
 */
export const Route = createFileRoute('/_authenticated/world-plugins/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: WorldPluginsPage,
})
