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
import { describe, expect, test, vi } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ParamField } = await import('../rh-portal')
type SchemaParam = import('../../api').SchemaParam

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

function renderField(
  param: SchemaParam,
  value: string,
  onChange: (v: string) => void
) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ParamField
        param={param}
        value={value}
        onChange={onChange}
        errors={{}}
        site=''
        uploadAvailable={false}
        onUploadingChange={() => undefined}
      />
    </I18nextProvider>
  )
}

// The curl import infers `switch` for a boolean node such as
// {"nodeId":"256","fieldName":"value","fieldValue":"false"}.
const switchParam: SchemaParam = {
  nodeId: '256',
  fieldName: 'value',
  label: '启用高清',
  type: 'switch',
}

describe('schema parameter control mapping', () => {
  test('renders a toggle for a switch parameter and submits "true" when turned on', () => {
    const onChange = vi.fn()
    renderField(switchParam, 'false', onChange)

    const toggle = screen.getByRole('switch')
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    expect(screen.getByText('Disabled')).toBeInTheDocument()

    fireEvent.click(toggle)
    expect(onChange).toHaveBeenCalledWith('true')
  })

  test('shows a switch parameter as enabled when its default is "true"', () => {
    renderField(switchParam, 'true', vi.fn())

    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByText('Enabled')).toBeInTheDocument()
  })

  test('turns a switch parameter off by submitting "false"', () => {
    const onChange = vi.fn()
    renderField(switchParam, 'true', onChange)

    fireEvent.click(screen.getByRole('switch'))
    expect(onChange).toHaveBeenCalledWith('false')
  })
})
