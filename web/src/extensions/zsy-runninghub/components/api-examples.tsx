/*
Copyright (C) 2023-2026 QuantumNous

RunningHub app center: copy-paste API samples.

Renders the three requests a third-party service needs to drive one app —
run, query, cancel — for the caller to paste into its own code. The samples use
a relay API key, which is the credential a service should hold; the same routes
also accept a dashboard session or a personal access token.

All prose goes through t(); the snippet text is built by lib/api-samples.ts.
*/

import { Terminal } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { AppView } from '../api'
import {
  API_KEY_PLACEHOLDER,
  API_SAMPLE_LANGS,
  API_SAMPLE_LANG_HIGHLIGHT,
  API_SAMPLE_LANG_LABELS,
  buildApiSample,
  type ApiSampleEndpoint,
  type ApiSampleLang,
} from '../lib/api-samples'

const ENDPOINTS: { value: ApiSampleEndpoint; label: string }[] = [
  { value: 'run', label: 'Run app' },
  { value: 'query', label: 'Query task' },
  { value: 'cancel', label: 'Cancel task' },
]

export function ApiExamples(props: { app: AppView }) {
  const { t } = useTranslation()
  const [endpoint, setEndpoint] = useState<ApiSampleEndpoint>('run')
  const [lang, setLang] = useState<ApiSampleLang>('curl')

  const code = useMemo(
    () =>
      buildApiSample({
        endpoint,
        lang,
        appId: props.app.id,
        schema: props.app.paramSchema,
        // The gateway serves this page, so the page origin is also the API origin.
        baseUrl:
          typeof window === 'undefined'
            ? 'http://localhost:3000'
            : window.location.origin,
      }),
    [endpoint, lang, props.app.id, props.app.paramSchema]
  )

  return (
    <section className='bg-card min-w-0 rounded-xl border p-4'>
      <h3 className='mb-1 flex items-center gap-1.5 text-sm font-semibold'>
        <Terminal className='text-muted-foreground/70 size-3.5' />
        {t('API Examples')}
      </h3>
      <p className='text-muted-foreground text-xs leading-relaxed'>
        {t('Call this app from your own service with an API key.')}{' '}
        {t('An API key can only query and cancel the runs it submitted.')}
      </p>

      <div className='mt-3 flex flex-wrap items-center gap-2'>
        <Tabs
          value={endpoint}
          onValueChange={(value) => setEndpoint(value as ApiSampleEndpoint)}
        >
          <TabsList className='bg-muted/40 h-8 p-0.5'>
            {ENDPOINTS.map((candidate) => (
              <TabsTrigger
                key={candidate.value}
                value={candidate.value}
                className='h-7 px-2.5 text-xs'
              >
                {t(candidate.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={lang}
          onValueChange={(value) => setLang(value as ApiSampleLang)}
          className='ml-auto'
        >
          <TabsList className='bg-muted/40 h-8 p-0.5'>
            {API_SAMPLE_LANGS.map((candidate) => (
              <TabsTrigger
                key={candidate}
                value={candidate}
                className='h-7 px-2.5 text-xs'
              >
                {API_SAMPLE_LANG_LABELS[candidate]}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      <CodeBlock code={code} language={API_SAMPLE_LANG_HIGHLIGHT[lang]}>
        <CodeBlockCopyButton />
      </CodeBlock>

      <p className='text-muted-foreground text-xs'>
        {t('Replace')}{' '}
        <code className='bg-muted rounded px-1 py-0.5 font-mono text-[11px]'>
          {API_KEY_PLACEHOLDER}
        </code>{' '}
        {t('with the API key from your token settings.')}
        {endpoint === 'run' && ` ${t('Values are keyed by nodeId.fieldName.')}`}
      </p>
    </section>
  )
}
