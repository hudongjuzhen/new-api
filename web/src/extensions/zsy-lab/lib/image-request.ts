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
/**
 * Request builders for the lab image playground (OpenAI images API:
 * /v1/images/generations JSON body and /v1/images/edits multipart fields).
 *
 * Contract: `auto` parameters are omitted so upstream defaults apply, and the
 * image count is clamped to a positive bounded integer before it can reach
 * billing (defense in depth with relaykit dto.MaxImageN on the backend).
 */
export const IMAGE_EDIT_MAX_IMAGES = 16
export const IMAGE_UPLOAD_MAX_BYTES = 20 * 1024 * 1024
export const IMAGE_UPLOAD_ACCEPT = 'image/jpeg,image/png,image/webp'
const IMAGE_N_MAX = 16

export interface ImageRunParams {
  prompt: string
  size: string
  quality: string
  n: number
  outputFormat: string
  background: string
  moderation: string
}

function clampImageN(n: number): number {
  if (!Number.isFinite(n)) return 1
  const rounded = Math.round(n)
  return Math.min(Math.max(rounded, 1), IMAGE_N_MAX)
}

function omitAuto(value: string): string | undefined {
  const trimmed = value.trim()
  return trimmed && trimmed !== 'auto' ? trimmed : undefined
}

export interface ImagesGenerationBody {
  model: string
  prompt: string
  n: number
  size?: string
  quality?: string
  output_format?: string
  background?: string
  moderation?: string
}

export function buildImagesGenerationBody(
  model: string,
  params: ImageRunParams
): ImagesGenerationBody {
  return {
    model,
    prompt: params.prompt,
    n: clampImageN(params.n),
    size: omitAuto(params.size),
    quality: omitAuto(params.quality),
    output_format: omitAuto(params.outputFormat),
    background: omitAuto(params.background),
    moderation: omitAuto(params.moderation),
  }
}

/** Non-file multipart fields for /v1/images/edits, in upload order. */
export function buildImageEditValueFields(
  model: string,
  params: ImageRunParams
): Array<[string, string]> {
  const fields: Array<[string, string]> = [
    ['model', model],
    ['prompt', params.prompt],
    ['n', String(clampImageN(params.n))],
  ]
  const size = omitAuto(params.size)
  if (size) fields.push(['size', size])
  const quality = omitAuto(params.quality)
  if (quality) fields.push(['quality', quality])
  const outputFormat = omitAuto(params.outputFormat)
  if (outputFormat) fields.push(['output_format', outputFormat])
  const background = omitAuto(params.background)
  if (background) fields.push(['background', background])
  const moderation = omitAuto(params.moderation)
  if (moderation) fields.push(['moderation', moderation])
  return fields
}

export function isSupportedImageFile(file: File): boolean {
  if (file.size > IMAGE_UPLOAD_MAX_BYTES) return false
  return /image\/(jpeg|png|webp)/i.test(file.type)
}

/** Decoded output images: b64_json becomes a data URI, url passes through. */
export function extractResultImages(
  body: { data?: Array<{ url?: string; b64_json?: string }> },
  outputFormat: string
): string[] {
  const mime = omitAuto(outputFormat) || 'png'
  return (body.data ?? [])
    .map((item) => {
      if (item.b64_json) {
        return `data:image/${mime};base64,${item.b64_json}`
      }
      return item.url ?? ''
    })
    .filter((src) => src.length > 0)
}
