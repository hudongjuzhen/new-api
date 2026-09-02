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
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'

import { handleUpdateModelSort } from '../lib'

type SortCellProps = {
  id: number
  sort: number
}

export function SortCell({ id, sort }: SortCellProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(String(sort))
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [editing])

  const startEdit = () => {
    setValue(String(sort))
    setEditing(true)
  }

  const commit = () => {
    setEditing(false)
    const parsed = Math.trunc(Number(value))
    const next = Number.isFinite(parsed) ? parsed : 0
    if (next !== sort) {
      handleUpdateModelSort(id, next, queryClient)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      commit()
    } else if (e.key === 'Escape') {
      setEditing(false)
    }
  }

  if (editing) {
    return (
      <Input
        ref={inputRef}
        type='number'
        step={1}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={commit}
        onKeyDown={handleKeyDown}
        className='h-7 w-20 px-1.5 text-sm'
        aria-label={t('Ranking')}
      />
    )
  }

  return (
    <Badge
      variant='secondary'
      onDoubleClick={startEdit}
      title={t('Double-click to edit ranking')}
      className='w-12 cursor-text justify-center px-1.5 py-0.5 font-mono text-sm tabular-nums'
    >
      {sort}
    </Badge>
  )
}
