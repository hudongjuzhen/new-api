/*
Copyright (C) 2023-2026 QuantumNous

RunningHub plugin frontend registration.

Registers sidebar menu entries into the host's extension anchor
(EXT_MENU_GROUPS). Translations live in the core locale files
(src/i18n/locales/*.json) so the host i18n tooling (i18n:sync,
find-missing-keys) covers them like any other UI string.
*/

import { Boxes } from 'lucide-react'

import { ROLE } from '@/lib/roles'
import { EXT_MENU_GROUPS } from '@/extensions/menus'

// Register admin sidebar menu group
EXT_MENU_GROUPS.push({
  id: 'zsy-runninghub',
  title: 'RunningHub',
  items: [
    {
      title: 'RH Apps',
      url: '/rh-apps',
      icon: Boxes,
      requiredRole: ROLE.ADMIN,
    },
    {
      title: 'RH Instances',
      url: '/rh-instances',
      icon: Boxes,
      requiredRole: ROLE.ADMIN,
    },
  ],
})
