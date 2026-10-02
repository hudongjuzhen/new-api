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
 * Request builders for the lab audio playground (Seed Audio / seed-audio-1.0).
 *
 * Contract, taken from the upstream audio-generation reference and implemented by
 * this gateway's adaptor (relay/channel/doubaoaudio):
 *
 *   POST /v1/audio/generations
 *   { "model", "text_prompt", "references": [...], "audio_config": {...} }
 *   → { "code": "success", "data": [{ "audio", "url", "duration",
 *                                    "original_duration", "format" }] }
 *
 * It is a **synchronous** call: the audio comes back in the response, so there is
 * no task to submit or poll. `audio` is base64 of the same clip `url` points at,
 * and `url` is a temporary upstream link with a short lifetime — which is why the
 * playground plays the base64 payload rather than the URL.
 *
 * The endpoint is NOT the Ark task API: seed-audio-1.0 is a 豆包语音 model, and
 * posting it to Ark answers InvalidEndpointOrModel.NotFound.
 *
 * Every bound below is also enforced server-side; the playground keeps them so an
 * obviously invalid request never leaves the browser.
 */

export type AudioOutputFormat = 'wav' | 'mp3' | 'pcm' | 'ogg_opus'

/** Gateway endpoint for synchronous audio generation. */
export const AUDIO_GENERATION_ENDPOINT = '/v1/audio/generations'

export const AUDIO_OUTPUT_FORMATS: AudioOutputFormat[] = [
  'wav',
  'mp3',
  'pcm',
  'ogg_opus',
]

/** Text prompt ceiling accepted by the upstream model (characters). */
export const AUDIO_MAX_PROMPT_CHARS = 3000

/** Reference limits from the upstream parameter reference. */
export const AUDIO_MAX_AUDIO_REFERENCES = 3
export const AUDIO_MAX_IMAGE_REFERENCES = 1

/**
 * Sample rates the upstream accepts per output format. `ogg_opus` only accepts
 * 48000, so the format and the rate cannot be chosen independently.
 */
const SAMPLE_RATES_BY_FORMAT: Record<AudioOutputFormat, number[]> = {
  wav: [8000, 16000, 24000, 32000, 40000, 44100, 48000],
  pcm: [8000, 16000, 24000, 32000, 40000, 44100, 48000],
  mp3: [8000, 16000, 24000, 32000, 44100, 48000],
  ogg_opus: [48000],
}

/** Default sample rate per format, matching the upstream defaults. */
const DEFAULT_SAMPLE_RATE_BY_FORMAT: Record<AudioOutputFormat, number> = {
  wav: 40000,
  pcm: 40000,
  mp3: 44100,
  ogg_opus: 48000,
}

/** Ranges of the audio_config knobs, shared by the form and its clamping. */
const SPEECH_RATE_RANGE = { min: -50, max: 100 }
const LOUDNESS_RATE_RANGE = { min: -50, max: 100 }
const PITCH_RATE_RANGE = { min: -12, max: 12 }

/** MIME type the browser needs to decode each output format. */
export const AUDIO_MIME_BY_FORMAT: Record<AudioOutputFormat, string> = {
  wav: 'audio/wav',
  mp3: 'audio/mpeg',
  pcm: 'audio/wav',
  ogg_opus: 'audio/ogg',
}

export interface AudioReference {
  type: 'audio_url' | 'image_url'
  url: string
}

export interface AudioRunParams {
  textPrompt: string
  format: AudioOutputFormat
  sampleRate: number
  speechRate: number
  loudnessRate: number
  pitchRate: number
  references: AudioReference[]
}

export interface AudioGenerationBody {
  model: string
  text_prompt: string
  audio_config: {
    format: AudioOutputFormat
    sample_rate: number
    speech_rate: number
    loudness_rate: number
    pitch_rate: number
  }
  references?: Array<Record<string, string>>
}

export function sampleRatesForFormat(format: AudioOutputFormat): number[] {
  return SAMPLE_RATES_BY_FORMAT[format]
}

export function defaultSampleRateForFormat(format: AudioOutputFormat): number {
  return DEFAULT_SAMPLE_RATE_BY_FORMAT[format]
}

/**
 * Snap a sample rate onto one the format accepts. Changing the output format can
 * otherwise leave a rate the upstream rejects (for example 44100 with ogg_opus),
 * which would fail the whole request instead of degrading gracefully.
 */
export function normalizeSampleRate(
  format: AudioOutputFormat,
  sampleRate: number
): number {
  const allowed = SAMPLE_RATES_BY_FORMAT[format]
  if (allowed.includes(sampleRate)) return sampleRate
  return DEFAULT_SAMPLE_RATE_BY_FORMAT[format]
}

function clampInt(value: number, min: number, max: number, fallback = 0): number {
  if (!Number.isFinite(value)) return fallback
  return Math.min(Math.max(Math.round(value), min), max)
}

export function clampSpeechRate(value: number): number {
  return clampInt(value, SPEECH_RATE_RANGE.min, SPEECH_RATE_RANGE.max)
}

export function clampLoudnessRate(value: number): number {
  return clampInt(value, LOUDNESS_RATE_RANGE.min, LOUDNESS_RATE_RANGE.max)
}

export function clampPitchRate(value: number): number {
  return clampInt(value, PITCH_RATE_RANGE.min, PITCH_RATE_RANGE.max)
}

/**
 * The `references` list, or undefined when nothing was attached.
 *
 * Upstream accepts at most three audio references or one image reference and
 * forbids mixing the two kinds, so a mixed list resolves to the stricter image
 * mode instead of producing a request that can only fail.
 */
export function buildAudioReferences(
  references: AudioReference[]
): Array<Record<string, string>> | undefined {
  const audio: Array<Record<string, string>> = []
  let image: Record<string, string> | undefined
  for (const reference of references) {
    const url = reference.url.trim()
    if (!url) continue
    if (reference.type === 'image_url') {
      if (!image) image = { image_url: url }
      continue
    }
    if (audio.length < AUDIO_MAX_AUDIO_REFERENCES) {
      audio.push({ audio_url: url })
    }
  }
  if (image) return [image]
  return audio.length > 0 ? audio : undefined
}

/**
 * Build the gateway request body. Knobs are always sent because the upstream
 * reference defines a default for each and the adaptor forwards them verbatim.
 */
export function buildAudioGenerationBody(
  model: string,
  params: AudioRunParams
): AudioGenerationBody {
  const format = params.format
  return {
    model,
    text_prompt: params.textPrompt.trim(),
    audio_config: {
      format,
      sample_rate: normalizeSampleRate(format, params.sampleRate),
      speech_rate: clampSpeechRate(params.speechRate),
      loudness_rate: clampLoudnessRate(params.loudnessRate),
      pitch_rate: clampPitchRate(params.pitchRate),
    },
    references: buildAudioReferences(params.references),
  }
}

/** One result entry of the gateway response. */
export interface AudioGenerationResult {
  audio?: string
  url?: string
  duration?: number
  original_duration?: number
  format?: string
}

export interface AudioGenerationResponse {
  code?: string
  message?: string
  data?: AudioGenerationResult[]
}

/**
 * Decode the first result into a playable source.
 *
 * The returned source is the base64 payload as a data URI: it cannot expire,
 * unlike the upstream `url` (valid for two hours), and unlike a gateway task
 * proxy path — which does not exist for a synchronous call.
 */
export function decodeAudioResult(
  response: AudioGenerationResponse,
  format: AudioOutputFormat
): { src: string; seconds: number } {
  const result = response.data?.[0]
  if (!result) return { src: '', seconds: 0 }
  const mime = AUDIO_MIME_BY_FORMAT[format] ?? 'audio/wav'
  const src = result.audio
    ? `data:${mime};base64,${result.audio}`
    : (result.url ?? '')
  return { src, seconds: result.original_duration ?? 0 }
}
