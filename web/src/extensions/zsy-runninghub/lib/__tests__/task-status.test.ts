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

import { rhCancelKind, rhStatusKey } from '../task-status'

describe('RunningHub task status mapping', () => {
  test.each([
    ['QUEUED', 'Queued'],
    ['queued', 'Queued'],
    ['SUCCESS', 'Success'],
    ['succeeded', 'Success'],
    ['FAILURE', 'Failed'],
    ['failed', 'Failed'],
    ['IN_PROGRESS', 'In progress'],
    ['SUBMITTED', 'In progress'],
  ])('labels %s as %s', (status, expected) => {
    expect(rhStatusKey(status)).toBe(expected)
  })

  test('labels an unknown status as in progress rather than done', () => {
    expect(rhStatusKey('')).toBe('In progress')
    expect(rhStatusKey('WEIRD_STATE')).toBe('In progress')
  })

  test.each([
    ['QUEUED', 'queued'],
    ['IN_PROGRESS', 'running'],
    ['SUBMITTED', 'running'],
    ['NOT_START', 'running'],
  ])('offers cancel for %s as a %s task', (status, expected) => {
    expect(rhCancelKind(status)).toBe(expected)
  })

  test.each([
    ['SUCCESS'],
    ['FAILURE'],
    [''],
    ['WEIRD_STATE'],
  ])('offers no cancel for the terminal status %s', (status) => {
    expect(rhCancelKind(status)).toBeNull()
  })
})
