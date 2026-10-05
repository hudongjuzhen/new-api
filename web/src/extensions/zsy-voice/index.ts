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
 * Voice Plaza frontend registration (zsy/voice on the Go side).
 *
 * Contributes one admin-only sidebar entry into the host's extension anchor
 * (EXT_MENU_GROUPS). Translations live in the core locale files
 * (src/i18n/locales/*.json) so the host i18n tooling covers them like any other
 * UI string.
 */

import { AudioLines } from 'lucide-react'

import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { ROLE } from '@/lib/roles'

// The page manages every voice, published or not, so it is admin-only. The
// title is a plain-English i18n key: the host renders extension menu titles
// through t(), so this module must never call useTranslation().
EXT_MENU_GROUPS.push({
  id: 'zsy-voice',
  title: 'Voice Plaza',
  items: [
    {
      title: 'Voice Plaza',
      url: '/voice-plaza',
      icon: AudioLines,
      requiredRole: ROLE.ADMIN,
    },
  ],
})
