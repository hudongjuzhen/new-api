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
 * Sample-audio rules for the Voice Plaza, mirroring the backend upload
 * endpoint (zsy/voice/controllers_upload.go): the same allow-list, the same
 * 20MB cap, so a file rejected here would also have been rejected there.
 */
export const VOICE_AUDIO_MAX_BYTES = 20 * 1024 * 1024

export const VOICE_AUDIO_ACCEPT = '.mp3,.wav,.m4a,.aac,.ogg,.opus,.flac,.webm'

const ALLOWED_EXTENSIONS = new Set([
  'mp3',
  'wav',
  'm4a',
  'aac',
  'ogg',
  'opus',
  'flac',
  'webm',
])

/** i18n keys returned by validateVoiceAudioFile. */
export const VOICE_AUDIO_UNSUPPORTED_MESSAGE =
  'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)'
export const VOICE_AUDIO_TOO_LARGE_MESSAGE =
  'Audio file must be 20MB or smaller'

function extensionOf(fileName: string): string {
  const dot = fileName.lastIndexOf('.')
  if (dot < 0 || dot === fileName.length - 1) return ''
  return fileName.slice(dot + 1).toLowerCase()
}

/**
 * Checks a picked file before a byte leaves the browser.
 *
 * A name without an extension is accepted on purpose: a recorder posting a
 * Blob has none, and the backend sniffs its content type instead.
 *
 * @returns the i18n key of the failure, or null when the file may be uploaded.
 */
export function validateVoiceAudioFile(file: {
  name: string
  size: number
}): string | null {
  const extension = extensionOf(file.name)
  if (extension !== '' && !ALLOWED_EXTENSIONS.has(extension)) {
    return VOICE_AUDIO_UNSUPPORTED_MESSAGE
  }
  if (file.size > VOICE_AUDIO_MAX_BYTES) {
    return VOICE_AUDIO_TOO_LARGE_MESSAGE
  }
  return null
}

/** Human-readable byte size for the audio cell and the upload hint. */
export function formatVoiceAudioSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  const rounded =
    unit === 0 || value >= 10 ? Math.round(value) : value.toFixed(1)
  return `${rounded} ${units[unit]}`
}
