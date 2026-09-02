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

import { GROUP_COLOR_PRESETS, hexToRgba } from '../colors'

const HEX_RE = /^#?[0-9a-fA-F]{6}$/

describe('GROUP_COLOR_PRESETS', () => {
  test('every preset is a valid 6-digit hex color', () => {
    for (const preset of GROUP_COLOR_PRESETS) {
      expect(preset).toMatch(HEX_RE)
    }
  })
})

describe('hexToRgba', () => {
  test('converts a 6-digit hex with a hash into rgba', () => {
    expect(hexToRgba('#3B82F6', 0.5)).toBe('rgba(59, 130, 246, 0.5)')
  })

  test('accepts lowercase hex', () => {
    expect(hexToRgba('#3b82f6', 1)).toBe('rgba(59, 130, 246, 1)')
  })

  test('accepts a hash-less hex', () => {
    expect(hexToRgba('3B82F6', 0.25)).toBe('rgba(59, 130, 246, 0.25)')
  })

  test('tolerates surrounding whitespace', () => {
    expect(hexToRgba('  #FF0000  ', 1)).toBe('rgba(255, 0, 0, 1)')
  })

  test('falls back to transparent for invalid input', () => {
    expect(hexToRgba('red', 0.5)).toBe('rgba(0, 0, 0, 0)')
    expect(hexToRgba('#123', 0.5)).toBe('rgba(0, 0, 0, 0)')
    expect(hexToRgba('#gggggg', 0.5)).toBe('rgba(0, 0, 0, 0)')
    expect(hexToRgba('', 0.5)).toBe('rgba(0, 0, 0, 0)')
  })
})
