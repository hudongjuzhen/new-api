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

const COPY_HINT =
  'Copied from {{name}}. Adjust the fields and save to create a new app.'

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: { 'Copy App': 'Copy App', [COPY_HINT]: COPY_HINT },
    },
  },
})

const sourceApp: AppView = {
  id: 7,
  createdAt: 1700000000,
  updatedAt: 1700000100,
  name: 'Portrait Retouch',
  slug: '2051268528824700930',
  kind: 'ai_app',
  upstreamId: '2051268528824700930',
  description: 'Studio retouch preset',
  coverUrl: 'https://files.rh.local/cover.png',
  published: true,
  adminOnly: false,
  paramSchema: [
    {
      nodeId: '642',
      fieldName: 'image',
      label: 'Image',
      type: 'image',
      required: true,
    },
  ],
  perCallBilling: false,
  fixedQuotaPerCall: 0,
  perSecondBilling: true,
  quotaPerSecond: 2500,
  secondsExpr: '229-212',
  modelBaseRateRatio: 1.5,
  site: 'cn',
  categoryId: 3,
  categoryName: 'Retouch',
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

async function openCopyDialog(user: ReturnType<typeof userEvent.setup>) {
  renderPage()
  await user.click(await screen.findByRole('button', { name: 'Copy App' }))
  return screen.findByRole('dialog')
}

beforeEach(() => {
  api.listApps.mockResolvedValue({
    items: [sourceApp],
    total: 1,
    page: 1,
    pageSize: 100,
    totalPages: 1,
    kindCounts: { ai_app: 1 },
  })
  api.listCategories.mockResolvedValue([])
  api.createApp.mockResolvedValue({ ...sourceApp, id: 8 })
  api.updateApp.mockResolvedValue(sourceApp)
})

describe('RunningHub app record copy', () => {
  test('Copy App opens a create form pre-filled from the listed record', async () => {
    const user = userEvent.setup()

    await openCopyDialog(user)

    expect(screen.getByLabelText('App Name')).toHaveValue(
      'Portrait Retouch_copy'
    )
    expect(screen.getByLabelText('Upstream ID')).toHaveValue(
      '2051268528824700930'
    )
    expect(screen.getByDisplayValue('Image')).toBeInTheDocument()
  })

  test('the copy dialog points at the record it was copied from', async () => {
    const user = userEvent.setup()

    await openCopyDialog(user)

    expect(
      screen.getByText(
        'Copied from Portrait Retouch. Adjust the fields and save to create a new app.'
      )
    ).toBeInTheDocument()
  })

  test('saving the copy creates a new record instead of updating the source', async () => {
    const user = userEvent.setup()
    await openCopyDialog(user)

    const nameInput = screen.getByLabelText('App Name')
    await user.clear(nameInput)
    await user.type(nameInput, 'Portrait Retouch v2')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.createApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.createApp.mock.calls[0][0]
    expect(payload).toMatchObject({
      name: 'Portrait Retouch v2',
      upstreamId: '2051268528824700930',
      slug: '2051268528824700930',
      paramSchema: sourceApp.paramSchema,
      perSecondBilling: true,
      quotaPerSecond: 2500,
    })
    expect(api.updateApp).not.toHaveBeenCalled()
  })

  test('creating a blank app does not inherit the copied record', async () => {
    const user = userEvent.setup()
    await openCopyDialog(user)

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    await user.click(screen.getByRole('button', { name: 'Create App' }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByLabelText('App Name')).toHaveValue('')
  })
})
