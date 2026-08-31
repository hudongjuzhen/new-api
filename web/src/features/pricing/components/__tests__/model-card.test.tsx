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
import { ModelCard } from '../model-card'

// The lobe icon library cannot load inside Node ESM (emoji-mart JSON imports);
// icon rendering is irrelevant to the card actions under test.
vi.mock('@/lib/lobe-icon', () => ({ getLobeIcon: () => null }))

const baseModel: PricingModel = {
  id: 1,
  model_name: 'gpt-4o',
  quota_type: 0,
  model_ratio: 1.25,
  completion_ratio: 4,
  enable_groups: ['default'],
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Try Now': 'Try Now',
    Details: 'Details',
    Copy: 'Copy',
  })
})

function renderCard(
  options: {
    onClick?: () => void
    onTry?: () => void
    model?: PricingModel
  } = {}
) {
  return render(
    <ModelCard
      model={options.model ?? baseModel}
      onClick={options.onClick ?? (() => undefined)}
      onTry={options.onTry}
    />
  )
}

describe('ModelCard try now action', () => {
  test('invokes onTry without opening details when Try Now is clicked', async () => {
    const user = userEvent.setup()
    const onTry = vi.fn()
    const onClick = vi.fn()

    renderCard({ onTry, onClick })

    await user.click(screen.getByRole('button', { name: /Try Now/ }))

    expect(onTry).toHaveBeenCalledTimes(1)
    expect(onClick).not.toHaveBeenCalled()
  })

  test('keeps the details button as the onClick trigger', async () => {
    const user = userEvent.setup()
    const onTry = vi.fn()
    const onClick = vi.fn()

    renderCard({ onTry, onClick })

    await user.click(screen.getByRole('button', { name: /Details/ }))

    expect(onClick).toHaveBeenCalledTimes(1)
    expect(onTry).not.toHaveBeenCalled()
  })

  test('hides Try Now when no onTry handler is provided', () => {
    renderCard()

    expect(
      screen.queryByRole('button', { name: /Try Now/ })
    ).not.toBeInTheDocument()
  })
})
