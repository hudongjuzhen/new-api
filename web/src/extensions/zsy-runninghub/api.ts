/*
Copyright (C) 2023-2026 QuantumNous

RunningHub admin API client. All endpoints are under
/dashboard/zsy/rh/* and require admin auth (enforced by backend
requireAdminAuth middleware). Channel list reuses the host /api/channel
endpoint filtered by the RunningHub channel type.
*/

import { api } from '@/lib/api'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface SchemaOption {
  label: string
  value: string
}

export interface SchemaParam {
  nodeId: string
  fieldName: string
  label: string
  type: string
  required?: boolean
  defaultValue?: string
  placeholder?: string
  min?: number
  max?: number
  options?: SchemaOption[]
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
  channelId: number
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
  channelId: number
}

export interface RhChannel {
  id: number
  name: string
}

export interface FetchTemplateResult {
  appName: string
  upstreamId: string
  schema: SchemaParam[]
  schemaErrors?: { field: string; message: string }[]
}

export interface KeyPoolEntry {
  id: number
  key: string
  enabled: boolean
  remark: string
  occupancy: number
}

export interface AppKeypoolResult {
  appId: number
  channelId: number
  channelName: string
  total: number
  enabled: number
  keys: KeyPoolEntry[]
}

export interface KeypoolRefreshResult {
  appId: number
  keysAdded: number
  keysDisabled: number
  keysRestored: number
  pendingReleased: number
  keys: { poolId: number; key: string; enabled: boolean; occupancy: number }[]
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

/**
 * One-click fetch of a RunningHub app's parameter template. Uses the selected
 * channel's RH key to call the upstream apiCallDemo and infers the schema.
 */
export async function fetchAppTemplate(channelId: number, upstreamId: string) {
  const res = await api.post<{
    success: boolean
    data?: FetchTemplateResult
    message?: string
  }>('/dashboard/zsy/rh/apps/fetch-template', { channelId, upstreamId })
  return res.data.data
}

// ---------------------------------------------------------------------------
// App keypool API
// ---------------------------------------------------------------------------

export async function getAppKeypool(id: number): Promise<AppKeypoolResult> {
  const res = await api.get<{ success: boolean; data: AppKeypoolResult }>(
    `/dashboard/zsy/rh/apps/${id}/keypool`
  )
  return res.data.data
}

export async function refreshAppKeypool(
  id: number
): Promise<KeypoolRefreshResult> {
  const res = await api.post<{ success: boolean; data: KeypoolRefreshResult }>(
    `/dashboard/zsy/rh/apps/${id}/keypool-refresh`
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// RunningHub channels (host /api/channel filtered by type 61)
// ---------------------------------------------------------------------------

export async function getRhChannels(): Promise<RhChannel[]> {
  const res = await api.get<{
    success: boolean
    data?: { items?: RhChannel[] }
  }>('/api/channel', { params: { type: 61, page: 1, pageSize: 100 } })
  return res.data.data?.items ?? []
}

// ---------------------------------------------------------------------------
// User-side app center API (public + user auth)
// ---------------------------------------------------------------------------

export interface RunAppResult {
  taskId: string
  status: string
  upstreamTaskId: string
  raw: unknown
}

/** TaskDto mirrors the host's relay.TaskModel2Dto envelope used by task logs. */
export interface TaskDto {
  id: number
  task_id: string
  platform: string
  group: string
  quota: number
  action: string
  status: string
  fail_reason: string
  result_url: string
  submit_time: number
  start_time: number
  finish_time: number
  progress: string
  username: string
  properties?: Record<string, unknown>
  data?: unknown
}

export interface TaskListPage {
  page: number
  page_size: number
  total: number
  items: TaskDto[]
}

/** List published, non-admin-only apps visible to the current caller. */
export async function listPublicApps(params?: {
  keyword?: string
  kind?: string
  p?: number
  page_size?: number
}): Promise<AppListResult> {
  const res = await api.get<{ success: boolean; data: AppListResult }>(
    '/api/zsy/rh/apps',
    { params }
  )
  return res.data.data
}

/** Detail shape used by the dynamic form renderer (includes paramSchema). */
export async function getPublicAppDetail(id: number): Promise<AppView> {
  const res = await api.get<{ success: boolean; data: AppView }>(
    `/api/zsy/rh/apps/${id}`
  )
  return res.data.data
}

/** Submit an app run. `values` are keyed by `nodeId.fieldName` per schema. */
export async function runApp(
  id: number,
  values: Record<string, unknown>,
  instanceType?: string
): Promise<RunAppResult> {
  const res = await api.post<{ success: boolean; data: RunAppResult }>(
    `/api/zsy/rh/apps/${id}/run`,
    { values, instanceType }
  )
  return res.data.data
}

/** Fetch a single task record (TaskDto) by public task_id. */
export async function getTaskResult(taskId: string): Promise<TaskDto> {
  const res = await api.get<{ success: boolean; data: TaskDto }>(
    `/api/zsy/rh/apps/task/${taskId}`
  )
  return res.data.data
}

/** Paginated list of the current user's RunningHub generation records. */
export async function listMyRhTasks(params?: {
  status?: string
  p?: number
  page_size?: number
}): Promise<TaskListPage> {
  const res = await api.get<{ success: boolean; data: TaskListPage }>(
    '/api/zsy/rh/apps/tasks',
    { params }
  )
  return res.data.data
}