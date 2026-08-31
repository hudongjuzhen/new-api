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
import { SSE } from 'sse.js'
import { useCallback, useEffect, useRef, useState } from 'react'

import {
  isStreamClosedReadyState,
  isStreamDoneMessage,
  parseStreamErrorDetails,
  parseStreamMessageUpdates,
} from '@/features/playground/lib'
import { ERROR_MESSAGES } from '@/features/playground/constants'
import { getFreshAuthHeaders } from '@/lib/api'

const SESSION_CHAT_COMPLETIONS_ENDPOINT = '/pg/chat/completions'
const TOKEN_CHAT_COMPLETIONS_ENDPOINT = '/v1/chat/completions'
const MAX_RAW_LINES = 200

interface StreamEventSource {
  readyState?: number
  addEventListener: (
    type: string,
    listener: (event: Event & { data?: string; readyState?: number }) => void
  ) => void
  close: () => void
  stream: () => void
}

export interface LabRunState {
  phase: 'idle' | 'running' | 'done' | 'error'
  content: string
  reasoning: string
  rawLines: string[]
  error: string | null
  durationMs: number | null
}

export interface LabRunResult {
  status: 'success' | 'error'
  durationMs: number
  content: string
  error?: string
}

const INITIAL_STATE: LabRunState = {
  phase: 'idle',
  content: '',
  reasoning: '',
  rawLines: [],
  error: null,
  durationMs: null,
}

export interface LabRunOptions {
  /** API key (token) to run with; requests then go through /v1 with the
   * key's own group instead of the signed-in session's /pg endpoint. */
  apiKey?: string
}

/**
 * Streaming run controller for the lab playground: posts to the
 * session-authenticated /pg/chat/completions endpoint and keeps both the
 * parsed text updates and the raw SSE lines (for the JSON output view).
 * When an API key is provided, the request is sent to /v1/chat/completions
 * with Bearer auth so it executes under the key's group.
 */
export function useLabRun(onSettled?: (result: LabRunResult) => void) {
  const [state, setState] = useState<LabRunState>(INITIAL_STATE)
  const sourceRef = useRef<StreamEventSource | null>(null)
  const generationRef = useRef(0)
  const startedAtRef = useRef(0)
  const contentRef = useRef('')
  const onSettledRef = useRef(onSettled)
  useEffect(() => {
    onSettledRef.current = onSettled
  }, [onSettled])

  const settle = useCallback(
    (generation: number, status: 'success' | 'error', error?: string) => {
      if (generationRef.current !== generation) return
      generationRef.current += 1
      sourceRef.current?.close()
      sourceRef.current = null
      const durationMs = startedAtRef.current
        ? Date.now() - startedAtRef.current
        : 0
      setState((prev) => ({
        ...prev,
        phase: status === 'error' ? 'error' : 'done',
        error: error ?? null,
        durationMs: prev.durationMs ?? durationMs,
      }))
      onSettledRef.current?.({
        status,
        durationMs,
        content: contentRef.current,
        error,
      })
    },
    []
  )

  const stop = useCallback(() => {
    if (!sourceRef.current) return
    settle(generationRef.current, 'success')
  }, [settle])

  const run = useCallback(
    (payload: Record<string, unknown>, options?: LabRunOptions) => {
      const generation = generationRef.current + 1
      generationRef.current = generation
      sourceRef.current?.close()
      sourceRef.current = null
      contentRef.current = ''
      startedAtRef.current = Date.now()
      setState({ ...INITIAL_STATE, phase: 'running' })

      const apiKey = options?.apiKey?.trim() || ''

      void (async () => {
        let headers: Record<string, string>
        if (apiKey) {
          headers = {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${apiKey}`,
          }
        } else {
          try {
            headers = await getFreshAuthHeaders()
          } catch (error: unknown) {
            if (generationRef.current !== generation) return
            settle(
              generation,
              'error',
              error instanceof Error
                ? error.message
                : ERROR_MESSAGES.STREAM_START_ERROR
            )
            return
          }
        }
        if (generationRef.current !== generation) return

        const endpoint = apiKey
          ? TOKEN_CHAT_COMPLETIONS_ENDPOINT
          : SESSION_CHAT_COMPLETIONS_ENDPOINT
        let source: StreamEventSource
        try {
          source = new SSE(endpoint, {
            headers,
            method: 'POST',
            payload: JSON.stringify(payload),
          }) as StreamEventSource
        } catch {
          settle(generation, 'error', ERROR_MESSAGES.STREAM_START_ERROR)
          return
        }
        sourceRef.current = source

        const appendUpdate = (type: 'reasoning' | 'content', chunk: string) => {
          setState((prev) => ({ ...prev, [type]: prev[type] + chunk }))
        }
        const appendRawLine = (data: string) => {
          setState((prev) => ({
            ...prev,
            rawLines: [...prev.rawLines, data].slice(-MAX_RAW_LINES),
          }))
        }

        source.addEventListener('message', (event) => {
          if (generationRef.current !== generation) return
          const data = event.data ?? ''
          if (isStreamDoneMessage(data)) {
            settle(generation, 'success')
            return
          }
          appendRawLine(data)
          try {
            const updates = parseStreamMessageUpdates(data)
            for (const update of updates) {
              if (update.type === 'content') {
                contentRef.current += update.chunk
              }
              appendUpdate(update.type, update.chunk)
            }
          } catch {
            settle(generation, 'error', ERROR_MESSAGES.PARSE_ERROR)
          }
        })

        source.addEventListener('error', (event) => {
          if (generationRef.current !== generation) return
          if (isStreamClosedReadyState(source.readyState)) return
          const { errorMessage } = parseStreamErrorDetails(event.data)
          settle(generation, 'error', errorMessage)
        })

        try {
          source.stream()
        } catch {
          settle(generation, 'error', ERROR_MESSAGES.STREAM_START_ERROR)
        }
      })()
    },
    [settle]
  )

  useEffect(
    () => () => {
      generationRef.current += 1
      sourceRef.current?.close()
      sourceRef.current = null
    },
    []
  )

  return { state, run, stop, isRunning: state.phase === 'running' }
}
