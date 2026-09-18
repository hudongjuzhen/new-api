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

import { channelSchema } from '../../types'
import {
  buildSettingJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  MAX_CHANNEL_CONCURRENCY,
  normalizeMaxConcurrency,
  transformChannelToFormDefaults,
} from '../channel-form'

function channelWithSetting(setting: Record<string, unknown>) {
  return channelSchema.parse({
    id: 1,
    type: 61,
    key: 'rh-key',
    status: 1,
    name: 'RunningHub',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    setting: JSON.stringify(setting),
  })
}

describe('channel max concurrency', () => {
  test.each([
    ['undefined', undefined],
    ['null', null],
    ['zero', 0],
    ['negative', -3],
    ['NaN', Number.NaN],
  ])('normalizes %s to 0 (unlimited)', (_name, input) => {
    expect(normalizeMaxConcurrency(input)).toBe(0)
  })

  test('clamps to the backend cap and truncates fractions', () => {
    expect(normalizeMaxConcurrency(MAX_CHANNEL_CONCURRENCY + 1)).toBe(
      MAX_CHANNEL_CONCURRENCY
    )
    expect(normalizeMaxConcurrency(2.7)).toBe(2)
    expect(normalizeMaxConcurrency(3)).toBe(3)
  })

  test('omits the cap when unlimited so untouched channels keep equivalent JSON', () => {
    const setting = JSON.parse(
      buildSettingJSON({ ...CHANNEL_FORM_DEFAULT_VALUES, max_concurrency: 0 })
    )
    expect(setting).not.toHaveProperty('max_concurrency')
  })

  test('persists the configured cap', () => {
    const setting = JSON.parse(
      buildSettingJSON({ ...CHANNEL_FORM_DEFAULT_VALUES, max_concurrency: 2 })
    )
    expect(setting.max_concurrency).toBe(2)
  })

  test('round-trips the cap from a stored channel', () => {
    const channel = channelWithSetting({ max_concurrency: 4 })
    expect(transformChannelToFormDefaults(channel).max_concurrency).toBe(4)
  })

  test('treats an absent cap as unlimited when loading a channel', () => {
    const channel = channelWithSetting({ proxy: '' })
    expect(transformChannelToFormDefaults(channel).max_concurrency).toBe(0)
  })

  test('accepts the boundaries and rejects values outside them', () => {
    const base = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'RH',
      key: 'rh-key',
      models: 'rh-app-1',
    }
    expect(
      channelFormSchema.safeParse({ ...base, max_concurrency: 0 }).success
    ).toBe(true)
    expect(
      channelFormSchema.safeParse({
        ...base,
        max_concurrency: MAX_CHANNEL_CONCURRENCY,
      }).success
    ).toBe(true)
    expect(
      channelFormSchema.safeParse({
        ...base,
        max_concurrency: MAX_CHANNEL_CONCURRENCY + 1,
      }).success
    ).toBe(false)
  })
})
