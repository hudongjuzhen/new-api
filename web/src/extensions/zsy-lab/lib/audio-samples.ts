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
 * Copy-paste call samples for the audio-generation model (Seed Audio).
 *
 * The samples are plain strings so they can be unit-tested without a DOM, and
 * they are what an integrator copies verbatim — the endpoint, the credential
 * header and the request keys are therefore a contract, not presentation.
 *
 * Shape of the call, mirroring this gateway's adaptor
 * (relay/channel/doubaoaudio):
 *
 *   POST /v1/audio/generations
 *   { "model", "text_prompt", "references", "audio_config" }
 *   → { "code": "success", "data": [{ "audio", "url", "duration",
 *                                    "original_duration" }] }
 *
 * It is synchronous — the audio and its duration arrive in the response — so
 * there is nothing to poll, and the base64 `audio` field is what you play or
 * save. `url` is a temporary upstream link, not a stable one.
 */
import {
  AUDIO_GENERATION_ENDPOINT,
  type AudioGenerationBody,
} from './audio-request'

export type AudioSampleLang = 'curl' | 'python' | 'javascript'

export const AUDIO_SAMPLE_LANGS: AudioSampleLang[] = [
  'curl',
  'python',
  'javascript',
]

export const AUDIO_SAMPLE_LANG_LABELS: Record<AudioSampleLang, string> = {
  curl: 'cURL',
  python: 'Python',
  javascript: 'JavaScript',
}

export const AUDIO_SAMPLE_LANG_HIGHLIGHT: Record<
  AudioSampleLang,
  'bash' | 'python' | 'javascript'
> = {
  curl: 'bash',
  python: 'python',
  javascript: 'javascript',
}

/** Placeholders stay ASCII: they are code, not UI copy. */
export const AUDIO_API_KEY_PLACEHOLDER = 'sk-YOUR_TOKEN'

const SAMPLE_TEXT =
  '欢迎使用音频生成模型，本段音频由文字直接合成，并支持指定语速、音调与输出格式。'

/** The request body the sample sends, identical to the playground's. */
export function buildAudioSampleBody(model: string): AudioGenerationBody {
  return {
    model,
    text_prompt: SAMPLE_TEXT,
    audio_config: {
      format: 'mp3',
      sample_rate: 24000,
      speech_rate: 0,
      loudness_rate: 0,
      pitch_rate: 0,
    },
  }
}

/** Indented JSON body for embedding in Python / JavaScript samples. */
function bodyLiteral(body: AudioGenerationBody, indent: string): string {
  return JSON.stringify(body, null, 2).replaceAll('\n', `\n${indent}`)
}

/** The response shape the sample reads, shown as a comment. */
const RESPONSE_COMMENT =
  '# → {"code":"success","data":[{"audio":"<base64>","url":"https://…","duration":12.5,"original_duration":12.5,"format":"mp3"}]}'

export function buildAudioGenerationSample(
  lang: AudioSampleLang,
  ctx: { baseUrl: string; model: string }
): string {
  const url = `${ctx.baseUrl}${AUDIO_GENERATION_ENDPOINT}`
  const body = buildAudioSampleBody(ctx.model)

  if (lang === 'curl') {
    return [
      `curl ${url} \\`,
      `  -H "Content-Type: application/json" \\`,
      `  -H "Authorization: Bearer ${AUDIO_API_KEY_PLACEHOLDER}" \\`,
      `  -d '${JSON.stringify(body, null, 2)}'`,
      '',
      RESPONSE_COMMENT,
    ].join('\n')
  }

  if (lang === 'python') {
    return [
      'import base64',
      'import requests',
      '',
      `body = ${bodyLiteral(body, '')}`,
      '',
      `response = requests.post(`,
      `    "${url}",`,
      `    headers={"Authorization": "Bearer ${AUDIO_API_KEY_PLACEHOLDER}"},`,
      '    json=body,',
      ')',
      'result = response.json()["data"][0]',
      '',
      '# The audio is returned inline, so there is no task to poll.',
      'with open("output.mp3", "wb") as file:',
      '    file.write(base64.b64decode(result["audio"]))',
      'print(result["original_duration"], "seconds")',
    ].join('\n')
  }

  return [
    `const response = await fetch('${url}', {`,
    `  method: 'POST',`,
    `  headers: {`,
    `    'Content-Type': 'application/json',`,
    `    Authorization: 'Bearer ${AUDIO_API_KEY_PLACEHOLDER}',`,
    `  },`,
    `  body: JSON.stringify(${bodyLiteral(body, '  ')}),`,
    `})`,
    '',
    `const { data } = await response.json()`,
    `const [result] = data`,
    '',
    `// Inline base64: playable straight away, and it cannot expire.`,
    `const audio = new Audio('data:audio/mpeg;base64,' + result.audio)`,
    `await audio.play()`,
    `console.log(result.original_duration, 'seconds')`,
  ].join('\n')
}
