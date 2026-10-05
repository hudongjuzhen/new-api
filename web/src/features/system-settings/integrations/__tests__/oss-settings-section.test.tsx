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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import {
  OSSSettingsSection,
  type OSSSettingsDefaults,
} from '../oss-settings-section'

const api = vi.hoisted(() => ({
  updateSystemOption: vi.fn(),
  testOSSConnection: vi.fn(),
}))

vi.mock('../../api', () => api)

// The section only consumes useBlocker for the unsaved-changes guard, so the
// test renders it without a real router.
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useBlocker: () => ({ status: 'idle' }),
}))

const baseDefaults: OSSSettingsDefaults = {
  oss_setting: {
    enabled: false,
    endpoint: '',
    bucket: '',
    access_key_id: '',
    access_key_secret: '',
    path_prefix: 'uploads',
    custom_domain: '',
    use_ssl: true,
  },
}

const enabledDefaults: OSSSettingsDefaults = {
  oss_setting: {
    ...baseDefaults.oss_setting,
    enabled: true,
    endpoint: 'oss-cn-hangzhou.aliyuncs.com',
    bucket: 'my-bucket',
    access_key_id: 'test-ak',
  },
}

const actionContainers: HTMLDivElement[] = []

function renderOSSSection(defaultValues: OSSSettingsDefaults) {
  const actionsContainer = document.createElement('div')
  document.body.appendChild(actionsContainer)
  actionContainers.push(actionsContainer)

  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <OSSSettingsSection defaultValues={defaultValues} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )

  return queryClient
}

describe('Aliyun OSS settings section', () => {
  beforeEach(() => {
    api.updateSystemOption.mockResolvedValue({ success: true, message: '' })
    api.testOSSConnection.mockResolvedValue({
      success: true,
      message: '',
      data: { endpoint: 'oss-cn-hangzhou.aliyuncs.com', bucket: 'my-bucket' },
    })
  })

  afterEach(() => {
    for (const container of actionContainers.splice(0)) {
      container.remove()
    }
  })

  test('keeps the connection fields hidden until Aliyun OSS is enabled', () => {
    renderOSSSection(baseDefaults)

    const enableSwitch = screen.getByRole('switch', {
      name: 'Enable Aliyun OSS',
    })
    expect(screen.queryByLabelText('OSS Endpoint')).not.toBeInTheDocument()

    fireEvent.click(enableSwitch)

    expect(screen.getByLabelText('OSS Endpoint')).toBeInTheDocument()
    expect(screen.getByLabelText('AccessKey Secret')).toBeInTheDocument()
    expect(screen.getByLabelText('Object Key Prefix')).toHaveValue('uploads')
  })

  test('blocks saving with a field error while required fields are empty', async () => {
    renderOSSSection({
      oss_setting: { ...baseDefaults.oss_setting, enabled: true },
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save OSS settings' }))

    await waitFor(() => {
      expect(
        screen.getAllByText('Required when Aliyun OSS is enabled')
      ).toHaveLength(3)
    })
    expect(api.updateSystemOption).not.toHaveBeenCalled()
    expect(screen.getByLabelText('OSS Endpoint')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  })

  test('saves only the changed option and never clears the stored secret', async () => {
    renderOSSSection(enabledDefaults)

    fireEvent.change(screen.getByLabelText('Object Key Prefix'), {
      target: { value: 'assets' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save OSS settings' }))

    await waitFor(() => {
      expect(api.updateSystemOption).toHaveBeenCalledTimes(1)
    })
    expect(api.updateSystemOption).toHaveBeenCalledWith({
      key: 'oss_setting.path_prefix',
      value: 'assets',
    })
  })

  test('sends a newly entered AccessKey Secret so the credential can be rotated', async () => {
    renderOSSSection(enabledDefaults)

    fireEvent.change(screen.getByLabelText('AccessKey Secret'), {
      target: { value: 'rotated-secret' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save OSS settings' }))

    await waitFor(() => {
      expect(api.updateSystemOption).toHaveBeenCalledWith({
        key: 'oss_setting.access_key_secret',
        value: 'rotated-secret',
      })
    })
  })

  test('reports a successful connection test of the saved configuration', async () => {
    renderOSSSection(enabledDefaults)

    fireEvent.click(screen.getByRole('button', { name: 'Test Connection' }))

    await waitFor(() => {
      expect(api.testOSSConnection).toHaveBeenCalledTimes(1)
    })
    expect(
      await screen.findByText(
        'The configured bucket accepts uploads and deletes.'
      )
    ).toBeInTheDocument()
    expect(api.updateSystemOption).not.toHaveBeenCalled()
  })

  test('surfaces the OSS error message when the connection test fails', async () => {
    api.testOSSConnection.mockResolvedValue({
      success: false,
      message: '请填写 OSS AccessKey Secret',
    })
    renderOSSSection(enabledDefaults)

    fireEvent.click(screen.getByRole('button', { name: 'Test Connection' }))

    expect(await screen.findByText('Connection failed')).toBeInTheDocument()
    expect(screen.getByText('请填写 OSS AccessKey Secret')).toBeInTheDocument()
  })
})
