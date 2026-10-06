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
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const toneApi = vi.hoisted(() => ({
  listTones: vi.fn(),
  createTone: vi.fn(),
  updateTone: vi.fn(),
  deleteTone: vi.fn(),
  exportTones: vi.fn(),
  importTonesCsv: vi.fn(),
  fetchToneStandard: vi.fn(),
}))

// Only the network calls are replaced: the URL helpers stay real so the
// published-endpoint assertions protect the actual paths.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  ...toneApi,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: { server_address: 'https://gw.example.com' },
    loading: false,
    error: null,
  }),
}))

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { TonePlazaPage } = await import('../tone-plaza-page')

type ToneView = import('../../api').ToneView

// The assertions below query the English source strings the components pass
// through t(); registering them as identity translations keeps that contract
// explicit, including the interpolated ones.
const identity: Record<string, string> = {}
for (const key of [
  'Tone Plaza',
  'Tone Standard',
  'Tone Name',
  'Tone',
  'Tone Prompt',
  'New Tone',
  'Edit Tone',
  'Delete Tone',
  'Introduction',
  'Category',
  'Language',
  'All',
  'All languages',
  'Uncategorized',
  'Not specified',
  'Literary',
  'Business',
  'Marketing',
  'Warm',
  'Calm',
  'On Shelf',
  'Off Shelf',
  'Shelf Status',
  'Page size',
  'Search',
  'Search by name, prompt or sample',
  'No tones yet',
  'Tone name is required',
  'Tone prompt is required',
  'Sample',
  'Sample Input',
  'Sample Output',
  'Status',
  'Sort Order',
  'Actions',
  'Edit',
  'Delete',
  'Cancel',
  'Save',
  'Saving...',
  'Close',
  'Previous',
  'Next',
  'Tone List API',
  'Tone Standard API',
  'Import',
  'Export',
  'Export started',
  'Import Tones',
  'Import Mode',
  'Create and update by name',
  'Create only',
  'CSV File',
  'Importing...',
  'Import Result',
  'Import failed',
  'Choose a CSV file',
  'Created: {{count}}',
  'Updated: {{count}}',
  'Failed: {{count}}',
  'Row {{row}}',
  'Imported: {{created}} created, {{updated}} updated, {{failed}} failed',
  'Total {{count}} tones',
  'Page {{page}} / {{total}}',
  '{{count}} per page',
  'Suitable Scenes',
  'Delete "{{name}}"? This cannot be undone.',
]) {
  identity[key] = key
}

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: identity } },
})

function storedTone(overrides: Partial<ToneView> = {}): ToneView {
  return {
    id: 1,
    createdAt: 1700000000,
    updatedAt: 1700000000,
    name: 'Warm storyteller',
    description: 'Calm and unhurried',
    prompt: 'Write in short declarative sentences.',
    category: 'literary',
    tone: 'warm',
    language: 'zh',
    scenes: ['Newsletter', 'Documentation'],
    sampleInput: 'The release ships today.',
    sampleOutput: 'Today, the release ships.',
    enabled: true,
    sortOrder: 0,
    ...overrides,
  }
}

function pageResult(
  items: ToneView[],
  overrides: Partial<{
    total: number
    page: number
    pageSize: number
    totalPages: number
  }> = {}
) {
  return {
    items,
    total: items.length,
    page: 1,
    pageSize: 20,
    totalPages: items.length > 0 ? 1 : 0,
    ...overrides,
  }
}

/** A standard response shaped exactly like the endpoint's contract. */
function standardPayload(overrides: Record<string, unknown> = {}) {
  return {
    success: true,
    data: {
      version: '2.0.0',
      categories: [
        {
          value: 'literary',
          label: '文学',
          labelEn: 'Literary',
          desc: '散文与小说',
        },
        {
          value: 'business',
          label: '商务',
          labelEn: 'Business',
          desc: '',
        },
      ],
      tones: [
        { value: 'warm', label: '温暖', labelEn: 'Warm', desc: '' },
        { value: 'calm', label: '平静', labelEn: 'Calm', desc: '' },
      ],
      limits: { name: 191 },
      examplePair: {
        field: 'sampleInput / sampleOutput',
        convention: 'The pair is the only way to see what a tone does.',
        whyItMatters: 'A tone can neither be seen nor heard.',
      },
      ...overrides,
    },
  }
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <TonePlazaPage />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

/** Parameters of the n-th listTones call. */
function listCall(index = 0) {
  return toneApi.listTones.mock.calls[index][0] as Record<string, unknown>
}

beforeEach(() => {
  toneApi.listTones.mockResolvedValue(pageResult([storedTone()]))
  toneApi.createTone.mockResolvedValue(storedTone({ id: 2 }))
  toneApi.updateTone.mockResolvedValue(storedTone())
  toneApi.deleteTone.mockResolvedValue(undefined)
  toneApi.fetchToneStandard.mockResolvedValue(standardPayload())
  toneApi.exportTones.mockResolvedValue({
    blob: new Blob(['\ufeffname,prompt\n']),
    filename: 'zsy-tones-20260101-000000.csv',
  })
  toneApi.importTonesCsv.mockResolvedValue({
    total: 2,
    created: 1,
    updated: 1,
    failed: 0,
    errors: [],
  })
})

describe('Tone Plaza admin page', () => {
  test('lists the tones the API returns, with their vocabulary and prompt', async () => {
    toneApi.listTones.mockResolvedValue(
      pageResult([
        storedTone(),
        storedTone({
          id: 2,
          name: 'Bare tone',
          category: '',
          tone: '',
          language: '',
          scenes: [],
          prompt: '',
          sampleInput: '',
          sampleOutput: '',
          enabled: false,
        }),
      ])
    )

    renderPage()

    expect(await screen.findByText('Warm storyteller')).toBeInTheDocument()
    expect(
      screen.getByText('Write in short declarative sentences.')
    ).toBeInTheDocument()
    expect(screen.getByText('Bare tone')).toBeInTheDocument()
    expect(screen.getByText('On Shelf')).toBeInTheDocument()
    expect(screen.getByText('Off Shelf')).toBeInTheDocument()
    // The row with no vocabulary at all says so instead of showing a blank cell.
    expect(screen.getByText('Not specified')).toBeInTheDocument()
    expect(screen.getAllByText('Uncategorized').length).toBeGreaterThan(0)
  })

  test('shows the empty state when the plaza has no tones', async () => {
    toneApi.listTones.mockResolvedValue(pageResult([]))

    renderPage()

    expect(await screen.findByText('No tones yet')).toBeInTheDocument()
    expect(screen.getByText('Total 0 tones')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  test('requests the next page and keeps the active filters', async () => {
    const user = userEvent.setup()
    toneApi.listTones.mockResolvedValue(
      pageResult([storedTone()], { total: 40, totalPages: 2 })
    )

    renderPage()
    await screen.findByText('Warm storyteller')

    expect(listCall()).toMatchObject({ page: 1, pageSize: 20 })
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Next' }))

    await waitFor(() => expect(listCall(1)).toMatchObject({ page: 2 }))
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeEnabled()
  })

  test('searches by keyword from the first page', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.type(
      screen.getByLabelText('Search by name, prompt or sample'),
      'storyteller'
    )
    await user.click(screen.getByRole('button', { name: 'Search' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ keyword: 'storyteller' })
    )
    expect(listCall(1)).toMatchObject({ page: 1 })
  })

  test('filters the list down to off-shelf tones', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByLabelText('Shelf Status'))
    await user.click(await screen.findByRole('option', { name: 'Off Shelf' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ enabled: false, page: 1 })
    )
  })

  test('filters by the category and tone the standard publishes', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByLabelText('Category'))
    // The option list comes from the endpoint's vocabulary, not from a local list
    // of its own — `Business` is only there because the response listed it.
    await user.click(await screen.findByRole('option', { name: 'Literary' }))
    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ category: 'literary', page: 1 })
    )

    await user.click(screen.getByLabelText('Tone'))
    await user.click(await screen.findByRole('option', { name: 'Calm' }))
    await waitFor(() => expect(listCall(2)).toMatchObject({ tone: 'calm' }))
    expect(listCall(2)).toMatchObject({ category: 'literary' })
  })

  test('a filter value that is not set means every row', async () => {
    renderPage()
    await screen.findByText('Warm storyteller')

    expect(listCall()).toMatchObject({ category: '', tone: '', language: '' })
    expect(listCall()).toMatchObject({ enabled: undefined })
  })

  test('filters the plaza by language', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByLabelText('Language'))
    await user.click(await screen.findByRole('option', { name: 'Japanese' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ language: 'ja', page: 1 })
    )
  })

  test('creates a tone, published to the plaza by default', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'New Tone' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Tone Name'), '  新文风  ')
    await user.type(dialog.getByLabelText('Tone Prompt'), '  用短句。  ')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(toneApi.createTone).toHaveBeenCalledTimes(1))
    expect(toneApi.createTone.mock.calls[0][0]).toMatchObject({
      name: '新文风',
      prompt: '用短句。',
      enabled: true,
      sortOrder: 0,
      // The standard's recommended default flavour, not an empty value.
      tone: 'plain',
      sampleInput: '',
      sampleOutput: '',
    })
    expect(toneApi.updateTone).not.toHaveBeenCalled()
  })

  test('refuses to save a tone without a name', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'New Tone' }))
    // A prompt alone is not enough: the name is what an upsert matches on.
    await user.type(screen.getByLabelText('Tone Prompt'), 'Write plainly.')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Tone name is required')).toBeInTheDocument()
    expect(toneApi.createTone).not.toHaveBeenCalled()
  })

  test('refuses to save a tone without a prompt', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'New Tone' }))
    // A tone without an instruction is an empty style for every consumer, which
    // is why the backend rejects it — so the dialog must not let it through.
    await user.type(screen.getByLabelText('Tone Name'), 'No instruction')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Tone prompt is required')
    ).toBeInTheDocument()
    expect(toneApi.createTone).not.toHaveBeenCalled()
  })

  test('submits the vocabulary, scenes, language and sample pair typed into the form', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'New Tone' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Tone Name'), 'Business brief')
    await user.type(dialog.getByLabelText('Tone Prompt'), 'Keep it factual.')
    await user.click(dialog.getByLabelText('Category'))
    await user.click(await screen.findByRole('option', { name: 'Business' }))
    await user.click(dialog.getByLabelText('Tone'))
    await user.click(await screen.findByRole('option', { name: 'Calm' }))
    await user.type(dialog.getByLabelText('Language'), 'ZH')
    await user.type(
      dialog.getByLabelText('Suitable Scenes'),
      'Docs, Newsletter; Docs'
    )
    await user.type(dialog.getByLabelText('Sample Input'), 'plain sentence')
    await user.type(dialog.getByLabelText('Sample Output'), 'calm sentence')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(toneApi.createTone).toHaveBeenCalledTimes(1))
    expect(toneApi.createTone.mock.calls[0][0]).toMatchObject({
      name: 'Business brief',
      prompt: 'Keep it factual.',
      category: 'business',
      tone: 'calm',
      // The tag is normalized to lower case, matching the backend.
      language: 'zh',
      // Separators are normalized and duplicates dropped, matching the backend.
      scenes: ['Docs', 'Newsletter'],
      sampleInput: 'plain sentence',
      sampleOutput: 'calm sentence',
    })
  })

  test('keeps a new tone off the shelf when the switch is turned off', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'New Tone' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Tone Name'), 'Draft tone')
    await user.type(dialog.getByLabelText('Tone Prompt'), 'Draft instruction.')
    await user.click(dialog.getByRole('switch', { name: 'On Shelf' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(toneApi.createTone).toHaveBeenCalledTimes(1))
    expect(toneApi.createTone.mock.calls[0][0]).toMatchObject({
      name: 'Draft tone',
      enabled: false,
    })
  })

  test('edits a stored tone and submits the loaded values', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'Edit' }))

    expect(await screen.findByLabelText('Tone Name')).toHaveValue(
      'Warm storyteller'
    )
    expect(screen.getByLabelText('Tone Prompt')).toHaveValue(
      'Write in short declarative sentences.'
    )

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(toneApi.updateTone).toHaveBeenCalledTimes(1))
    expect(toneApi.updateTone.mock.calls[0][0]).toBe(1)
    expect(toneApi.updateTone.mock.calls[0][1]).toMatchObject({
      name: 'Warm storyteller',
      category: 'literary',
      tone: 'warm',
      prompt: 'Write in short declarative sentences.',
    })
    expect(toneApi.createTone).not.toHaveBeenCalled()
  })

  test('deletes a tone only after the confirmation is accepted', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(toneApi.deleteTone).not.toHaveBeenCalled()

    await user.click(await screen.findByRole('button', { name: 'Delete Tone' }))

    await waitFor(() => expect(toneApi.deleteTone).toHaveBeenCalledWith(1))
  })

  test('shows a sample pair side by side', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'Sample' }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText('Sample Input')).toBeInTheDocument()
    expect(dialog.getByText('Sample Output')).toBeInTheDocument()
    expect(dialog.getByText('The release ships today.')).toBeInTheDocument()
    expect(dialog.getByText('Today, the release ships.')).toBeInTheDocument()
    // The standard explains what the pair is for; the dialog shows its note.
    expect(
      dialog.getByText('The pair is the only way to see what a tone does.')
    ).toBeInTheDocument()
  })

  test('hides the sample button when both halves of the pair are empty', async () => {
    toneApi.listTones.mockResolvedValue(
      pageResult([
        storedTone(),
        storedTone({
          id: 2,
          name: 'No sample',
          sampleInput: '',
          sampleOutput: '',
        }),
      ])
    )

    renderPage()
    await screen.findByText('No sample')

    // Two rows, but only the one with a pair offers the button: a control that
    // can never do anything reads as broken.
    expect(screen.getAllByRole('button', { name: 'Sample' })).toHaveLength(1)
  })

  test('shows the version of the standard it is rendering from', async () => {
    renderPage()
    await screen.findByText('Warm storyteller')

    await waitFor(() =>
      expect(screen.getByText('Tone Standard v2.0.0')).toBeInTheDocument()
    )
  })

  test('falls back to the mirror, quietly, when the standard is unreachable', async () => {
    toneApi.fetchToneStandard.mockRejectedValue(new Error('offline'))

    renderPage()
    await screen.findByText('Warm storyteller')

    // The mirror's version is what is in force, and it says so.
    await waitFor(() =>
      expect(screen.getByText('Tone Standard v1.0.0')).toBeInTheDocument()
    )
    const chip = screen.getByText('Tone Standard v1.0.0')
    expect(chip.getAttribute('title')).toBe(
      'Built-in mirror: the standard endpoint is unreachable.'
    )

    // Quietly means quietly: no error toast, and the vocabulary still works.
    expect(screen.queryByText('Request failed')).toBeNull()
    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Category'))
    expect(
      await screen.findByRole('option', { name: 'Literary' })
    ).toBeInTheDocument()
  })

  test('uses the caps the standard reports, not the mirror defaults', async () => {
    const user = userEvent.setup()
    toneApi.fetchToneStandard.mockResolvedValue(
      standardPayload({ limits: { name: 4 } })
    )

    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'New Tone' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Tone Name'), 'toolong')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // The message quotes the cap the response published, so the dialog refuses
    // what the deployed API would refuse.
    expect(
      await screen.findByText('Tone name is too long (max 4 characters)')
    ).toBeInTheDocument()
    expect(toneApi.createTone).not.toHaveBeenCalled()
  })

  test('publishes both public endpoints to integrators', async () => {
    renderPage()
    await screen.findByText('Warm storyteller')

    expect(
      screen.getByText(
        'https://gw.example.com/api/zsy/tone/list?page=1&page_size=20'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText('https://gw.example.com/api/zsy/tone/standard')
    ).toBeInTheDocument()
  })

  test('exports the active selection as a CSV download', async () => {
    const user = userEvent.setup()
    const createObjectURL = vi.fn(() => 'blob:tones')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', {
      value: createObjectURL,
      configurable: true,
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      value: revokeObjectURL,
      configurable: true,
    })
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => {})

    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByLabelText('Category'))
    await user.click(await screen.findByRole('option', { name: 'Business' }))
    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ category: 'business' })
    )

    await user.click(screen.getByRole('button', { name: 'Export' }))

    await waitFor(() => expect(toneApi.exportTones).toHaveBeenCalledTimes(1))
    expect(toneApi.exportTones.mock.calls[0][0]).toMatchObject({
      keyword: '',
      category: 'business',
    })
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(click).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:tones')
  })

  test('imports a CSV and reports what happened per row', async () => {
    const user = userEvent.setup()
    toneApi.importTonesCsv.mockResolvedValue({
      total: 3,
      created: 1,
      updated: 1,
      failed: 1,
      errors: [{ row: 4, name: '坏文风', message: '非法类别 "robot"' }],
      warnings: ['已忽略无法识别的列: 备注列'],
    })

    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'Import' }))

    const file = new File(['name,prompt\n'], 'tones.csv', {
      type: 'text/csv',
    })
    await user.upload(await screen.findByLabelText('CSV File'), file)
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() =>
      expect(toneApi.importTonesCsv).toHaveBeenCalledTimes(1)
    )
    expect(toneApi.importTonesCsv.mock.calls[0][0]).toBe(file)
    expect(toneApi.importTonesCsv.mock.calls[0][1]).toBe('upsert')

    expect(await screen.findByText('Import Result')).toBeInTheDocument()
    expect(screen.getByText('Created: 1')).toBeInTheDocument()
    expect(screen.getByText('Updated: 1')).toBeInTheDocument()
    expect(screen.getByText('Failed: 1')).toBeInTheDocument()
    expect(screen.getByText('Row 4')).toBeInTheDocument()
    expect(screen.getByText(/非法类别/)).toBeInTheDocument()
    expect(screen.getByText('已忽略无法识别的列: 备注列')).toBeInTheDocument()
  })

  test('refuses to import before a file is chosen', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Warm storyteller')

    await user.click(screen.getByRole('button', { name: 'Import' }))
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() => expect(toneApi.importTonesCsv).not.toHaveBeenCalled())
  })
})
