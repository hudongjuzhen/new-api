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

import {
  formatVoiceAudioSize,
  validateVoiceAudioFile,
  VOICE_AUDIO_MAX_BYTES,
  VOICE_AUDIO_TOO_LARGE_MESSAGE,
  VOICE_AUDIO_UNSUPPORTED_MESSAGE,
} from '../voice-audio'

describe('validateVoiceAudioFile', () => {
  test('accepts every format the backend allow-list covers', () => {
    const accepted = [
      'sample.mp3',
      'sample.wav',
      'sample.m4a',
      'sample.aac',
      'sample.ogg',
      'sample.opus',
      'sample.flac',
      'sample.webm',
    ]

    for (const name of accepted) {
      expect(validateVoiceAudioFile({ name, size: 1024 })).toBeNull()
    }
  })

  test('matches the extension case-insensitively', () => {
    expect(
      validateVoiceAudioFile({ name: 'Sample.MP3', size: 1024 })
    ).toBeNull()
  })

  test('accepts a name without an extension so the backend can sniff it', () => {
    expect(validateVoiceAudioFile({ name: 'blob', size: 1024 })).toBeNull()
  })

  test('rejects an unsupported extension', () => {
    expect(validateVoiceAudioFile({ name: 'notes.txt', size: 10 })).toBe(
      VOICE_AUDIO_UNSUPPORTED_MESSAGE
    )
    expect(validateVoiceAudioFile({ name: 'clip.mp4', size: 10 })).toBe(
      VOICE_AUDIO_UNSUPPORTED_MESSAGE
    )
  })

  test('rejects a file above the 20MB cap and accepts one exactly at it', () => {
    expect(
      validateVoiceAudioFile({
        name: 'big.mp3',
        size: VOICE_AUDIO_MAX_BYTES + 1,
      })
    ).toBe(VOICE_AUDIO_TOO_LARGE_MESSAGE)
    expect(
      validateVoiceAudioFile({ name: 'big.mp3', size: VOICE_AUDIO_MAX_BYTES })
    ).toBeNull()
  })

  test('reports the format problem before the size problem', () => {
    expect(
      validateVoiceAudioFile({
        name: 'notes.txt',
        size: VOICE_AUDIO_MAX_BYTES * 2,
      })
    ).toBe(VOICE_AUDIO_UNSUPPORTED_MESSAGE)
  })
})

describe('formatVoiceAudioSize', () => {
  test('renders bytes, kilobytes, megabytes and gigabytes', () => {
    expect(formatVoiceAudioSize(0)).toBe('0 B')
    expect(formatVoiceAudioSize(512)).toBe('512 B')
    expect(formatVoiceAudioSize(2048)).toBe('2.0 KB')
    expect(formatVoiceAudioSize(15 * 1024)).toBe('15 KB')
    expect(formatVoiceAudioSize(1.5 * 1024 * 1024)).toBe('1.5 MB')
    expect(formatVoiceAudioSize(1024 * 1024 * 1024)).toBe('1.0 GB')
  })

  test('falls back to 0 B for a missing or invalid size', () => {
    expect(formatVoiceAudioSize(-1)).toBe('0 B')
    expect(formatVoiceAudioSize(Number.NaN)).toBe('0 B')
    expect(formatVoiceAudioSize(Number.POSITIVE_INFINITY)).toBe('0 B')
  })
})
