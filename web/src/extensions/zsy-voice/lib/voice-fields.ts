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
 * The demographic attributes of a voice — gender, age range and the "适合场景"
 * tag list — mirroring the backend contract (zsy/voice/models.go): the same
 * controlled vocabularies, the same caps, so the dialog refuses exactly what the
 * API would reject.
 */

export interface VoiceOption {
  /** Wire value stored by the API; '' means "not specified". */
  value: string
  /** i18n key the UI renders. */
  labelKey: string
}

/** Values accepted by the `gender` field. */
export const GENDER_OPTIONS: VoiceOption[] = [
  { value: '', labelKey: 'Not specified' },
  { value: 'female', labelKey: 'Female' },
  { value: 'male', labelKey: 'Male' },
  { value: 'neutral', labelKey: 'Neutral' },
]

/** Values accepted by the `ageRange` field. */
export const AGE_RANGE_OPTIONS: VoiceOption[] = [
  { value: '', labelKey: 'Not specified' },
  { value: 'child', labelKey: 'Child' },
  { value: 'teen', labelKey: 'Teen' },
  { value: 'young', labelKey: 'Young' },
  { value: 'middle', labelKey: 'Middle-aged' },
  { value: 'senior', labelKey: 'Senior' },
]

/** Scene list bounds; the labels below carry the numbers so a change stays in one place. */
export const VOICE_SCENES_MAX = 8
export const VOICE_SCENE_MAX_LENGTH = 24
export const VOICE_SCENES_MESSAGE_TOO_MANY = 'At most 8 scenes'
export const VOICE_SCENES_MESSAGE_TOO_LONG =
  'Each scene must be 24 characters or fewer'

/**
 * Language tags offered as filters/suggestions: the ones the Volcengine library
 * uses plus the common ones. The stored value stays free-form — providers spell
 * languages their own way ("mx" for Mexican Spanish) and the API accepts any
 * short tag — so this list only drives the pickers.
 */
export const VOICE_LANGUAGE_CODES = [
  'zh',
  'en',
  'ja',
  'ko',
  'es',
  'fr',
  'de',
  'ru',
  'pt',
  'it',
  'ar',
  'th',
  'vi',
  'id',
  'ms',
  'tl',
  'mx',
  'hi',
  'tr',
  'nl',
]

/** Language tag pattern accepted by the backend (lower-case, may contain - or _). */
export const VOICE_LANGUAGE_PATTERN = /^[a-z][a-z0-9_-]*$/

/** Separators accepted in the scene editor, matching the backend parser. */
const SCENE_SEPARATORS = /[,，、;；|]/

/**
 * Localized name of a language tag. Intl knows every language, so the plaza gets
 * 20 language names in 7 UI languages without a single translation key; a tag
 * Intl does not recognize (Volcengine's "mx") falls back to the raw code.
 */
export function voiceLanguageLabel(code: string, locale?: string): string {
  if (code === '') return ''
  try {
    const display = new Intl.DisplayNames(locale ? [locale] : undefined, {
      type: 'language',
      fallback: 'code',
    })
    return display.of(code) ?? code
  } catch {
    return code
  }
}

/**
 * Label of a stored vocabulary value, or null when nothing is set / the value is
 * unknown (the table then shows a placeholder instead of a raw identifier).
 */
export function voiceOptionLabelKey(
  options: VoiceOption[],
  value: string
): string | null {
  if (value === '') return null
  const option = options.find((item) => item.value === value)
  return option ? option.labelKey : value
}

/** Scenes as the editor shows them: one comma-separated line. */
export function formatSceneList(scenes: string[]): string {
  return scenes.join(', ')
}

/**
 * Parses the editor's line into the API array, dropping blanks and duplicates in
 * the order the operator typed them.
 */
export function parseSceneList(text: string): string[] {
  const seen = new Set<string>()
  const scenes: string[] = []
  for (const part of text.split(SCENE_SEPARATORS)) {
    const scene = part.trim()
    if (scene === '' || seen.has(scene)) continue
    seen.add(scene)
    scenes.push(scene)
  }
  return scenes
}
