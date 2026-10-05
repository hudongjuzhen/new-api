/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

/**
 * Voice Plaza admin API client (zsy/voice on the Go side).
 *
 * Every admin endpoint lives under /dashboard/zsy/voice and is guarded by the
 * host's admin-auth middleware; the public catalog endpoint is anonymous and is
 * only referenced here so the page can show integrators its exact URL.
 */

/** Public, paginated catalog consumed by third parties. */
export const PUBLIC_VOICE_LIST_PATH = '/api/zsy/voice/list'

const ADMIN_BASE = '/dashboard/zsy/voice'

/** One voice row, as returned by every plugin endpoint. */
export interface VoiceView {
  id: number
  createdAt: number
  updatedAt: number
  name: string
  description: string
  /** The upstream `voice_type` value a TTS request sends. */
  voiceType: string
  /** Controlled vocabulary: '' | male | female | neutral. */
  gender: string
  /** Controlled vocabulary: '' | child | teen | young | middle | senior. */
  ageRange: string
  /** Lower-case language tag, e.g. "zh", "en", "pt-br"; '' when unspecified. */
  language: string
  /** Suitable scenes ("适合场景"), e.g. ["客服播报", "有声书"]. */
  scenes: string[]
  /** Portrait shown in the plaza; '' when the voice has no avatar. */
  avatarUrl: string
  audioUrl: string
  audioName: string
  audioSize: number
  enabled: boolean
  sortOrder: number
}

export interface VoiceListResult {
  items: VoiceView[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

/** Write payload of create/update. */
export interface VoiceUpsertDTO {
  name: string
  description: string
  voiceType: string
  gender: string
  ageRange: string
  language: string
  scenes: string[]
  avatarUrl: string
  audioUrl: string
  audioName: string
  audioSize: number
  enabled: boolean
  sortOrder: number
}

export interface VoiceUploadResult {
  /** Gateway-served path to store on the voice row, e.g. /uploads/voices/202601/x.mp3 */
  url: string
  filename: string
  originalName: string
  size: number
  mimeType: string
}

export interface VoiceListParams {
  keyword?: string
  voiceType?: string
  /** Controlled vocabulary value, or '' to accept every gender. */
  gender?: string
  /** Controlled vocabulary value, or '' to accept every age range. */
  ageRange?: string
  /** Language tag (case-insensitive), or '' to accept every language. */
  language?: string
  /** Omit to list both on- and off-shelf voices. */
  enabled?: boolean
  page?: number
  pageSize?: number
}

/**
 * Query parameters shared by the list and the export, so an export always covers
 * exactly the selection the table is showing.
 */
function voiceFilterParams(params: VoiceListParams): Record<string, unknown> {
  return {
    keyword: params.keyword || undefined,
    voice_type: params.voiceType || undefined,
    gender: params.gender || undefined,
    age_range: params.ageRange || undefined,
    language: params.language || undefined,
    enabled: typeof params.enabled === 'boolean' ? params.enabled : undefined,
  }
}

/** Paginated admin list: every voice, on shelf or not. */
export async function listVoices(
  params: VoiceListParams = {}
): Promise<VoiceListResult> {
  const res = await api.get<{ success: boolean; data: VoiceListResult }>(
    `${ADMIN_BASE}/list`,
    {
      params: {
        ...voiceFilterParams(params),
        page: params.page,
        page_size: params.pageSize,
      },
    }
  )
  return res.data.data
}

export async function createVoice(dto: VoiceUpsertDTO): Promise<VoiceView> {
  const res = await api.post<{ success: boolean; data: VoiceView }>(
    ADMIN_BASE,
    dto
  )
  return res.data.data
}

export async function updateVoice(
  id: number,
  dto: Partial<VoiceUpsertDTO>
): Promise<VoiceView> {
  const res = await api.put<{ success: boolean; data: VoiceView }>(
    `${ADMIN_BASE}/${id}`,
    dto
  )
  return res.data.data
}

export async function deleteVoice(id: number): Promise<void> {
  await api.delete(`${ADMIN_BASE}/${id}`)
}

/**
 * Upload one sample audio file. Business errors (unsupported format, file too
 * large) are surfaced to the caller so the dialog can show them inline.
 */
export async function uploadVoiceAudio(file: File): Promise<VoiceUploadResult> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<{
    success: boolean
    message?: string
    data?: VoiceUploadResult
  }>(`${ADMIN_BASE}/upload`, formData, { skipBusinessError: true })

  const payload = res.data
  if (!payload.success || !payload.data) {
    throw new Error(payload.message || 'Upload failed')
  }
  return payload.data
}

/** Absolute URL of the public catalog, for the integrator card on the page. */
export function publicVoiceListUrl(baseUrl: string, pageSize = 20): string {
  const base = baseUrl.replace(/\/+$/, '')
  return `${base}${PUBLIC_VOICE_LIST_PATH}?page=1&page_size=${pageSize}`
}

// ---------------------------------------------------------------------------
// CSV import / export
// ---------------------------------------------------------------------------

/** Import modes accepted by the backend. */
export type VoiceImportMode = 'upsert' | 'create'

/** One rejected CSV row, with the record number the operator must fix. */
export interface VoiceImportRowError {
  row: number
  name?: string
  message: string
}

export interface VoiceImportResult {
  total: number
  created: number
  updated: number
  failed: number
  errors: VoiceImportRowError[]
  warnings?: string[]
}

/** Filename the server chose for an export; the attachment name is authoritative. */
function filenameFromDisposition(
  disposition: string | undefined,
  fallback: string
): string {
  if (!disposition) return fallback
  const match = /filename="?([^";]+)"?/i.exec(disposition)
  return match ? match[1] : fallback
}

/**
 * Download every voice matching the filters as CSV. The blob is returned as-is
 * so the caller decides how to hand it to the browser.
 */
export async function exportVoices(
  params: VoiceListParams = {}
): Promise<{ blob: Blob; filename: string }> {
  const res = await api.get<Blob>(`${ADMIN_BASE}/export`, {
    params: voiceFilterParams(params),
    responseType: 'blob',
  })
  return {
    blob: res.data,
    filename: filenameFromDisposition(
      res.headers?.['content-disposition'],
      'zsy-voices.csv'
    ),
  }
}

/**
 * Import a CSV. Business failures (a broken header, an unknown mode) are
 * surfaced to the caller; per-row failures come back inside the result.
 */
export async function importVoicesCsv(
  file: File,
  mode: VoiceImportMode
): Promise<VoiceImportResult> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<{
    success: boolean
    message?: string
    data?: VoiceImportResult
  }>(`${ADMIN_BASE}/import`, formData, {
    params: { mode },
    skipBusinessError: true,
  })

  const payload = res.data
  if (!payload.success || !payload.data) {
    throw new Error(payload.message || 'Import failed')
  }
  return payload.data
}
