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
import { describe, expect, it } from 'vitest'

import { isAudioGenModel, isImageGenModel } from '../lib/model'

describe('isAudioGenModel', () => {
  // The lab routes the playground from this decision, so it must follow the
  // backend's metered-billing marker: anything else would render a form whose
  // request the relay does not answer as audio.
  it('flags a model the backend marks as metered with a unit', () => {
    expect(
      isAudioGenModel({ billing_mode: 'metered', metered_unit: 'minute' })
    ).toBe(true)
  })

  it('does not flag per-request, tiered or unit-less models', () => {
    expect(isAudioGenModel({})).toBe(false)
    expect(isAudioGenModel({ billing_mode: 'tiered_expr' })).toBe(false)
    expect(isAudioGenModel({ billing_mode: 'metered' })).toBe(false)
    expect(isAudioGenModel(null)).toBe(false)
    expect(isAudioGenModel(undefined)).toBe(false)
  })
})

describe('isImageGenModel', () => {
  it('flags the image endpoint capability and the known family name', () => {
    expect(
      isImageGenModel({
        model_name: 'seedream-4-0',
        supported_endpoint_types: ['image-generation'],
      })
    ).toBe(true)
    expect(
      isImageGenModel({
        model_name: 'gpt-image-1',
        supported_endpoint_types: [],
      })
    ).toBe(true)
  })

  it('does not flag an audio model', () => {
    expect(
      isImageGenModel({
        model_name: 'seed-audio-1.0',
        supported_endpoint_types: ['openai'],
      })
    ).toBe(false)
  })
})
