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
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { useStatus } from '@/hooks/use-status'

import { isAudioGenModel } from './lib/model'

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

/**
 * Audio-generation parameters, named the way the request body nests them: the
 * text field sits at the top level while every `audio_config.*` row is a knob the
 * model reads. Bounds match the upstream reference and the playground's clamping.
 */
const AUDIO_PARAMETERS: LabApiParameter[] = [
  {
    name: 'model',
    type: 'string',
    required: true,
    descriptionKey: 'Model identifier to call, e.g. seed-audio-1.0',
  },
  {
    name: 'text_prompt',
    type: 'string',
    required: true,
    descriptionKey:
      'Text to synthesize, or a natural-language prompt describing the audio; at most 3000 characters',
  },
  {
    name: 'references',
    type: 'array',
    required: false,
    descriptionKey:
      'Reference resources: up to 3 audio_url entries (30s / 10MB each) or 1 image_url entry; audio and image references cannot be mixed',
  },
  {
    name: 'audio_config.format',
    type: 'string',
    required: false,
    defaultValue: 'wav',
    descriptionKey: 'Output audio format: wav / mp3 / pcm / ogg_opus',
  },
  {
    name: 'audio_config.sample_rate',
    type: 'integer',
    required: false,
    defaultValue: '24000',
    descriptionKey: 'Output sample rate in Hz; ogg_opus supports 48000 only',
  },
  {
    name: 'audio_config.speech_rate',
    type: 'integer',
    required: false,
    defaultValue: '0',
    descriptionKey: 'Speech rate, -50 (0.5x) to 100 (2x); 0 keeps it as is',
  },
  {
    name: 'audio_config.loudness_rate',
    type: 'integer',
    required: false,
    defaultValue: '0',
    descriptionKey: 'Volume, -50 (0.5x) to 100 (2x); 0 keeps it as is',
  },
  {
    name: 'audio_config.pitch_rate',
    type: 'integer',
    required: false,
    defaultValue: '0',
    descriptionKey: 'Pitch, -12 to 12; 0 keeps the original pitch',
  },
  {
    name: 'watermark.aigc_watermark',
    type: 'boolean',
    required: false,
    defaultValue: 'false',
    descriptionKey:
      'Adds the inaudible AI-generated marker to the audio header',
  },
]

const AUDIO_RESPONSE_FIELDS: LabApiParameter[] = [
  {
    name: 'audio',
    type: 'string',
    required: true,
    descriptionKey:
      'The generated audio, base64-encoded; play or save it directly, since a synchronous call has no result URL to fetch later',
  },
  {
    name: 'original_duration',
    type: 'number',
    required: true,
    descriptionKey:
      'Raw model output length in seconds (at most 120); this is what the request is billed on',
  },
  {
    name: 'duration',
    type: 'number',
    required: false,
    descriptionKey:
      'Length after speed and post-processing; differs from original_duration when speech_rate is set',
  },
  {
    name: 'url',
    type: 'string',
    required: false,
    descriptionKey:
      'Temporary upstream link to the same audio, valid for a short time only',
  },
  {
    name: 'format',
    type: 'string',
    required: false,
    descriptionKey: 'Output format actually requested, useful when saving a file',
  },
]

/** Parameter table shared by the chat and audio API references. */
function ParameterTable(props: { parameters: LabApiParameter[] }) {
  const { t } = useTranslation()

  return (
    <StaticDataTable
      className='mt-3 rounded-none border-0'
      data={props.parameters}
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
  )
}

/**
 * API reference for the async audio-generation surface. It shows the task
 * endpoints rather than chat completions because the model is only reachable
 * through them, and the parameters come from the upstream audio-generation
 * reference as this gateway forwards them.
 */
function AudioApiReference(props: { model?: string; baseUrl: string }) {
  const { t } = useTranslation()

  const endpoints = [
    {
      method: 'POST',
      name: t('Generate audio'),
      path: '/v1/audio/generations',
      note: t(
        'Returns the audio inline, base64-encoded, together with its duration; billing follows that duration.'
      ),
    },
  ]

  return (
    <div className='space-y-4'>
      <div>
        <h2 className='text-lg font-semibold'>{t('API reference')}</h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Audio generation is a synchronous API. Current model:')}{' '}
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
            <div
              className='space-y-2 py-3 first:pt-0 last:pb-0'
              key={endpoint.path}
            >
              <div className='flex flex-wrap items-center gap-2'>
                <Badge className='font-mono'>{endpoint.method}</Badge>
                <span className='text-sm font-medium'>{endpoint.name}</span>
              </div>
              <div className='bg-muted/30 flex items-center justify-between gap-2 rounded-md border px-3 py-2'>
                <code className='text-foreground truncate font-mono text-xs'>
                  {props.baseUrl}
                  {endpoint.path}
                </code>
                <CopyButton
                  aria-label={t('Copy to clipboard')}
                  iconClassName='size-3.5'
                  value={`${props.baseUrl}${endpoint.path}`}
                />
              </div>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <code className='text-muted-foreground font-mono text-xs'>
                  Authorization: Bearer &lt;your-token&gt;
                </code>
                <span className='text-muted-foreground/70 text-xs'>
                  {endpoint.note}
                </span>
              </div>
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
            'The Playground tab calls this endpoint with one of your API keys, and the gateway forwards it to the audio service with the key configured on the channel. Usage is billed to your account exactly like a regular API call.'
          )}
        </p>
      </section>

      <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
        <h3 className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
          {t('Supported parameters')}
        </h3>
        <ParameterTable parameters={AUDIO_PARAMETERS} />
      </section>

      <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
        <h3 className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
          {t('Response fields')}
        </h3>
        <ParameterTable parameters={AUDIO_RESPONSE_FIELDS} />
      </section>
    </div>
  )
}

export function LabApi(props: { model?: string }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { models } = usePricingData()

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

  const modelInfo = useMemo(
    () => models.find((item) => item.model_name === props.model) ?? null,
    [models, props.model]
  )
  if (isAudioGenModel(modelInfo)) {
    return <AudioApiReference baseUrl={baseUrl} model={props.model} />
  }

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
            <div
              className='space-y-2 py-3 first:pt-0 last:pb-0'
              key={endpoint.path}
            >
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
        <ParameterTable parameters={PARAMETERS} />
      </section>
    </div>
  )
}
