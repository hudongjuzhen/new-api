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
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import type { PricingModel } from '../../types'
import { PricingTable } from '../pricing-table'

// The lobe icon library cannot load inside Node ESM (emoji-mart JSON imports);
// icon rendering is irrelevant to the table actions under test.
vi.mock('@/lib/lobe-icon', () => ({ getLobeIcon: () => null }))

const models: PricingModel[] = [
  {
    id: 1,
    model_name: 'gpt-4o',
    quota_type: 0,
    model_ratio: 1.25,
    completion_ratio: 4,
    enable_groups: ['default'],
  },
  {
    id: 2,
    model_name: 'claude-3-5-sonnet',
    quota_type: 0,
    model_ratio: 1.5,
    completion_ratio: 5,
    enable_groups: ['vip'],
  },
]

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Try Now': 'Try Now',
    Actions: 'Actions',
    Model: 'Model',
  })
})

function renderTable(
  options: {
    onModelClick?: (modelName: string) => void
    onTryModel?: (modelName: string) => void
  } = {}
) {
  return render(
    <PricingTable
      models={models}
      onModelClick={options.onModelClick}
      onTryModel={options.onTryModel}
    />
  )
}

describe('PricingTable try now column', () => {
  test('offers one Try Now button per model row', () => {
    renderTable({ onTryModel: () => undefined })

    const tryButtons = screen.getAllByRole('button', { name: /Try Now/ })

    expect(tryButtons).toHaveLength(models.length)
  })

  test('invokes onTryModel with the row model and skips the row click', async () => {
    const user = userEvent.setup()
    const onTryModel = vi.fn()
    const onModelClick = vi.fn()

    renderTable({ onModelClick, onTryModel })

    await user.click(screen.getAllByRole('button', { name: /Try Now/ })[1])

    expect(onTryModel).toHaveBeenCalledTimes(1)
    expect(onTryModel).toHaveBeenCalledWith('claude-3-5-sonnet')
    expect(onModelClick).not.toHaveBeenCalled()
  })

  test('omits the actions cell when no onTryModel handler is provided', () => {
    renderTable()

    expect(
      screen.queryByRole('button', { name: /Try Now/ })
    ).not.toBeInTheDocument()
  })
})
