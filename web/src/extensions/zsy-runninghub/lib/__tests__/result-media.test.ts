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
import { describe, expect, test } from 'vitest'

import type { TaskDto } from '../../api'
import {
  extractResults,
  fileNameFromUrl,
  inferResultKind,
  isDownloadOnlyKind,
  resultExtension,
} from '../result-media'

function task(payload: Partial<TaskDto>): TaskDto {
  return {
    id: 1,
    task_id: 'task-1',
    platform: '61',
    group: 'default',
    quota: 0,
    action: 'rh_ai_app',
    status: 'SUCCESS',
    fail_reason: '',
    result_url: '',
    submit_time: 0,
    start_time: 0,
    finish_time: 0,
    progress: '100%',
    username: 'tester',
    ...payload,
  }
}

describe('result classification', () => {
  test.each([
    ['https://files.rh.local/out.png', 'image'],
    ['https://files.rh.local/out.WEBP', 'image'],
    ['https://files.rh.local/clip.mp4', 'video'],
    ['https://files.rh.local/clip.webm?token=1', 'video'],
    ['https://files.rh.local/voice.mp3', 'audio'],
    ['https://files.rh.local/voice.flac', 'audio'],
    ['https://files.rh.local/log.txt', 'text'],
    ['https://files.rh.local/data.json', 'text'],
    ['https://files.rh.local/table.csv', 'text'],
    ['https://files.rh.local/bundle.zip', 'archive'],
    ['https://files.rh.local/bundle.7z', 'archive'],
    ['https://files.rh.local/model.safetensors', 'file'],
    ['https://files.rh.local/download?id=9', 'file'],
  ])('classifies %s as %s', (url, kind) => {
    expect(inferResultKind(url)).toBe(kind)
  })

  test('falls back to the upstream outputType for extension-less URLs', () => {
    expect(
      inferResultKind('https://files.rh.local/download?id=9', 'video')
    ).toBe('video')
    expect(
      inferResultKind('https://files.rh.local/download?id=9', 'AUDIO')
    ).toBe('audio')
    expect(
      inferResultKind('https://files.rh.local/download?id=9', 'nonsense')
    ).toBe('file')
  })

  test('keeps the URL extension authoritative over a wrong hint', () => {
    expect(inferResultKind('https://files.rh.local/out.png', 'text')).toBe(
      'image'
    )
  })

  test('only archives and unknown binaries are download-only', () => {
    expect(isDownloadOnlyKind('archive')).toBe(true)
    expect(isDownloadOnlyKind('file')).toBe(true)
    expect(isDownloadOnlyKind('image')).toBe(false)
    expect(isDownloadOnlyKind('video')).toBe(false)
    expect(isDownloadOnlyKind('audio')).toBe(false)
    expect(isDownloadOnlyKind('text')).toBe(false)
  })
})

describe('result URL helpers', () => {
  test('reads the extension through query strings and fragments', () => {
    expect(resultExtension('https://files.rh.local/out.png?a=1')).toBe('png')
    expect(resultExtension('https://files.rh.local/out.png#preview')).toBe(
      'png'
    )
    expect(resultExtension('https://files.rh.local/download?id=9')).toBe('')
  })

  test('derives a decoded file name and tolerates nameless URLs', () => {
    expect(
      fileNameFromUrl('https://files.rh.local/a/%E4%B8%AD%E6%96%87.zip')
    ).toBe('中文.zip')
    expect(fileNameFromUrl('https://files.rh.local/a/out.png?token=1')).toBe(
      'out.png'
    )
    expect(fileNameFromUrl('https://files.rh.local/')).toBe('')
  })
})

describe('result extraction', () => {
  test('extracts every kind in upstream order and dedupes repeated URLs', () => {
    const results = extractResults(
      task({
        data: {
          results: [
            {
              url: 'https://files.rh.local/a.png',
              nodeId: '9',
              outputType: 'image',
            },
            { url: 'https://files.rh.local/a.png' },
            { url: 'https://files.rh.local/b.mp4' },
            { value: 'https://files.rh.local/c.zip' },
          ],
        },
      })
    )

    expect(results.map((item) => item.kind)).toEqual([
      'image',
      'video',
      'archive',
    ])
    expect(results[0].nodeId).toBe('9')
    expect(results[2].fileName).toBe('c.zip')
  })

  test('accepts a bare results array', () => {
    const results = extractResults(
      task({ data: [{ url: 'https://files.rh.local/voice.wav' }] })
    )
    expect(results).toHaveLength(1)
    expect(results[0].kind).toBe('audio')
  })

  test('keeps text-only results so their content can be shown', () => {
    const results = extractResults(
      task({ data: { results: [{ text: 'caption' }] } })
    )
    expect(results).toEqual([
      expect.objectContaining({ kind: 'text', text: 'caption', fileName: '' }),
    ])
  })

  test('never renders a non-http value as a result', () => {
    const results = extractResults(
      task({
        data: {
          results: [
            { url: '/任务超时（1440分钟）' },
            { url: 'javascript:alert(1)' },
            { text: '   ' },
          ],
        },
      })
    )
    expect(results).toEqual([])
  })

  test('falls back to the task result_url when no results were stored', () => {
    const results = extractResults(
      task({ result_url: 'https://files.rh.local/only.png' })
    )
    expect(results).toHaveLength(1)
    expect(results[0].kind).toBe('image')
  })

  test('returns nothing for a task without results', () => {
    expect(extractResults(task({}))).toEqual([])
  })
})
