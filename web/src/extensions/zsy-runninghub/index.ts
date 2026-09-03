/*
Copyright (C) 2023-2026 QuantumNous

RunningHub plugin frontend registration.

Registers sidebar menu entries into the host's extension anchor
(EXT_MENU_GROUPS). Translations live in the core locale files
(src/i18n/locales/*.json) so the host i18n tooling (i18n:sync,
find-missing-keys) covers them like any other UI string.
*/

import { Boxes, Sparkles } from 'lucide-react'

import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { ROLE } from '@/lib/roles'

// Register sidebar menu group. "RH App Center" is available to every
// authenticated user; "RH Apps" (management) is admin-only.
EXT_MENU_GROUPS.push({
  id: 'zsy-runninghub',
  title: 'RunningHub',
  items: [
    {
      title: 'RH App Center',
      url: '/rh-app-center',
      icon: Sparkles,
    },
    {
      title: 'RH Apps',
      url: '/rh-apps',
      icon: Boxes,
      requiredRole: ROLE.ADMIN,
    },
  ],
})
