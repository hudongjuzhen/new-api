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
import {
  ChevronDownIcon,
  ImagePlusIcon,
  Loader2Icon,
  PlayIcon,
  SquareIcon,
  XIcon,
} from 'lucide-react'
import { nanoid } from 'nanoid'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { uploadPlaygroundImage } from '@/features/playground/api'
import { IMAGE_UPLOAD } from '@/features/playground/constants'
import { Link } from '@tanstack/react-router'
import { cn } from '@/lib/utils'

import { appendLabHistory } from './lib/history'
import { extractGeneratedImages } from './lib/image'
import { useLabKeys } from './use-lab-keys'
import { useLabRun } from './use-lab-run'

type InputMode = 'form' | 'json'

interface PendingImage {
  id: string
  name: string
  previewUrl: string
  url?: string
  status: 'uploading' | 'ready' | 'error'
}

interface LabPayloadMessage {
  role: string
  content: unknown
}

function extractPromptPreview(payload: Record<string, unknown>): string {
  const messages = Array.isArray(payload.messages)
    ? (payload.messages as LabPayloadMessage[])
    : []
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i]
    if (message?.role !== 'user') continue
    if (typeof message.content === 'string') return message.content
    if (Array.isArray(message.content)) {
      const textPart = message.content.find(
        (part) => part && part.type === 'text'
      ) as { text?: string } | undefined
      return textPart?.text ?? ''
    }
    return ''
  }
  return ''
}

function ParameterSlider(props: {
  label: string
  value: number
  min: number
  max: number
  step: number
  onChange: (value: number) => void
}) {
  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-sm font-medium'>{props.label}</span>
        <Badge variant='outline' className='font-mono'>
          {props.value.toFixed(2)}
        </Badge>
      </div>
      <Slider
        max={props.max}
        min={props.min}
        step={props.step}
        value={[props.value]}
        onValueChange={(next) =>
          props.onChange(Array.isArray(next) ? next[0] : next)
        }
      />
    </div>
  )
}

export function LabPlayground(props: { model?: string }) {
  const { t } = useTranslation()
  const lastPromptRef = useRef('')
  const run = useLabRun((result) => {
    appendLabHistory({
      id: nanoid(),
      model: props.model ?? '',
      prompt: lastPromptRef.current.slice(0, 120),
      status: result.status,
      createdAt: Date.now(),
      durationMs: result.durationMs,
      error: result.error?.slice(0, 200),
    })
  })
  const { state, run: sendRun, stop, isRunning } = run

  const [inputMode, setInputMode] = useState<InputMode>('form')
  const [promptText, setPromptText] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [pendingImages, setPendingImages] = useState<PendingImage[]>([])
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(4096)
  const [topP, setTopP] = useState(1)
  const [frequencyPenalty, setFrequencyPenalty] = useState(0)
  const [presencePenalty, setPresencePenalty] = useState(0)
  const [stream, setStream] = useState(true)
  const [jsonDraft, setJsonDraft] = useState('')
  const outputScrollRef = useRef<HTMLDivElement | null>(null)

  const {
    user,
    enabledKeys,
    selectedKey,
    setSelectedKeyId,
    resolveRealKey,
  } = useLabKeys()

  useEffect(() => {
    const node = outputScrollRef.current
    if (node) node.scrollTop = node.scrollHeight
  }, [state.content, state.reasoning, state.rawLines])

  const buildFormPayload = (): Record<string, unknown> => {
    const readyUrls = pendingImages
      .filter((image) => image.status === 'ready' && image.url)
      .map((image) => image.url as string)
    const userContent: unknown =
      readyUrls.length > 0
        ? [
            { type: 'text', text: promptText },
            ...readyUrls.map((url) => ({
              type: 'image_url',
              image_url: { url },
            })),
          ]
        : promptText
    const messages: LabPayloadMessage[] = []
    if (systemPrompt.trim()) {
      messages.push({ role: 'system', content: systemPrompt.trim() })
    }
    messages.push({ role: 'user', content: userContent })
    return {
      model: props.model,
      messages,
      stream,
      temperature,
      max_tokens: maxTokens,
      top_p: topP,
      frequency_penalty: frequencyPenalty,
      presence_penalty: presencePenalty,
    }
  }

  const syncJsonDraft = () => {
    setJsonDraft(JSON.stringify(buildFormPayload(), null, 2))
  }

  const switchInputMode = (mode: InputMode) => {
    if (mode === 'json' && inputMode === 'form') {
      syncJsonDraft()
    }
    setInputMode(mode)
  }

  const handleImagesPicked = (files: FileList | null) => {
    const imageFiles = [...(files ?? [])].filter((file) =>
      file.type.startsWith('image/')
    )
    if (imageFiles.length === 0) {
      toast.error(t('Please select image files'))
      return
    }
    const remainingSlots = IMAGE_UPLOAD.MAX_FILES - pendingImages.length
    if (remainingSlots <= 0) {
      toast.error(
        t('You can attach up to {{count}} images', {
          count: IMAGE_UPLOAD.MAX_FILES,
        })
      )
      return
    }
    for (const file of imageFiles.slice(0, remainingSlots)) {
      const id = nanoid()
      const previewUrl = URL.createObjectURL(file)
      setPendingImages((prev) => [
        ...prev,
        { id, name: file.name, previewUrl, status: 'uploading' },
      ])
      uploadPlaygroundImage(file)
        .then((url) => {
          setPendingImages((prev) =>
            prev.map((image) =>
              image.id === id ? { ...image, url, status: 'ready' } : image
            )
          )
        })
        .catch(() => {
          setPendingImages((prev) =>
            prev.map((image) =>
              image.id === id ? { ...image, status: 'error' } : image
            )
          )
          toast.error(t('Image upload failed'))
        })
    }
  }

  const removeImage = (id: string) => {
    setPendingImages((prev) => {
      const target = prev.find((image) => image.id === id)
      if (target) URL.revokeObjectURL(target.previewUrl)
      return prev.filter((image) => image.id !== id)
    })
  }

  const handleRun = async () => {
    if (!props.model || isRunning) return

    let apiKey: string | undefined
    if (selectedKey) {
      try {
        apiKey = await resolveRealKey(selectedKey)
      } catch {
        toast.error(t('Failed to fetch the API key'))
        return
      }
    }

    if (inputMode === 'json') {
      let payload: Record<string, unknown>
      try {
        const parsed: unknown = JSON.parse(jsonDraft)
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          throw new Error('invalid')
        }
        payload = parsed as Record<string, unknown>
        if (!Array.isArray(payload.messages)) {
          throw new Error('invalid')
        }
      } catch {
        toast.error(t('Invalid JSON request body'))
        return
      }
      if (!payload.model) payload.model = props.model
      if (typeof payload.stream !== 'boolean') payload.stream = true
      lastPromptRef.current = extractPromptPreview(payload)
      sendRun(payload, { apiKey })
      return
    }

    if (!promptText.trim() && pendingImages.length === 0) {
      toast.error(t('Please enter the conversation content'))
      return
    }
    if (pendingImages.some((image) => image.status === 'uploading')) {
      toast.info(t('Please wait for the image to finish uploading'))
      return
    }
    lastPromptRef.current = promptText
    sendRun(buildFormPayload(), { apiKey })
  }

  const hasOutput = state.content.length > 0 || state.reasoning.length > 0
  const { images: generatedImages, displayContent } = useMemo(
    () => extractGeneratedImages(state.content),
    [state.content]
  )
  const outputJson =
    state.rawLines.length > 0 ? state.rawLines.join('\n') : ''

  let actionButton: ReactNode
  if (!user) {
    actionButton = (
      <Button
        className='w-full'
        render={
          <Link
            search={{
              redirect: props.model ? `/lab?model=${props.model}` : '/lab',
            }}
            to='/sign-in'
          />
        }
      >
        {t('Sign in to try')}
      </Button>
    )
  } else if (isRunning) {
    actionButton = (
      <Button className='w-full' onClick={stop} variant='destructive'>
        <SquareIcon className='fill-current size-4' />
        {t('Stop')}
      </Button>
    )
  } else {
    actionButton = (
      <Button className='w-full' disabled={!props.model} onClick={handleRun}>
        <PlayIcon className='size-4' />
        {t('Run')}
      </Button>
    )
  }

  return (
    <div className='grid gap-4 lg:grid-cols-2'>
      {/* INPUT panel */}
      <section className='border-border/60 bg-card/60 rounded-xl border shadow-sm'>
        <div className='border-border/60 flex items-center justify-between gap-2 border-b px-4 py-3'>
          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
              {t('Input')}
            </span>
            <Tabs
              value={inputMode}
              onValueChange={(value) => switchInputMode(value as InputMode)}
            >
              <TabsList className='h-7 p-0.5'>
                <TabsTrigger value='form' className='h-6 px-2.5 text-xs'>
                  {t('Form')}
                </TabsTrigger>
                <TabsTrigger value='json' className='h-6 px-2.5 text-xs'>
                  {t('JSON')}
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </div>
        </div>

        <div className='space-y-4 px-4 py-4'>
          {inputMode === 'form' ? (
            <>
              <div className='flex flex-wrap items-center gap-2'>
                <span className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
                  {t('API Format')}
                </span>
                <Button size='xs' className='h-7 px-2.5 text-xs'>
                  {t('OpenAI Chat')}
                </Button>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        size='xs'
                        variant='outline'
                        disabled
                        className='h-7 px-2.5 text-xs'
                      >
                        {t('Anthropic')}
                      </Button>
                    }
                  />
                  <TooltipContent>
                    <p>
                      {t(
                        'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).'
                      )}
                    </p>
                  </TooltipContent>
                </Tooltip>
                <code className='text-muted-foreground/70 ml-1 font-mono text-xs'>
                  /v1/chat/completions
                </code>
              </div>

              <div className='space-y-2'>
                <label
                  className='text-sm font-medium'
                  htmlFor='lab-prompt-text'
                >
                  {t('Conversation content')}
                </label>
                <Textarea
                  id='lab-prompt-text'
                  className='min-h-28'
                  disabled={isRunning}
                  onChange={(event) => setPromptText(event.target.value)}
                  placeholder={t('Explain quantum entanglement in plain language')}
                  value={promptText}
                />
              </div>

              <div className='flex flex-wrap items-center gap-2'>
                <label className='relative cursor-pointer'>
                  <input
                    accept={IMAGE_UPLOAD.ACCEPT}
                    className='sr-only'
                    disabled={isRunning}
                    multiple
                    onChange={(event) => {
                      handleImagesPicked(event.target.files)
                      event.target.value = ''
                    }}
                    type='file'
                  />
                  <span className='border-border/70 text-muted-foreground hover:bg-muted/60 hover:text-foreground inline-flex h-8 items-center gap-1.5 rounded-md border px-3 text-xs font-medium transition-colors'>
                    <ImagePlusIcon className='size-3.5' />
                    {t('Upload images')}
                  </span>
                </label>
                {pendingImages.map((image) => (
                  <div
                    className='border-border/60 relative size-12 overflow-hidden rounded-md border'
                    key={image.id}
                  >
                    <img
                      alt={image.name}
                      className='size-full object-cover'
                      src={image.previewUrl}
                    />
                    {image.status === 'uploading' && (
                      <div className='absolute inset-0 flex items-center justify-center bg-black/40'>
                        <Loader2Icon className='size-3 animate-spin text-white' />
                      </div>
                    )}
                    {image.status === 'error' && (
                      <div className='bg-destructive/70 absolute inset-0' />
                    )}
                    <button
                      aria-label={t('Remove image')}
                      className='absolute top-0 right-0 flex size-4 items-center justify-center rounded-full bg-black/60 text-white hover:bg-black/80'
                      onClick={() => removeImage(image.id)}
                      type='button'
                    >
                      <XIcon className='size-2.5' />
                    </button>
                  </div>
                ))}
              </div>

              <div className='space-y-2'>
                <label
                  className='text-sm font-medium'
                  htmlFor='lab-system-prompt'
                >
                  {t('System prompt')}
                </label>
                <Textarea
                  id='lab-system-prompt'
                  className='text-muted-foreground min-h-16'
                  disabled={isRunning}
                  onChange={(event) => setSystemPrompt(event.target.value)}
                  placeholder={t(
                    'Optional: set the model role, background or behavior constraints…'
                  )}
                  value={systemPrompt}
                />
              </div>

              <Collapsible>
                <CollapsibleTrigger className='group/params w-full'>
                  <span className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-xs font-medium transition-colors'>
                    {t('More parameters')}
                    <ChevronDownIcon className='size-3.5 transition-transform group-data-[panel-open]/params:rotate-180' />
                  </span>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className='grid gap-4 pt-4 sm:grid-cols-2'>
                    <ParameterSlider
                      label={t('Temperature')}
                      max={2}
                      min={0}
                      onChange={setTemperature}
                      step={0.01}
                      value={temperature}
                    />
                    <div className='space-y-2'>
                      <div className='flex items-center justify-between gap-2'>
                        <span className='text-sm font-medium'>
                          {t('Max output tokens')}
                        </span>
                        <Badge variant='outline' className='font-mono'>
                          {maxTokens}
                        </Badge>
                      </div>
                      <Input
                        disabled={isRunning}
                        min={1}
                        onChange={(event) =>
                          setMaxTokens(
                            Math.max(1, Number(event.target.value) || 1)
                          )
                        }
                        type='number'
                        value={maxTokens}
                      />
                    </div>
                    <ParameterSlider
                      label={t('Top P')}
                      max={1}
                      min={0}
                      onChange={setTopP}
                      step={0.01}
                      value={topP}
                    />
                    <ParameterSlider
                      label={t('Frequency penalty')}
                      max={2}
                      min={0}
                      onChange={setFrequencyPenalty}
                      step={0.01}
                      value={frequencyPenalty}
                    />
                    <ParameterSlider
                      label={t('Presence penalty')}
                      max={2}
                      min={0}
                      onChange={setPresencePenalty}
                      step={0.01}
                      value={presencePenalty}
                    />
                  </div>
                </CollapsibleContent>
              </Collapsible>
            </>
          ) : (
            <Textarea
              className='min-h-[26rem] font-mono text-xs'
              disabled={isRunning}
              onChange={(event) => setJsonDraft(event.target.value)}
              spellCheck={false}
              value={jsonDraft}
            />
          )}

          {user && (
            <div className='space-y-2'>
              <div className='flex items-center gap-2'>
                <span className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
                  {t('API Key')}
                </span>
                {selectedKey && (
                  <Badge
                    className='font-mono text-xs font-normal'
                    variant='outline'
                  >
                    {selectedKey.group
                      ? selectedKey.group
                      : t('Follow user group')}
                  </Badge>
                )}
              </div>
              <Select
                items={[
                  { value: 'session', label: t('Signed-in session (default)') },
                  ...enabledKeys.map((key) => ({
                    value: String(key.id),
                    label: key.name,
                  })),
                ]}
                value={selectedKey ? String(selectedKey.id) : 'session'}
                onValueChange={(value) =>
                  value !== null &&
                  setSelectedKeyId(value === 'session' ? '' : value)
                }
              >
                <SelectTrigger className='w-full' disabled={isRunning}>
                  <SelectValue>
                    {selectedKey
                      ? selectedKey.name
                      : t('Signed-in session (default)')}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectItem value='session'>
                    {t('Signed-in session (default)')}
                  </SelectItem>
                  {enabledKeys.map((key) => (
                    <SelectItem key={key.id} value={String(key.id)}>
                      {key.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedKey && (
                <p className='text-muted-foreground text-xs'>
                  {t('Requests will run with the group of the selected API key.')}
                </p>
              )}
            </div>
          )}

          <div className='border-border/60 flex items-center justify-between gap-3 border-t pt-4'>
            <div className='flex items-center gap-2'>
              <Switch
                checked={stream}
                disabled={isRunning}
                id='lab-stream-switch'
                onCheckedChange={setStream}
              />
              <div className='space-y-0.5'>
                <label className='text-sm font-medium' htmlFor='lab-stream-switch'>
                  {t('Streaming output')}
                </label>
                <p className='text-muted-foreground text-xs'>
                  {t('Stream tokens as they are generated; disable to wait for the full response.')}
                </p>
              </div>
            </div>
          </div>

          {actionButton}
          {!props.model && (
            <p className='text-muted-foreground/70 text-center text-xs'>
              {t('No model selected. Pick one from the model marketplace first.')}
            </p>
          )}
        </div>
      </section>

      {/* OUTPUT panel */}
      <section className='border-border/60 bg-card/60 rounded-xl border shadow-sm'>
        <div className='border-border/60 flex items-center justify-between gap-2 border-b px-4 py-3'>
          <span className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
            {t('Output')}
          </span>
          <div className='flex items-center gap-2'>
            {state.durationMs != null && (
              <span className='text-muted-foreground/70 font-mono text-xs tabular-nums'>
                {state.durationMs >= 1000
                  ? `${(state.durationMs / 1000).toFixed(1)}s`
                  : `${state.durationMs}ms`}
              </span>
            )}
            {state.content && displayContent && (
              <CopyButton value={displayContent} iconClassName='size-3.5' />
            )}
          </div>
        </div>
        <Tabs defaultValue='preview'>
          <div className='border-border/60 border-b px-4 py-2'>
            <TabsList className='h-7 p-0.5'>
              <TabsTrigger className='h-6 px-2.5 text-xs' value='preview'>
                {t('Preview')}
              </TabsTrigger>
              <TabsTrigger className='h-6 px-2.5 text-xs' value='json'>
                {t('JSON')}
              </TabsTrigger>
            </TabsList>
          </div>
          <div
            className='max-h-[32rem] min-h-[16rem] overflow-y-auto px-4 py-4'
            ref={outputScrollRef}
          >
            {isRunning && !hasOutput && (
              <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                <Loader2Icon className='size-4 animate-spin' />
                {t('Generating…')}
              </div>
            )}
            {state.phase === 'error' && state.error && (
              <p className='text-destructive mb-3 text-sm whitespace-pre-wrap'>
                {state.error}
              </p>
            )}
            <TabsContent value='preview' className='outline-none'>
              {state.reasoning && (
                <div className='bg-muted/40 text-muted-foreground mb-3 rounded-lg border px-3 py-2 text-xs whitespace-pre-wrap'>
                  {state.reasoning}
                </div>
              )}
              {generatedImages.length > 0 && (
                <div className='mb-3 flex flex-wrap gap-2'>
                  {generatedImages.map((src) => (
                    <img
                      alt=''
                      className='border-border/60 max-h-64 max-w-full rounded-lg border object-contain'
                      key={src}
                      src={src}
                    />
                  ))}
                </div>
              )}
              {hasOutput && displayContent ? (
                <div className='text-sm leading-relaxed whitespace-pre-wrap'>
                  {displayContent}
                </div>
              ) : null}
              {!hasOutput &&
                !isRunning &&
                state.phase !== 'error' && (
                  <p className='text-muted-foreground/60 text-sm'>
                    {t('Run a request to see the model response here.')}
                  </p>
                )}
            </TabsContent>
            <TabsContent value='json' className='outline-none'>
              {outputJson ? (
                <pre
                  className={cn(
                    'text-muted-foreground font-mono text-xs break-all whitespace-pre-wrap'
                  )}
                >
                  {outputJson}
                </pre>
              ) : (
                <p className='text-muted-foreground/60 text-sm'>
                  {t('Raw stream events will appear here.')}
                </p>
              )}
            </TabsContent>
          </div>
        </Tabs>
      </section>
    </div>
  )
}
