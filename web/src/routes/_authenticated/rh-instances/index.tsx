/*
Copyright (C) 2023-2026 QuantumNous

RunningHub Instances admin route.
*/

import { createFileRoute, redirect } from '@tanstack/react-router'

import { RhInstancesPage } from '@/extensions/zsy-runninghub/pages/instances-page'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/rh-instances/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: RhInstancesPage,
})
