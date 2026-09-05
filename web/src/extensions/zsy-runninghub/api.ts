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
  perSecondBilling: boolean
  quotaPerSecond: number
  modelBaseRateRatio: number
  site: string
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
  perSecondBilling: boolean
  quotaPerSecond: number
  modelBaseRateRatio: number
  site: string
}

export interface FetchTemplateResult {
  appName: string
  upstreamId: string
  schema: SchemaParam[]
  schemaErrors?: { field: string; message: string }[]
}

/** Parser output of a pasted RunningHub curl request example. */
export interface ParseCurlResult {
  kind: string
  upstreamId: string
  baseUrl?: string
  slug?: string
  nodeInfoList?: Array<{
    nodeId: string
    fieldName: string
    field: string
    fieldValue: string
    fieldData?: string
    description?: string
  }>
  schema: SchemaParam[]
  schemaErrors?: { field: string; message: string }[]
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

/**
 * Parse a pasted RunningHub curl request example and draft the app fields
 * (kind, upstream id, app name) plus the parameter schema. Reuses the admin
 * curl-parser endpoint so admins do not have to copy node ids by hand.
 *
 * Business errors (success:false) are surfaced by the caller instead of the
 * global interceptor so the message can be i18n'd in the form.
 */
export async function parseCurlRequest(curl: string) {
  const res = await api.post<{
    success: boolean
    data?: ParseCurlResult
    message?: string
  }>('/dashboard/zsy/rh/apps/parse-curl', { curl }, { skipBusinessError: true })
  return res.data
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
  opts: { instanceType?: string; tokenId?: number } = {}
): Promise<RunAppResult> {
  const res = await api.post<{ success: boolean; data: RunAppResult }>(
    `/api/zsy/rh/apps/${id}/run`,
    {
      values,
      instanceType: opts.instanceType ?? 'default',
      tokenId: opts.tokenId,
    }
  )
  return res.data.data
}

/**
 * Check whether the RunningHub site has at least one enabled channel that can
 * accept media uploads. `site` is the app's site value ('' | 'cn' | 'intl');
 * '' resolves to the default domestic site.
 */
export async function getUploadChannelStatus(site: string): Promise<{
  available: boolean
  count: number
}> {
  const res = await api.get<{
    success: boolean
    data?: { available: boolean; count: number }
  }>('/api/zsy/rh/upload-channel', { params: { site } })
  return res.data.data ?? { available: false, count: 0 }
}

/**
 * Upload a media file (image/audio/video) to the RunningHub site the app's
 * `site` field declares. The backend picks the first enabled channel of the
 * site's type and returns the upstream fileName (e.g. `openapi/xxxx.png`)
 * which is what image/audio/video node inputs expect as their fieldValue.
 */
export async function uploadAppMedia(
  site: string,
  file: File
): Promise<{ fileName: string; url: string }> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<{
    success: boolean
    message?: string
    data?: { fileName: string; url: string }
  }>('/api/zsy/rh/upload', formData, {
    params: { site },
    skipBusinessError: true,
  })
  const data = res.data
  if (!data.success || !data.data?.fileName) {
    throw new Error(data.message || 'Upload failed')
  }
  return data.data
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
