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

import {
  AUDIO_GENERATION_ENDPOINT,
  AUDIO_MAX_AUDIO_REFERENCES,
  buildAudioGenerationBody,
  buildAudioReferences,
  clampLoudnessRate,
  clampPitchRate,
  clampSpeechRate,
  decodeAudioResult,
  defaultSampleRateForFormat,
  normalizeSampleRate,
  sampleRatesForFormat,
  type AudioRunParams,
} from '../lib/audio-request'

function buildParams(overrides: Partial<AudioRunParams> = {}): AudioRunParams {
  return {
    textPrompt: '  欢迎使用音频生成模型。  ',
    format: 'mp3',
    sampleRate: 24000,
    speechRate: 0,
    loudnessRate: 0,
    pitchRate: 0,
    references: [],
    ...overrides,
  }
}

describe('buildAudioGenerationBody', () => {
  // The whole feature broke on sending the wrong body shape to the wrong
  // endpoint, so the field names are the contract under test here.
  it('uses the upstream field names and nests the knobs under audio_config', () => {
    const body = buildAudioGenerationBody('seed-audio-1.0', buildParams())

    expect(body).toEqual({
      model: 'seed-audio-1.0',
      text_prompt: '欢迎使用音频生成模型。',
      audio_config: {
        format: 'mp3',
        sample_rate: 24000,
        speech_rate: 0,
        loudness_rate: 0,
        pitch_rate: 0,
      },
      references: undefined,
    })
    expect(body).not.toHaveProperty('prompt')
    expect(body).not.toHaveProperty('metadata')
    expect(AUDIO_GENERATION_ENDPOINT).toBe('/v1/audio/generations')
  })

  it('clamps the audio_config knobs to their documented ranges', () => {
    const body = buildAudioGenerationBody(
      'seed-audio-1.0',
      buildParams({
        speechRate: 250,
        loudnessRate: -250,
        pitchRate: 40,
        sampleRate: 99999,
      })
    )

    expect(body.audio_config.speech_rate).toBe(100)
    expect(body.audio_config.loudness_rate).toBe(-50)
    expect(body.audio_config.pitch_rate).toBe(12)
    // mp3 does not accept an arbitrary rate; it falls back to its default.
    expect(body.audio_config.sample_rate).toBe(
      defaultSampleRateForFormat('mp3')
    )
  })
})

describe('clamp helpers', () => {
  it('rounds to whole numbers and applies each range', () => {
    expect(clampSpeechRate(-51)).toBe(-50)
    expect(clampLoudnessRate(101)).toBe(100)
    expect(clampPitchRate(-13)).toBe(-12)
  })

  it('treats a non-numeric input as the neutral value', () => {
    expect(clampSpeechRate(Number.NaN)).toBe(0)
    expect(clampPitchRate(Number.NaN)).toBe(0)
  })
})

describe('normalizeSampleRate', () => {
  it('keeps a rate the format accepts', () => {
    expect(normalizeSampleRate('wav', 44100)).toBe(44100)
    expect(normalizeSampleRate('ogg_opus', 48000)).toBe(48000)
  })

  it('falls back to the format default for an unsupported rate', () => {
    // ogg_opus only supports 48000; sending 44100 would fail upstream.
    expect(normalizeSampleRate('ogg_opus', 44100)).toBe(48000)
    expect(normalizeSampleRate('mp3', 40000)).toBe(44100)
  })

  it('lists only rates the format accepts', () => {
    expect(sampleRatesForFormat('ogg_opus')).toEqual([48000])
    expect(sampleRatesForFormat('mp3')).not.toContain(40000)
    expect(sampleRatesForFormat('wav')).toContain(40000)
  })
})

describe('buildAudioReferences', () => {
  it('returns undefined when nothing was attached', () => {
    expect(buildAudioReferences([])).toBeUndefined()
    expect(
      buildAudioReferences([{ type: 'audio_url', url: '  ' }])
    ).toBeUndefined()
  })

  it('maps audio references onto audio_url entries', () => {
    const references = buildAudioReferences([
      { type: 'audio_url', url: 'https://cdn.example.com/a.mp3' },
      { type: 'audio_url', url: 'https://cdn.example.com/b.wav' },
    ])

    expect(references).toEqual([
      { audio_url: 'https://cdn.example.com/a.mp3' },
      { audio_url: 'https://cdn.example.com/b.wav' },
    ])
  })

  it('caps audio references at the upstream limit of three', () => {
    const references = buildAudioReferences(
      Array.from({ length: AUDIO_MAX_AUDIO_REFERENCES + 2 }, (_, index) => ({
        type: 'audio_url' as const,
        url: `https://cdn.example.com/${index}.mp3`,
      }))
    )

    expect(references).toHaveLength(AUDIO_MAX_AUDIO_REFERENCES)
  })

  // The upstream rejects a mixed list, so the stricter image mode wins instead
  // of producing a request that can only fail.
  it('sends the single image reference when both kinds are attached', () => {
    const references = buildAudioReferences([
      { type: 'audio_url', url: 'https://cdn.example.com/a.mp3' },
      { type: 'image_url', url: 'https://cdn.example.com/ref.png' },
    ])

    expect(references).toEqual([
      { image_url: 'https://cdn.example.com/ref.png' },
    ])
  })

  it('keeps only the first image reference', () => {
    const references = buildAudioReferences([
      { type: 'image_url', url: 'https://cdn.example.com/one.png' },
      { type: 'image_url', url: 'https://cdn.example.com/two.png' },
    ])

    expect(references).toEqual([
      { image_url: 'https://cdn.example.com/one.png' },
    ])
  })
})

describe('decodeAudioResult', () => {
  // The response is the only place the audio exists: a synchronous call has no
  // task id to fetch it from later, and the upstream URL expires.
  it('decodes the inline base64 payload into a playable data URI', () => {
    const decoded = decodeAudioResult(
      {
        code: 'success',
        data: [{ audio: 'QUJD', original_duration: 12.5, format: 'mp3' }],
      },
      'mp3'
    )

    expect(decoded.src).toBe('data:audio/mpeg;base64,QUJD')
    expect(decoded.seconds).toBe(12.5)
  })

  it('falls back to the temporary url when no base64 payload is present', () => {
    const decoded = decodeAudioResult(
      { data: [{ url: 'https://example.com/a.mp3', original_duration: 8 }] },
      'wav'
    )

    expect(decoded.src).toBe('https://example.com/a.mp3')
    expect(decoded.seconds).toBe(8)
  })

  it('returns nothing for an empty or failed response', () => {
    expect(decodeAudioResult({}, 'mp3')).toEqual({ src: '', seconds: 0 })
    expect(decodeAudioResult({ data: [] }, 'mp3')).toEqual({
      src: '',
      seconds: 0,
    })
  })
})
