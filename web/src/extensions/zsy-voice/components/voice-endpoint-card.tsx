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

import { publicVoiceListUrl, PUBLIC_VOICE_LIST_PATH } from '../api'

/**
 * Shows integrators the exact URL of the public catalog this page publishes,
 * with the pagination parameters it accepts. The host's configured public
 * address is preferred over the browser's origin so the copied URL is the one
 * third parties can reach.
 */
export function VoiceEndpointCard() {
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
  const listUrl = publicVoiceListUrl(baseUrl)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Voice List API')}</CardTitle>
        <CardDescription>
          {t(
            'On-shelf voices are published read-only; no authentication is required.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-3'>
        <div className='flex items-center gap-2 rounded-md border p-3'>
          <code className='min-w-0 flex-1 truncate font-mono text-sm'>
            {listUrl}
          </code>
          <CopyButton value={listUrl} aria-label={t('Copy endpoint')} />
        </div>
        <dl className='text-muted-foreground grid gap-1 text-sm sm:grid-cols-2'>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Path')}</dt>
            <dd className='font-mono'>{PUBLIC_VOICE_LIST_PATH}</dd>
          </div>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Pagination')}</dt>
            <dd className='font-mono'>page / page_size</dd>
          </div>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Filters')}</dt>
            <dd className='font-mono'>keyword / voice_type</dd>
          </div>
          <div className='flex gap-2'>
            <dt className='font-medium'>{t('Response')}</dt>
            <dd className='font-mono'>items / total / page / pageSize</dd>
          </div>
        </dl>
      </CardContent>
    </Card>
  )
}
