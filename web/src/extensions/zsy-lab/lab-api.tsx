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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StaticDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { useStatus } from '@/hooks/use-status'

interface EndpointRow {
  method: string
  name: string
  compat: string
  path: string
  auth: string
}

interface LabApiParameter {
  name: string
  type: string
  required: boolean
  defaultValue?: string
  descriptionKey: string
}

const PARAMETERS: LabApiParameter[] = [
  {
    name: 'model',
    type: 'string',
    required: true,
    descriptionKey: 'Model identifier to call',
  },
  {
    name: 'messages',
    type: 'array',
    required: true,
    descriptionKey: 'Conversation messages (system / user / assistant roles)',
  },
  {
    name: 'stream',
    type: 'boolean',
    required: false,
    defaultValue: 'false',
    descriptionKey: 'Stream the response as server-sent events',
  },
  {
    name: 'temperature',
    type: 'number',
    required: false,
    defaultValue: '1',
    descriptionKey: 'Sampling temperature, between 0 and 2',
  },
  {
    name: 'top_p',
    type: 'number',
    required: false,
    defaultValue: '1',
    descriptionKey: 'Nucleus sampling, between 0 and 1',
  },
  {
    name: 'max_tokens',
    type: 'integer',
    required: false,
    descriptionKey: 'Maximum number of tokens to generate',
  },
  {
    name: 'frequency_penalty',
    type: 'number',
    required: false,
    defaultValue: '0',
    descriptionKey: 'Penalizes repeated tokens, between -2 and 2',
  },
  {
    name: 'presence_penalty',
    type: 'number',
    required: false,
    defaultValue: '0',
    descriptionKey: 'Encourages new topics, between -2 and 2',
  },
]

export function LabApi(props: { model?: string }) {
  const { t } = useTranslation()
  const { status } = useStatus()

  const baseUrl = useMemo(() => {
    const candidate =
      (status as Record<string, unknown> | null)?.server_address ??
      (status?.data as Record<string, unknown> | undefined)?.server_address
    if (typeof candidate === 'string' && candidate.length > 0) {
      return candidate.replace(/\/$/, '')
    }
    if (typeof window !== 'undefined') return window.location.origin
    return 'https://api.example.com'
  }, [status])

  const endpoints: EndpointRow[] = [
    {
      method: 'POST',
      name: t('Chat Completions'),
      compat: t('OpenAI compatible'),
      path: '/v1/chat/completions',
      auth: 'Authorization: Bearer <your-token>',
    },
    {
      method: 'POST',
      name: t('Messages'),
      compat: t('Anthropic compatible'),
      path: '/v1/messages',
      auth: 'x-api-key: <your-token>',
    },
  ]

  return (
    <div className='space-y-4'>
      <div>
        <h2 className='text-lg font-semibold'>{t('API reference')}</h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Endpoints below accept an API key. Current model:')}{' '}
          <code className='bg-muted rounded px-1.5 py-0.5 font-mono text-xs'>
            {props.model || t('No model selected')}
          </code>
        </p>
      </div>

      <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
        <h3 className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
          {t('Endpoints')}
        </h3>
        <div className='divide-border/60 mt-3 divide-y'>
          {endpoints.map((endpoint) => (
            <div className='space-y-2 py-3 first:pt-0 last:pb-0' key={endpoint.path}>
              <div className='flex flex-wrap items-center gap-2'>
                <Badge className='font-mono'>{endpoint.method}</Badge>
                <span className='text-sm font-medium'>{endpoint.name}</span>
                <Badge variant='outline' className='text-xs'>
                  {endpoint.compat}
                </Badge>
              </div>
              <div className='bg-muted/30 flex items-center justify-between gap-2 rounded-md border px-3 py-2'>
                <code className='text-foreground truncate font-mono text-xs'>
                  {baseUrl}
                  {endpoint.path}
                </code>
                <CopyButton
                  aria-label={t('Copy to clipboard')}
                  iconClassName='size-3.5'
                  value={`${baseUrl}${endpoint.path}`}
                />
              </div>
              <code className='text-muted-foreground block font-mono text-xs'>
                {endpoint.auth}
              </code>
            </div>
          ))}
        </div>
      </section>

      <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
        <h3 className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
          {t('Session channel')}
        </h3>
        <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>
          {t(
            'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.'
          )}
        </p>
      </section>

      <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
        <h3 className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
          {t('Supported parameters')}
        </h3>
        <StaticDataTable
          className='mt-3 rounded-none border-0'
          data={PARAMETERS}
          getRowKey={(parameter) => parameter.name}
          headerRowClassName='hover:bg-transparent'
          tableClassName='text-sm'
          columns={[
            {
              id: 'name',
              header: t('Parameter'),
              cell: (parameter) => (
                <code className='font-mono text-sm font-medium'>
                  {parameter.name}
                </code>
              ),
            },
            {
              id: 'type',
              header: t('Type'),
              cell: (parameter) => (
                <Badge
                  variant='secondary'
                  className='font-mono text-xs font-normal'
                >
                  {parameter.type}
                </Badge>
              ),
            },
            {
              id: 'required',
              header: t('Required'),
              cell: (parameter) =>
                parameter.required ? (
                  <Badge
                    variant='outline'
                    className='border-rose-500/40 text-rose-600 dark:text-rose-400'
                  >
                    {t('required')}
                  </Badge>
                ) : (
                  <span className='text-muted-foreground text-sm'>
                    {parameter.defaultValue ?? '—'}
                  </span>
                ),
            },
            {
              id: 'description',
              header: t('Description'),
              cell: (parameter) => t(parameter.descriptionKey),
            },
          ]}
        />
      </section>
    </div>
  )
}
