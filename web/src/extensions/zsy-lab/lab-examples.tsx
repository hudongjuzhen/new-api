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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { BundledLanguage } from 'shiki/bundle/web'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useStatus } from '@/hooks/use-status'

type Lang = 'curl' | 'python' | 'javascript'

const LANG_LABELS: Record<Lang, string> = {
  curl: 'cURL',
  python: 'Python',
  javascript: 'JavaScript',
}

const LANG_HIGHLIGHT: Record<Lang, BundledLanguage> = {
  curl: 'bash',
  python: 'python',
  javascript: 'javascript',
}

interface SampleContext {
  baseUrl: string
  model: string
}

function buildOpenAiSample(lang: Lang, ctx: SampleContext): string {
  const url = `${ctx.baseUrl}/v1/chat/completions`
  const userMessage = 'Explain quantum entanglement in plain language.'
  if (lang === 'curl') {
    return [
      `curl ${url} \\`,
      `  -H "Content-Type: application/json" \\`,
      `  -H "Authorization: Bearer sk-YOUR_TOKEN" \\`,
      `  -d '{`,
      `    "model": "${ctx.model}",`,
      `    "messages": [`,
      `      { "role": "user", "content": "${userMessage}" }`,
      `    ],`,
      `    "stream": true`,
      `  }'`,
    ].join('\n')
  }
  if (lang === 'python') {
    return [
      'from openai import OpenAI',
      '',
      'client = OpenAI(',
      `    base_url="${ctx.baseUrl}/v1",`,
      '    api_key="sk-YOUR_TOKEN",',
      ')',
      '',
      'completion = client.chat.completions.create(',
      `    model="${ctx.model}",`,
      `    messages=[{"role": "user", "content": "${userMessage}"}],`,
      '    stream=True,',
      ')',
      '',
      'for chunk in completion:',
      '    print(chunk.choices[0].delta.content or "", end="")',
    ].join('\n')
  }
  return [
    `const response = await fetch('${url}', {`,
    `  method: 'POST',`,
    `  headers: {`,
    `    'Content-Type': 'application/json',`,
    `    Authorization: 'Bearer sk-YOUR_TOKEN',`,
    `  },`,
    `  body: JSON.stringify({`,
    `    model: '${ctx.model}',`,
    `    messages: [{ role: 'user', content: '${userMessage}' }],`,
    `    stream: true,`,
    `  }),`,
    `})`,
    '',
    `const data = await response.json()`,
    `console.log(data)`,
  ].join('\n')
}

function buildAnthropicSample(lang: Lang, ctx: SampleContext): string {
  const url = `${ctx.baseUrl}/v1/messages`
  const userMessage = 'Explain quantum entanglement in plain language.'
  if (lang === 'curl') {
    return [
      `curl ${url} \\`,
      `  -H "Content-Type: application/json" \\`,
      `  -H "x-api-key: sk-YOUR_TOKEN" \\`,
      `  -H "anthropic-version: 2023-06-01" \\`,
      `  -d '{`,
      `    "model": "${ctx.model}",`,
      `    "max_tokens": 1024,`,
      `    "messages": [`,
      `      { "role": "user", "content": "${userMessage}" }`,
      `    ]`,
      `  }'`,
    ].join('\n')
  }
  if (lang === 'python') {
    return [
      'import anthropic',
      '',
      'client = anthropic.Anthropic(',
      `    base_url="${ctx.baseUrl}",`,
      '    api_key="sk-YOUR_TOKEN",',
      ')',
      '',
      'message = client.messages.create(',
      `    model="${ctx.model}",`,
      '    max_tokens=1024,',
      `    messages=[{"role": "user", "content": "${userMessage}"}],`,
      ')',
      '',
      'print(message.content[0].text)',
    ].join('\n')
  }
  return [
    `const response = await fetch('${url}', {`,
    `  method: 'POST',`,
    `  headers: {`,
    `    'Content-Type': 'application/json',`,
    `    'x-api-key': 'sk-YOUR_TOKEN',`,
    `    'anthropic-version': '2023-06-01',`,
    `  },`,
    `  body: JSON.stringify({`,
    `    model: '${ctx.model}',`,
    `    max_tokens: 1024,`,
    `    messages: [{ role: 'user', content: '${userMessage}' }],`,
    `  }),`,
    `})`,
    '',
    `const data = await response.json()`,
    `console.log(data.content[0].text)`,
  ].join('\n')
}

function SampleCard(props: {
  title: string
  titleBadge?: string
  endpoint: string
  buildSample: (lang: Lang, ctx: SampleContext) => string
  context: SampleContext
}) {
  const { t } = useTranslation()
  const [lang, setLang] = useState<Lang>('curl')
  const code = props.buildSample(lang, props.context)

  return (
    <section className='border-border/60 bg-card/60 rounded-xl border p-4 shadow-sm'>
      <div className='flex flex-wrap items-center gap-2'>
        <Badge variant='secondary' className='font-medium'>
          {props.title}
        </Badge>
        {props.titleBadge && (
          <span className='text-muted-foreground text-xs'>
            {props.titleBadge}
          </span>
        )}
        <Tabs
          className='ml-auto'
          value={lang}
          onValueChange={(value) => setLang(value as Lang)}
        >
          <TabsList className='h-7 p-0.5'>
            {(Object.keys(LANG_LABELS) as Lang[]).map((item) => (
              <TabsTrigger
                className='h-6 px-2.5 text-xs'
                key={item}
                value={item}
              >
                {LANG_LABELS[item]}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
      <div className='text-muted-foreground mt-2 flex items-center gap-1.5 text-xs'>
        <code className='bg-muted rounded px-1.5 py-0.5 font-mono'>
          {props.endpoint}
        </code>
        <span>{t('compatible endpoint')}</span>
      </div>
      <div className='mt-3'>
        <CodeBlock code={code} language={LANG_HIGHLIGHT[lang]}>
          <CodeBlockCopyButton />
        </CodeBlock>
      </div>
    </section>
  )
}

export function LabExamples(props: { model?: string }) {
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

  const context: SampleContext = { baseUrl, model: props.model ?? '' }

  return (
    <div className='space-y-4'>
      <div>
        <h2 className='text-lg font-semibold'>{t('Code samples')}</h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Both gateway-compatible call styles are shown below. Current model:')}
        </p>
        <Badge variant='outline' className='mt-2 font-mono'>
          {props.model || t('No model selected')}
        </Badge>
      </div>

      <SampleCard
        buildSample={buildOpenAiSample}
        context={context}
        endpoint='/v1/chat/completions'
        title={t('Option 1: OpenAI Chat compatible')}
        titleBadge={t('Recommended for most SDKs')}
      />
      <SampleCard
        buildSample={buildAnthropicSample}
        context={context}
        endpoint='/v1/messages'
        title={t('Option 2: Anthropic compatible')}
        titleBadge={t('Uses x-api-key and anthropic-version headers')}
      />

      <div className='border-border/60 bg-muted/30 text-muted-foreground rounded-lg border px-3 py-2 text-xs leading-relaxed'>
        {t(
          'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.'
        )}
      </div>
    </div>
  )
}
