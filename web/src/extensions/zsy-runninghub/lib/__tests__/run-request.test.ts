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
import { beforeEach, describe, expect, test, vi } from 'vitest'

const post = vi.hoisted(() => vi.fn())

// The app center's run request must never carry an instance type: the compute
// instance is an app-level setting, and a value in the body overrides it for
// that single run (see submitAppRun on the backend).
vi.mock('@/lib/api', () => ({
  api: {
    post,
    get: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

const { runApp } = await import('../../api')

beforeEach(() => {
  post.mockReset()
  post.mockResolvedValue({ data: { success: true, data: { taskId: 'task_1' } } })
})

describe('RunningHub run request body', () => {
  test('an app-center run carries no instanceType, so the app setting applies', async () => {
    await runApp(13, { '122.prompt': 'a tiny castle' }, { tokenId: 7 })

    expect(post).toHaveBeenCalledTimes(1)
    const [url, body] = post.mock.calls[0]
    expect(url).toBe('/api/zsy/rh/apps/13/run')
    expect(body).toEqual({
      values: { '122.prompt': 'a tiny castle' },
      tokenId: 7,
    })
    expect(body).not.toHaveProperty('instanceType')
  })

  test('a run without a chosen key omits the token instead of sending null', async () => {
    await runApp(13, { '122.prompt': 'a tiny castle' })

    const [, body] = post.mock.calls[0]
    expect(Object.keys(body)).not.toContain('instanceType')
    expect(body.tokenId).toBeUndefined()
  })
})
