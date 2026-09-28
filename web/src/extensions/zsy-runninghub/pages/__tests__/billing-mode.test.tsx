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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  listApps: vi.fn(),
  listCategories: vi.fn(),
  createApp: vi.fn(),
  updateApp: vi.fn(),
  deleteApp: vi.fn(),
  parseCurlRequest: vi.fn(),
  createCategory: vi.fn(),
  updateCategory: vi.fn(),
  deleteCategory: vi.fn(),
}))

vi.mock('../../api', () => api)

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { RhAppsPage } = await import('../apps-page')
type AppView = import('../../api').AppView
type AppCreateDTO = import('../../api').AppCreateDTO

// The admin form renders labels and mode names through t(); the identity
// translations below are the English source strings the keys hold.
const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Per-Character Billing': 'Per-Character Billing',
        'Per-Second Billing': 'Per-Second Billing',
        'Quota Per Character ({{currency}})': 'Quota Per Character ({{currency}})',
        'Quota Per Second ({{currency}})': 'Quota Per Second ({{currency}})',
        'Character Field': 'Character Field',
        'Seconds Field': 'Seconds Field',
        Cancel: 'Cancel',
      },
    },
  },
})

function storedApp(overrides: Partial<AppView> = {}): AppView {
  return {
    id: 7,
    createdAt: 1700000000,
    updatedAt: 1700000100,
    name: 'Copy Rewriter',
    slug: '2051268528824700930',
    kind: 'ai_app',
    upstreamId: '2051268528824700930',
    description: '',
    coverUrl: '',
    published: true,
    adminOnly: false,
    paramSchema: [
      {
        nodeId: '122',
        fieldName: 'prompt',
        label: '提示词',
        type: 'textarea',
        required: true,
      },
    ],
    perCallBilling: false,
    fixedQuotaPerCall: 0,
    perSecondBilling: false,
    quotaPerSecond: 0,
    secondsExpr: '',
    perCharBilling: true,
    quotaPerChar: 1000,
    charCountExpr: 'len(122)',
    modelBaseRateRatio: 1.0,
    site: 'cn',
    categoryId: null,
    categoryName: null,
    ...overrides,
  }
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <RhAppsPage />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

/** Toggle the app form's billing mode through the shadcn/Base UI select. */
async function chooseBillingMode(
  user: ReturnType<typeof userEvent.setup>,
  mode: string
) {
  await user.click(screen.getByLabelText('Billing Mode'))
  await user.click(await screen.findByRole('option', { name: mode }))
}

beforeEach(() => {
  api.listApps.mockResolvedValue({
    items: [storedApp()],
    total: 1,
    page: 1,
    pageSize: 100,
    totalPages: 1,
    kindCounts: { ai_app: 1 },
  })
  api.listCategories.mockResolvedValue([])
  api.createApp.mockResolvedValue(storedApp({ id: 8 }))
  api.updateApp.mockResolvedValue(storedApp())
})

describe('RunningHub per-character billing mode', () => {
  test('lists the stored app as per-character billed', async () => {
    renderPage()

    expect(await screen.findByText('Per-Character Billing')).toBeInTheDocument()
  })

  test('editing a per-character app loads its quota and character field', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit' }))

    expect(screen.getByLabelText('Quota Per Character (USD)')).toBeVisible()
    expect(screen.getByLabelText('Character Field')).toHaveValue('len(122)')
    expect(screen.queryByLabelText('Seconds Field')).not.toBeInTheDocument()
  })

  test('choosing per-character billing reveals its price and field inputs', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create App' }))
    expect(screen.queryByLabelText('Character Field')).not.toBeInTheDocument()

    await chooseBillingMode(user, 'Per-Character Billing')

    expect(screen.getByLabelText('Quota Per Character (USD)')).toBeVisible()
    expect(
      screen.getByPlaceholderText('nodeId=212 / len(212)')
    ).toBeInTheDocument()
  })

  test('leaving per-character billing hides its inputs again', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create App' }))
    await chooseBillingMode(user, 'Per-Character Billing')
    expect(screen.getByLabelText('Character Field')).toBeInTheDocument()

    await chooseBillingMode(user, 'Per-Second Billing')

    expect(screen.queryByLabelText('Character Field')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Seconds Field')).toBeInTheDocument()
  })

  test('saving a per-character app submits the price, the field and the mode flag', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create App' }))
    await user.type(screen.getByLabelText('App Name'), 'Copy Rewriter')
    await user.type(
      screen.getByLabelText('Upstream ID'),
      '2051268528824700930'
    )
    await chooseBillingMode(user, 'Per-Character Billing')
    await user.type(
      screen.getByLabelText('Quota Per Character (USD)'),
      '0.002'
    )
    await user.type(screen.getByLabelText('Character Field'), 'len(122)')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.createApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.createApp.mock.calls[0][0]
    expect(payload).toMatchObject({
      perCharBilling: true,
      perCallBilling: false,
      perSecondBilling: false,
      quotaPerChar: 1000,
      charCountExpr: 'len(122)',
    })
    expect(api.updateApp).not.toHaveBeenCalled()
  })
})
