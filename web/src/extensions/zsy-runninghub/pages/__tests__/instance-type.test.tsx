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

// The select renders the upstream tokens themselves; these are the labels the
// form shows around them.
const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Instance Type': 'Instance Type',
        'Per-Call Billing': 'Per-Call Billing',
        Site: 'Site',
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
    name: 'Portrait Retouch',
    slug: '2051268528824700930',
    kind: 'ai_app',
    upstreamId: '2051268528824700930',
    description: '',
    coverUrl: '',
    published: true,
    adminOnly: false,
    sort: 7,
    paramSchema: [],
    perCallBilling: true,
    fixedQuotaPerCall: 1000,
    perSecondBilling: false,
    quotaPerSecond: 0,
    secondsExpr: '',
    perCharBilling: false,
    quotaPerChar: 0,
    charCountExpr: '',
    modelBaseRateRatio: 1.0,
    instanceType: 'default',
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

/** Pick an instance type through the shadcn/Base UI select. */
async function chooseInstanceType(
  user: ReturnType<typeof userEvent.setup>,
  value: string
) {
  await user.click(screen.getByLabelText('Instance Type'))
  await user.click(await screen.findByRole('option', { name: value }))
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

describe('RunningHub app instance type', () => {
  test('the list shows the instance each app runs on', async () => {
    api.listApps.mockResolvedValue({
      items: [storedApp({ instanceType: 'ultra' })],
      total: 1,
      page: 1,
      pageSize: 100,
      totalPages: 1,
      kindCounts: { ai_app: 1 },
    })
    renderPage()

    expect(await screen.findByText('ultra')).toBeInTheDocument()
  })

  test('an app saved without the field is listed as the default instance', async () => {
    api.listApps.mockResolvedValue({
      items: [storedApp({ instanceType: '' })],
      total: 1,
      page: 1,
      pageSize: 100,
      totalPages: 1,
      kindCounts: { ai_app: 1 },
    })
    renderPage()

    expect(await screen.findByText('default')).toBeInTheDocument()
  })

  test('editing an app opens the selector on its stored instance', async () => {
    const user = userEvent.setup()
    api.listApps.mockResolvedValue({
      items: [storedApp({ instanceType: 'plus' })],
      total: 1,
      page: 1,
      pageSize: 100,
      totalPages: 1,
      kindCounts: { ai_app: 1 },
    })
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit' }))

    expect(screen.getByLabelText('Instance Type')).toHaveTextContent('plus')
  })

  test('offers exactly the three instance types RH accepts', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create App' }))
    await user.click(screen.getByLabelText('Instance Type'))

    const options = await screen.findAllByRole('option')
    expect(options.map((option) => option.textContent)).toEqual([
      'default',
      'plus',
      'ultra',
    ])
  })

  test('saving an edited app submits the chosen instance type', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit' }))
    await chooseInstanceType(user, 'ultra')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.instanceType).toBe('ultra')
    expect(api.createApp).not.toHaveBeenCalled()
  })

  test('a new app defaults to the default instance without touching the field', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create App' }))
    await user.type(screen.getByLabelText('App Name'), 'Fresh App')
    await user.type(screen.getByLabelText('Upstream ID'), '2051268528824700001')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.createApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.createApp.mock.calls[0][0]
    expect(payload.instanceType).toBe('default')
  })
})
