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
 * Avatar Plaza admin API client (zsy/avatar on the Go side).
 *
 * Every admin endpoint lives under /dashboard/zsy/avatar and is guarded by the
 * host's admin-auth middleware; the public catalog endpoint is anonymous and is
 * only referenced here so the page can show integrators its exact URL.
 */

/** Public, paginated catalog of on-shelf personas, consumed by other apps. */
export const PUBLIC_AVATAR_LIST_PATH = '/api/zsy/avatar/list'

const ADMIN_BASE = '/dashboard/zsy/avatar'

/** One persona row, as returned by every plugin endpoint. */
export interface AvatarView {
  id: number
  createdAt: number
  updatedAt: number
  name: string
  description: string
  /** Cover picture (封面图): a gateway /uploads/images/... path or an absolute http(s) URL. */
  imageUrl: string
  /** Full-body photo (全身照); '' when the persona does not carry one yet. */
  fullBodyUrl: string
  /** Four-view sheet (四视图); '' when the persona does not carry one yet. */
  fourViewUrl: string
  /** Expression sheet (表情图); '' when the persona does not carry one yet. */
  expressionUrl: string
  /** Controlled vocabulary: '' | male | female | neutral. */
  gender: string
  /** Controlled vocabulary: '' | child | teen | young | middle | senior. */
  ageRange: string
  /** Controlled vocabulary: '' | asian | black | white | latino | middle_eastern | south_asian | mixed. */
  race: string
  /** Suitable scenes ("适合场景"), e.g. ["客服播报", "有声书"]. */
  scenes: string[]
  /** `voice_type` of the voice this persona speaks with; '' when unset. */
  voiceId: string
  /** Whether voiceId exists in the Voice Plaza catalog. */
  voiceAvailable: boolean
  /** Display name of that voice; '' when it is not catalogued. */
  voiceName: string
  /** Sample audio resolved from the linked voice; '' when there is none. */
  voiceSampleUrl: string
  voiceSampleName: string
  enabled: boolean
  sortOrder: number
}

export interface AvatarListResult {
  items: AvatarView[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

/**
 * Write payload of create/update.
 *
 * The voice sample is deliberately absent: it always follows the voice named by
 * `voiceId`, so the plaza stays the single source of truth for the audio.
 */
export interface AvatarUpsertDTO {
  name: string
  description: string
  /** The four pictures, each '' when it is not set. */
  imageUrl: string
  fullBodyUrl: string
  fourViewUrl: string
  expressionUrl: string
  gender: string
  ageRange: string
  race: string
  scenes: string[]
  voiceId: string
  enabled: boolean
  sortOrder: number
}

export interface AvatarListParams {
  keyword?: string
  /** Controlled vocabulary value, or '' to accept every gender. */
  gender?: string
  /** Controlled vocabulary value, or '' to accept every age range. */
  ageRange?: string
  /** Controlled vocabulary value, or '' to accept every ethnicity. */
  race?: string
  /** Exact `voice_type`, or '' to accept every voice. */
  voiceId?: string
  /** Omit to list both on- and off-shelf personas. */
  enabled?: boolean
  page?: number
  pageSize?: number
}

/**
 * Query parameters shared by the list and the export, so an export always covers
 * exactly the selection the table is showing.
 */
function avatarFilterParams(params: AvatarListParams): Record<string, unknown> {
  return {
    keyword: params.keyword || undefined,
    gender: params.gender || undefined,
    age_range: params.ageRange || undefined,
    race: params.race || undefined,
    voice_id: params.voiceId || undefined,
    enabled: typeof params.enabled === 'boolean' ? params.enabled : undefined,
  }
}

/** Paginated admin list: every persona, on shelf or not. */
export async function listAvatars(
  params: AvatarListParams = {}
): Promise<AvatarListResult> {
  const res = await api.get<{ success: boolean; data: AvatarListResult }>(
    `${ADMIN_BASE}/list`,
    {
      params: {
        ...avatarFilterParams(params),
        page: params.page,
        page_size: params.pageSize,
      },
    }
  )
  return res.data.data
}

export async function createAvatar(dto: AvatarUpsertDTO): Promise<AvatarView> {
  const res = await api.post<{ success: boolean; data: AvatarView }>(
    ADMIN_BASE,
    dto
  )
  return res.data.data
}

export async function updateAvatar(
  id: number,
  dto: Partial<AvatarUpsertDTO>
): Promise<AvatarView> {
  const res = await api.put<{ success: boolean; data: AvatarView }>(
    `${ADMIN_BASE}/${id}`,
    dto
  )
  return res.data.data
}

export async function deleteAvatar(id: number): Promise<void> {
  await api.delete(`${ADMIN_BASE}/${id}`)
}

/** Absolute URL of the public catalog, for the integrator card on the page. */
export function publicAvatarListUrl(baseUrl: string, pageSize = 20): string {
  const base = baseUrl.replace(/\/+$/, '')
  return `${base}${PUBLIC_AVATAR_LIST_PATH}?page=1&page_size=${pageSize}`
}

// ---------------------------------------------------------------------------
// CSV import / export
// ---------------------------------------------------------------------------

/** Import modes accepted by the backend. */
export type AvatarImportMode = 'upsert' | 'create'

/** One rejected CSV row, with the record number the operator must fix. */
export interface AvatarImportRowError {
  row: number
  name?: string
  message: string
}

export interface AvatarImportResult {
  total: number
  created: number
  updated: number
  failed: number
  errors: AvatarImportRowError[]
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
 * Download every persona matching the filters as CSV. The blob is returned as-is
 * so the caller decides how to hand it to the browser.
 */
export async function exportAvatars(
  params: AvatarListParams = {}
): Promise<{ blob: Blob; filename: string }> {
  const res = await api.get<Blob>(`${ADMIN_BASE}/export`, {
    params: avatarFilterParams(params),
    responseType: 'blob',
  })
  return {
    blob: res.data,
    filename: filenameFromDisposition(
      res.headers?.['content-disposition'],
      'zsy-avatars.csv'
    ),
  }
}

/**
 * Import a CSV. Business failures (a broken header, an unknown mode) are surfaced
 * to the caller; per-row failures come back inside the result.
 */
export async function importAvatarsCsv(
  file: File,
  mode: AvatarImportMode
): Promise<AvatarImportResult> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<{
    success: boolean
    message?: string
    data?: AvatarImportResult
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
