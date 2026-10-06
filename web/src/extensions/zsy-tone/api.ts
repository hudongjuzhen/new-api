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
 * Tone Plaza admin API client (zsy/tone on the Go side).
 *
 * Every admin endpoint lives under /dashboard/zsy/tone and is guarded by the
 * host's admin-auth middleware. Two endpoints are public and anonymous: the
 * paginated catalog third parties consume, and the standard definition the
 * vocabulary is published under.
 *
 * Unlike the voice plaza there is no upload endpoint: a tone is text only
 * (prompt + an optional sample pair), so this client carries no media type and
 * no multipart request.
 */

/** Public, paginated catalog consumed by third parties. Only enabled rows. */
export const PUBLIC_TONE_LIST_PATH = '/api/zsy/tone/list'

/** Public definition of the vocabulary, the caps and the compatibility policy. */
export const PUBLIC_TONE_STANDARD_PATH = '/api/zsy/tone/standard'

const ADMIN_BASE = '/dashboard/zsy/tone'

/** One tone row, as returned by every plugin endpoint. */
export interface ToneView {
  id: number
  createdAt: number
  updatedAt: number
  name: string
  description: string
  /** The instruction a downstream model follows; the core of the row. */
  prompt: string
  /** Controlled vocabulary: '' | literary | business | academic | media | spoken | technical | marketing. */
  category: string
  /** Controlled vocabulary: '' | warm | calm | sharp | humorous | solemn | lively | plain. */
  tone: string
  /** Lower-case language tag, e.g. "zh", "en", "pt-br"; '' when unspecified. */
  language: string
  /** Suitable scenes ("适合场景"), e.g. ["公众号长文", "技术文档"]. */
  scenes: string[]
  /** Before / after pair showing what this tone does; both may be ''. */
  sampleInput: string
  sampleOutput: string
  enabled: boolean
  sortOrder: number
}

export interface ToneListResult {
  items: ToneView[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

/** Write payload of create/update. */
export interface ToneUpsertDTO {
  name: string
  description: string
  prompt: string
  category: string
  tone: string
  language: string
  scenes: string[]
  sampleInput: string
  sampleOutput: string
  enabled: boolean
  sortOrder: number
}

export interface ToneListParams {
  keyword?: string
  /** Controlled vocabulary value, or '' to accept every category. */
  category?: string
  /** Controlled vocabulary value, or '' to accept every tone. */
  tone?: string
  /** Language tag (case-insensitive), or '' to accept every language. */
  language?: string
  /** Omit to list both on- and off-shelf tones. */
  enabled?: boolean
  page?: number
  pageSize?: number
}

/**
 * Query parameters shared by the list and the export, so an export always covers
 * exactly the selection the table is showing.
 */
function toneFilterParams(params: ToneListParams): Record<string, unknown> {
  return {
    keyword: params.keyword || undefined,
    category: params.category || undefined,
    tone: params.tone || undefined,
    language: params.language || undefined,
    enabled: typeof params.enabled === 'boolean' ? params.enabled : undefined,
  }
}

/** Paginated admin list: every tone, on shelf or not. */
export async function listTones(
  params: ToneListParams = {}
): Promise<ToneListResult> {
  const res = await api.get<{ success: boolean; data: ToneListResult }>(
    `${ADMIN_BASE}/list`,
    {
      params: {
        ...toneFilterParams(params),
        page: params.page,
        page_size: params.pageSize,
      },
    }
  )
  return res.data.data
}

export async function createTone(dto: ToneUpsertDTO): Promise<ToneView> {
  const res = await api.post<{ success: boolean; data: ToneView }>(
    ADMIN_BASE,
    dto
  )
  return res.data.data
}

export async function updateTone(
  id: number,
  dto: Partial<ToneUpsertDTO>
): Promise<ToneView> {
  const res = await api.put<{ success: boolean; data: ToneView }>(
    `${ADMIN_BASE}/${id}`,
    dto
  )
  return res.data.data
}

export async function deleteTone(id: number): Promise<void> {
  await api.delete(`${ADMIN_BASE}/${id}`)
}

/**
 * The published standard: which values exist, what the caps are, and what the
 * backend promises about them. Returned raw — the caller normalizes it against
 * its local mirror (see lib/tone-fields.ts) so a partial payload still renders.
 *
 * `skipErrorHandler` keeps a *missing* standard from firing the host's global
 * error toast: the plaza treats the endpoint as an enhancement and falls back to
 * its mirror, so a failure here is not something to interrupt an operator with.
 */
export async function fetchToneStandard(): Promise<unknown> {
  const res = await api.get<unknown>(PUBLIC_TONE_STANDARD_PATH, {
    skipErrorHandler: true,
  })
  return res.data
}

/** Absolute URL of the public catalog, for the integrator card on the page. */
export function publicToneListUrl(baseUrl: string, pageSize = 20): string {
  const base = baseUrl.replace(/\/+$/, '')
  return `${base}${PUBLIC_TONE_LIST_PATH}?page=1&page_size=${pageSize}`
}

/** Absolute URL of the public standard, for the integrator card on the page. */
export function publicToneStandardUrl(baseUrl: string): string {
  const base = baseUrl.replace(/\/+$/, '')
  return `${base}${PUBLIC_TONE_STANDARD_PATH}`
}

// ---------------------------------------------------------------------------
// CSV import / export
// ---------------------------------------------------------------------------

/** Import modes accepted by the backend. */
export type ToneImportMode = 'upsert' | 'create'

/** One rejected CSV row, with the record number the operator must fix. */
export interface ToneImportRowError {
  row: number
  name?: string
  message: string
}

export interface ToneImportResult {
  total: number
  created: number
  updated: number
  failed: number
  errors: ToneImportRowError[]
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
 * Download every tone matching the filters as CSV. The blob is returned as-is so
 * the caller decides how to hand it to the browser.
 */
export async function exportTones(
  params: ToneListParams = {}
): Promise<{ blob: Blob; filename: string }> {
  const res = await api.get<Blob>(`${ADMIN_BASE}/export`, {
    params: toneFilterParams(params),
    responseType: 'blob',
  })
  return {
    blob: res.data,
    filename: filenameFromDisposition(
      res.headers?.['content-disposition'],
      'zsy-tones.csv'
    ),
  }
}

/**
 * Import a CSV. Business failures (a broken header, an unknown mode) are
 * surfaced to the caller; per-row failures come back inside the result.
 */
export async function importTonesCsv(
  file: File,
  mode: ToneImportMode
): Promise<ToneImportResult> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<{
    success: boolean
    message?: string
    data?: ToneImportResult
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
