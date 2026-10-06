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
 * zsy-voice + zsy-avatar frontend registration (zsy/voice and zsy/avatar on the
 * Go side).
 *
 * Contributes one admin-only sidebar group, "Public Data", holding the shared
 * catalog pages: 音色广场 (voices) and 形象广场 (personas). Both are editorial data
 * an operator maintains for other applications, which is why they live together
 * under one group instead of each claiming a top-level entry.
 *
 * Translations live in the core locale files (src/i18n/locales/*.json) so the
 * host i18n tooling covers them like any other UI string.
 */

import { AudioLines, UserRound } from 'lucide-react'

import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { ROLE } from '@/lib/roles'

// The pages manage every row, published or not, so they are admin-only. The
// titles are plain-English i18n keys: the host renders extension menu titles
// through t(), so this module must never call useTranslation().
//
// Both entries are direct children of the group. The host translates a group's
// title and each of its items, so keeping the catalogs flat is what makes the
// sidebar fully localized; an extra nesting level would render its titles in
// their raw English key form.
EXT_MENU_GROUPS.push({
  id: 'zsy-voice',
  title: 'Public Data',
  items: [
    {
      title: 'Voice Plaza',
      url: '/voice-plaza',
      icon: AudioLines,
      requiredRole: ROLE.ADMIN,
    },
    {
      title: 'Avatar Plaza',
      url: '/avatar-plaza',
      icon: UserRound,
      requiredRole: ROLE.ADMIN,
    },
  ],
})
