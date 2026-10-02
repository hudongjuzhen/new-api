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
  AudioLinesIcon,
  ChevronDownIcon,
  DownloadIcon,
  Loader2Icon,
  PlayIcon,
  PlusIcon,
  SquareIcon,
  XIcon,
} from 'lucide-react'
import { nanoid } from 'nanoid'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import {
  AUDIO_GENERATION_ENDPOINT,
  AUDIO_MAX_AUDIO_REFERENCES,
  AUDIO_MAX_IMAGE_REFERENCES,
  AUDIO_MAX_PROMPT_CHARS,
  AUDIO_OUTPUT_FORMATS,
  buildAudioGenerationBody,
  clampLoudnessRate,
  clampPitchRate,
  clampSpeechRate,
  decodeAudioResult,
  defaultSampleRateForFormat,
  normalizeSampleRate,
  sampleRatesForFormat,
  type AudioGenerationResponse,
  type AudioOutputFormat,
  type AudioReference,
} from './lib/audio-request'
import { appendLabHistory } from './lib/history'
import { useLabKeys } from './use-lab-keys'

interface AudioRunResult {
  phase: 'idle' | 'running' | 'done' | 'error'
  src: string
  seconds: number
  raw: string
  error: string
  durationMs: number | null
}

const IDLE_RESULT: AudioRunResult = {
  phase: 'idle',
  src: '',
  seconds: 0,
  raw: '',
  error: '',
  durationMs: null,
}

/** Tolerant parse: a non-JSON body must surface as an error, not a crash. */
function parseJsonResponse(text: string): Record<string, unknown> | null {
  try {
    const parsed: unknown = JSON.parse(text)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }
    return parsed as Record<string, unknown>
  } catch {
    return null
  }
}

function errorMessageFrom(
  payload: Record<string, unknown> | null,
  status: number
): string {
  const error = payload?.error
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message) return message
  }
  if (typeof payload?.message === 'string' && payload.message) {
    return payload.message
  }
  return `HTTP ${status}`
}

export function LabAudioPlayground(props: { model?: string }) {
  const { t } = useTranslation()
  const [textPrompt, setTextPrompt] = useState('')
  const [format, setFormat] = useState<AudioOutputFormat>('mp3')
  const [sampleRate, setSampleRate] = useState(
    defaultSampleRateForFormat('mp3')
  )
  const [speechRate, setSpeechRate] = useState(0)
  const [loudnessRate, setLoudnessRate] = useState(0)
  const [pitchRate, setPitchRate] = useState(0)
  const [references, setReferences] = useState<AudioReference[]>([])
  const [referenceDraft, setReferenceDraft] = useState('')
  const [result, setResult] = useState<AudioRunResult>(IDLE_RESULT)
  const [abortController, setAbortController] =
    useState<AbortController | null>(null)

  const { user, enabledKeys, selectedKey, setSelectedKeyId, resolveRealKey } =
    useLabKeys()

  const isRunning = result.phase === 'running'

  const changeFormat = (next: AudioOutputFormat) => {
    setFormat(next)
    setSampleRate((current) => normalizeSampleRate(next, current))
  }

  const addReference = (type: AudioReference['type']) => {
    const url = referenceDraft.trim()
    if (!url) return
    const sameKind = references.filter(
      (reference) => reference.type === type
    ).length
    const limit =
      type === 'image_url'
        ? AUDIO_MAX_IMAGE_REFERENCES
        : AUDIO_MAX_AUDIO_REFERENCES
    if (sameKind >= limit) {
      toast.error(
        type === 'image_url'
          ? t('Image references are limited to {{count}} per request.', {
              count: limit,
            })
          : t('Audio references are limited to {{count}} per request.', {
              count: limit,
            })
      )
      return
    }
    // Image and audio references are mutually exclusive upstream, so attaching
    // one kind replaces the other instead of building an invalid mix.
    setReferences((prev) => [
      ...prev.filter((reference) => reference.type === type),
      { type, url },
    ])
    setReferenceDraft('')
  }

  const handleStop = () => {
    abortController?.abort()
    setAbortController(null)
    setResult((prev) => ({ ...prev, phase: 'idle' }))
  }

  const handleRun = async () => {
    if (!props.model || isRunning) return
    if (!selectedKey) {
      toast.error(t('Select an API key to run audio generation.'))
      return
    }
    const trimmedPrompt = textPrompt.trim()
    if (!trimmedPrompt) {
      toast.error(t('Please enter the text or prompt to synthesize.'))
      return
    }
    if (trimmedPrompt.length > AUDIO_MAX_PROMPT_CHARS) {
      toast.error(
        t('The prompt supports at most {{count}} characters.', {
          count: AUDIO_MAX_PROMPT_CHARS,
        })
      )
      return
    }
    let apiKey: string
    try {
      apiKey = await resolveRealKey(selectedKey)
    } catch {
      toast.error(t('Failed to fetch the API key'))
      return
    }

    const body = buildAudioGenerationBody(props.model, {
      textPrompt: trimmedPrompt,
      format,
      sampleRate,
      speechRate,
      loudnessRate,
      pitchRate,
      references,
    })
    const controller = new AbortController()
    setAbortController(controller)
    const startedAt = Date.now()
    setResult({
      ...IDLE_RESULT,
      phase: 'running',
      raw: JSON.stringify(body, null, 2),
    })

    const recordHistory = (
      status: 'success' | 'error',
      error?: string,
      durationMs?: number
    ) => {
      appendLabHistory({
        id: nanoid(),
        model: props.model ?? '',
        prompt: trimmedPrompt.slice(0, 120),
        status,
        createdAt: Date.now(),
        durationMs,
        error: error?.slice(0, 200),
      })
    }

    let rawText = ''
    try {
      const response = await fetch(AUDIO_GENERATION_ENDPOINT, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${apiKey}`,
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      })
      rawText = await response.text()
      const parsed = parseJsonResponse(rawText)
      if (!response.ok || !parsed) {
        throw new Error(errorMessageFrom(parsed, response.status))
      }

      const decoded = decodeAudioResult(
        parsed as AudioGenerationResponse,
        format
      )
      if (!decoded.src) {
        throw new Error(t('No audio in the API response.'))
      }
      const durationMs = Date.now() - startedAt
      setResult({
        phase: 'done',
        src: decoded.src,
        seconds: decoded.seconds,
        raw: rawText,
        error: '',
        durationMs,
      })
      recordHistory('success', undefined, durationMs)
    } catch (error) {
      if (controller.signal.aborted) return
      const message =
        error instanceof Error && error.message
          ? error.message
          : t('Audio API request failed')
      setResult((prev) => ({ ...prev, phase: 'error', error: message }))
      recordHistory('error', message)
    } finally {
      setAbortController(null)
    }
  }

  const renderParamSelect = (options: {
    value: string
    onChange: (value: string) => void
    items: Array<{ value: string; label: string }>
    label: string
  }) => (
    <div className='space-y-2'>
      <span className='text-sm font-medium'>{options.label}</span>
      <Select
        items={options.items}
        onValueChange={(value) => value !== null && options.onChange(value)}
        value={options.value}
      >
        <SelectTrigger className='w-full' disabled={isRunning}>
          <SelectValue>
            {options.items.find((item) => item.value === options.value)?.label ??
              options.value}
          </SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          {options.items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )

  const renderNumberField = (options: {
    label: string
    value: number
    min: number
    max: number
    onCommit: (value: number) => void
    hint?: string
  }) => (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-sm font-medium'>{options.label}</span>
        <Badge variant='outline' className='font-mono'>
          {options.value}
        </Badge>
      </div>
      <Input
        disabled={isRunning}
        max={options.max}
        min={options.min}
        onBlur={(event) =>
          options.onCommit(Number(event.target.value.trim()) || 0)
        }
        onChange={(event) => options.onCommit(Number(event.target.value) || 0)}
        type='number'
        value={options.value}
      />
      {options.hint && (
        <p className='text-muted-foreground text-xs'>{options.hint}</p>
      )}
    </div>
  )

  return (
    <div className='grid gap-4 lg:grid-cols-2'>
      {/* INPUT panel */}
      <section className='border-border/60 bg-card/60 rounded-xl border shadow-sm'>
        <div className='border-border/60 flex items-center gap-2 border-b px-4 py-3'>
          <span className='text-muted-foreground text-xs font-semibold tracking-wider uppercase'>
            {t('Input')}
          </span>
          <code className='text-muted-foreground/70 ml-auto font-mono text-xs'>
            {AUDIO_GENERATION_ENDPOINT}
          </code>
        </div>
        <div className='space-y-4 px-4 py-4'>
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
                items={enabledKeys.map((key) => ({
                  value: String(key.id),
                  label: key.name,
                }))}
                onValueChange={(value) =>
                  value !== null && setSelectedKeyId(value)
                }
                value={selectedKey ? String(selectedKey.id) : ''}
              >
                <SelectTrigger className='w-full' disabled={isRunning}>
                  <SelectValue>
                    {selectedKey
                      ? selectedKey.name
                      : t('Select an API key to run audio generation.')}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  {enabledKeys.map((key) => (
                    <SelectItem key={key.id} value={String(key.id)}>
                      {key.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {!selectedKey && (
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'This channel calls the audio service with its own API key, so the playground needs one instead of your signed-in session.'
                  )}
                </p>
              )}
            </div>
          )}

          <div className='space-y-2'>
            <div className='flex items-center justify-between gap-2'>
              <span className='text-sm font-medium'>
                {t('Text or prompt to synthesize')}
              </span>
              <span className='text-muted-foreground/70 font-mono text-xs tabular-nums'>
                {textPrompt.length} / {AUDIO_MAX_PROMPT_CHARS}
              </span>
            </div>
            <Textarea
              className='min-h-32'
              disabled={isRunning}
              maxLength={AUDIO_MAX_PROMPT_CHARS}
              onChange={(event) => setTextPrompt(event.target.value)}
              placeholder={t(
                'Paste the text to read aloud, or describe the audio you want (ambience, sound effects, timeline).'
              )}
              value={textPrompt}
            />
          </div>

          <div className='space-y-2'>
            <span className='text-sm font-medium'>
              {t('Reference audio / image (optional)')}
            </span>
            <p className='text-muted-foreground text-xs leading-relaxed'>
              {t(
                'Up to 3 audio references (≤ 30s, wav / mp3 / pcm / ogg_opus) or 1 image reference (jpeg / png / webp). Audio and image references cannot be mixed.'
              )}
            </p>
            <div className='flex flex-wrap gap-2'>
              <Input
                className='h-9 min-w-48 flex-1'
                disabled={isRunning}
                onChange={(event) => setReferenceDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    addReference('audio_url')
                  }
                }}
                placeholder={t('Paste a public audio or image URL')}
                value={referenceDraft}
              />
              <Button
                className='h-9'
                disabled={isRunning || !referenceDraft.trim()}
                onClick={() => addReference('audio_url')}
                size='sm'
                variant='outline'
              >
                <PlusIcon className='size-3.5' />
                {t('Add audio')}
              </Button>
              <Button
                className='h-9'
                disabled={isRunning || !referenceDraft.trim()}
                onClick={() => addReference('image_url')}
                size='sm'
                variant='outline'
              >
                <PlusIcon className='size-3.5' />
                {t('Add image')}
              </Button>
            </div>
            {references.length > 0 && (
              <div className='flex flex-wrap gap-1.5'>
                {references.map((reference) => (
                  <span
                    className='bg-muted/60 inline-flex max-w-full items-center gap-1.5 rounded-md border px-2 py-1 text-xs'
                    key={reference.url}
                  >
                    <Badge
                      className='font-mono text-[10px] font-normal'
                      variant='secondary'
                    >
                      {reference.type === 'image_url'
                        ? t('Image')
                        : t('Audio')}
                    </Badge>
                    <span className='truncate font-mono'>{reference.url}</span>
                    <button
                      aria-label={t('Remove')}
                      className='text-muted-foreground hover:text-foreground'
                      onClick={() =>
                        setReferences((prev) =>
                          prev.filter((item) => item.url !== reference.url)
                        )
                      }
                      type='button'
                    >
                      <XIcon className='size-3' />
                    </button>
                  </span>
                ))}
              </div>
            )}
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
                {renderParamSelect({
                  items: AUDIO_OUTPUT_FORMATS.map((value) => ({
                    value,
                    label: value,
                  })),
                  label: t('Output format'),
                  onChange: (value) => changeFormat(value as AudioOutputFormat),
                  value: format,
                })}
                {renderParamSelect({
                  items: sampleRatesForFormat(format).map((value) => ({
                    value: String(value),
                    label: `${value} Hz`,
                  })),
                  label: t('Sample rate'),
                  onChange: (value) => setSampleRate(Number(value)),
                  value: String(sampleRate),
                })}
                {renderNumberField({
                  label: t('Speech rate'),
                  value: speechRate,
                  min: -50,
                  max: 100,
                  onCommit: (value) => setSpeechRate(clampSpeechRate(value)),
                  hint: t('-50 is 0.5× speed, 100 is 2× speed, 0 keeps it as is.'),
                })}
                {renderNumberField({
                  label: t('Loudness'),
                  value: loudnessRate,
                  min: -50,
                  max: 100,
                  onCommit: (value) => setLoudnessRate(clampLoudnessRate(value)),
                  hint: t('-50 is 0.5× volume, 100 is 2× volume, 0 keeps it as is.'),
                })}
                {renderNumberField({
                  label: t('Pitch'),
                  value: pitchRate,
                  min: -12,
                  max: 12,
                  onCommit: (value) => setPitchRate(clampPitchRate(value)),
                  hint: t('Range -12 to 12; 0 keeps the original pitch.'),
                })}
              </div>
            </CollapsibleContent>
          </Collapsible>

          {user && isRunning ? (
            <Button
              className='w-full'
              onClick={handleStop}
              variant='destructive'
            >
              <SquareIcon className='size-4 fill-current' />
              {t('Stop')}
            </Button>
          ) : (
            <Button
              className='w-full'
              disabled={isRunning || !props.model}
              onClick={() => void handleRun()}
            >
              {isRunning ? (
                <Loader2Icon className='size-4 animate-spin' />
              ) : (
                <PlayIcon className='size-4' />
              )}
              {t('Run')}
            </Button>
          )}
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
            {result.seconds > 0 && (
              <span className='text-muted-foreground/70 font-mono text-xs tabular-nums'>
                {t('{{count}}s audio', { count: result.seconds })}
              </span>
            )}
            {result.durationMs != null && (
              <span className='text-muted-foreground/70 font-mono text-xs tabular-nums'>
                {result.durationMs >= 1000
                  ? `${(result.durationMs / 1000).toFixed(1)}s`
                  : `${result.durationMs}ms`}
              </span>
            )}
            {result.raw && (
              <CopyButton iconClassName='size-3.5' value={result.raw} />
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
          <div className='max-h-[36rem] min-h-[16rem] overflow-y-auto px-4 py-4'>
            <TabsContent className='outline-none' value='preview'>
              {result.phase === 'error' && result.error && (
                <p className='text-destructive mb-3 text-sm whitespace-pre-wrap'>
                  {result.error}
                </p>
              )}
              {isRunning && (
                <div className='text-muted-foreground flex items-center gap-2 py-8 text-sm'>
                  <Loader2Icon className='size-4 animate-spin' />
                  {t('Generating audio, this can take up to a minute…')}
                </div>
              )}
              {result.src && (
                <div className='space-y-3'>
                  <audio className='w-full' controls src={result.src} />
                  <div className='flex flex-wrap items-center gap-2'>
                    <Button
                      render={
                        <a download={`audio.${format}`} href={result.src} />
                      }
                      size='sm'
                      variant='outline'
                    >
                      <DownloadIcon className='size-3.5' />
                      {t('Download audio')}
                    </Button>
                    <span className='text-muted-foreground font-mono text-xs'>
                      {format} · {sampleRate} Hz
                    </span>
                  </div>
                </div>
              )}
              {!isRunning && !result.src && result.phase !== 'error' && (
                <div className='border-border/60 bg-muted/20 flex h-72 flex-col items-center justify-center gap-2 rounded-xl border border-dashed px-6 text-center'>
                  <AudioLinesIcon className='text-muted-foreground/50 size-10' />
                  {textPrompt ? (
                    <p className='text-muted-foreground line-clamp-3 max-w-xs text-sm'>
                      {textPrompt}
                    </p>
                  ) : (
                    <p className='text-muted-foreground/60 text-sm'>
                      {t('Run a request to see the model response here.')}
                    </p>
                  )}
                  <span className='text-muted-foreground/60 font-mono text-xs'>
                    {format} · {sampleRate} Hz
                  </span>
                </div>
              )}
            </TabsContent>
            <TabsContent className='outline-none' value='json'>
              {result.raw ? (
                <pre className='text-muted-foreground font-mono text-xs break-all whitespace-pre-wrap'>
                  {result.raw}
                </pre>
              ) : (
                <p className='text-muted-foreground/60 text-sm'>
                  {t('The submitted body and the API response will appear here.')}
                </p>
              )}
            </TabsContent>
          </div>
        </Tabs>
      </section>
    </div>
  )
}
