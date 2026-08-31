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
import { describe, expect, it } from 'vitest'

import { extractGeneratedImages } from '../lib/image'

const PNG_URI = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=='
const WEBP_URI = 'data:image/webp;base64,UklGRhIA'

describe('extractGeneratedImages', () => {
  it('returns content untouched when it has no image markdown', () => {
    const content = '{"skipped_mainline":true}\nplain text'
    expect(extractGeneratedImages(content)).toEqual({
      images: [],
      displayContent: content,
    })
  })

  it('extracts a complete data-URI image and strips it from the text', () => {
    const content = `{"skipped_mainline":true}\n![image_1](${PNG_URI})`
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([PNG_URI])
    expect(result.displayContent).toBe('{"skipped_mainline":true}\n')
  })

  it('keeps surrounding text and handles multiple images', () => {
    const content = `intro ![a](${PNG_URI}) middle ![b](${WEBP_URI}) tail`
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([PNG_URI, WEBP_URI])
    expect(result.displayContent).toBe('intro  middle  tail')
  })

  it('extracts an inline-code remote image URL', () => {
    const url = 'https://oss.filenest.top/uploads/a4df0b2d-5df5.png'
    const content = `> ✅ 绘图已完成\n\n\`${url}\`\n\n`
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([url])
    expect(result.displayContent).toBe('> ✅ 绘图已完成\n\n\n\n')
  })

  it('extracts a markdown image with a remote URL', () => {
    const url = 'https://cdn.example.com/out.webp?sign=abc'
    const content = `![image_1](${url})`
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([url])
    expect(result.displayContent).toBe('')
  })

  it('keeps inline code and links without an image extension as text', () => {
    const content = '`npm run build` [docs](https://example.com/page)'
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([])
    expect(result.displayContent).toBe(content)
  })

  it('trims a trailing incomplete image payload while streaming', () => {
    const content = `![image_1](${PNG_URI})\n![image_2](data:image/png;base64,AAAA`
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([PNG_URI])
    expect(result.displayContent).toBe('\n')
  })

  it('leaves invalid base64 payloads as plain text', () => {
    const content = '![x](data:image/png;base64,not$valid!)'
    const result = extractGeneratedImages(content)
    expect(result.images).toEqual([])
    expect(result.displayContent).toBe(content)
  })

  it('handles oversized payloads beyond the playground 30k limit', () => {
    const bigPayload = 'A'.repeat(200_000)
    const content = `![image_1](data:image/png;base64,${bigPayload})`
    const result = extractGeneratedImages(content)
    expect(result.images).toHaveLength(1)
    expect(result.displayContent).toBe('')
  })
})
