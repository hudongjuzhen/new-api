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
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useLabRun } from '../use-lab-run'

const { FakeSSE, getFreshAuthHeaders } = vi.hoisted(() => {
  class FakeSSE {
    static instances: FakeSSE[] = []

    url: string
    options: {
      headers: Record<string, string>
      method?: string
      payload?: string
    }
    closed = false

    constructor(
      url: string,
      options: {
        headers: Record<string, string>
        method?: string
        payload?: string
      }
    ) {
      this.url = url
      this.options = options
      FakeSSE.instances.push(this)
    }

    addEventListener = vi.fn()
    close() {
      this.closed = true
    }
    stream = vi.fn()
  }

  const getFreshAuthHeaders = vi.fn(async () => ({
    'Content-Type': 'application/json',
    Authorization: 'Bearer session-jwt',
  }))

  return { FakeSSE, getFreshAuthHeaders }
})

vi.mock('sse.js', () => ({ SSE: FakeSSE }))
vi.mock('@/lib/api', () => ({ getFreshAuthHeaders }))

const PAYLOAD = { model: 'gpt-test', messages: [], stream: true }

describe('useLabRun endpoint selection', () => {
  beforeEach(() => {
    FakeSSE.instances = []
  })

  it('runs with the selected API key via /v1 Bearer auth and skips session headers', async () => {
    const { result } = renderHook(() => useLabRun())

    act(() => {
      result.current.run(PAYLOAD, { apiKey: 'sk-test-key-123' })
    })

    expect(FakeSSE.instances).toHaveLength(1)
    const source = FakeSSE.instances[0]
    expect(source.url).toBe('/v1/chat/completions')
    expect(source.options.method).toBe('POST')
    expect(source.options.payload).toBe(JSON.stringify(PAYLOAD))
    expect(source.options.headers.Authorization).toBe('Bearer sk-test-key-123')
    expect(getFreshAuthHeaders).not.toHaveBeenCalled()
    expect(result.current.isRunning).toBe(true)
  })

  it('falls back to the signed-in session endpoint when no key is selected', async () => {
    const { result } = renderHook(() => useLabRun())

    await act(async () => {
      result.current.run(PAYLOAD)
    })

    expect(FakeSSE.instances).toHaveLength(1)
    const source = FakeSSE.instances[0]
    expect(source.url).toBe('/pg/chat/completions')
    expect(source.options.headers.Authorization).toBe('Bearer session-jwt')
    expect(getFreshAuthHeaders).toHaveBeenCalledTimes(1)
  })

  it('treats a blank apiKey option as session mode', async () => {
    const { result } = renderHook(() => useLabRun())

    await act(async () => {
      result.current.run(PAYLOAD, { apiKey: '   ' })
    })

    expect(FakeSSE.instances).toHaveLength(1)
    expect(FakeSSE.instances[0].url).toBe('/pg/chat/completions')
    expect(getFreshAuthHeaders).toHaveBeenCalledTimes(1)
  })
})
