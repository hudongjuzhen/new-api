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
 * zsy-tone frontend registration (zsy/tone on the Go side).
 *
 * 文风广场 (the tone plaza) is editorial data an operator maintains for other
 * applications — exactly the kind of catalog the "Public Data" group exists for.
 * It therefore joins that group instead of claiming a top-level entry of its own.
 *
 * Translations live in the core locale files (src/i18n/locales/*.json) so the
 * host i18n tooling covers them like any other UI string.
 */

import { Feather } from 'lucide-react'

import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { ROLE } from '@/lib/roles'

// The page manages every row, published or not, so it is admin-only. The title is
// a plain-English i18n key: the host renders extension menu titles through t(),
// so this module must never call useTranslation().
//
// The entry is a direct child of the group — no extra nesting level, because the
// host translates a group's title and each of its items, and a nested level would
// render its titles in their raw English key form.
const TONE_PLAZA_ITEM = {
  title: 'Tone Plaza',
  url: '/tone-plaza',
  icon: Feather,
  requiredRole: ROLE.ADMIN,
}

const PUBLIC_DATA_GROUP_TITLE = 'Public Data'

/*
 * Join the existing "Public Data" group rather than pushing a group of our own.
 *
 * The group is a sidebar section: a second NavGroup with the same title renders as
 * a second「公共数据」block with its own header, so the sidebar would show the
 * catalog split across two identical-looking sections. The voice plugin creates
 * the group and the avatar page already relies on it being shared, so a catalog
 * added later belongs in it too.
 *
 * The fallback matters because this lookup runs at import time and the group is
 * created by another module's side effect: `src/extensions/index.ts` imports
 * './zsy-voice' before './zsy-tone', which is what makes the group exist by the
 * time this file is evaluated. That ordering is a fact about the barrel file
 * rather than something this module controls, so it does not depend on it: if the
 * group is ever absent (a build that tree-shakes the voice plugin away, a future
 * reordering), the plaza still gets a home instead of silently disappearing from
 * the sidebar.
 */
const publicDataGroup = EXT_MENU_GROUPS.find(
  (group) => group.title === PUBLIC_DATA_GROUP_TITLE
)

if (publicDataGroup) {
  publicDataGroup.items.push(TONE_PLAZA_ITEM)
} else {
  EXT_MENU_GROUPS.push({
    id: 'zsy-tone',
    title: PUBLIC_DATA_GROUP_TITLE,
    items: [TONE_PLAZA_ITEM],
  })
}
