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

import type { PricingModel } from '../../types'
import { getAvailableGroups } from '../model-helpers'

const model: PricingModel = {
  id: 1,
  model_name: 'gpt-4o',
  quota_type: 0,
  model_ratio: 1.25,
  completion_ratio: 4,
  enable_groups: ['低价渠道', '官方渠道', 'default'],
}

describe('getAvailableGroups', () => {
  test('excludes groups without a configured group ratio', () => {
    // The backend injects the user's own group (e.g. "default") into
    // usable_group even when it has no ratio configured; the model square
    // must not offer such groups for filtering.
    const usableGroup = {
      低价渠道: { desc: '低价渠道', ratio: 0.3 },
      官方渠道: { desc: '官方渠道', ratio: 1 },
      default: { desc: '用户分组', ratio: 1 },
    }
    const groupRatio = { 低价渠道: 0.3, 官方渠道: 1 }

    expect(getAvailableGroups(model, usableGroup, groupRatio)).toEqual([
      '低价渠道',
      '官方渠道',
    ])
  })

  test('keeps every usable group that has a configured ratio', () => {
    const usableGroup = {
      低价渠道: { desc: '低价渠道', ratio: 0.3 },
      中价渠道: { desc: '中价渠道', ratio: 0.5 },
    }
    const groupRatio = { 低价渠道: 0.3, 中价渠道: 0.5 }

    expect(
      getAvailableGroups(
        { ...model, enable_groups: ['低价渠道', '中价渠道'] },
        usableGroup,
        groupRatio
      )
    ).toEqual(['低价渠道', '中价渠道'])
  })
})
