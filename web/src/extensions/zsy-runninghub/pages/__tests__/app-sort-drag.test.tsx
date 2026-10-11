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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const api = vi.hoisted(() => ({
  listApps: vi.fn(),
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

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Drag {{group}} to reorder': 'Drag {{group}} to reorder',
        'Drag to reorder': 'Drag to reorder',
        'Sort order updated': 'Sort order updated',
      },
    },
  },
})

/**
 * Three apps in the order the list endpoint answers with: the largest `sort`
 * value first. `id` and `sort` agree here, which is how a list nobody has
 * dragged yet looks.
 */
function listedApp(id: number, name: string): AppView {
  return {
    id,
    createdAt: 1700000000 + id,
    updatedAt: 1700000100 + id,
    name,
    slug: `app-${id}`,
    kind: 'ai_app',
    upstreamId: `20512685288247009${id}`,
    description: '',
    coverUrl: '',
    published: true,
    adminOnly: false,
    sort: id,
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
    site: 'cn',
    categoryId: null,
    categoryName: null,
  }
}

// Display order is Zeta (3), Beta (2), Alpha (1).
const LISTED = [
  listedApp(3, 'Zeta'),
  listedApp(2, 'Beta'),
  listedApp(1, 'Alpha'),
]

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

/** The app rows of the table, header row excluded, in display order. */
async function appRows() {
  // The first paint holds the header plus a "Loading..." row, so waiting for
  // "some row" would hand back the placeholder.
  await waitFor(() =>
    expect(screen.getAllByRole('row')).toHaveLength(LISTED.length + 1)
  )
  return screen.getAllByRole('row').slice(1)
}

/** Drag `source` onto the top half of `target` (jsdom reports a zero-height
 * rect, so the pointer always counts as being in the upper half). */
function dragRowOnto(source: HTMLElement, target: HTMLElement) {
  const dataTransfer = { effectAllowed: '', dropEffect: '', setData: vi.fn() }
  fireEvent.dragStart(source, { dataTransfer })
  fireEvent.dragOver(target, { dataTransfer })
  fireEvent.drop(target, { dataTransfer })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.listApps.mockResolvedValue({
    items: LISTED,
    total: LISTED.length,
    page: 1,
    pageSize: 100,
    totalPages: 1,
    kindCounts: { ai_app: LISTED.length },
  })
  api.listCategories.mockResolvedValue([])
  api.reorderApps.mockResolvedValue(3)
})

describe('RunningHub app list sorting', () => {
  test('every row carries a labelled drag handle', async () => {
    renderPage()

    const rows = await appRows()
    expect(rows).toHaveLength(3)
    expect(
      screen.getByRole('button', { name: 'Drag Zeta to reorder' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Drag Alpha to reorder' })
    ).toBeInTheDocument()
  })

  test('dropping a row onto another row posts the rearranged order', async () => {
    renderPage()

    const rows = await appRows()
    // Drag the last row (Alpha, id 1) onto the first row (Zeta, id 3).
    dragRowOnto(rows[2], rows[0])

    await waitFor(() => expect(api.reorderApps).toHaveBeenCalledTimes(1))
    expect(api.reorderApps).toHaveBeenCalledWith([1, 3, 2])
  })

  test('a row dropped onto itself does not post an order', async () => {
    renderPage()

    const rows = await appRows()
    dragRowOnto(rows[1], rows[1])

    await waitFor(() => expect(screen.getByText('Zeta')).toBeInTheDocument())
    expect(api.reorderApps).not.toHaveBeenCalled()
  })

  test('ArrowUp on a handle moves the app one position towards the front', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(
      await screen.findByRole('button', { name: 'Drag Beta to reorder' })
    )
    await user.keyboard('{ArrowUp}')

    await waitFor(() => expect(api.reorderApps).toHaveBeenCalledTimes(1))
    expect(api.reorderApps).toHaveBeenCalledWith([2, 3, 1])
  })

  test('ArrowDown at the end of the list leaves the order untouched', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(
      await screen.findByRole('button', { name: 'Drag Alpha to reorder' })
    )
    await user.keyboard('{ArrowDown}')

    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
    expect(api.reorderApps).not.toHaveBeenCalled()
  })

  test('the reordered rows are shown before the refetch answers', async () => {
    renderPage()
    const rows = await appRows()

    // The reload after a reorder never answers: whatever the table shows next
    // can only come from the optimistic update, not from the server.
    api.listApps.mockImplementation(() => new Promise(() => {}))
    dragRowOnto(rows[2], rows[0])

    await waitFor(() => expect(api.reorderApps).toHaveBeenCalledTimes(1))
    const names = (await appRows()).map((row) => row.textContent)
    expect(names[0]).toContain('Alpha')
    expect(names[1]).toContain('Zeta')
  })
})
