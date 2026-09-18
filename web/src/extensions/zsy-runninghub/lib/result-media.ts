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
 * Result classification for the RunningHub app center.
 *
 * A run can return more than images: workflows also emit audio, video, zipped
 * bundles and plain text. Every renderer decision (thumbnail, preview dialog,
 * download) keys off the kind computed here, so classification is kept as pure,
 * testable logic instead of being scattered through the page component.
 */

import type { TaskDto } from '../api'

export type RhResultKind =
  | 'image'
  | 'video'
  | 'audio'
  | 'text'
  | 'archive'
  | 'file'

export interface RhResultItem {
  /** Stable identity for lists and dialog keys. */
  key: string
  kind: RhResultKind
  /** Absolute http(s) URL, absent for inline text results. */
  url?: string
  /** Inline text payload (RunningHub `results[].text`), previewed as-is. */
  text?: string
  /** File name taken from the URL path, used in download labels. */
  fileName: string
  /** Upstream node that produced the value, when RunningHub reports it. */
  nodeId?: string
}

/** Extension → kind. Unknown extensions fall back to the outputType hint. */
const EXTENSION_KINDS: Record<string, RhResultKind> = {
  // images
  png: 'image',
  jpg: 'image',
  jpeg: 'image',
  webp: 'image',
  gif: 'image',
  bmp: 'image',
  avif: 'image',
  svg: 'image',
  tif: 'image',
  tiff: 'image',
  ico: 'image',
  // video
  mp4: 'video',
  webm: 'video',
  mov: 'video',
  m4v: 'video',
  mkv: 'video',
  avi: 'video',
  // audio
  mp3: 'audio',
  wav: 'audio',
  flac: 'audio',
  ogg: 'audio',
  oga: 'audio',
  m4a: 'audio',
  aac: 'audio',
  opus: 'audio',
  wma: 'audio',
  // text
  txt: 'text',
  md: 'text',
  markdown: 'text',
  json: 'text',
  csv: 'text',
  tsv: 'text',
  log: 'text',
  srt: 'text',
  vtt: 'text',
  xml: 'text',
  yml: 'text',
  yaml: 'text',
  // archives
  zip: 'archive',
  rar: 'archive',
  '7z': 'archive',
  tar: 'archive',
  gz: 'archive',
  tgz: 'archive',
  bz2: 'archive',
  xz: 'archive',
}

/** RunningHub `outputType` → kind, used only when the URL has no extension. */
const OUTPUT_TYPE_KINDS: Record<string, RhResultKind> = {
  image: 'image',
  img: 'image',
  picture: 'image',
  video: 'video',
  movie: 'video',
  audio: 'audio',
  music: 'audio',
  sound: 'audio',
  text: 'text',
  string: 'text',
  zip: 'archive',
  archive: 'archive',
}

/**
 * Path part of a URL (query and fragment removed). Falls back to string
 * parsing for relative or malformed values so the host is never treated as a
 * path segment.
 */
function pathnameOf(rawUrl: string): string {
  const trimmed = rawUrl.trim()
  try {
    return new URL(trimmed).pathname
  } catch {
    const withoutScheme = trimmed
      .split(/[?#]/)[0]
      .replace(/^[a-z][a-z0-9+.-]*:\/\//i, '')
    const slash = withoutScheme.indexOf('/')
    return slash >= 0 ? withoutScheme.slice(slash) : ''
  }
}

/** Lower-cased extension of a URL path, or '' when it has none. */
export function resultExtension(url: string): string {
  const match = /\.([a-z0-9]+)$/i.exec(pathnameOf(url))
  if (!match) return ''
  return match[1].toLowerCase()
}

/**
 * Decoded file name of a URL (last path segment). Returns '' for URLs without
 * a usable name, e.g. `https://host/` or a query-only download link.
 */
export function fileNameFromUrl(url: string): string {
  const segments = pathnameOf(url).split('/').filter(Boolean)
  const last = segments.at(-1)
  if (!last) return ''
  try {
    return decodeURIComponent(last)
  } catch {
    return last
  }
}

/**
 * Classify one result value. The URL extension is authoritative because it
 * describes the actual file; `outputType` only fills the gap for extension-less
 * URLs (RunningHub serves some results from a path without a suffix).
 */
export function inferResultKind(
  url: string | undefined,
  outputType?: string
): RhResultKind {
  if (url) {
    const byExtension = EXTENSION_KINDS[resultExtension(url)]
    if (byExtension) return byExtension
  }
  const hint = (outputType || '').trim().toLowerCase()
  if (hint && OUTPUT_TYPE_KINDS[hint]) {
    return OUTPUT_TYPE_KINDS[hint]
  }
  return 'file'
}

/** Archives and unknown binaries have no inline preview — they download. */
export function isDownloadOnlyKind(kind: RhResultKind): boolean {
  return kind === 'archive' || kind === 'file'
}

function isAbsoluteHttpUrl(value: unknown): value is string {
  return typeof value === 'string' && /^https?:\/\//i.test(value)
}

/**
 * Extract every renderable result of a task, in upstream order.
 *
 * Accepted shapes (the poller stores the raw RunningHub query response in
 * `task.data`):
 *   - `{ results: [{ url, nodeId, outputType, text }] }` — the V2 flat shape
 *   - `[{ ... }]` — a bare results array
 *
 * Only absolute http(s) URLs are treated as files: a relative path (or an error
 * message that leaked into `url`) must never become an `<img src>` or `<a href>`
 * because the browser would navigate the SPA. Text-only results (no URL) are
 * kept so their content can be previewed without a fetch.
 */
export function extractResults(task: TaskDto): RhResultItem[] {
  const raw = task?.data
  let results: Array<Record<string, unknown>> = []
  if (Array.isArray(raw)) {
    results = raw as Array<Record<string, unknown>>
  } else if (raw && typeof raw === 'object') {
    const nested = (raw as { results?: unknown }).results
    if (Array.isArray(nested)) {
      results = nested as Array<Record<string, unknown>>
    }
  }

  const items: RhResultItem[] = []
  const seen = new Set<string>()
  results.forEach((entry, index) => {
    let url = ''
    if (isAbsoluteHttpUrl(entry?.url)) {
      url = entry.url
    } else if (isAbsoluteHttpUrl(entry?.value)) {
      url = entry.value as string
    }
    const outputType =
      typeof entry?.outputType === 'string' ? entry.outputType : ''
    const nodeId = typeof entry?.nodeId === 'string' ? entry.nodeId : ''
    if (url) {
      if (seen.has(url)) return
      seen.add(url)
      items.push({
        key: url,
        url,
        kind: inferResultKind(url, outputType),
        fileName: fileNameFromUrl(url),
        nodeId,
      })
      return
    }
    // Inline text result: RunningHub returns the payload itself instead of a
    // file, and there is nothing to download.
    const text = typeof entry?.text === 'string' ? entry.text : ''
    if (text.trim() !== '') {
      const key = `text:${index}`
      items.push({
        key,
        kind: 'text',
        text,
        fileName: '',
        nodeId,
      })
    }
  })

  if (items.length === 0 && isAbsoluteHttpUrl(task?.result_url)) {
    items.push({
      key: task.result_url,
      url: task.result_url,
      kind: inferResultKind(task.result_url),
      fileName: fileNameFromUrl(task.result_url),
    })
  }
  return items
}
