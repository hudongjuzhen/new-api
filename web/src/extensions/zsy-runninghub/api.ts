/*
Copyright (C) 2023-2026 QuantumNous

RunningHub admin API client. All endpoints are under
/dashboard/zsy/rh/* and require admin auth (enforced by backend
requireAdminAuth middleware).
*/

import { api } from '@/lib/api'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface SchemaParam {
  nodeId: string
  fieldName: string
  label: string
  type: string
  required?: boolean
  defaultValue?: string
  options?: string[]
}

export interface AppView {
  id: number
  createdAt: number
  updatedAt: number
  name: string
  slug: string
  kind: string
  upstreamId: string
  description: string
  coverUrl: string
  published: boolean
  adminOnly: boolean
  paramSchema: SchemaParam[]
  perCallBilling: boolean
  fixedQuotaPerCall: number
  modelBaseRateRatio: number
}

export interface AppListResult {
  items: AppView[]
  total: number
  page: number
  pageSize: number
  totalPages: number
  kindCounts: Record<string, number>
}

export interface AppCreateDTO {
  name: string
  slug: string
  kind: string
  upstreamId: string
  description: string
  coverUrl: string
  published: boolean
  adminOnly: boolean
  paramSchema: SchemaParam[]
  perCallBilling: boolean
  fixedQuotaPerCall: number
  modelBaseRateRatio: number
}

export interface InstanceView {
  id: number
  createdAt: number
  updatedAt: number
  name: string
  appId: number
  appName: string
  channelName: string
  channelId: number
  keypoolCount: number
  enabled: boolean
}

export interface InstanceListResult {
  items: InstanceView[]
  total: number
  page: number
  pageSize: number
}

export interface InstanceCreateDTO {
  name: string
  appId: number
  channelId: number
  enabled: boolean
}

// ---------------------------------------------------------------------------
// Apps API
// ---------------------------------------------------------------------------

export async function listApps(params?: {
  keyword?: string
  page?: number
  pageSize?: number
}) {
  const res = await api.get<{ success: boolean; data: AppListResult }>(
    '/dashboard/zsy/rh/apps',
    { params }
  )
  return res.data.data
}

export async function getApp(id: number) {
  const res = await api.get<{ success: boolean; data: AppView }>(
    `/dashboard/zsy/rh/apps/${id}`
  )
  return res.data.data
}

export async function createApp(dto: AppCreateDTO) {
  const res = await api.post<{ success: boolean; data: AppView }>(
    '/dashboard/zsy/rh/apps',
    dto
  )
  return res.data.data
}

export async function updateApp(id: number, dto: AppCreateDTO) {
  const res = await api.put<{ success: boolean; data: AppView }>(
    `/dashboard/zsy/rh/apps/${id}`,
    dto
  )
  return res.data.data
}

export async function deleteApp(id: number) {
  await api.delete(`/dashboard/zsy/rh/apps/${id}`)
}

export async function syncAppsFromChannel(channelId: number) {
  const res = await api.post<{ success: boolean; message?: string }>(
    '/dashboard/zsy/rh/apps/sync-from-channel',
    { channelId }
  )
  return res.data
}

// ---------------------------------------------------------------------------
// Instances API
// ---------------------------------------------------------------------------

export async function listInstances(params?: {
  keyword?: string
  page?: number
  pageSize?: number
}) {
  const res = await api.get<{ success: boolean; data: InstanceListResult }>(
    '/dashboard/zsy/rh/instances',
    { params }
  )
  return res.data.data
}

export async function createInstance(dto: InstanceCreateDTO) {
  const res = await api.post<{ success: boolean; data: InstanceView }>(
    '/dashboard/zsy/rh/instances',
    dto
  )
  return res.data.data
}

export async function updateInstance(id: number, dto: Partial<InstanceCreateDTO>) {
  const res = await api.put<{ success: boolean; data: InstanceView }>(
    `/dashboard/zsy/rh/instances/${id}`,
    dto
  )
  return res.data.data
}

export async function deleteInstance(id: number) {
  await api.delete(`/dashboard/zsy/rh/instances/${id}`)
}

export async function refreshKeypool(id: number) {
  const res = await api.post<{ success: boolean; message?: string }>(
    `/dashboard/zsy/rh/instances/${id}/keypool-refresh`
  )
  return res.data
}
