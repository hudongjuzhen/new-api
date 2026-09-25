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

import type { AppView } from '../../api'
import { appCopyDraft, appToCreateDTO } from '../app-copy'

function sourceApp(overrides: Partial<AppView> = {}): AppView {
  return {
    id: 7,
    createdAt: 1700000000,
    updatedAt: 1700000100,
    name: 'Portrait Retouch',
    slug: '2051268528824700930',
    kind: 'ai_app',
    upstreamId: '2051268528824700930',
    description: 'Studio retouch preset',
    coverUrl: 'https://files.rh.local/cover.png',
    published: true,
    adminOnly: false,
    paramSchema: [
      {
        nodeId: '642',
        fieldName: 'image',
        label: 'Image',
        type: 'image',
        required: true,
      },
      {
        nodeId: '755',
        fieldName: 'value',
        label: 'Size',
        type: 'select',
        options: [{ label: '1:1', value: '1:1' }],
      },
    ],
    perCallBilling: false,
    fixedQuotaPerCall: 0,
    perSecondBilling: true,
    quotaPerSecond: 2500,
    secondsExpr: '229-212',
    modelBaseRateRatio: 1.5,
    site: 'cn',
    categoryId: 3,
    categoryName: 'Retouch',
    ...overrides,
  }
}

describe('RunningHub app drafts', () => {
  test('appToCreateDTO maps every editable field of the stored record', () => {
    expect(appToCreateDTO(sourceApp())).toEqual({
      name: 'Portrait Retouch',
      slug: '2051268528824700930',
      kind: 'ai_app',
      upstreamId: '2051268528824700930',
      description: 'Studio retouch preset',
      coverUrl: 'https://files.rh.local/cover.png',
      published: true,
      adminOnly: false,
      paramSchema: [
        {
          nodeId: '642',
          fieldName: 'image',
          label: 'Image',
          type: 'image',
          required: true,
        },
        {
          nodeId: '755',
          fieldName: 'value',
          label: 'Size',
          type: 'select',
          options: [{ label: '1:1', value: '1:1' }],
        },
      ],
      perCallBilling: false,
      fixedQuotaPerCall: 0,
      perSecondBilling: true,
      quotaPerSecond: 2500,
      secondsExpr: '229-212',
      modelBaseRateRatio: 1.5,
      site: 'cn',
      categoryId: 3,
    })
  })

  test('an unassigned category stays unassigned in the draft', () => {
    expect(
      appToCreateDTO(sourceApp({ categoryId: null })).categoryId
    ).toBeNull()
  })

  test('the draft owns its parameter template', () => {
    const app = sourceApp()

    const draft = appToCreateDTO(app)
    draft.paramSchema[0].label = 'Source image'
    const options = draft.paramSchema[1].options
    if (options) options[0].value = '16:9'

    expect(app.paramSchema[0].label).toBe('Image')
    expect(app.paramSchema[1].options?.[0].value).toBe('1:1')
  })

  test('appCopyDraft only renames the record, everything else is identical', () => {
    const draft = appCopyDraft(sourceApp(), ['Portrait Retouch'])

    expect(draft.name).toBe('Portrait Retouch_copy')
    expect(draft).toEqual({
      ...appToCreateDTO(sourceApp()),
      name: 'Portrait Retouch_copy',
    })
  })

  test('repeated copies of one record keep getting distinct names', () => {
    const existingNames = [
      'Portrait Retouch',
      'Portrait Retouch_copy',
      'Portrait Retouch_copy2',
    ]

    expect(appCopyDraft(sourceApp(), existingNames).name).toBe(
      'Portrait Retouch_copy3'
    )
  })

  test('names differing only in case count as taken', () => {
    expect(appCopyDraft(sourceApp(), ['portrait retouch_COPY']).name).toBe(
      'Portrait Retouch_copy2'
    )
  })
})
