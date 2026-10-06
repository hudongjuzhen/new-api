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
import { useState } from 'react'
import { toast } from 'sonner'
import { describe, expect, test, vi } from 'vitest'

import { CoverImageField } from '../cover-image-field'

const COVER_INPUT_LABEL = 'Cover Image'
const FILE_INPUT_LABEL = 'Upload image'

function imageFile(name = 'cover.png', type = 'image/png', size = 1024) {
  const file = new File(['x'], name, { type })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

function pickFile(file: File) {
  fireEvent.change(screen.getByLabelText(FILE_INPUT_LABEL), {
    target: { files: [file] },
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('RunningHub app cover image field', () => {
  test('a picked image is uploaded and its stored URL becomes the cover', async () => {
    const uploadImage = vi
      .fn()
      .mockResolvedValue('/uploads/images/202610/a.png')
    const onChange = vi.fn()
    render(
      <CoverImageField value='' onChange={onChange} uploadImage={uploadImage} />
    )

    pickFile(imageFile())

    await waitFor(() =>
      expect(onChange).toHaveBeenCalledWith('/uploads/images/202610/a.png')
    )
    expect(uploadImage).toHaveBeenCalledTimes(1)
    expect(uploadImage.mock.calls[0][0]).toBeInstanceOf(File)
  })

  test('the upload button reports progress while the file is in flight', async () => {
    const pending = deferred<string>()
    const uploadImage = vi.fn().mockReturnValue(pending.promise)
    render(
      <CoverImageField value='' onChange={vi.fn()} uploadImage={uploadImage} />
    )

    pickFile(imageFile())

    expect(
      await screen.findByRole('button', { name: 'Uploading...' })
    ).toBeDisabled()
    pending.resolve('/uploads/images/202610/a.png')
    expect(
      await screen.findByRole('button', { name: 'Upload image' })
    ).toBeEnabled()
  })

  test('a non-image file is rejected without uploading', async () => {
    const uploadImage = vi.fn()
    const errorSpy = vi.spyOn(toast, 'error').mockImplementation(() => '')
    render(
      <CoverImageField value='' onChange={vi.fn()} uploadImage={uploadImage} />
    )

    pickFile(imageFile('notes.txt', 'text/plain'))

    await waitFor(() =>
      expect(errorSpy).toHaveBeenCalledWith('Please select image files')
    )
    expect(uploadImage).not.toHaveBeenCalled()
  })

  test('an oversized image is rejected without uploading', async () => {
    const uploadImage = vi.fn()
    const errorSpy = vi.spyOn(toast, 'error').mockImplementation(() => '')
    render(
      <CoverImageField value='' onChange={vi.fn()} uploadImage={uploadImage} />
    )

    pickFile(imageFile('huge.png', 'image/png', 11 * 1024 * 1024))

    await waitFor(() =>
      expect(errorSpy).toHaveBeenCalledWith(
        'Cover image must be 10MB or smaller'
      )
    )
    expect(uploadImage).not.toHaveBeenCalled()
  })

  test('a failed upload keeps the previous cover and reports the reason', async () => {
    const uploadImage = vi
      .fn()
      .mockRejectedValue(new Error('上传失败: 服务不可用'))
    const onChange = vi.fn()
    const errorSpy = vi.spyOn(toast, 'error').mockImplementation(() => '')
    render(
      <CoverImageField
        value='/uploads/images/202609/old.png'
        onChange={onChange}
        uploadImage={uploadImage}
      />
    )

    pickFile(imageFile())

    await waitFor(() =>
      expect(errorSpy).toHaveBeenCalledWith('上传失败: 服务不可用')
    )
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByLabelText(COVER_INPUT_LABEL)).toHaveValue(
      '/uploads/images/202609/old.png'
    )
  })

  test('Remove image clears the stored cover', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <CoverImageField
        value='/uploads/images/202609/old.png'
        onChange={onChange}
        uploadImage={vi.fn()}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Remove image' }))

    expect(onChange).toHaveBeenCalledWith('')
  })

  test('a typed URL replaces the stored cover', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    function ControlledField() {
      const [cover, setCover] = useState('')
      return (
        <CoverImageField
          value={cover}
          uploadImage={vi.fn()}
          onChange={(next) => {
            onChange(next)
            setCover(next)
          }}
        />
      )
    }
    render(<ControlledField />)

    await user.type(
      screen.getByLabelText(COVER_INPUT_LABEL),
      'https://cdn.example.com/a.png'
    )

    expect(onChange).toHaveBeenLastCalledWith('https://cdn.example.com/a.png')
  })

  test('an empty cover shows the placeholder instead of an image', () => {
    render(
      <CoverImageField value='' onChange={vi.fn()} uploadImage={vi.fn()} />
    )

    expect(screen.getByText('No cover image')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Remove image' })
    ).not.toBeInTheDocument()
  })

  test('the cover preview is rendered with the app cover as its accessible name', () => {
    render(
      <CoverImageField
        value='/uploads/images/202610/a.png'
        onChange={vi.fn()}
        uploadImage={vi.fn()}
      />
    )

    expect(
      screen.getByRole('img', { name: COVER_INPUT_LABEL })
    ).toHaveAttribute('src', '/uploads/images/202610/a.png')
  })

  test('a disabled field blocks picking and removing a cover', () => {
    render(
      <CoverImageField
        value='/uploads/images/202610/a.png'
        onChange={vi.fn()}
        uploadImage={vi.fn()}
        disabled
      />
    )

    expect(screen.getByLabelText(FILE_INPUT_LABEL)).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Upload image' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove image' })).toBeDisabled()
  })
})
