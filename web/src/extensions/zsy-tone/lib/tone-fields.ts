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
 * The Tone Plaza vocabulary — a **mirror** of the published tone standard, not
 * its source of truth.
 *
 * # Where the authority lives
 *
 * The backend serves `GET /api/zsy/tone/standard` (`zsy/tone/standard.go`): the
 * categories, the tones, the field caps and the compatibility promise. That
 * response is what the page renders from — its option lists drive the filters
 * and the form, and its `limits` drive validation. Everything in this file is
 * the **fallback** used while that request is in flight and whenever it fails,
 * so the plaza still works against a backend that does not serve the endpoint
 * yet (an older deploy, a proxy that blocks it).
 *
 * `normalizeToneStandard()` is the seam: it folds a wire payload onto this
 * mirror, so a partial or malformed response degrades field by field instead of
 * blanking the page.
 *
 * # Why the mirror must not drift into a second authority
 *
 * The temptation is to treat the constants below as "the vocabulary" and let the
 * endpoint be decoration. Then two copies of one fact exist and nothing keeps
 * them equal, which is a failure this project has already paid for once: the
 * audio-plaza consumer noted two endpoints that share one batch of controlled
 * vocabulary while **each hardcoded its own label table**, and the only thing
 * holding them together was a test asserting the two tables were literally the
 * same object (see `D:\MV项目\aimv-studio\docs\15-形象广场.md` §3.3 — the symptom
 * recorded there is one card reading `young` while the other reads 青年, with
 * neither side looking broken).
 *
 * So the rules here are:
 *  1. A value's **existence** comes from the server when it answers; the mirror
 *     only supplies values the server did not mention (notably the empty
 *     "unspecified" option, which the standard need not list).
 *  2. A value's **label** is resolved through an i18n key we ship in all seven
 *     locales, keyed by the mirrored value — never by string-matching a server
 *     label. A value the mirror has never seen falls back to the server's own
 *     `labelEn` / `label`, so a newly added category shows real text rather than
 *     a raw identifier or a missing-key artefact.
 *  3. A **cap** is taken from `limits`; the numbers below are defaults, not a
 *     second set of magic numbers to edit alongside the backend.
 */

/** One selectable value of a controlled vocabulary. */
export interface ToneStandardOption {
  /** Wire value stored by the API; '' means "unspecified" and is always offered. */
  value: string
  /**
   * i18n key of the label. Empty for a value only the server knows about — those
   * render from `labelEn` / `label` instead (see `toneOptionText`).
   */
  labelKey: string
  /** Server's default-language label; '' for a mirrored value. */
  label: string
  /** Server's English label; '' for a mirrored value. */
  labelEn: string
  /** Server's one-line explanation; '' when it sent none. */
  desc: string
}

/** Field caps enforced by the backend, and therefore by the form. */
export interface ToneLimits {
  name: number
  description: number
  prompt: number
  sample: number
  language: number
  scenes: number
  sceneLength: number
  sortOrderAbs: number
  maxPageSize: number
}

/** What the standard says about the before/after pair, when it says anything. */
export interface ToneExamplePair {
  field: string
  convention: string
  whyItMatters: string
}

export interface ToneStandard {
  version: string
  categories: ToneStandardOption[]
  tones: ToneStandardOption[]
  limits: ToneLimits
  compatibility: string[]
  examplePair: ToneExamplePair | null
  /** `server` when the endpoint answered, `mirror` when this file is all we have. */
  source: 'server' | 'mirror'
}

/**
 * A mirrored option: the English phrase doubles as its i18n key and as the label
 * of last resort. The mirror carries no server prose, so `label` / `labelEn`
 * repeat the phrase only so a value that loses its `labelKey` still renders
 * something readable.
 */
function mirror(value: string, labelKey: string): ToneStandardOption {
  return { value, labelKey, label: labelKey, labelEn: labelKey, desc: '' }
}

/** Values accepted by the `category` field. */
export const TONE_CATEGORY_OPTIONS: ToneStandardOption[] = [
  mirror('', 'Uncategorized'),
  mirror('literary', 'Literary'),
  mirror('business', 'Business'),
  mirror('academic', 'Academic'),
  mirror('media', 'Media'),
  mirror('spoken', 'Spoken'),
  mirror('technical', 'Technical'),
  mirror('marketing', 'Marketing'),
]

/**
 * Values accepted by the `tone` field — the flavour axis of a tone row (a 文风's
 * 语气), as opposed to the row itself, which is also called a "tone".
 */
export const TONE_STYLE_OPTIONS: ToneStandardOption[] = [
  mirror('', 'Not specified'),
  mirror('warm', 'Warm'),
  mirror('calm', 'Calm'),
  mirror('sharp', 'Sharp'),
  mirror('humorous', 'Humorous'),
  mirror('solemn', 'Solemn'),
  mirror('lively', 'Lively'),
  mirror('plain', 'Plain'),
]

/** Version reported when the standard endpoint cannot be reached. */
export const TONE_STANDARD_VERSION_FALLBACK = '1.0.0'

/** Caps reported when the standard endpoint cannot be reached. */
export const TONE_LIMITS_FALLBACK: ToneLimits = {
  name: 191,
  description: 2000,
  prompt: 8000,
  sample: 4000,
  language: 16,
  scenes: 8,
  sceneLength: 24,
  sortOrderAbs: 1_000_000,
  maxPageSize: 100,
}

/** The mirror as a complete standard, so callers never have to branch on null. */
export const TONE_STANDARD_FALLBACK: ToneStandard = {
  version: TONE_STANDARD_VERSION_FALLBACK,
  categories: TONE_CATEGORY_OPTIONS,
  tones: TONE_STYLE_OPTIONS,
  limits: TONE_LIMITS_FALLBACK,
  compatibility: [],
  examplePair: null,
  source: 'mirror',
}

/** Language tags offered as filters: the common ones, plus what the voice side uses. */
export const TONE_LANGUAGE_CODES = [
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
  'hi',
  'tr',
  'nl',
]

/** Language tag pattern accepted by the backend (lower-case, may contain - or _). */
export const TONE_LANGUAGE_PATTERN = /^[a-z][a-z0-9_-]*$/

/** Separators accepted in the scene editor, matching the backend parser. */
const SCENE_SEPARATORS = /[,，、;；|]/

/** A trimmed non-empty string, or null when the value is not usable text. */
function stringOf(value: unknown): string | null {
  if (typeof value !== 'string') return null
  const text = value.trim()
  return text === '' ? null : text
}

/**
 * A JSON object — which an array is not.
 *
 * `typeof [] === 'object'` is a real trap on this path: an endpoint answering
 * `[]` (or a proxy wrapping the body in an array) would otherwise sail past the
 * "did we get a payload" check and be reported as a server answer carrying
 * nothing, instead of as the mirror. The distinction is user-visible: it decides
 * whether the version chip claims the live standard or the built-in copy.
 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/** A positive whole number, or `fallback` when the value is missing or unusable. */
function positiveInt(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback
  return Math.trunc(parsed)
}

/** The standard payload, with the host's `{ success, data }` envelope peeled off. */
function unwrapEnvelope(raw: unknown): unknown {
  if (!isRecord(raw)) return raw
  const data = raw.data
  return isRecord(data) ? data : raw
}

/**
 * One axis of the vocabulary, server entries first.
 *
 * The empty value is always present: it is how the UI clears a filter and how a
 * form says "no category", and the standard is not required to list it.
 */
function normalizeAxis(
  raw: unknown,
  fallback: ToneStandardOption[]
): ToneStandardOption[] {
  const list = Array.isArray(raw) ? raw : []
  const options: ToneStandardOption[] = []
  for (const item of list) {
    if (!isRecord(item)) continue
    const entry = item
    const value = typeof entry.value === 'string' ? entry.value.trim() : null
    // '' is a real value only when it is the "unspecified" entry, so it is not
    // rejected here; a non-string value is.
    if (value === null) continue
    if (options.some((option) => option.value === value)) continue
    const known = fallback.find((option) => option.value === value)
    options.push({
      value,
      // Only mirrored values have a translation we shipped; an unknown value is
      // rendered from the server's own label instead.
      labelKey: known ? known.labelKey : '',
      label: stringOf(entry.label) ?? '',
      labelEn: stringOf(entry.labelEn) ?? '',
      desc: stringOf(entry.desc) ?? '',
    })
  }
  // An answer that carries no usable option is no better than no answer.
  if (options.length === 0) return fallback
  const unspecified = fallback.find((option) => option.value === '')
  if (unspecified && !options.some((option) => option.value === '')) {
    options.unshift(unspecified)
  }
  return options
}

function normalizeLimits(raw: unknown): ToneLimits {
  const source = isRecord(raw) ? raw : {}
  const limits: ToneLimits = { ...TONE_LIMITS_FALLBACK }
  for (const key of Object.keys(TONE_LIMITS_FALLBACK) as (keyof ToneLimits)[]) {
    limits[key] = positiveInt(source[key], TONE_LIMITS_FALLBACK[key])
  }
  return limits
}

function normalizeExamplePair(raw: unknown): ToneExamplePair | null {
  if (!isRecord(raw)) return null
  const source = raw
  const pair: ToneExamplePair = {
    field: stringOf(source.field) ?? '',
    convention: stringOf(source.convention) ?? '',
    whyItMatters: stringOf(source.whyItMatters) ?? '',
  }
  if (!pair.field && !pair.convention && !pair.whyItMatters) return null
  return pair
}

/**
 * Folds a `/api/zsy/tone/standard` payload onto the mirror.
 *
 * Every field is resolved independently, so a response that carries the
 * vocabulary but not the limits (or vice versa) still improves on the mirror
 * rather than being discarded wholesale. A payload that is not an object at all
 * yields the mirror with `source: 'mirror'`.
 */
export function normalizeToneStandard(raw: unknown): ToneStandard {
  const body = unwrapEnvelope(raw)
  if (!isRecord(body)) return TONE_STANDARD_FALLBACK
  const source = body
  const compatibility = Array.isArray(source.compatibility)
    ? source.compatibility
        .map((item) => stringOf(item))
        .filter((item): item is string => item !== null)
    : []
  return {
    version: stringOf(source.version) ?? TONE_STANDARD_VERSION_FALLBACK,
    categories: normalizeAxis(source.categories, TONE_CATEGORY_OPTIONS),
    tones: normalizeAxis(source.tones, TONE_STYLE_OPTIONS),
    limits: normalizeLimits(source.limits),
    compatibility,
    examplePair: normalizeExamplePair(source.examplePair),
    source: 'server',
  }
}

/** The option stored for `value`, or null when nothing is set or it is unknown. */
export function toneOptionOf(
  options: ToneStandardOption[],
  value: string
): ToneStandardOption | null {
  if (value === '') return null
  return options.find((option) => option.value === value) ?? null
}

/**
 * Display text of one option: our translation when we ship one, otherwise
 * whatever the server called it. Never a bare identifier, and never a
 * missing-key artefact, because an unknown value resolves to its server label.
 */
export function toneOptionText(
  option: ToneStandardOption | null,
  t: (key: string) => string
): string {
  if (!option) return ''
  if (option.labelKey) return t(option.labelKey)
  return option.labelEn || option.label || option.value
}

/**
 * Display text of a stored value: '' when nothing is set, the translated option
 * when the vocabulary knows it, and the raw value for an identifier the UI has
 * never seen (showing it beats showing nothing — the operator can still tell two
 * rows apart).
 */
export function toneValueText(
  options: ToneStandardOption[],
  value: string,
  t: (key: string) => string
): string {
  if (value === '') return ''
  return toneOptionText(toneOptionOf(options, value), t) || value
}

/**
 * Localized name of a language tag. Intl knows every language, so the plaza gets
 * 20 language names in 7 UI languages without a single translation key; a tag
 * Intl does not recognize falls back to the raw code.
 */
export function toneLanguageLabel(code: string, locale?: string): string {
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
