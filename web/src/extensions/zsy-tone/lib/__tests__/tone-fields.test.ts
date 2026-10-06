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
  formatSceneList,
  normalizeToneStandard,
  parseSceneList,
  TONE_CATEGORY_OPTIONS,
  TONE_LANGUAGE_CODES,
  TONE_LANGUAGE_PATTERN,
  TONE_LIMITS_FALLBACK,
  TONE_STANDARD_FALLBACK,
  TONE_STANDARD_VERSION_FALLBACK,
  TONE_STYLE_OPTIONS,
  toneLanguageLabel,
  toneOptionOf,
  toneOptionText,
  toneValueText,
} from '../tone-fields'
import { createToneFormSchema, EMPTY_TONE_FORM, hasToneSample } from '../tone-form-schema'

/** The UI renders keys through t(); an identity map keeps the contract explicit. */
const t = (key: string) => key

describe('the mirrored vocabulary', () => {
  test('offers the values the standard publishes, with "" meaning unspecified', () => {
    expect(TONE_CATEGORY_OPTIONS.map((option) => option.value)).toEqual([
      '',
      'literary',
      'business',
      'academic',
      'media',
      'spoken',
      'technical',
      'marketing',
    ])
    expect(TONE_STYLE_OPTIONS.map((option) => option.value)).toEqual([
      '',
      'warm',
      'calm',
      'sharp',
      'humorous',
      'solemn',
      'lively',
      'plain',
    ])
  })

  test('mirrors the caps the backend publishes', () => {
    expect(TONE_LIMITS_FALLBACK).toEqual({
      name: 191,
      description: 2000,
      prompt: 8000,
      sample: 4000,
      language: 16,
      scenes: 8,
      sceneLength: 24,
      sortOrderAbs: 1_000_000,
      maxPageSize: 100,
    })
  })
})

describe('normalizeToneStandard', () => {
  test('takes the vocabulary and the caps from the endpoint', () => {
    const standard = normalizeToneStandard({
      version: '2.1.0',
      categories: [
        {
          value: 'literary',
          label: '文学',
          labelEn: 'Literary',
          desc: '散文与小说',
        },
      ],
      tones: [{ value: 'warm', label: '温暖', labelEn: 'Warm', desc: '柔和' }],
      limits: { name: 120, prompt: 4000 },
      compatibility: ['已发布的取值不会改名'],
      examplePair: {
        field: 'sampleInput / sampleOutput',
        convention: '同一句话的改写',
        whyItMatters: '文风看不见也听不见',
      },
    })

    expect(standard.source).toBe('server')
    expect(standard.version).toBe('2.1.0')
    expect(standard.limits.name).toBe(120)
    expect(standard.limits.prompt).toBe(4000)
    // Caps the response did not mention keep the mirror's value.
    expect(standard.limits.sample).toBe(TONE_LIMITS_FALLBACK.sample)
    expect(standard.compatibility).toEqual(['已发布的取值不会改名'])
    expect(standard.examplePair?.whyItMatters).toBe('文风看不见也听不见')
    expect(standard.categories.map((option) => option.value)).toEqual([
      '',
      'literary',
    ])
    expect(standard.tones.map((option) => option.value)).toEqual(['', 'warm'])
  })

  test('unwraps the host { success, data } envelope', () => {
    const standard = normalizeToneStandard({
      success: true,
      data: { version: '3.0.0', categories: [{ value: 'media' }] },
    })
    expect(standard.source).toBe('server')
    expect(standard.version).toBe('3.0.0')
    expect(standard.categories.map((option) => option.value)).toEqual([
      '',
      'media',
    ])
  })

  test('keeps a value the mirror does not know, with the label the server gave it', () => {
    const standard = normalizeToneStandard({
      categories: [{ value: 'poetic', label: '诗意', labelEn: 'Poetic' }],
    })
    const option = standard.categories.find((item) => item.value === 'poetic')

    // No translation is shipped for a value we have never seen, so the label
    // must come from the response rather than turning into a missing-key string.
    expect(option?.labelKey).toBe('')
    expect(toneOptionText(option ?? null, t)).toBe('Poetic')
  })

  test('translated labels come from the mirror, not from the server prose', () => {
    const standard = normalizeToneStandard({
      categories: [{ value: 'literary', label: '文学', labelEn: 'Literary' }],
    })
    const option = standard.categories.find((item) => item.value === 'literary')
    // labelKey is what makes the options localizable in all seven UI languages.
    expect(option?.labelKey).toBe('Literary')
    // The server's own prose is still carried, for the option tooltip.
    expect(option?.desc).toBe('')
    expect(option?.label).toBe('文学')
  })

  test('always offers the unspecified option, even when the standard omits it', () => {
    const standard = normalizeToneStandard({
      tones: [{ value: 'warm' }],
    })
    expect(standard.tones[0].value).toBe('')
  })

  test('does not duplicate the unspecified option when the standard lists it', () => {
    const standard = normalizeToneStandard({
      tones: [{ value: '' }, { value: 'warm' }],
    })
    expect(standard.tones.map((option) => option.value)).toEqual(['', 'warm'])
  })

  test('an empty or unusable vocabulary keeps the mirror instead of blanking the UI', () => {
    expect(normalizeToneStandard({ categories: [] }).categories).toBe(
      TONE_CATEGORY_OPTIONS
    )
    expect(normalizeToneStandard({ categories: 'nope' }).categories).toBe(
      TONE_CATEGORY_OPTIONS
    )
    expect(
      normalizeToneStandard({ categories: [{ label: 'no value' }] }).categories
    ).toBe(TONE_CATEGORY_OPTIONS)
  })

  test('drops junk limits and duplicate options rather than trusting them', () => {
    const standard = normalizeToneStandard({
      tones: [{ value: 'warm' }, { value: 'warm' }, { value: 'calm' }],
      limits: { name: 0, prompt: -5, sample: 'abc', language: 12.9 },
    })
    expect(standard.tones.map((option) => option.value)).toEqual([
      '',
      'warm',
      'calm',
    ])
    expect(standard.limits.name).toBe(TONE_LIMITS_FALLBACK.name)
    expect(standard.limits.prompt).toBe(TONE_LIMITS_FALLBACK.prompt)
    expect(standard.limits.sample).toBe(TONE_LIMITS_FALLBACK.sample)
    // A fractional cap is truncated, not rejected.
    expect(standard.limits.language).toBe(12)
  })

  test('a payload that is not an object yields the mirror', () => {
    for (const raw of [undefined, null, '', 42, []]) {
      const standard = normalizeToneStandard(raw)
      expect(standard.source).toBe('mirror')
      expect(standard.version).toBe(TONE_STANDARD_VERSION_FALLBACK)
      expect(standard.categories).toBe(TONE_CATEGORY_OPTIONS)
    }
  })

  test('the mirror is a complete standard, so callers never branch on null', () => {
    expect(TONE_STANDARD_FALLBACK.source).toBe('mirror')
    expect(TONE_STANDARD_FALLBACK.limits).toBe(TONE_LIMITS_FALLBACK)
    expect(TONE_STANDARD_FALLBACK.compatibility).toEqual([])
    expect(TONE_STANDARD_FALLBACK.examplePair).toBeNull()
  })

  test('an empty examplePair is dropped rather than rendered blank', () => {
    expect(normalizeToneStandard({ examplePair: {} }).examplePair).toBeNull()
    expect(
      normalizeToneStandard({ examplePair: { convention: '  ' } }).examplePair
    ).toBeNull()
  })
})

describe('toneValueText', () => {
  const t2 = (key: string) => `[${key}]`

  test('translates a value the vocabulary knows', () => {
    expect(toneValueText(TONE_CATEGORY_OPTIONS, 'literary', t2)).toBe(
      '[Literary]'
    )
    expect(toneValueText(TONE_STYLE_OPTIONS, 'warm', t2)).toBe('[Warm]')
  })

  test('shows nothing when no value is set', () => {
    expect(toneValueText(TONE_CATEGORY_OPTIONS, '', t2)).toBe('')
  })

  test('shows the raw identifier for a value the vocabulary does not know', () => {
    // Showing it beats showing nothing: the operator can still tell rows apart.
    expect(toneValueText(TONE_CATEGORY_OPTIONS, 'poetic', t2)).toBe('poetic')
  })

  test('toneOptionOf returns null for the unspecified value', () => {
    expect(toneOptionOf(TONE_CATEGORY_OPTIONS, '')).toBeNull()
    expect(toneOptionOf(TONE_CATEGORY_OPTIONS, 'media')?.labelKey).toBe('Media')
  })
})

describe('parseSceneList', () => {
  test('splits on the separators an operator types', () => {
    expect(parseSceneList('Newsletter, Docs; Blog | Ads')).toEqual([
      'Newsletter',
      'Docs',
      'Blog',
      'Ads',
    ])
    expect(parseSceneList('公众号长文，技术文档、产品文案；新闻稿')).toEqual([
      '公众号长文',
      '技术文档',
      '产品文案',
      '新闻稿',
    ])
  })

  test('trims, drops blanks and de-duplicates in typing order', () => {
    expect(parseSceneList('  Docs ,, Docs , ,Newsletter ')).toEqual([
      'Docs',
      'Newsletter',
    ])
  })

  test('an empty line means no scenes', () => {
    expect(parseSceneList('')).toEqual([])
    expect(parseSceneList('  ,  、 ')).toEqual([])
  })
})

describe('formatSceneList', () => {
  test('round-trips a scene list through the editor format', () => {
    const scenes = ['公众号长文', '技术文档']
    expect(formatSceneList(scenes)).toBe('公众号长文, 技术文档')
    expect(parseSceneList(formatSceneList(scenes))).toEqual(scenes)
  })

  test('an empty list formats as an empty line', () => {
    expect(formatSceneList([])).toBe('')
  })
})

describe('toneLanguageLabel', () => {
  test('renders a language name in the requested UI language', () => {
    expect(toneLanguageLabel('zh', 'en')).toBe('Chinese')
    expect(toneLanguageLabel('ja', 'en')).toBe('Japanese')
    expect(toneLanguageLabel('en', 'zh')).toBe('英语')
  })

  test('falls back to the raw tag for provider specific codes', () => {
    expect(toneLanguageLabel('mx', 'en')).toBe('mx')
  })

  test('an empty tag has no label', () => {
    expect(toneLanguageLabel('', 'en')).toBe('')
  })

  test('suggestion list holds the codes the plaza filters by', () => {
    expect(TONE_LANGUAGE_CODES).toContain('zh')
    expect(TONE_LANGUAGE_CODES).toContain('vi')
    expect(new Set(TONE_LANGUAGE_CODES).size).toBe(TONE_LANGUAGE_CODES.length)
  })

  test('the accepted tag pattern matches the backend rule', () => {
    for (const tag of ['zh', 'en', 'pt-br', 'zh_tw', 'yue2']) {
      expect(TONE_LANGUAGE_PATTERN.test(tag)).toBe(true)
    }
    for (const tag of ['', 'zh ', 'ZH', '1zh', '-zh', 'zh!']) {
      expect(TONE_LANGUAGE_PATTERN.test(tag)).toBe(false)
    }
  })
})

describe('createToneFormSchema', () => {
  const parse = (
    values: Partial<Record<string, unknown>>,
    limits = TONE_LIMITS_FALLBACK
  ) => createToneFormSchema(limits).safeParse({ ...validForm(), ...values })

  test('requires a name and a prompt', () => {
    expect(parse({}).success).toBe(true)

    const withoutName = parse({ name: '   ' })
    expect(withoutName.success).toBe(false)
    expect(withoutName.error?.issues[0].message).toBe('Tone name is required')

    // The standard is explicit that a tone without an instruction is an empty
    // style for every consumer, and the backend validator rejects it.
    const withoutPrompt = parse({ prompt: '   ' })
    expect(withoutPrompt.success).toBe(false)
    expect(withoutPrompt.error?.issues[0].message).toBe(
      'Tone prompt is required'
    )
  })

  test('enforces the caps of the limits it was built with', () => {
    const limits = { ...TONE_LIMITS_FALLBACK, name: 5, prompt: 20 }
    expect(parse({ name: 'abcde' }, limits).success).toBe(true)

    const tooLong = parse({ name: 'abcdef' }, limits)
    expect(tooLong.success).toBe(false)
    expect(tooLong.error?.issues[0].message).toBe(
      'Tone name is too long (max 5 characters)'
    )

    expect(parse({ prompt: 'x'.repeat(21) }, limits).success).toBe(false)
  })

  test('caps the sample pair and the language tag', () => {
    const limits = { ...TONE_LIMITS_FALLBACK, sample: 4, language: 3 }
    expect(parse({ sampleInput: 'abcd' }, limits).success).toBe(true)
    expect(parse({ sampleInput: 'abcde' }, limits).success).toBe(false)
    expect(parse({ sampleOutput: 'abcde' }, limits).success).toBe(false)
    expect(parse({ language: 'abc' }, limits).success).toBe(true)
    expect(parse({ language: 'abcd' }, limits).success).toBe(false)
  })

  test('caps the scene list and each scene', () => {
    const limits = { ...TONE_LIMITS_FALLBACK, scenes: 2, sceneLength: 3 }
    expect(parse({ scenes: 'abc, def' }, limits).success).toBe(true)

    const tooMany = parse({ scenes: 'abc, def, ghi' }, limits)
    expect(tooMany.success).toBe(false)
    expect(tooMany.error?.issues[0].message).toBe('At most 2 scenes')

    const tooLong = parse({ scenes: 'abcd, def' }, limits)
    expect(tooLong.success).toBe(false)
    expect(tooLong.error?.issues[0].message).toBe(
      'Each scene must be 3 characters or fewer'
    )
  })

  test('counts a scene by characters, not by UTF-16 units', () => {
    // Four CJK characters are four characters; the cap must agree with the
    // backend, which counts runes.
    expect(
      parse({ scenes: '公众号长文' }, { ...TONE_LIMITS_FALLBACK, sceneLength: 5 })
        .success
    ).toBe(true)
    expect(
      parse({ scenes: '公众号长文章' }, { ...TONE_LIMITS_FALLBACK, sceneLength: 5 })
        .success
    ).toBe(false)
  })

  test('accepts an empty language tag but rejects a malformed one', () => {
    expect(parse({ language: '' }).success).toBe(true)
    expect(parse({ language: 'ZH' }).success).toBe(true)
    expect(parse({ language: '1zh' }).success).toBe(false)
  })

  test('bounds the sort order by the published absolute cap', () => {
    const limits = { ...TONE_LIMITS_FALLBACK, sortOrderAbs: 10 }
    expect(parse({ sortOrder: -10 }, limits).success).toBe(true)
    expect(parse({ sortOrder: 11 }, limits).success).toBe(false)
  })
})

describe('hasToneSample', () => {
  test('is true when either half of the pair has content', () => {
    expect(hasToneSample({ sampleInput: 'a', sampleOutput: '' })).toBe(true)
    expect(hasToneSample({ sampleInput: '', sampleOutput: 'b' })).toBe(true)
    expect(hasToneSample({ sampleInput: 'a', sampleOutput: 'b' })).toBe(true)
  })

  test('is false only when both halves are empty', () => {
    expect(hasToneSample({ sampleInput: '', sampleOutput: '' })).toBe(false)
    expect(hasToneSample({ sampleInput: '  ', sampleOutput: '\n' })).toBe(false)
  })
})

describe('EMPTY_TONE_FORM', () => {
  test('starts a new tone on the shelf, at the standard default flavour', () => {
    expect(EMPTY_TONE_FORM.enabled).toBe(true)
    expect(EMPTY_TONE_FORM.sortOrder).toBe(0)
    // The standard distinguishes empty ("not classified yet") from `plain`
    // ("meant to be plain") and calls the latter the more useful answer for a
    // tone whose flavour is undecided.
    expect(EMPTY_TONE_FORM.tone).toBe('plain')
    // The category axis has no such recommended value, so it stays empty.
    expect(EMPTY_TONE_FORM.category).toBe('')
  })
})

/** A form that passes every rule, so each test can vary one field. */
function validForm() {
  return {
    name: 'Warm storyteller',
    description: '',
    prompt: 'Write plainly.',
    category: 'literary',
    tone: 'warm',
    language: 'zh',
    scenes: 'Newsletter',
    sampleInput: '',
    sampleOutput: '',
    enabled: true,
    sortOrder: 0,
  }
}
