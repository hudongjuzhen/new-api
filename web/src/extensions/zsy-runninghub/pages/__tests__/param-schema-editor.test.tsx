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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  listApps: vi.fn(),
  listSites: vi.fn(),
  listCategories: vi.fn(),
  createApp: vi.fn(),
  updateApp: vi.fn(),
  deleteApp: vi.fn(),
  reorderApps: vi.fn(),
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
type SchemaParam = import('../../api').SchemaParam

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Default value': 'Default value',
        'No default': 'No default',
        'Drag {{field}} to reorder': 'Drag {{field}} to reorder',
        'Node ID': 'Node ID',
        'Field name': 'Field name',
        'Add Parameter': 'Add Parameter',
        Enabled: 'Enabled',
        Disabled: 'Disabled',
        Edit: 'Edit',
        Save: 'Save',
      },
    },
  },
})

/** Two text fields plus a dropdown and a toggle, in stored order. */
const STORED_SCHEMA: SchemaParam[] = [
  {
    nodeId: '122',
    fieldName: 'prompt',
    label: '提示词',
    type: 'textarea',
    required: true,
    defaultValue: 'A quiet harbor',
  },
  {
    nodeId: '642',
    fieldName: 'image',
    label: 'Input Image',
    type: 'image',
    required: true,
  },
  {
    nodeId: '755',
    fieldName: 'value',
    label: '尺寸',
    type: 'select',
    required: true,
    options: [
      { label: '1k', value: '1k' },
      { label: '2k', value: '2k' },
    ],
  },
  {
    nodeId: '256',
    fieldName: 'use_personal_queue',
    label: '个人队列',
    type: 'switch',
    required: true,
  },
]

const storedApp = (overrides: Partial<AppView> = {}): AppView => ({
  id: 7,
  createdAt: 1700000000,
  updatedAt: 1700000100,
  name: 'Prompt Rewriter',
  slug: '2051268528824700930',
  kind: 'ai_app',
  upstreamId: '2051268528824700930',
  siteUpstreams: {
    cn: { upstreamId: '2051268528824700930', slug: '2051268528824700930' },
  },
  description: '',
  coverUrl: '',
  published: true,
  adminOnly: false,
  sort: 7,
  paramSchema: STORED_SCHEMA,
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
  site: 'cn',
  categoryId: null,
  categoryName: null,
  ...overrides,
})

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

/** The parameter cards in the order the form renders them. */
function paramCards() {
  return screen.getAllByRole('listitem')
}

/** Node id of the parameter each card edits, in display order. */
function displayedNodeIds() {
  return paramCards().map(
    (card) => (within(card).getByLabelText('Node ID') as HTMLInputElement).value
  )
}

/** Card of the parameter that edits `nodeId`. */
function cardOf(nodeId: string) {
  const card = paramCards().find(
    (candidate) =>
      (within(candidate).getByLabelText('Node ID') as HTMLInputElement)
        .value === nodeId
  )
  if (!card) throw new Error(`no parameter card for node ${nodeId}`)
  return card
}

/** The reorder handle of the card that edits `nodeId`. */
function handleOf(nodeId: string) {
  return within(cardOf(nodeId)).getByRole('button', { name: /to reorder$/ })
}

/** Drag `sourceNodeId`'s handle onto `targetNodeId`'s card. */
function dragCardOnto(sourceNodeId: string, targetNodeId: string) {
  const dataTransfer = { effectAllowed: '', dropEffect: '', setData: vi.fn() }
  fireEvent.dragStart(handleOf(sourceNodeId), { dataTransfer })
  // jsdom reports a zero-height rect, so the pointer always counts as being in
  // the upper half of the card: the drop lands before it.
  fireEvent.dragOver(cardOf(targetNodeId), { dataTransfer })
  fireEvent.drop(cardOf(targetNodeId), { dataTransfer })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.listApps.mockResolvedValue({
    items: [storedApp()],
    total: 1,
    page: 1,
    pageSize: 100,
    totalPages: 1,
    kindCounts: { ai_app: 1 },
  })
  api.listSites.mockResolvedValue([
    { key: 'cn', label: 'RunningHub', channelType: 61 },
    { key: 'intl', label: 'RunningHub Intl', channelType: 62 },
  ])
  api.listCategories.mockResolvedValue([])
  api.updateApp.mockResolvedValue(storedApp())
})

describe('RunningHub parameter order', () => {
  test('the editor lists the stored parameters in their saved order', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    expect(displayedNodeIds()).toEqual(['122', '642', '755', '256'])
  })

  test('every parameter carries a drag handle named after its field', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    // The handle is labelled per field so a screen reader hears which one it
    // moves instead of the same words on every row.
    expect(
      within(cardOf('122')).getByRole('button', {
        name: 'Drag 提示词 to reorder',
      })
    ).toBeInTheDocument()
    expect(
      within(cardOf('642')).getByRole('button', {
        name: 'Drag Input Image to reorder',
      })
    ).toBeInTheDocument()
  })

  test('dropping a later parameter onto an earlier one saves the new order', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    dragCardOnto('256', '122')
    expect(displayedNodeIds()).toEqual(['256', '122', '642', '755'])

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema.map((p) => p.nodeId)).toEqual([
      '256',
      '122',
      '642',
      '755',
    ])
  })

  test('ArrowDown on a handle moves its parameter one position back', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    handleOf('122').focus()
    await user.keyboard('{ArrowDown}')
    expect(displayedNodeIds()).toEqual(['642', '122', '755', '256'])

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema.map((p) => p.fieldName)).toEqual([
      'image',
      'prompt',
      'value',
      'use_personal_queue',
    ])
  })

  test('a reordered parameter keeps its own node id and default', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    // Moving a card must carry its content: reordering must never hand one
    // field's node id or default to another one's position.
    dragCardOnto('256', '122')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema[0].fieldName).toBe('use_personal_queue')
    expect(payload.paramSchema[1]).toMatchObject({
      nodeId: '122',
      fieldName: 'prompt',
      defaultValue: 'A quiet harbor',
    })
  })

  test('ArrowUp on the first handle leaves the order untouched', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    handleOf('122').focus()
    await user.keyboard('{ArrowUp}')

    expect(displayedNodeIds()).toEqual(['122', '642', '755', '256'])
    expect(api.updateApp).not.toHaveBeenCalled()
  })

  test('a parameter dropped onto itself keeps the order', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    dragCardOnto('642', '642')

    expect(displayedNodeIds()).toEqual(['122', '642', '755', '256'])
    expect(api.updateApp).not.toHaveBeenCalled()
  })
})

describe('RunningHub parameter default value', () => {
  test('a stored default is shown in its field', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    expect(within(cardOf('122')).getByLabelText('Default value')).toHaveValue(
      'A quiet harbor'
    )
    expect(within(cardOf('642')).getByLabelText('Default value')).toHaveValue(
      ''
    )
  })

  test('a typed default is submitted with its own parameter only', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    const defaultInput = within(cardOf('642')).getByLabelText('Default value')
    await user.type(defaultInput, 'openapi/sample.png')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema[1].defaultValue).toBe('openapi/sample.png')
    expect(payload.paramSchema[0].defaultValue).toBe('A quiet harbor')
  })

  test('a dropdown parameter picks its default from the declared options', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    await user.click(within(cardOf('755')).getByLabelText('Default value'))
    await user.click(await screen.findByRole('option', { name: '2k' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema[2].defaultValue).toBe('2k')
  })

  test('a toggle parameter stores its default as the submitted token', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    // A toggle default is queried by role: the switch also labels the form
    // input Base UI keeps next to it, so the label text matches two nodes.
    const toggle = within(cardOf('256')).getByRole('switch', {
      name: 'Default value',
    })
    expect(toggle).not.toBeChecked()
    await user.click(toggle)
    expect(toggle).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema[3].defaultValue).toBe('true')
  })

  test('a dropdown default no option matches any more stays visible and selected', async () => {
    const user = userEvent.setup()
    // The options were rewritten after '4k' had been stored as the default.
    api.listApps.mockResolvedValue({
      items: [
        storedApp({
          paramSchema: [
            {
              nodeId: '755',
              fieldName: 'value',
              label: '尺寸',
              type: 'select',
              required: true,
              defaultValue: '4k',
              options: [
                { label: '1k', value: '1k' },
                { label: '2k', value: '2k' },
              ],
            },
          ],
        }),
      ],
      total: 1,
      page: 1,
      pageSize: 100,
      totalPages: 1,
      kindCounts: { ai_app: 1 },
    })
    await openEditDialog(user)

    expect(
      within(cardOf('755')).getByLabelText('Default value')
    ).toHaveTextContent('4k')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema[0].defaultValue).toBe('4k')
  })

  test('clearing a stored default submits an empty value', async () => {
    const user = userEvent.setup()
    await openEditDialog(user)

    const defaultInput = within(cardOf('122')).getByLabelText('Default value')
    await user.clear(defaultInput)
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateApp).toHaveBeenCalledTimes(1))
    const payload: AppCreateDTO = api.updateApp.mock.calls[0][1]
    expect(payload.paramSchema[0].defaultValue).toBe('')
  })
})
