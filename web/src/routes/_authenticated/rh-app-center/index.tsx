/*
Copyright (C) 2023-2026 QuantumNous

RunningHub user-side application center route. Available to any authenticated
user (no admin guard); admin entries live under /rh-apps.
*/

import { createFileRoute } from '@tanstack/react-router'

import { RhPortalPage } from '@/extensions/zsy-runninghub/pages/rh-portal'

export const Route = createFileRoute('/_authenticated/rh-app-center/')({
  component: RhPortalPage,
})