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
import { CheckCircle2Icon, ClockIcon, XCircleIcon } from 'lucide-react'
import dayjs from 'dayjs'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { loadLabHistory, LAB_HISTORY_UPDATED_EVENT, type LabHistoryEntry } from './lib/history'

const PREVIEW_COUNT = 10

function HistoryRow(props: { entry: LabHistoryEntry }) {
  const { t } = useTranslation()
  const entry = props.entry

  return (
    <div className='flex items-start gap-3 px-3 py-2.5'>
      {entry.status === 'success' ? (
        <CheckCircle2Icon className='mt-0.5 size-4 shrink-0 text-emerald-500' />
      ) : (
        <XCircleIcon className='text-destructive mt-0.5 size-4 shrink-0' />
      )}
      <div className='min-w-0 flex-1'>
        <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
          <code className='bg-muted rounded px-1.5 py-0.5 font-mono text-xs'>
            {entry.model || t('No model selected')}
          </code>
          <span className='text-muted-foreground truncate text-sm'>
            {entry.prompt || t('(empty prompt)')}
          </span>
        </div>
        {entry.error && (
          <p className='text-destructive/80 mt-1 truncate text-xs'>
            {entry.error}
          </p>
        )}
      </div>
      <div className='text-muted-foreground/70 shrink-0 text-right text-xs tabular-nums'>
        {entry.durationMs != null && (
          <div>
            {entry.durationMs >= 1000
              ? `${(entry.durationMs / 1000).toFixed(1)}s`
              : `${entry.durationMs}ms`}
          </div>
        )}
        <div>{dayjs(entry.createdAt).format('MM-DD HH:mm:ss')}</div>
      </div>
    </div>
  )
}

export function LabHistory() {
  const { t } = useTranslation()
  const [entries, setEntries] = useState<LabHistoryEntry[]>([])
  const [showAll, setShowAll] = useState(false)

  useEffect(() => {
    const refresh = () => setEntries(loadLabHistory())
    refresh()
    window.addEventListener(LAB_HISTORY_UPDATED_EVENT, refresh)
    window.addEventListener('storage', refresh)
    return () => {
      window.removeEventListener(LAB_HISTORY_UPDATED_EVENT, refresh)
      window.removeEventListener('storage', refresh)
    }
  }, [])

  const visibleEntries = showAll ? entries : entries.slice(0, PREVIEW_COUNT)

  return (
    <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
      <div className='flex items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <ClockIcon className='text-muted-foreground size-4' />
          <span className='text-sm font-semibold'>{t('Usage history')}</span>
          <span className='text-muted-foreground/60 text-xs'>
            ({t('cached locally for 24h')})
          </span>
        </div>
        {entries.length > PREVIEW_COUNT && (
          <Button
            className='text-muted-foreground h-auto px-1 py-0.5 text-xs'
            onClick={() => setShowAll((value) => !value)}
            size='sm'
            variant='ghost'
          >
            {showAll ? t('Show less') : t('View all')}
          </Button>
        )}
      </div>

      {entries.length === 0 ? (
        <div className='border-border/60 text-muted-foreground/70 mt-3 rounded-lg border border-dashed px-4 py-10 text-center text-sm'>
          {t(
            'No records yet. They will be logged automatically after your first request.'
          )}
        </div>
      ) : (
        <div className='divide-border/60 mt-2 divide-y'>
          {visibleEntries.map((entry) => (
            <HistoryRow entry={entry} key={entry.id} />
          ))}
        </div>
      )}
    </section>
  )
}
