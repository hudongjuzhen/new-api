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
  listSites: vi.fn(),
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

vi.mock('@/features/playground/api', () => ({
  uploadPlaygroundImage: vi.fn(),
}))

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { RhAppsPage } = await import('../apps-page')
type AppView = import('../../api').AppView
type AppCreateDTO = import('../../api').AppCreateDTO

/**
 * The backend site registry, as the form reads it. Adding a site to sites.go is
 * what makes a new identifier block appear, so this list is the whole input the
 * form needs.
 */
const SITES = [
  { key: 'cn', label: 'RunningHub', channelType: 61 },
  { key: 'intl', label: 'RunningHub Intl', channelType: 62 },
]

/** Composed labels, mirroring the locale entries (`{{site}} Upstream ID`). */
const CN_UPSTREAM = 'RunningHub Upstream ID'
const CN_IDENTIFIER = 'RunningHub Identifier'
const INTL_UPSTREAM = 'RunningHub Intl Upstream ID'
const INTL_IDENTIFIER = 'RunningHub Intl Identifier'

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        '{{site}} Upstream ID': '{{site}} Upstream ID',
        '{{site}} Identifier': '{{site}} Identifier',
        RunningHub: 'RunningHub',
        'RunningHub Intl': 'RunningHub Intl',
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
    slug: 'intl-identifier',
    kind: 'ai_app',
    upstreamId: '2099000000000000001',
    siteUpstreams: {
      cn: { upstreamId: '2051268528824700930', slug: 'cn-identifier' },
      intl: { upstreamId: '2099000000000000001', slug: 'intl-identifier' },
    },
    description: '',
    coverUrl: '',
    published: true,
    adminOnly: false,
    sort: 7,
    paramSchema: [],
    perCallBilling: false,
    fixedQuotaPerCall: 0,
    perSecondBilling: false,
    quotaPerSecond: 0,
    secondsExpr: '',
    perCharBilling: false,
    quotaPerChar: 0,
    charCountExpr: '',
    modelBaseRateRatio: 1.0,
    instanceType: 'default',
    site: 'intl',
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

async function openCreateDialog(user: ReturnType<typeof userEvent.setup>) {
  renderPage()
  await user.click(await screen.findByRole('button', { name: 'Create App' }))
  // The blocks are rendered from the site registry query, so they appear one
  // tick after the dialog.
  return screen.findByLabelText(CN_UPSTREAM)
}

/** Pick a site through the shadcn/Base UI select. */
async function chooseSite(
  user: ReturnType<typeof userEvent.setup>,
  label: string
) {
  await user.click(screen.getByLabelText('Site'))
  await user.click(await screen.findByRole('option', { name: label }))
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
  api.listSites.mockResolvedValue(SITES)
  api.listCategories.mockResolvedValue([])
  api.createApp.mockResolvedValue(storedApp({ id: 8 }))
  api.updateApp.mockResolvedValue(storedApp())
})

describe('RunningHub app per-site upstream identifiers', () => {
  test('the form offers one upstream id and identifier per registered site', async () => {
    const user = userEvent.setup()
    await openCreateDialog(user)

    expect(screen.getByLabelText(CN_UPSTREAM)).toBeInTheDocument()
    expect(screen.getByLabelText(CN_IDENTIFIER)).toBeInTheDocument()
    expect(screen.getByLabelText(INTL_UPSTREAM)).toBeInTheDocument()
    expect(screen.getByLabelText(INTL_IDENTIFIER)).toBeInTheDocument()
  })

  test('editing an app opens on the identifiers stored for each site', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit' }))

    expect(await screen.findByLabelText(CN_UPSTREAM)).toHaveValue(
      '2051268528824700930'
    )
    expect(screen.getByLabelText(CN_IDENTIFIER)).toHaveValue('cn-identifier')
    expect(screen.getByLabelText(INTL_UPSTREAM)).toHaveValue(
      '2099000000000000001'
    )
    expect(screen.getByLabelText(INTL_IDENTIFIER)).toHaveValue(
      'intl-identifier'
    )
  })

  test('saving files each value under its own site', async () => {
    const user = userEvent.setup()
    await openCreateDialog(user)

    await user.type(screen.getByLabelText('App Name'), 'Dual Site App')
    await user.type(screen.getByLabelText(CN_UPSTREAM), 'cn-app-id')
    await user.type(screen.getByLabelText(CN_IDENTIFIER), 'cn-label')
    await user.type(screen.getByLabelText(INTL_UPSTREAM), 'intl-app-id')
    await user.type(screen.getByLabelText(INTL_IDENTIFIER), 'intl-label')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.createApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.createApp.mock.calls[0][0]
    expect(payload.siteUpstreams).toEqual({
      cn: { upstreamId: 'cn-app-id', slug: 'cn-label' },
      intl: { upstreamId: 'intl-app-id', slug: 'intl-label' },
    })
  })

  test('only the site the app runs on is required to carry an upstream id', async () => {
    const user = userEvent.setup()
    await openCreateDialog(user)

    expect(screen.getByLabelText(CN_UPSTREAM)).not.toBeRequired()
    expect(screen.getByLabelText(INTL_UPSTREAM)).not.toBeRequired()

    await chooseSite(user, 'RunningHub Intl')

    expect(screen.getByLabelText(INTL_UPSTREAM)).toBeRequired()
    expect(screen.getByLabelText(CN_UPSTREAM)).not.toBeRequired()
  })

  test('importing a request example fills in the site it came from', async () => {
    const user = userEvent.setup()
    await openCreateDialog(user)
    api.parseCurlRequest.mockResolvedValue({
      success: true,
      data: {
        kind: 'ai_app',
        upstreamId: '2099000000999',
        slug: '2099000000999',
        // The host the pasted curl points at is the international site.
        site: 'intl',
        schema: [],
      },
    })

    await user.type(
      screen.getByLabelText('Request Example'),
      "curl --request POST 'https://www.runninghub.ai/openapi/v2/run/ai-app/2099000000999'"
    )
    await user.click(screen.getByRole('button', { name: 'Fetch Template' }))

    await waitFor(() =>
      expect(screen.getByLabelText(INTL_UPSTREAM)).toHaveValue('2099000000999')
    )
    expect(screen.getByLabelText(CN_UPSTREAM)).toHaveValue('')
  })

  test('the list shows the upstream id the app uses on its own site', async () => {
    renderPage()

    // The row's site is intl, so its id is the international one — not the
    // primary value a stale row might carry.
    expect(await screen.findByText('2099000000000000001')).toBeInTheDocument()
  })
})
