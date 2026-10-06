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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { useStatus } from '@/hooks/use-status'

import {
  publicToneListUrl,
  publicToneStandardUrl,
  PUBLIC_TONE_LIST_PATH,
} from '../api'
import type { ToneStandard } from '../lib/tone-fields'

/**
 * Shows integrators the exact URLs this page publishes, with the parameters the
 * catalog accepts.
 *
 * Two endpoints are listed rather than one: the catalog is the data, while the
 * standard is the contract behind it — which values exist, what the caps are,
 * and what the backend promises about changing them. A consumer reading only the
 * catalog would still be guessing at the vocabulary, which is why the plaza
 * publishes both and shows them side by side.
 *
 * The compatibility promises are rendered verbatim. They are the reason a
 * consumer can safely fall back to showing an unrecognised value instead of
 * dropping the row, so an integrator needs to read them here rather than infer
 * them: they ship inside the standard's own response, in the language the
 * standard was written in.
 *
 * The host's configured public address is preferred over the browser's origin so
 * the copied URLs are the ones third parties can reach.
 */
export function ToneEndpointCard(props: { standard: ToneStandard }) {
  const { t } = useTranslation()
  const { status } = useStatus()

  const serverAddress =
    (status as Record<string, unknown> | null)?.server_address ??
    (status?.data as Record<string, unknown> | undefined)?.server_address
  let baseUrl = ''
  if (typeof serverAddress === 'string' && serverAddress !== '') {
    baseUrl = serverAddress
  } else if (typeof window !== 'undefined') {
    baseUrl = window.location.origin
  }
  const listUrl = publicToneListUrl(baseUrl)
  const standardUrl = publicToneStandardUrl(baseUrl)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Tone List API')}</CardTitle>
        <CardDescription>
          {t(
            'On-shelf tones are published read-only; no authentication is required.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='flex items-center gap-2 rounded-md border p-3'>
          <code className='min-w-0 flex-1 truncate font-mono text-sm'>
            {listUrl}
          </code>
          <CopyButton value={listUrl} aria-label={t('Copy endpoint')} />
        </div>
        <dl className='text-muted-foreground grid gap-1 text-sm sm:grid-cols-2'>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Path')}</dt>
            <dd className='font-mono'>{PUBLIC_TONE_LIST_PATH}</dd>
          </div>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Pagination')}</dt>
            <dd className='font-mono'>page / page_size</dd>
          </div>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Filters')}</dt>
            <dd className='font-mono'>keyword / category / tone</dd>
          </div>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Response')}</dt>
            <dd className='font-mono'>items / total / page / pageSize</dd>
          </div>
        </dl>

        <div className='space-y-2 border-t pt-4'>
          <p className='text-sm font-medium'>{t('Tone Standard API')}</p>
          <div className='flex items-center gap-2 rounded-md border p-3'>
            <code className='min-w-0 flex-1 truncate font-mono text-sm'>
              {standardUrl}
            </code>
            <CopyButton value={standardUrl} aria-label={t('Copy endpoint')} />
          </div>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Vocabulary, field caps and compatibility promises every consumer can rely on.'
            )}
          </p>
          {props.standard.compatibility.length > 0 ? (
            <ul className='text-muted-foreground list-inside list-disc space-y-1 text-xs'>
              {props.standard.compatibility.map((promise) => (
                <li key={promise}>{promise}</li>
              ))}
            </ul>
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}
