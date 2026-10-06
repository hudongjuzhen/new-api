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

const avatarApi = vi.hoisted(() => ({
  listAvatars: vi.fn(),
  createAvatar: vi.fn(),
  updateAvatar: vi.fn(),
  deleteAvatar: vi.fn(),
  exportAvatars: vi.fn(),
  importAvatarsCsv: vi.fn(),
}))

// Only the network calls are replaced: the URL helpers stay real so the
// published-endpoint assertion protects the actual path.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  ...avatarApi,
}))

vi.mock('@/features/playground/api', () => ({
  uploadPlaygroundImage: vi.fn(),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: { server_address: 'https://gw.example.com' },
    loading: false,
    error: null,
  }),
}))

const { uploadPlaygroundImage } = await import('@/features/playground/api')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { AvatarPlazaPage } = await import('../avatar-plaza-page')

type AvatarView = import('../../api').AvatarView

// The assertions below query the English source strings the components pass
// through t(); registering them as identity translations keeps that contract
// explicit, including the interpolated ones.
const identity: Record<string, string> = {}
for (const key of [
  'Avatar Plaza',
  'New Avatar',
  'Edit Avatar',
  'Delete Avatar',
  'Avatar Name',
  'Introduction',
  'Cover Image',
  'Full-body Photo',
  'Four Views',
  'Expression Sheet',
  'Image preview',
  'No image',
  'Upload image',
  'Uploading...',
  'Remove image',
  'Voice ID',
  'Voice Sample',
  'Status',
  'Sort Order',
  'Actions',
  'Attributes',
  'Gender',
  'Age Range',
  'Race',
  'Suitable Scenes',
  'Not specified',
  'Female',
  'Male',
  'Asian',
  'Black',
  'Mixed',
  'On Shelf',
  'Off Shelf',
  'All',
  'Shelf Status',
  'Page size',
  'Search',
  'Search by name or voice id',
  'No avatars yet',
  'No voice linked',
  'No sample audio for this voice',
  'Edit',
  'Delete',
  'Cancel',
  'Save',
  'Saving...',
  'Previous',
  'Next',
  'Avatar List API',
  'Import',
  'Export',
  'Export started',
  'Import Avatars',
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
  'Total {{count}} avatars',
  'Page {{page}} / {{total}}',
  '{{count}} per page',
  'Avatar name is required',
]) {
  identity[key] = key
}

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: identity } },
})

function storedAvatar(overrides: Partial<AvatarView> = {}): AvatarView {
  return {
    id: 1,
    createdAt: 1700000000,
    updatedAt: 1700000000,
    name: '客服小雨',
    description: '温柔的客服形象',
    imageUrl: 'https://cdn.example.com/x.png',
    fullBodyUrl: 'https://cdn.example.com/x-full.png',
    fourViewUrl: 'https://cdn.example.com/x-four-view.png',
    expressionUrl: 'https://cdn.example.com/x-expression.png',
    gender: 'female',
    ageRange: 'young',
    race: 'asian',
    scenes: ['客服播报', '有声书'],
    voiceId: 'zh_female_vv_uranus_bigtts',
    voiceAvailable: true,
    voiceName: 'Vivi 2.0',
    voiceSampleUrl: '/uploads/voices/202601/vivi.wav',
    voiceSampleName: 'vivi.wav',
    enabled: true,
    sortOrder: 0,
    ...overrides,
  }
}

function pageResult(
  items: AvatarView[],
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
        <AvatarPlazaPage />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

/** Parameters of the n-th listAvatars call. */
function listCall(index = 0) {
  return avatarApi.listAvatars.mock.calls[index][0] as Record<string, unknown>
}

beforeEach(() => {
  vi.mocked(uploadPlaygroundImage).mockReset()
  avatarApi.listAvatars.mockResolvedValue(pageResult([storedAvatar()]))
  avatarApi.createAvatar.mockResolvedValue(storedAvatar({ id: 2 }))
  avatarApi.updateAvatar.mockResolvedValue(storedAvatar())
  avatarApi.deleteAvatar.mockResolvedValue(undefined)
  avatarApi.exportAvatars.mockResolvedValue({
    blob: new Blob(['\ufeffname,image_url\n']),
    filename: 'zsy-avatars-20260101-000000.csv',
  })
  avatarApi.importAvatarsCsv.mockResolvedValue({
    total: 2,
    created: 1,
    updated: 1,
    failed: 0,
    errors: [],
  })
})

describe('Avatar Plaza admin page', () => {
  test('lists the personas the API returns, with their linked voice sample', async () => {
    avatarApi.listAvatars.mockResolvedValue(
      pageResult([
        storedAvatar(),
        storedAvatar({
          id: 2,
          name: '无音色形象',
          voiceId: '',
          voiceAvailable: false,
          voiceName: '',
          voiceSampleUrl: '',
          enabled: false,
        }),
      ])
    )

    renderPage()

    expect(await screen.findByText('客服小雨')).toBeInTheDocument()
    expect(screen.getByText('zh_female_vv_uranus_bigtts')).toBeInTheDocument()
    expect(screen.getByText('无音色形象')).toBeInTheDocument()
    expect(screen.getByText('On Shelf')).toBeInTheDocument()
    expect(screen.getByText('Off Shelf')).toBeInTheDocument()
    expect(screen.getAllByLabelText('Voice Sample')).toHaveLength(1)
    // The row reports the missing link twice: under the name and in the voice
    // sample column.
    expect(screen.getAllByText('No voice linked')).toHaveLength(2)
  })

  test('shows the empty state when the plaza has no personas', async () => {
    avatarApi.listAvatars.mockResolvedValue(pageResult([]))

    renderPage()

    expect(await screen.findByText('No avatars yet')).toBeInTheDocument()
    expect(screen.getByText('Total 0 avatars')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  test('requests the next page and keeps the active filters', async () => {
    const user = userEvent.setup()
    avatarApi.listAvatars.mockResolvedValue(
      pageResult([storedAvatar()], { total: 40, totalPages: 2 })
    )

    renderPage()
    await screen.findByText('客服小雨')

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
    await screen.findByText('客服小雨')

    await user.type(
      screen.getByLabelText('Search by name or voice id'),
      'uranus'
    )
    await user.click(screen.getByRole('button', { name: 'Search' }))

    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ keyword: 'uranus' })
    )
    expect(listCall(1)).toMatchObject({ page: 1 })
  })

  test('filters the plaza by shelf, gender, age range and race', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByLabelText('Shelf Status'))
    await user.click(await screen.findByRole('option', { name: 'Off Shelf' }))
    await waitFor(() =>
      expect(listCall(1)).toMatchObject({ enabled: false, page: 1 })
    )

    await user.click(screen.getByLabelText('Gender'))
    await user.click(await screen.findByRole('option', { name: 'Male' }))
    await waitFor(() => expect(listCall(2)).toMatchObject({ gender: 'male' }))

    await user.click(screen.getByLabelText('Age Range'))
    await user.click(await screen.findByRole('option', { name: 'Senior' }))
    await waitFor(() =>
      expect(listCall(3)).toMatchObject({ ageRange: 'senior' })
    )
    expect(listCall(3)).toMatchObject({ enabled: false, gender: 'male' })

    await user.click(screen.getByLabelText('Race'))
    await user.click(await screen.findByRole('option', { name: 'Black' }))
    await waitFor(() => expect(listCall(4)).toMatchObject({ race: 'black' }))
    expect(listCall(4)).toMatchObject({
      ageRange: 'senior',
      gender: 'male',
      enabled: false,
    })
  })

  test('publishes the public list endpoint to integrators', async () => {
    renderPage()
    await screen.findByText('客服小雨')

    expect(
      screen.getByText(
        'https://gw.example.com/api/zsy/avatar/list?page=1&page_size=20'
      )
    ).toBeInTheDocument()
  })

  test('shows the demographic attributes and scenes of each persona', async () => {
    renderPage()
    await screen.findByText('客服小雨')

    expect(screen.getByText('Female')).toBeInTheDocument()
    expect(screen.getByText('Young')).toBeInTheDocument()
    expect(screen.getByText('Asian')).toBeInTheDocument()
    expect(screen.getByText('客服播报')).toBeInTheDocument()
    expect(screen.getByText('有声书')).toBeInTheDocument()
  })

  test('shows a placeholder when a persona has no attributes at all', async () => {
    avatarApi.listAvatars.mockResolvedValue(
      pageResult([
        storedAvatar({
          name: 'Bare Avatar',
          gender: '',
          ageRange: '',
          race: '',
          scenes: [],
        }),
      ])
    )

    renderPage()
    await screen.findByText('Bare Avatar')

    expect(screen.getByText('Not specified')).toBeInTheDocument()
  })

  test('renders the uploaded picture of each persona', async () => {
    renderPage()
    await screen.findByText('客服小雨')

    expect(
      document.querySelector('img[src="https://cdn.example.com/x.png"]')
    ).not.toBeNull()
  })

  test('falls back to a placeholder when the picture cannot be loaded', async () => {
    renderPage()
    await screen.findByText('客服小雨')

    const image = document.querySelector(
      'img[src="https://cdn.example.com/x.png"]'
    )
    expect(image).not.toBeNull()

    // A blocked or dead picture URL must not leave a broken image in the row.
    fireEvent.error(image as Element)

    await waitFor(() =>
      expect(
        document.querySelector('img[src="https://cdn.example.com/x.png"]')
      ).toBeNull()
    )
  })

  test('shows the reference pictures a persona carries beside its cover', async () => {
    renderPage()
    await screen.findByText('客服小雨')

    expect(screen.getByAltText('Full-body Photo')).toHaveAttribute(
      'src',
      'https://cdn.example.com/x-full.png'
    )
    expect(screen.getByAltText('Four Views')).toHaveAttribute(
      'src',
      'https://cdn.example.com/x-four-view.png'
    )
    expect(screen.getByAltText('Expression Sheet')).toHaveAttribute(
      'src',
      'https://cdn.example.com/x-expression.png'
    )
  })

  test('shows no reference strip for a persona that only has a cover', async () => {
    avatarApi.listAvatars.mockResolvedValue(
      pageResult([
        storedAvatar({
          name: 'Cover Only',
          fullBodyUrl: '',
          fourViewUrl: '',
          expressionUrl: '',
        }),
      ])
    )

    renderPage()
    await screen.findByText('Cover Only')

    expect(screen.queryByAltText('Full-body Photo')).not.toBeInTheDocument()
    expect(screen.queryByAltText('Four Views')).not.toBeInTheDocument()
    expect(screen.queryByAltText('Expression Sheet')).not.toBeInTheDocument()
  })

  test('drops a broken reference picture without hiding the cover', async () => {
    renderPage()
    await screen.findByText('客服小雨')

    fireEvent.error(screen.getByAltText('Four Views'))

    await waitFor(() =>
      expect(screen.queryByAltText('Four Views')).not.toBeInTheDocument()
    )
    expect(screen.getByAltText('Full-body Photo')).toBeInTheDocument()
    expect(
      document.querySelector('img[src="https://cdn.example.com/x.png"]')
    ).not.toBeNull()
  })

  test('explains a persona whose voice is not in the Voice Plaza', async () => {
    avatarApi.listAvatars.mockResolvedValue(
      pageResult([
        storedAvatar({
          voiceAvailable: false,
          voiceName: '',
          voiceSampleUrl: '',
        }),
      ])
    )

    renderPage()
    await screen.findByText('客服小雨')

    expect(
      screen.getByText('No sample audio for this voice')
    ).toBeInTheDocument()
  })

  test('creates a persona, published to the plaza by default', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'New Avatar' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Avatar Name'), '  新形象  ')
    await user.type(
      dialog.getByLabelText('Voice ID'),
      'zh_female_vv_uranus_bigtts'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(avatarApi.createAvatar).toHaveBeenCalledTimes(1))
    expect(avatarApi.createAvatar.mock.calls[0][0]).toMatchObject({
      name: '新形象',
      voiceId: 'zh_female_vv_uranus_bigtts',
      enabled: true,
      sortOrder: 0,
      imageUrl: '',
      fullBodyUrl: '',
      fourViewUrl: '',
      expressionUrl: '',
    })
    expect(avatarApi.updateAvatar).not.toHaveBeenCalled()
  })

  test('refuses to save a persona without a name', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'New Avatar' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Avatar name is required')
    ).toBeInTheDocument()
    expect(avatarApi.createAvatar).not.toHaveBeenCalled()
  })

  test('submits the attributes and scenes typed into the form', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'New Avatar' }))
    // The filter row carries the same labels as the form, so the form fields are
    // queried inside the dialog.
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Avatar Name'), 'Narrator')
    await user.click(dialog.getByLabelText('Gender'))
    await user.click(await screen.findByRole('option', { name: 'Male' }))
    await user.click(dialog.getByLabelText('Age Range'))
    await user.click(await screen.findByRole('option', { name: 'Middle-aged' }))
    await user.click(dialog.getByLabelText('Race'))
    await user.click(await screen.findByRole('option', { name: 'Mixed' }))
    await user.type(
      dialog.getByLabelText('Suitable Scenes'),
      'Audiobook, News; Audiobook'
    )
    await user.type(
      dialog.getByLabelText('Cover Image'),
      'https://cdn.example.com/narrator.png'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(avatarApi.createAvatar).toHaveBeenCalledTimes(1))
    expect(avatarApi.createAvatar.mock.calls[0][0]).toMatchObject({
      name: 'Narrator',
      gender: 'male',
      ageRange: 'middle',
      race: 'mixed',
      imageUrl: 'https://cdn.example.com/narrator.png',
      // Separators are normalized and duplicates dropped, matching the backend.
      scenes: ['Audiobook', 'News'],
    })
  })

  test('submits the three reference pictures typed into the form', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'New Avatar' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Avatar Name'), 'Reference Set')
    await user.type(
      dialog.getByLabelText('Full-body Photo'),
      'https://cdn.example.com/full.png'
    )
    await user.type(
      dialog.getByLabelText('Four Views'),
      'https://cdn.example.com/four-view.png'
    )
    await user.type(
      dialog.getByLabelText('Expression Sheet'),
      'https://cdn.example.com/expression.png'
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(avatarApi.createAvatar).toHaveBeenCalledTimes(1))
    expect(avatarApi.createAvatar.mock.calls[0][0]).toMatchObject({
      fullBodyUrl: 'https://cdn.example.com/full.png',
      fourViewUrl: 'https://cdn.example.com/four-view.png',
      expressionUrl: 'https://cdn.example.com/expression.png',
    })
  })

  test('loads every picture of a stored persona into the edit form', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const dialog = within(await screen.findByRole('dialog'))

    expect(dialog.getByLabelText('Cover Image')).toHaveValue(
      'https://cdn.example.com/x.png'
    )
    expect(dialog.getByLabelText('Full-body Photo')).toHaveValue(
      'https://cdn.example.com/x-full.png'
    )
    expect(dialog.getByLabelText('Four Views')).toHaveValue(
      'https://cdn.example.com/x-four-view.png'
    )
    expect(dialog.getByLabelText('Expression Sheet')).toHaveValue(
      'https://cdn.example.com/x-expression.png'
    )
  })

  test('uploads the picked picture and submits its stored URL', async () => {
    const user = userEvent.setup()
    vi.mocked(uploadPlaygroundImage).mockResolvedValue(
      '/uploads/images/202601/abc.png'
    )
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'New Avatar' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Avatar Name'), 'Uploaded')
    await user.upload(
      dialog.getByLabelText('Upload image · Cover Image'),
      new File(['x'], 'portrait.png', { type: 'image/png' })
    )

    await waitFor(() => expect(uploadPlaygroundImage).toHaveBeenCalledTimes(1))
    expect(dialog.getByLabelText('Cover Image')).toHaveValue(
      '/uploads/images/202601/abc.png'
    )

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(avatarApi.createAvatar).toHaveBeenCalledTimes(1))
    expect(avatarApi.createAvatar.mock.calls[0][0]).toMatchObject({
      imageUrl: '/uploads/images/202601/abc.png',
    })
  })

  test('keeps a new persona off the shelf when the switch is turned off', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'New Avatar' }))
    const dialog = within(await screen.findByRole('dialog'))
    await user.type(dialog.getByLabelText('Avatar Name'), 'Draft Avatar')
    await user.click(screen.getByRole('switch', { name: 'On Shelf' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(avatarApi.createAvatar).toHaveBeenCalledTimes(1))
    expect(avatarApi.createAvatar.mock.calls[0][0]).toMatchObject({
      name: 'Draft Avatar',
      enabled: false,
    })
  })

  test('edits a stored persona and submits the loaded values', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'Edit' }))

    expect(await screen.findByLabelText('Avatar Name')).toHaveValue('客服小雨')
    expect(screen.getByLabelText('Voice ID')).toHaveValue(
      'zh_female_vv_uranus_bigtts'
    )
    // The sample audio of the linked voice is shown while editing.
    expect(screen.getAllByLabelText('Voice Sample').length).toBeGreaterThan(0)

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(avatarApi.updateAvatar).toHaveBeenCalledTimes(1))
    expect(avatarApi.updateAvatar.mock.calls[0][0]).toBe(1)
    expect(avatarApi.updateAvatar.mock.calls[0][1]).toMatchObject({
      name: '客服小雨',
      race: 'asian',
      voiceId: 'zh_female_vv_uranus_bigtts',
      scenes: ['客服播报', '有声书'],
    })
    expect(avatarApi.updateAvatar.mock.calls[0][1]).not.toHaveProperty(
      'voiceSampleUrl'
    )
    expect(avatarApi.createAvatar).not.toHaveBeenCalled()
  })

  test('tells the operator when the edited voice id has no catalogued sample', async () => {
    const user = userEvent.setup()
    avatarApi.listAvatars.mockResolvedValue(
      pageResult([
        storedAvatar({
          voiceAvailable: false,
          voiceName: '',
          voiceSampleUrl: '',
        }),
      ])
    )
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'Edit' }))

    expect(
      await screen.findByText(
        'This voice id is not in the Voice Plaza, so no sample audio is available.'
      )
    ).toBeInTheDocument()
  })

  test('deletes a persona only after the confirmation is accepted', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(avatarApi.deleteAvatar).not.toHaveBeenCalled()

    await user.click(
      await screen.findByRole('button', { name: 'Delete Avatar' })
    )

    await waitFor(() => expect(avatarApi.deleteAvatar).toHaveBeenCalledWith(1))
  })

  test('exports the active selection as a CSV download', async () => {
    const user = userEvent.setup()
    const createObjectURL = vi.fn(() => 'blob:avatars')
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
    await screen.findByText('客服小雨')

    await user.click(screen.getByLabelText('Race'))
    await user.click(await screen.findByRole('option', { name: 'Asian' }))
    await waitFor(() => expect(listCall(1)).toMatchObject({ race: 'asian' }))

    await user.click(screen.getByRole('button', { name: 'Export' }))

    await waitFor(() =>
      expect(avatarApi.exportAvatars).toHaveBeenCalledTimes(1)
    )
    expect(avatarApi.exportAvatars.mock.calls[0][0]).toMatchObject({
      keyword: '',
      race: 'asian',
    })
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(click).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:avatars')
  })

  test('imports a CSV and reports what happened per row', async () => {
    const user = userEvent.setup()
    avatarApi.importAvatarsCsv.mockResolvedValue({
      total: 3,
      created: 1,
      updated: 1,
      failed: 1,
      errors: [{ row: 4, name: '坏形象', message: '非法种族 "martian"' }],
      warnings: ['已忽略无法识别的列: 备注列'],
    })

    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'Import' }))

    const file = new File(['name,image_url\n'], 'avatars.csv', {
      type: 'text/csv',
    })
    await user.upload(await screen.findByLabelText('CSV File'), file)
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() =>
      expect(avatarApi.importAvatarsCsv).toHaveBeenCalledTimes(1)
    )
    expect(avatarApi.importAvatarsCsv.mock.calls[0][0]).toBe(file)
    expect(avatarApi.importAvatarsCsv.mock.calls[0][1]).toBe('upsert')

    expect(await screen.findByText('Import Result')).toBeInTheDocument()
    expect(screen.getByText('Created: 1')).toBeInTheDocument()
    expect(screen.getByText('Updated: 1')).toBeInTheDocument()
    expect(screen.getByText('Failed: 1')).toBeInTheDocument()
    expect(screen.getByText('Row 4')).toBeInTheDocument()
    expect(screen.getByText(/非法种族/)).toBeInTheDocument()
    expect(screen.getByText('已忽略无法识别的列: 备注列')).toBeInTheDocument()
  })

  test('refuses to import before a file is chosen', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('客服小雨')

    await user.click(screen.getByRole('button', { name: 'Import' }))
    await user.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() =>
      expect(avatarApi.importAvatarsCsv).not.toHaveBeenCalled()
    )
  })
})
