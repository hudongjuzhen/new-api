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
  buildGroupPricingRowsForTest,
  serializeGroupPricingRowsForTest,
} from '../group-ratio-visual-editor'

describe('group color persistence in the pricing table editor', () => {
  test('builds rows from the color map and keeps only valid hex', () => {
    const rows = buildGroupPricingRowsForTest(
      '{"default":1}',
      '{}',
      '{}',
      '{"default":"#FF0000","vip":"#00ff00","broken":"red"}'
    )
    const byName = Object.fromEntries(rows.map((r) => [r.name, r]))
    expect(byName.default.color).toBe('#FF0000')
    expect(byName.vip.color).toBe('#00FF00')
    // Invalid colors are dropped back to auto.
    expect(byName.broken?.color ?? '').toBe('')
  })

  test('serializes colors into GroupColor and omits unset groups', () => {
    const rows = buildGroupPricingRowsForTest(
      '{"default":1,"vip":1}',
      '{}',
      '{}',
      '{"vip":"#00FF00"}'
    )
    const serialized = serializeGroupPricingRowsForTest(rows)
    expect(JSON.parse(serialized.GroupColor)).toEqual({ vip: '#00FF00' })
  })

  test('clearing a color removes it from the serialized map', () => {
    const rows = buildGroupPricingRowsForTest(
      '{"default":1,"vip":1}',
      '{}',
      '{}',
      '{"vip":"#00FF00"}'
    )
    rows[1] = { ...rows[1], color: '' }
    const serialized = serializeGroupPricingRowsForTest(rows)
    expect(JSON.parse(serialized.GroupColor)).toEqual({})
  })

  test('a group that only exists in the color map still appears as a row', () => {
    const rows = buildGroupPricingRowsForTest(
      '{}',
      '{}',
      '{}',
      '{"special":"#123456"}'
    )
    expect(rows.some((r) => r.name === 'special')).toBe(true)
  })
})
