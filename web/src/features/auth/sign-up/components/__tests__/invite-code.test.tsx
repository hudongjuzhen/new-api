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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

const registerMock = vi.fn()
const sendEmailVerificationMock = vi.fn()

let statusFixture: Record<string, unknown> = {}

vi.mock('@/features/auth/api', () => ({
  register: (...args: unknown[]) => registerMock(...args),
  wechatLoginByCode: vi.fn(),
  sendEmailVerification: (...args: unknown[]) =>
    sendEmailVerificationMock(...args),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: statusFixture, loading: false, error: null }),
}))

vi.mock('@/features/auth/hooks/use-auth-redirect', () => ({
  useAuthRedirect: () => ({
    redirectToLogin: vi.fn(),
    redirectToRegister: vi.fn(),
    redirectTo2FA: vi.fn(),
    handleLoginSuccess: vi.fn(),
  }),
}))

const { SignUpForm } = await import('../sign-up-form')

function fillCredentials() {
  fireEvent.change(screen.getByPlaceholderText('Enter your username'), {
    target: { value: 'invited_user' },
  })
  fireEvent.change(
    screen.getByPlaceholderText('Enter password (8-20 characters)'),
    { target: { value: 'pass12345' } }
  )
  fireEvent.change(screen.getByPlaceholderText('Confirm password'), {
    target: { value: 'pass12345' },
  })
}

function submit() {
  fireEvent.click(screen.getByRole('button', { name: 'Create account' }))
}

describe('SignUpForm invite code gate', () => {
  beforeEach(() => {
    registerMock.mockReset()
    registerMock.mockResolvedValue({ success: true })
    sendEmailVerificationMock.mockReset()
    window.localStorage.clear()
    statusFixture = { register_enabled: true, invite_code_required: true }
  })

  afterEach(() => {
    window.localStorage.clear()
  })

  test('refuses to register while the invite code is empty', async () => {
    render(<SignUpForm />)
    fillCredentials()

    submit()

    await waitFor(() => {
      expect(registerMock).not.toHaveBeenCalled()
    })
    expect(
      screen.getByPlaceholderText("Enter the inviter's invite code")
    ).toBeInTheDocument()
  })

  test('submits the invite code as aff_code and remembers it for other flows', async () => {
    render(<SignUpForm />)
    fillCredentials()

    fireEvent.change(
      screen.getByPlaceholderText("Enter the inviter's invite code"),
      { target: { value: 'INV-123' } }
    )
    submit()

    await waitFor(() => {
      expect(registerMock).toHaveBeenCalledTimes(1)
    })
    expect(registerMock).toHaveBeenCalledWith(
      expect.objectContaining({ username: 'invited_user', aff_code: 'INV-123' })
    )
    // The OAuth and WeChat flows read the code from storage.
    expect(window.localStorage.getItem('aff')).toBe('INV-123')
  })

  test('hides the invite code field when the site does not require one', () => {
    statusFixture = { register_enabled: true, invite_code_required: false }

    render(<SignUpForm />)

    expect(
      screen.queryByPlaceholderText("Enter the inviter's invite code")
    ).not.toBeInTheDocument()
  })
})
