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
import { useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'

import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { useAuthStore } from '@/stores/auth-store'

/**
 * API key selection shared by the chat and image lab playgrounds: loads the
 * signed-in user's enabled keys, tracks the selected one, and resolves its
 * masked key to the real secret on demand (cached per token id).
 */
export function useLabKeys() {
  const user = useAuthStore((state) => state.auth.user)
  const [selectedKeyId, setSelectedKeyId] = useState('')
  const realKeyCacheRef = useRef(new Map<number, string>())

  const { data: keysData } = useQuery({
    queryKey: ['lab-api-keys'],
    queryFn: () => getApiKeys({ p: 1, size: 100 }),
    enabled: !!user,
  })
  const enabledKeys = useMemo(
    () => (keysData?.data?.items ?? []).filter((key) => key.status === 1),
    [keysData]
  )
  const selectedKey =
    enabledKeys.find((key) => String(key.id) === selectedKeyId) ?? null

  const resolveRealKey = async (token: ApiKey): Promise<string> => {
    const cached = realKeyCacheRef.current.get(token.id)
    if (cached) return cached
    const res = await fetchTokenKey(token.id)
    const rawKey = res.data?.key
    if (!res.success || !rawKey) {
      throw new Error(res.message || 'empty key')
    }
    const fullKey = rawKey.startsWith('sk-') ? rawKey : `sk-${rawKey}`
    realKeyCacheRef.current.set(token.id, fullKey)
    return fullKey
  }

  return {
    user,
    enabledKeys,
    selectedKey,
    selectedKeyId,
    setSelectedKeyId,
    resolveRealKey,
  }
}
