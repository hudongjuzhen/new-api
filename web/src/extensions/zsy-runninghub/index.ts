/*
Copyright (C) 2023-2026 QuantumNous

RunningHub plugin frontend registration.

Registers sidebar menu entries and i18n translations into the host's
extension anchors (EXT_MENU_GROUPS / EXTENSION_LOCALES).
*/

import { Boxes } from 'lucide-react'

import { ROLE } from '@/lib/roles'
import { EXT_MENU_GROUPS } from '@/extensions/menus'
import { registerExtensionLocales } from '@/extensions/locales'
import { locales } from './locales'

// Register i18n strings
registerExtensionLocales(locales)

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
