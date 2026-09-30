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

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Node ID': 'Node ID',
        'Field name': 'Field name',
        'Add Parameter': 'Add Parameter',
        Required: 'Required',
        Edit: 'Edit',
        Save: 'Save',
      },
    },
  },
})

const storedApp: AppView = {
  id: 7,
  createdAt: 1700000000,
  updatedAt: 1700000100,
  name: 'Prompt Rewriter',
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
  perCharBilling: false,
  quotaPerChar: 0,
  charCountExpr: '',
  modelBaseRateRatio: 1.0,
  site: 'cn',
  categoryId: null,
  categoryName: null,
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

async function openEditDialog(user: ReturnType<typeof userEvent.setup>) {
  renderPage()
  await user.click(await screen.findByRole('button', { name: 'Edit' }))
  return screen.findByRole('dialog')
}

beforeEach(() => {
  api.listApps.mockResolvedValue({
    items: [storedApp],
    total: 1,
    page: 1,
    pageSize: 100,
    totalPages: 1,
    kindCounts: { ai_app: 1 },
  })
  api.listCategories.mockResolvedValue([])
  api.createApp.mockResolvedValue(storedApp)
  api.updateApp.mockResolvedValue(storedApp)
})

describe('RunningHub parameter node id editing', () => {
  test('editing an app shows the stored node id in an editable field', async () => {
    const user = userEvent.setup()

    await openEditDialog(user)

    expect(screen.getByLabelText('Node ID')).toHaveValue('122')
    expect(screen.getByLabelText('Field name')).toHaveValue('prompt')
  })

  test('a changed node id is submitted and leaves the field name untouched', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    const nodeIdInput = screen.getByLabelText('Node ID')
    await user.clear(nodeIdInput)
    await user.type(nodeIdInput, '999')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(api.updateApp.mock.calls[0][0]).toBe(7)
    expect(payload.paramSchema).toEqual([
      {
        nodeId: '999',
        fieldName: 'prompt',
        label: '提示词',
        type: 'textarea',
        required: true,
      },
    ])
  })

  test('the required toggle still drives the moved parameter control', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    const requiredToggle = screen.getByRole('switch', { name: 'Required' })
    expect(requiredToggle).toHaveAttribute('aria-checked', 'true')
    await user.click(requiredToggle)
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema).toEqual([
      {
        nodeId: '122',
        fieldName: 'prompt',
        label: '提示词',
        type: 'textarea',
        required: false,
      },
    ])
  })

  test('a manually added parameter takes its own node id and field name', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Create App' }))
    await user.type(screen.getByLabelText('App Name'), 'Image Upscaler')
    await user.type(screen.getByLabelText('Upstream ID'), '2051268528824700930')
    await user.click(screen.getByRole('button', { name: 'Add Parameter' }))

    await user.type(screen.getByLabelText('Node ID'), '642')
    await user.type(screen.getByLabelText('Field name'), 'image')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.createApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.createApp.mock.calls[0][0]
    expect(payload.paramSchema).toEqual([
      {
        nodeId: '642',
        fieldName: 'image',
        label: '',
        type: 'text',
        required: true,
      },
    ])
  })
})
