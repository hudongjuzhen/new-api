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
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

import { useGroupColorStore } from '@/stores/group-color-store'

import { GroupBadge } from '../group-badge'

afterEach(() => {
  useGroupColorStore.setState({ colors: {} })
})

describe('GroupBadge explicit group color', () => {
  test('uses the stable hash classes when no explicit color is set', () => {
    render(<GroupBadge group='default' />)
    const badge = screen.getByText('default').closest('[data-slot="status-badge"]')
    expect(badge).toBeTruthy()
    // Fallback to auto color keeps neutral class styling, never inline hex.
    expect(badge).not.toHaveAttribute('data-group-color')
  })

  test('renders the explicit HEX color as an inline color style', () => {
    useGroupColorStore.setState({ colors: { vip: '#FF0000' } })
    render(<GroupBadge group='vip' />)
    const badge = screen.getByText('vip').closest('[data-slot="status-badge"]')
    expect(badge).toHaveAttribute('data-group-color', '#FF0000')
    expect(badge).toHaveStyle({ color: 'rgb(255, 0, 0)' })
  })

  test('ignores explicit colors for the auto group', () => {
    useGroupColorStore.setState({ colors: { auto: '#FF0000' } })
    render(<GroupBadge group='auto' />)
    const badge = screen.getByText('Auto').closest('[data-slot="status-badge"]')
    expect(badge).not.toHaveAttribute('data-group-color')
  })

  test('colors the ratio pill with the explicit group color', () => {
    useGroupColorStore.setState({ colors: { vip: '#FF0000' } })
    render(<GroupBadge group='vip' ratio={0.8} />)
    // The innermost span carries the "0.8x" text; the pill with the inline
    // tint sits one level up.
    const pill = screen.getByText('0.8x').parentElement as HTMLElement
    expect(pill).toHaveStyle({ color: 'rgb(255, 0, 0)' })
    expect(pill).toHaveStyle({
      backgroundColor: 'rgba(255, 0, 0, 0.1)',
    })
  })
})
