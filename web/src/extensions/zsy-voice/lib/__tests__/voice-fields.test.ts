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
  AGE_RANGE_OPTIONS,
  formatSceneList,
  GENDER_OPTIONS,
  parseSceneList,
  VOICE_LANGUAGE_CODES,
  VOICE_LANGUAGE_PATTERN,
  voiceLanguageLabel,
  voiceOptionLabelKey,
} from '../voice-fields'

describe('voiceOptionLabelKey', () => {
  test('maps a stored vocabulary value to its label key', () => {
    expect(voiceOptionLabelKey(GENDER_OPTIONS, 'female')).toBe('Female')
    expect(voiceOptionLabelKey(AGE_RANGE_OPTIONS, 'middle')).toBe('Middle-aged')
  })

  test('returns null when nothing is set', () => {
    expect(voiceOptionLabelKey(GENDER_OPTIONS, '')).toBeNull()
    expect(voiceOptionLabelKey(AGE_RANGE_OPTIONS, '')).toBeNull()
  })

  test('falls back to the raw value for a vocabulary the UI does not know', () => {
    expect(voiceOptionLabelKey(GENDER_OPTIONS, 'robot')).toBe('robot')
  })

  test('offers the API vocabulary, with the empty value meaning unspecified', () => {
    expect(GENDER_OPTIONS.map((option) => option.value)).toEqual([
      '',
      'female',
      'male',
      'neutral',
    ])
    expect(AGE_RANGE_OPTIONS.map((option) => option.value)).toEqual([
      '',
      'child',
      'teen',
      'young',
      'middle',
      'senior',
    ])
  })
})

describe('parseSceneList', () => {
  test('splits on the separators an operator types', () => {
    expect(parseSceneList('Audiobook, News; Live | Support')).toEqual([
      'Audiobook',
      'News',
      'Live',
      'Support',
    ])
    expect(parseSceneList('客服播报，有声书、新闻')).toEqual([
      '客服播报',
      '有声书',
      '新闻',
    ])
  })

  test('trims, drops blanks and de-duplicates in typing order', () => {
    expect(parseSceneList('  News ,, News , ,Audiobook ')).toEqual([
      'News',
      'Audiobook',
    ])
  })

  test('an empty line means no scenes', () => {
    expect(parseSceneList('')).toEqual([])
    expect(parseSceneList('  ,  、 ')).toEqual([])
  })
})

describe('formatSceneList', () => {
  test('round-trips a scene list through the editor format', () => {
    const scenes = ['客服播报', '有声书']
    expect(formatSceneList(scenes)).toBe('客服播报, 有声书')
    expect(parseSceneList(formatSceneList(scenes))).toEqual(scenes)
  })

  test('an empty list formats as an empty line', () => {
    expect(formatSceneList([])).toBe('')
  })
})

describe('voiceLanguageLabel', () => {
  test('renders a language name in the requested UI language', () => {
    expect(voiceLanguageLabel('zh', 'en')).toBe('Chinese')
    expect(voiceLanguageLabel('ja', 'en')).toBe('Japanese')
    expect(voiceLanguageLabel('en', 'zh')).toBe('英语')
  })

  test('falls back to the raw tag for provider specific codes', () => {
    // Volcengine writes Mexican Spanish as "mx", which is not an ISO language.
    expect(voiceLanguageLabel('mx', 'en')).toBe('mx')
  })

  test('an empty tag has no label', () => {
    expect(voiceLanguageLabel('', 'en')).toBe('')
  })

  test('suggestion list holds the codes the plaza filters by', () => {
    expect(VOICE_LANGUAGE_CODES).toContain('zh')
    expect(VOICE_LANGUAGE_CODES).toContain('mx')
    expect(new Set(VOICE_LANGUAGE_CODES).size).toBe(VOICE_LANGUAGE_CODES.length)
  })

  test('the accepted tag pattern matches the backend rule', () => {
    for (const tag of ['zh', 'en', 'pt-br', 'zh_tw', 'yue2']) {
      expect(VOICE_LANGUAGE_PATTERN.test(tag)).toBe(true)
    }
    for (const tag of ['', 'zh ', 'ZH', '1zh', '-zh', 'zh!']) {
      expect(VOICE_LANGUAGE_PATTERN.test(tag)).toBe(false)
    }
  })
})
