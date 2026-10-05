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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const voiceApi = vi.hoisted(() => ({
  listVoices: vi.fn(),
  createVoice: vi.fn(),
  updateVoice: vi.fn(),
  deleteVoice: vi.fn(),
  uploadVoiceAudio: vi.fn(),
  exportVoices: vi.fn(),
  importVoicesCsv: vi.fn(),
}))

// Only the network calls are replaced: the URL helpers stay real so the
// published-endpoint assertion protects the actual path.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  ...voiceApi,
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
const { VoicePlazaPage } = await import('../voice-plaza-page')

type VoiceView = import('../../api').VoiceView

// The assertions below query the English source strings the components pass
// through t(); registering them as identity translations keeps that contract
// explicit, including the interpolated ones.
const identity: Record<string, string> = {}
for (const key of [
  'Voice Plaza',
  'New Voice',
  'Edit Voice',
  'Delete Voice',
  'Voice Name',
  'Introduction',
  'Audio Sample',
  'Status',
  'Sort Order',
  'Actions',
  'Attributes',
  'Gender',
  'Age Range',
  'Suitable Scenes',
  'Language',
  'All languages',
  'Avatar',
  'Avatar preview',
  'Not specified',
  'Female',
  'Male',
  'Middle-aged',
  'Separate scenes with a comma; at most 8 scenes.',
  'Import',
  'Export',
  'Export started',
  'Import Voices',
  'Import Mode',
  'Create and update by name',
  'Create only',
  'CSV File',
  'Importing...',
  'Close',
  'Import Result',
  'Import failed',
  'Choose a CSV file',
  'Created: {{count}}',
  'Updated: {{count}}',
  'Failed: {{count}}',
  'Row {{row}}',
  'Imported: {{created}} created, {{updated}} updated, {{failed}} failed',
  'On Shelf',
  'Off Shelf',
  'All',
  'Shelf Status',
  'Page size',
  'Search',
  'Search by name or voice_type',
  'No voices yet',
  'No audio sample',
  'Edit',
  'Delete',
  'Cancel',
  'Save',
  'Saving...',
  'Previous',
  'Next',
  'Voice List API',
  'Total {{count}} voices',
  'Page {{page}} / {{total}}',
  '{{count}} per page',
]) {
  identity[key] = key
}

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: identity } },
})

function storedVoice(overrides: Partial<VoiceView> = {}): VoiceView {
  return {
    id: 1,
    createdAt: 1700000000,
    updatedAt: 1700000000,
    name: 'Sweet Female',
    description: 'Warm and calm',
    voiceType: 'zh_female_xiaomei',
    gender: 'female',
    ageRange: 'young',
    language: 'zh',
    scenes: ['Customer service', 'Audiobook'],
    avatarUrl: 'https://cdn.example.com/avatar/a.png',
    audioUrl: '/uploads/voices/202601/a.mp3',
    audioName: 'a.mp3',
    audioSize: 2048,
    enabled: true,
    sortOrder: 0,
    ...overrides,
  }
}

function pageResult(
  items: VoiceView[],
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

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <VoicePlazaPage />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

/** Parameters of the n-th listVoices call. */
function listCall(index = 0) {
  return voiceApi.listVoices.mock.calls[index][0] as Record<string, unknown>
}

beforeEach(() => {
  voiceApi.listVoices.mockResolvedValue(pageResult([storedVoice()]))
  voiceApi.createVoice.mockResolvedValue(storedVoice({ id: 2 }))
  voiceApi.updateVoice.mockResolvedValue(storedVoice())
  voiceApi.deleteVoice.mockResolvedValue(undefined)
  voiceApi.uploadVoiceAudio.mockResolvedValue({
    url: '/uploads/voices/202601/b.mp3',
    filename: 'b.mp3',
    originalName: 'b.mp3',
    size: 4096,
    mimeType: 'audio/mpeg',
  })
  voiceApi.exportVoices.mockResolvedValue({
    blob: new Blob(['\ufeffname,voice_type\n']),
    filename: 'zsy-voices-20260101-000000.csv',
  })
  voiceApi.importVoicesCsv.mockResolvedValue({
    total: 2,
    created: 1,
    updated: 1,
    failed: 0,
    errors: [],
  })
})

describe('Voice Plaza admin page', () => {
  test('lists the voices the API returns, with their sample audio', async () => {
    voiceApi.listVoices.mockResolvedValue(
      pageResult([
        storedVoice(),
        storedVoice({
          id: 2,
          name: 'Narrator',
          voiceType: 'zh_male_narrator',
          audioUrl: '',
          audioSize: 0,
          enabled: false,
        }),
      ])
    )

    renderPage()

    expect(await screen.findByText('Sweet Female')).toBeInTheDocument()
    expect(screen.getByText('zh_female_xiaomei')).toBeInTheDocument()
    expect(screen.getByText('Narrator')).toBeInTheDocument()
    expect(screen.getByText('On Shelf')).toBeInTheDocument()
    expect(screen.getByText('Off Shelf')).toBeInTheDocument()
    expect(screen.getAllByLabelText('Audio Sample')).toHaveLength(1)
    expect(screen.getByText('No audio sample')).toBeInTheDocument()
  })

  test('shows the empty state when the plaza has no voices', async () => {
    voiceApi.listVoices.mockResolvedValue(pageResult([]))

    renderPage()

    expect(await screen.findByText('No voices yet')).toBeInTheDocument()
    expect(screen.getByText('Total 0 voices')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  test('requests the next page and keeps the active filters', async () => {
    const user = userEvent.setup()
    voiceApi.listVoices.mockResolvedValue(
      pageResult([storedVoice()], { total: 40, totalPages: 2 })
    )

    renderPage()
    await screen.findByText('Sweet Female')

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
    await screen.findByText('Sweet Female')

    await user.type(
      screen.getByLabelText('Search by name or voice_type'),
      'xiaomei'
    )
    await user.click(screen.getByRole('button', { name: 'Search' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ keyword: 'xiaomei' })
    )
    expect(listCall(1)).toMatchObject({ page: 1 })
  })

  test('filters the list down to off-shelf voices', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByLabelText('Shelf Status'))
    await user.click(await screen.findByRole('option', { name: 'Off Shelf' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ enabled: false, page: 1 })
    )
  })

  test('creates a voice, published to the plaza by default', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'New Voice' }))
    await user.type(screen.getByLabelText('Voice Name'), '  新音色  ')
    await user.type(screen.getByLabelText('voice_type'), 'zh_female_new')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(voiceApi.createVoice).toHaveBeenCalledTimes(1))
    expect(voiceApi.createVoice.mock.calls[0][0]).toMatchObject({
      name: '新音色',
      voiceType: 'zh_female_new',
      enabled: true,
      sortOrder: 0,
      audioUrl: '',
    })
    expect(voiceApi.updateVoice).not.toHaveBeenCalled()
  })

  test('refuses to save a voice without a name', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'New Voice' }))
    await user.type(screen.getByLabelText('voice_type'), 'zh_female_new')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Voice name is required')
    ).toBeInTheDocument()
    expect(voiceApi.createVoice).not.toHaveBeenCalled()
  })

  test('keeps a new voice off the shelf when the switch is turned off', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'New Voice' }))
    await user.type(screen.getByLabelText('Voice Name'), 'Draft Voice')
    await user.type(screen.getByLabelText('voice_type'), 'zh_male_draft')
    await user.click(screen.getByRole('switch', { name: 'On Shelf' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(voiceApi.createVoice).toHaveBeenCalledTimes(1))
    expect(voiceApi.createVoice.mock.calls[0][0]).toMatchObject({
      name: 'Draft Voice',
      enabled: false,
    })
  })

  test('edits a stored voice and submits the loaded values', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'Edit' }))

    expect(await screen.findByLabelText('Voice Name')).toHaveValue(
      'Sweet Female'
    )
    expect(screen.getByLabelText('voice_type')).toHaveValue('zh_female_xiaomei')

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(voiceApi.updateVoice).toHaveBeenCalledTimes(1))
    expect(voiceApi.updateVoice.mock.calls[0][0]).toBe(1)
    expect(voiceApi.updateVoice.mock.calls[0][1]).toMatchObject({
      name: 'Sweet Female',
      voiceType: 'zh_female_xiaomei',
      audioUrl: '/uploads/voices/202601/a.mp3',
    })
    expect(voiceApi.createVoice).not.toHaveBeenCalled()
  })

  test('deletes a voice only after the confirmation is accepted', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(voiceApi.deleteVoice).not.toHaveBeenCalled()

    await user.click(
      await screen.findByRole('button', { name: 'Delete Voice' })
    )

    await waitFor(() => expect(voiceApi.deleteVoice).toHaveBeenCalledWith(1))
  })

  test('publishes the public list endpoint to integrators', async () => {
    renderPage()
    await screen.findByText('Sweet Female')

    expect(
      screen.getByText(
        'https://gw.example.com/api/zsy/voice/list?page=1&page_size=20'
      )
    ).toBeInTheDocument()
  })

  test('shows the demographic attributes and scenes of each voice', async () => {
    renderPage()
    await screen.findByText('Sweet Female')

    expect(screen.getByText('Female')).toBeInTheDocument()
    expect(screen.getByText('Young')).toBeInTheDocument()
    expect(screen.getByText('Customer service')).toBeInTheDocument()
    expect(screen.getByText('Audiobook')).toBeInTheDocument()
  })

  test('shows a placeholder when a voice has no attributes at all', async () => {
    voiceApi.listVoices.mockResolvedValue(
      pageResult([
        storedVoice({
          gender: '',
          ageRange: '',
          language: '',
          scenes: [],
          name: 'Bare Voice',
        }),
      ])
    )

    renderPage()
    await screen.findByText('Bare Voice')

    expect(screen.getByText('Not specified')).toBeInTheDocument()
  })

  test('filters the plaza by gender and age range', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByLabelText('Gender'))
    await user.click(await screen.findByRole('option', { name: 'Male' }))
    await waitFor(() => expect(listCall(1)).toMatchObject({ gender: 'male' }))
    expect(listCall(1)).toMatchObject({ page: 1 })

    await user.click(screen.getByLabelText('Age Range'))
    await user.click(await screen.findByRole('option', { name: 'Senior' }))
    await waitFor(() =>
      expect(listCall(2)).toMatchObject({ ageRange: 'senior' })
    )
    expect(listCall(2)).toMatchObject({ gender: 'male' })
  })

  test('submits the attributes typed into the form', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'New Voice' }))
    // The filter row carries the same labels as the form, so the form fields
    // are queried inside the dialog.
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Voice Name'), 'Narrator')
    await user.type(dialog.getByLabelText('voice_type'), 'zh_male_narrator')
    await user.click(dialog.getByLabelText('Gender'))
    await user.click(await screen.findByRole('option', { name: 'Male' }))
    await user.click(dialog.getByLabelText('Age Range'))
    await user.click(await screen.findByRole('option', { name: 'Middle-aged' }))
    await user.type(dialog.getByLabelText('Language'), 'ZH')
    await user.type(
      dialog.getByLabelText('Suitable Scenes'),
      'Audiobook, News; Audiobook'
    )
    await user.type(
      dialog.getByLabelText('Avatar'),
      'https://cdn.example.com/avatar/narrator.png'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(voiceApi.createVoice).toHaveBeenCalledTimes(1))
    expect(voiceApi.createVoice.mock.calls[0][0]).toMatchObject({
      name: 'Narrator',
      gender: 'male',
      ageRange: 'middle',
      // The tag is normalized to lower case, matching the backend.
      language: 'zh',
      avatarUrl: 'https://cdn.example.com/avatar/narrator.png',
      // Separators are normalized and duplicates dropped, matching the backend.
      scenes: ['Audiobook', 'News'],
    })
  })

  test('shows the avatar and the localized language of each voice', async () => {
    renderPage()
    await screen.findByText('Sweet Female')

    const avatar = document.querySelector(
      'img[src="https://cdn.example.com/avatar/a.png"]'
    )
    expect(avatar).not.toBeNull()
    // Intl renders the language tag, so no per-language translation key exists.
    expect(screen.getByText('Chinese')).toBeInTheDocument()
  })

  test('filters the plaza by language', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByLabelText('Language'))
    await user.click(await screen.findByRole('option', { name: 'Japanese' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ language: 'ja', page: 1 })
    )
  })

  test('exports the active selection as a CSV download', async () => {
    const user = userEvent.setup()
    const createObjectURL = vi.fn(() => 'blob:voices')
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
    await screen.findByText('Sweet Female')

    await user.click(screen.getByLabelText('Gender'))
    await user.click(await screen.findByRole('option', { name: 'Male' }))
    await waitFor(() => expect(listCall(1)).toMatchObject({ gender: 'male' }))

    await user.click(screen.getByRole('button', { name: 'Export' }))

    await waitFor(() => expect(voiceApi.exportVoices).toHaveBeenCalledTimes(1))
    expect(voiceApi.exportVoices.mock.calls[0][0]).toMatchObject({
      keyword: '',
      gender: 'male',
    })
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(click).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:voices')
  })

  test('imports a CSV and reports what happened per row', async () => {
    const user = userEvent.setup()
    voiceApi.importVoicesCsv.mockResolvedValue({
      total: 3,
      created: 1,
      updated: 1,
      failed: 1,
      errors: [{ row: 4, name: '坏音色', message: '非法性别 "robot"' }],
      warnings: ['已忽略无法识别的列: 备注列'],
    })

    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'Import' }))

    const file = new File(['name,voice_type\n'], 'voices.csv', {
      type: 'text/csv',
    })
    await user.upload(await screen.findByLabelText('CSV File'), file)
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() =>
      expect(voiceApi.importVoicesCsv).toHaveBeenCalledTimes(1)
    )
    expect(voiceApi.importVoicesCsv.mock.calls[0][0]).toBe(file)
    expect(voiceApi.importVoicesCsv.mock.calls[0][1]).toBe('upsert')

    expect(await screen.findByText('Import Result')).toBeInTheDocument()
    expect(screen.getByText('Created: 1')).toBeInTheDocument()
    expect(screen.getByText('Updated: 1')).toBeInTheDocument()
    expect(screen.getByText('Failed: 1')).toBeInTheDocument()
    expect(screen.getByText('Row 4')).toBeInTheDocument()
    expect(screen.getByText(/非法性别/)).toBeInTheDocument()
    expect(screen.getByText('已忽略无法识别的列: 备注列')).toBeInTheDocument()
  })

  test('refuses to import before a file is chosen', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('Sweet Female')

    await user.click(screen.getByRole('button', { name: 'Import' }))
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() => expect(voiceApi.importVoicesCsv).not.toHaveBeenCalled())
  })
})
