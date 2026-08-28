/*
Copyright (C) 2023-2026 QuantumNous

RunningHub Apps admin route.
*/

import { createFileRoute, redirect } from '@tanstack/react-router'

import { RhAppsPage } from '@/extensions/zsy-runninghub/pages/apps-page'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/rh-apps/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: RhAppsPage,
})
