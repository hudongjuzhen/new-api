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

import type { AvatarView } from '../../api'
import {
  AGE_RANGE_OPTIONS,
  AVATAR_IMAGE_FIELDS,
  AVATAR_SCENES_MAX,
  avatarFormSchema,
  avatarOptionLabelKey,
  avatarToFormValues,
  EMPTY_AVATAR_FORM,
  GENDER_OPTIONS,
  parseSceneList,
  RACE_OPTIONS,
} from '../avatar-fields'

function storedAvatar(overrides: Partial<AvatarView> = {}): AvatarView {
  return {
    id: 1,
    createdAt: 1700000000,
    updatedAt: 1700000000,
    name: '客服小雨',
    description: '温柔的客服形象',
    imageUrl: 'https://cdn.example.com/x.png',
    fullBodyUrl: 'https://cdn.example.com/x-full.png',
    fourViewUrl: 'https://cdn.example.com/x-four-view.png',
    expressionUrl: 'https://cdn.example.com/x-expression.png',
    gender: 'female',
    ageRange: 'young',
    race: 'asian',
    scenes: ['客服播报', '有声书'],
    voiceId: 'zh_female_vv_uranus_bigtts',
    voiceAvailable: true,
    voiceName: 'Vivi 2.0',
    voiceSampleUrl: '/uploads/voices/202601/vivi.wav',
    voiceSampleName: 'vivi.wav',
    enabled: true,
    sortOrder: 0,
    ...overrides,
  }
}

describe('avatar attribute contract', () => {
  test('offers exactly the wire values the API validates', () => {
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
    expect(RACE_OPTIONS.map((option) => option.value)).toEqual([
      '',
      'asian',
      'black',
      'white',
      'latino',
      'middle_eastern',
      'south_asian',
      'mixed',
    ])
  })

  test('maps a stored value onto its label, and an unknown one onto itself', () => {
    expect(avatarOptionLabelKey(RACE_OPTIONS, 'asian')).toBe('Asian')
    expect(avatarOptionLabelKey(RACE_OPTIONS, 'martian')).toBe('martian')
  })

  test('treats an unset attribute as "no label" so the table can show a placeholder', () => {
    expect(avatarOptionLabelKey(GENDER_OPTIONS, '')).toBeNull()
    expect(avatarOptionLabelKey(AGE_RANGE_OPTIONS, '')).toBeNull()
    expect(avatarOptionLabelKey(RACE_OPTIONS, '')).toBeNull()
  })
})

describe('scene list editing', () => {
  test('splits on every separator the backend accepts', () => {
    expect(parseSceneList('客服播报,有声书')).toEqual(['客服播报', '有声书'])
    expect(parseSceneList('客服播报，有声书')).toEqual(['客服播报', '有声书'])
    expect(parseSceneList('客服播报、有声书')).toEqual(['客服播报', '有声书'])
    expect(parseSceneList('客服播报;有声书')).toEqual(['客服播报', '有声书'])
    expect(parseSceneList('客服播报|有声书')).toEqual(['客服播报', '有声书'])
  })

  test('drops blanks and duplicates while keeping the typed order', () => {
    expect(parseSceneList(' 有声书 , ,客服播报,有声书 ')).toEqual([
      '有声书',
      '客服播报',
    ])
  })

  test('returns an empty list for an empty line', () => {
    expect(parseSceneList('   ')).toEqual([])
  })
})

describe('avatar form values', () => {
  test('starts a new persona published with no picture and nothing else set', () => {
    expect(EMPTY_AVATAR_FORM).toMatchObject({
      name: '',
      imageUrl: '',
      fullBodyUrl: '',
      fourViewUrl: '',
      expressionUrl: '',
      voiceId: '',
      enabled: true,
      sortOrder: 0,
    })
  })

  test('loads a stored persona, including its four pictures', () => {
    const values = avatarToFormValues(storedAvatar())

    expect(values).toMatchObject({
      name: '客服小雨',
      imageUrl: 'https://cdn.example.com/x.png',
      fullBodyUrl: 'https://cdn.example.com/x-full.png',
      fourViewUrl: 'https://cdn.example.com/x-four-view.png',
      expressionUrl: 'https://cdn.example.com/x-expression.png',
      gender: 'female',
      ageRange: 'young',
      race: 'asian',
      voiceId: 'zh_female_vv_uranus_bigtts',
      scenes: '客服播报, 有声书',
    })
    // The resolved voice sample is not part of the payload: it follows the voice.
    expect(values).not.toHaveProperty('voiceSampleUrl')
  })

  test('round-trips scenes back to the API array', () => {
    const values = avatarToFormValues(storedAvatar())
    expect(parseSceneList(values.scenes)).toEqual(['客服播报', '有声书'])
  })

  test('keeps the scene editor inside the backend bounds', () => {
    expect(AVATAR_SCENES_MAX).toBe(8)
  })
})

describe('avatar picture contract', () => {
  test('offers the cover and its three reference pictures in order', () => {
    expect(
      AVATAR_IMAGE_FIELDS.map((image) => [image.name, image.labelKey])
    ).toEqual([
      ['imageUrl', 'Cover Image'],
      ['fullBodyUrl', 'Full-body Photo'],
      ['fourViewUrl', 'Four Views'],
      ['expressionUrl', 'Expression Sheet'],
    ])
  })

  test('rejects a picture URL the API would refuse, whichever picture it is', () => {
    const values = {
      ...EMPTY_AVATAR_FORM,
      name: '小雨',
      expressionUrl: 'javascript:alert(1)',
    }

    const result = avatarFormSchema.safeParse(values)
    expect(result.success).toBe(false)
  })

  test('accepts the two shapes the API stores for every picture', () => {
    for (const url of [
      '/uploads/images/202601/x.png',
      'https://cdn.example.com/x.png',
      '',
    ]) {
      const result = avatarFormSchema.safeParse({
        ...EMPTY_AVATAR_FORM,
        name: '小雨',
        fullBodyUrl: url,
      })
      expect(result.success, `rejected ${url}`).toBe(true)
    }
  })
})
