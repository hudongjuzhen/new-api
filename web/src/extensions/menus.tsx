/*
Copyright (C) 2023-2026 QuantumNous

Extension sidebar menu contributions.

Shape matches the core `NavGroup` type exported from components/layout/types.
Plugins push additional entries to `EXT_MENU_GROUPS` from their own `index.ts`.
The core host spreads `...EXT_MENU_GROUPS` into the default sidebar data
(see hooks/use-sidebar-data.ts).
*/

import type { NavGroupType } from '@/components/layout'

export interface ExtMenuGroup extends NavGroupType {}

export const EXT_MENU_GROUPS: ExtMenuGroup[] = []
