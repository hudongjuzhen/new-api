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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { ResultPreviewDialog, ResultTile } = await import('../rh-portal')
const { extractResults } = await import('../../lib/result-media')
type RhResultItem = import('../../lib/result-media').RhResultItem

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const imageItem: RhResultItem = {
  key: 'https://files.rh.local/out.png',
  url: 'https://files.rh.local/out.png',
  kind: 'image',
  fileName: 'out.png',
}
const videoItem: RhResultItem = {
  key: 'https://files.rh.local/clip.mp4',
  url: 'https://files.rh.local/clip.mp4',
  kind: 'video',
  fileName: 'clip.mp4',
}
const audioItem: RhResultItem = {
  key: 'https://files.rh.local/voice.mp3',
  url: 'https://files.rh.local/voice.mp3',
  kind: 'audio',
  fileName: 'voice.mp3',
}
const archiveItem: RhResultItem = {
  key: 'https://files.rh.local/bundle.zip',
  url: 'https://files.rh.local/bundle.zip',
  kind: 'archive',
  fileName: 'bundle.zip',
}
const textItem: RhResultItem = {
  key: 'text:0',
  kind: 'text',
  text: 'upstream caption',
  fileName: '',
}

function renderWithProviders(node: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>{node}</QueryClientProvider>
    </I18nextProvider>
  )
}

describe('result tile rendering', () => {
  test('renders a thumbnail for images', () => {
    const { container } = renderWithProviders(
      <ResultTile item={imageItem} onOpen={() => undefined} />
    )
    const image = container.querySelector('img')
    expect(image).toHaveAttribute('src', imageItem.url)
  })

  test('keeps the list cheap: no media request before the dialog opens', () => {
    renderWithProviders(
      <ResultTile item={videoItem} onOpen={() => undefined} />
    )
    expect(
      screen.getByRole('button', { name: 'Preview video' })
    ).toBeInTheDocument()
    // Embedding a media element here would fire one request per record on every
    // list render, so the tile stays an icon.
    expect(document.querySelector('video')).toBeNull()
    expect(document.querySelector('audio')).toBeNull()
  })

  test('opens audio and text results in the inline preview', () => {
    renderWithProviders(
      <ResultTile item={audioItem} onOpen={() => undefined} />
    )
    expect(
      screen.getByRole('button', { name: 'Preview audio' })
    ).toBeInTheDocument()

    renderWithProviders(<ResultTile item={textItem} onOpen={() => undefined} />)
    expect(
      screen.getByRole('button', { name: 'Preview text' })
    ).toBeInTheDocument()
  })

  test('offers archives and unknown binaries as a download, not a preview', () => {
    renderWithProviders(
      <ResultTile item={archiveItem} onOpen={() => undefined} />
    )
    const link = screen.getByRole('link', {
      name: 'Download file: bundle.zip',
    })
    expect(link).toHaveAttribute('href', archiveItem.url)
    expect(link).toHaveAttribute('download', 'bundle.zip')
  })
})

describe('result preview dialog', () => {
  const items = [imageItem, videoItem, audioItem, archiveItem, textItem]

  // The dialog renders into a portal, so its content is queried on the document
  // rather than inside the render container.
  test('renders the current result with the control its type needs', () => {
    renderWithProviders(
      <ResultPreviewDialog
        open
        onOpenChange={() => undefined}
        items={items}
        taskId='task-1'
        initialIndex={0}
      />
    )

    // image
    expect(document.querySelector('img')).toHaveAttribute('src', imageItem.url)
    // video
    fireEvent.click(screen.getByRole('button', { name: 'Next Result' }))
    expect(document.querySelector('video')).toHaveAttribute(
      'src',
      videoItem.url
    )
    // audio
    fireEvent.click(screen.getByRole('button', { name: 'Next Result' }))
    expect(document.querySelector('audio')).toHaveAttribute(
      'src',
      audioItem.url
    )
  })

  test('explains that archives download instead of rendering', () => {
    renderWithProviders(
      <ResultPreviewDialog
        open
        onOpenChange={() => undefined}
        items={items}
        taskId='task-1'
        initialIndex={3}
      />
    )

    expect(
      screen.getByText(
        'This file type cannot be previewed. Download it to open.'
      )
    ).toBeInTheDocument()
    const download = screen.getByRole('link', { name: 'Download file' })
    expect(download).toHaveAttribute('href', archiveItem.url)
    expect(document.querySelector('video')).toBeNull()
  })

  test('shows inline text results without fetching a file', () => {
    renderWithProviders(
      <ResultPreviewDialog
        open
        onOpenChange={() => undefined}
        items={items}
        taskId='task-1'
        initialIndex={4}
      />
    )

    expect(document.querySelector('pre')).toHaveTextContent('upstream caption')
    expect(document.querySelector('img')).toBeNull()
  })

  test('navigates within the record and stops at its ends', () => {
    renderWithProviders(
      <ResultPreviewDialog
        open
        onOpenChange={() => undefined}
        items={items}
        taskId='task-1'
        initialIndex={0}
      />
    )

    expect(
      screen.getByRole('button', { name: 'Previous Result' })
    ).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next Result' }))
    expect(
      screen.getByRole('button', { name: 'Previous Result' })
    ).toBeEnabled()
    expect(screen.getByTestId('lightbox-counter')).toHaveTextContent('2 of 5')
  })
})

describe('result extraction feeds the renderers', () => {
  test('classifies a mixed RunningHub payload and keeps text results', () => {
    const results = extractResults({
      data: {
        results: [
          { url: 'https://files.rh.local/out.png' },
          { url: 'https://files.rh.local/clip.mp4' },
          { url: 'https://files.rh.local/voice.mp3' },
          { url: 'https://files.rh.local/bundle.zip' },
          { url: 'https://files.rh.local/log.txt' },
          { text: 'inline caption' },
        ],
      },
    } as never)

    expect(results.map((item) => item.kind)).toEqual([
      'image',
      'video',
      'audio',
      'archive',
      'text',
      'text',
    ])
    expect(results[4].fileName).toBe('log.txt')
    expect(results[5].text).toBe('inline caption')
  })
})
