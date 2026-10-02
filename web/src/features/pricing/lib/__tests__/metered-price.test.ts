/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import {
  getMeteredRateLabelKey,
  getMeteredUnitNameKey,
  isMeteredPricingModel,
} from '../metered-price'

function buildModel(overrides: Partial<PricingModel>): PricingModel {
  return {
    model_name: 'test-model',
    quota_type: 1,
    model_ratio: 0,
    completion_ratio: 1,
    model_price: 0.375,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('isMeteredPricingModel', () => {
  it('flags a model the backend marks as metered with a unit', () => {
    expect(
      isMeteredPricingModel(
        buildModel({ billing_mode: 'metered', metered_unit: 'minute' })
      )
    ).toBe(true)
  })

  it('does not flag a flat per-request model', () => {
    expect(isMeteredPricingModel(buildModel({}))).toBe(false)
  })

  it('does not flag a dynamic pricing model', () => {
    expect(
      isMeteredPricingModel(buildModel({ billing_mode: 'tiered_expr' }))
    ).toBe(false)
  })

  it('does not flag a metered mode with no unit to display', () => {
    expect(isMeteredPricingModel(buildModel({ billing_mode: 'metered' }))).toBe(
      false
    )
  })
})

describe('metered unit label keys', () => {
  it('names the unit for a per-minute model', () => {
    const model = buildModel({ billing_mode: 'metered', metered_unit: 'minute' })
    expect(getMeteredUnitNameKey(model)).toBe('Minute')
    expect(getMeteredRateLabelKey(model)).toBe('Per minute')
  })

  it('names the unit for a per-second model', () => {
    const model = buildModel({ billing_mode: 'metered', metered_unit: 'second' })
    expect(getMeteredUnitNameKey(model)).toBe('Second')
    expect(getMeteredRateLabelKey(model)).toBe('Per second')
  })

  it('returns no label for models that are not metered, so callers keep their per-request wording', () => {
    const model = buildModel({})
    expect(getMeteredUnitNameKey(model)).toBeUndefined()
    expect(getMeteredRateLabelKey(model)).toBeUndefined()
  })

  it('returns no label for an unrecognized unit rather than guessing one', () => {
    const model = buildModel({ billing_mode: 'metered', metered_unit: 'hour' })
    expect(getMeteredUnitNameKey(model)).toBeUndefined()
    expect(getMeteredRateLabelKey(model)).toBeUndefined()
  })
})
