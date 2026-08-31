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
 * Image-generation models stream their output in several shapes:
 * - markdown image with a base64 data URI: `![image_1](data:image/png;base64,....)`
 * - markdown image with a remote URL: `![image_1](https://cdn.example/a.png)`
 * - inline-code image URL: `` `https://oss.example/uploads/a.png` ``
 *
 * The lab preview extracts every complete reference into renderable images
 * and strips them from the text view, so users see the picture instead of a
 * wall of base64 or a bare link. Unlike the playground helper this runs on
 * unbounded streaming content, so the scan is strictly linear (base64
 * payloads can exceed hundreds of kilobytes) and a trailing incomplete
 * reference is trimmed while the payload is still arriving.
 */
export interface LabImageExtraction {
  /** Complete image sources (data URIs or https URLs) for <img src>. */
  images: string[]
  /** Content with complete and incomplete image references removed. */
  displayContent: string
}

// Alternation of the three reference shapes. Capture groups:
// 1 = base64 data URI, 2 = remote URL (markdown image), 3 = inline-code URL.
// `[^)]*` / `[^\s)]+` / `[^\s`]+` keep every scan linear: a base64 payload
// never contains a closing paren, and remote URLs contain no whitespace.
const GENERATED_IMAGE_RE =
  /!\[[^\]]*\]\((data:image\/[A-Za-z0-9.+-]+;base64,[^)]*)\)|!\[[^\]]*\]\((https?:\/\/[^\s)]+\.(?:png|jpe?g|webp|gif)(?:\?[^\s)]*)?)\)|`(https?:\/\/[^\s`]+\.(?:png|jpe?g|webp|gif)(?:\?[^\s`]*)?)`/g

// A data-URI reference that started but has not received its closing `)` yet —
// always the tail of the content while the payload is streaming in.
const TRAILING_INCOMPLETE_IMAGE_RE =
  /!\[[^\]]*\]\(data:image\/[A-Za-z0-9.+-]+;base64,[^)]*$/

const BASE64_PAYLOAD_RE = /^[A-Za-z0-9+/=\s]+$/

export function extractGeneratedImages(content: string): LabImageExtraction {
  if (!content || (!content.includes('![') && !content.includes('`'))) {
    return { images: [], displayContent: content }
  }

  const images: string[] = []
  const textParts: string[] = []
  let cursor = 0

  for (const match of content.matchAll(GENERATED_IMAGE_RE)) {
    const src = match[1] ?? match[2] ?? match[3]
    if (!src) {
      continue
    }
    if (match[1] !== undefined) {
      const payload = src.slice(src.indexOf(',') + 1).trim()
      if (!BASE64_PAYLOAD_RE.test(payload)) {
        continue
      }
    }
    images.push(src)
    textParts.push(content.slice(cursor, match.index))
    cursor = match.index + match[0].length
  }

  let displayContent =
    textParts.length > 0 ? textParts.join('') + content.slice(cursor) : content
  const trailing = TRAILING_INCOMPLETE_IMAGE_RE.exec(displayContent)
  if (trailing && trailing.index !== undefined) {
    displayContent = displayContent.slice(0, trailing.index)
  }

  return { images, displayContent }
}
