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
  ImageIcon,
  Loader2Icon,
  PlayIcon,
  UploadCloudIcon,
  XIcon,
} from 'lucide-react'
import { nanoid } from 'nanoid'
import { useEffect, useRef, useState } from 'react'
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
import { cn } from '@/lib/utils'

import { appendLabHistory } from './lib/history'
import {
  buildImageEditValueFields,
  buildImagesGenerationBody,
  extractResultImages,
  IMAGE_EDIT_MAX_IMAGES,
  IMAGE_UPLOAD_ACCEPT,
  isSupportedImageFile,
  type ImageRunParams,
} from './lib/image-request'
import { useLabKeys } from './use-lab-keys'

const IMAGE_GENERATIONS_ENDPOINT = '/v1/images/generations'
const IMAGE_EDITS_ENDPOINT = '/v1/images/edits'

type ImageMode = 'generate' | 'edit'

interface EditImageItem {
  id: string
  file?: File
  url?: string
  /** Object URL for files, the source URL for pasted links. */
  preview: string
}

interface ImagesApiResponse {
  data?: Array<{ url?: string; b64_json?: string }>
  error?: { message?: string }
}

interface ImageRunResult {
  phase: 'idle' | 'running' | 'done' | 'error'
  images: string[]
  raw: string
  error: string
  durationMs: number | null
}

const IDLE_RESULT: ImageRunResult = {
  phase: 'idle',
  images: [],
  raw: '',
  error: '',
  durationMs: null,
}

const SIZE_OPTIONS = ['auto', '1024x1024', '1024x1536', '1536x1024']
const QUALITY_OPTIONS = ['auto', 'low', 'medium', 'high']
const OUTPUT_FORMAT_OPTIONS = ['png', 'jpeg', 'webp']
const BACKGROUND_OPTIONS = ['auto', 'transparent', 'opaque']
const MODERATION_OPTIONS = ['auto', 'low']

function optionLabel(value: string, t: (key: string) => string): string {
  return value === 'auto' ? t('Auto') : value
}

export function LabImagePlayground(props: { model?: string }) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<ImageMode>('generate')
  const [promptText, setPromptText] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [quality, setQuality] = useState('auto')
  const [n, setN] = useState(1)
  const [outputFormat, setOutputFormat] = useState('jpeg')
  const [background, setBackground] = useState('auto')
  const [moderation, setModeration] = useState('auto')
  const [editImages, setEditImages] = useState<EditImageItem[]>([])
  const [imageUrlDraft, setImageUrlDraft] = useState('')
  const [mask, setMask] = useState<EditImageItem | null>(null)
  const [maskUrlDraft, setMaskUrlDraft] = useState('')
  const [result, setResult] = useState<ImageRunResult>(IDLE_RESULT)

  const { user, enabledKeys, selectedKey, setSelectedKeyId, resolveRealKey } =
    useLabKeys()
  const editImagesRef = useRef<EditImageItem[]>([])
  useEffect(() => {
    editImagesRef.current = editImages
  }, [editImages])

  useEffect(() => {
    return () => {
      for (const item of editImagesRef.current) {
        if (item.file) URL.revokeObjectURL(item.preview)
      }
    }
  }, [])

  const isRunning = result.phase === 'running'

  const addFiles = (files: FileList | File[] | null) => {
    if (!files) return
    const accepted: EditImageItem[] = []
    let rejectedCount = 0
    for (const file of files) {
      if (editImages.length + accepted.length >= IMAGE_EDIT_MAX_IMAGES) {
        rejectedCount++
        continue
      }
      if (!isSupportedImageFile(file)) {
        toast.error(t('Unsupported image (JPG / PNG / WebP, max 20MB).'))
        continue
      }
      accepted.push({
        id: nanoid(),
        file,
        preview: URL.createObjectURL(file),
      })
    }
    if (rejectedCount > 0) {
      toast.error(
        t('Maximum {{count}} images allowed', { count: IMAGE_EDIT_MAX_IMAGES })
      )
    }
    if (accepted.length > 0) {
      setEditImages((prev) => [...prev, ...accepted])
    }
  }

  const removeEditImage = (id: string) => {
    setEditImages((prev) => {
      const target = prev.find((item) => item.id === id)
      if (target?.file) URL.revokeObjectURL(target.preview)
      return prev.filter((item) => item.id !== id)
    })
  }

  const addImageUrl = () => {
    const url = imageUrlDraft.trim()
    if (!url) return
    if (editImages.length >= IMAGE_EDIT_MAX_IMAGES) {
      toast.error(
        t('Maximum {{count}} images allowed', { count: IMAGE_EDIT_MAX_IMAGES })
      )
      return
    }
    setEditImages((prev) => [...prev, { id: nanoid(), url, preview: url }])
    setImageUrlDraft('')
  }

  const setMaskFromFiles = (files: FileList | File[] | null) => {
    const file = files?.[0]
    if (!file) return
    if (!isSupportedImageFile(file)) {
      toast.error(t('Unsupported image (JPG / PNG / WebP, max 20MB).'))
      return
    }
    setMask((prev) => {
      if (prev?.file) URL.revokeObjectURL(prev.preview)
      return { id: nanoid(), file, preview: URL.createObjectURL(file) }
    })
  }

  const addMaskUrl = () => {
    const url = maskUrlDraft.trim()
    if (!url) return
    setMask((prev) => {
      if (prev?.file) URL.revokeObjectURL(prev.preview)
      return { id: nanoid(), url, preview: url }
    })
    setMaskUrlDraft('')
  }

  const removeMask = () => {
    setMask((prev) => {
      if (prev?.file) URL.revokeObjectURL(prev.preview)
      return null
    })
  }

  const handleRun = async () => {
    if (!props.model || isRunning) return
    if (!selectedKey) {
      toast.error(t('Select an API key to run image requests.'))
      return
    }
    if (!promptText.trim()) {
      toast.error(t('Please enter the image description'))
      return
    }
    if (mode === 'edit' && editImages.length === 0) {
      toast.error(t('Upload at least one original image.'))
      return
    }
    let apiKey: string
    try {
      apiKey = await resolveRealKey(selectedKey)
    } catch {
      toast.error(t('Failed to fetch the API key'))
      return
    }

    const params: ImageRunParams = {
      prompt: promptText,
      size,
      quality,
      n,
      outputFormat,
      background,
      moderation,
    }
    const startedAt = Date.now()
    setResult({ phase: 'running', images: [], raw: '', error: '', durationMs: null })

    let rawText = ''
    try {
      let resp: Response
      if (mode === 'generate') {
        resp = await fetch(IMAGE_GENERATIONS_ENDPOINT, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${apiKey}`,
          },
          body: JSON.stringify(buildImagesGenerationBody(props.model, params)),
        })
      } else {
        const form = new FormData()
        for (const [key, value] of buildImageEditValueFields(
          props.model,
          params
        )) {
          form.append(key, value)
        }
        const files = editImages.filter(
          (item): item is EditImageItem & { file: File } => Boolean(item.file)
        )
        const urls = editImages.filter(
          (item): item is EditImageItem & { url: string } => Boolean(item.url)
        )
        const fieldName = files.length + urls.length > 1 ? 'image[]' : 'image'
        for (const item of files) {
          form.append(fieldName, item.file, item.file.name)
        }
        for (const item of urls) {
          form.append(fieldName, item.url)
        }
        if (mask?.file) {
          form.append('mask', mask.file, mask.file.name)
        } else if (mask?.url) {
          form.append('mask', mask.url)
        }
        resp = await fetch(IMAGE_EDITS_ENDPOINT, {
          method: 'POST',
          headers: { Authorization: `Bearer ${apiKey}` },
          body: form,
        })
      }
      rawText = await resp.text()
      let parsed: ImagesApiResponse | null = null
      try {
        parsed = JSON.parse(rawText) as ImagesApiResponse
      } catch {
        parsed = null
      }
      const errorMessage = parsed?.error?.message
      if (!resp.ok || errorMessage) {
        throw new Error(errorMessage || `HTTP ${resp.status}`)
      }
      const images = extractResultImages(parsed ?? {}, outputFormat)
      if (images.length === 0) {
        throw new Error(t('No image in the API response.'))
      }
      const durationMs = Date.now() - startedAt
      setResult({ phase: 'done', images, raw: rawText, error: '', durationMs })
      appendLabHistory({
        id: nanoid(),
        model: props.model,
        prompt: promptText,
        status: 'success',
        createdAt: Date.now(),
        durationMs,
      })
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? error.message
          : t('Image API request failed')
      const durationMs = Date.now() - startedAt
      setResult({ phase: 'error', images: [], raw: rawText, error: message, durationMs })
      appendLabHistory({
        id: nanoid(),
        model: props.model,
        prompt: promptText,
        status: 'error',
        createdAt: Date.now(),
        durationMs,
        error: message,
      })
    }
  }

  const renderUploadArea = (options: {
    onFiles: (files: FileList | File[] | null) => void
    multiple: boolean
  }) => (
    <label
      className='border-border/60 bg-background hover:border-primary/50 hover:bg-primary/5 flex cursor-pointer flex-col items-center justify-center gap-1 rounded-xl border border-dashed px-4 py-5 text-center transition-colors'
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => {
        event.preventDefault()
        options.onFiles(event.dataTransfer.files)
      }}
    >
      <UploadCloudIcon className='text-muted-foreground size-5' />
      <span className='text-muted-foreground text-xs'>
        {t('Click or drag images here')}
      </span>
      <span className='text-muted-foreground/70 text-xs'>
        {t('Supports JPG / PNG / WebP, up to 20MB each')}
      </span>
      <span className='bg-primary text-primary-foreground mt-1 inline-flex items-center gap-1 rounded-md px-3 py-1.5 text-xs font-medium'>
        <ImagePlusIcon className='size-3.5' />
        {t('Upload images')}
      </span>
      <input
        accept={IMAGE_UPLOAD_ACCEPT}
        className='hidden'
        multiple={options.multiple}
        type='file'
        onChange={(event) => {
          options.onFiles(event.target.files)
          event.target.value = ''
        }}
      />
    </label>
  )

  const renderParamSelect = (options: {
    value: string
    onChange: (value: string) => void
    items: string[]
    label: string
    disabled?: boolean
  }) => (
    <div className='space-y-2'>
      <span className='text-sm font-medium'>{options.label}</span>
      <Select
        items={options.items.map((value) => ({
          value,
          label: optionLabel(value, t),
        }))}
        onValueChange={(value) => value !== null && options.onChange(value)}
        value={options.value}
      >
        <SelectTrigger
          className='w-full'
          disabled={options.disabled ?? isRunning}
        >
          <SelectValue>{optionLabel(options.value, t)}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          {options.items.map((value) => (
            <SelectItem key={value} value={value}>
              {optionLabel(value, t)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
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
                items={[
                  { value: 'session', label: t('Signed-in session (default)') },
                  ...enabledKeys.map((key) => ({
                    value: String(key.id),
                    label: key.name,
                  })),
                ]}
                onValueChange={(value) =>
                  value !== null && setSelectedKeyId(value === 'session' ? '' : value)
                }
                value={selectedKey ? String(selectedKey.id) : 'session'}
              >
                <SelectTrigger className='w-full' disabled={isRunning}>
                  <SelectValue>
                    {selectedKey
                      ? selectedKey.name
                      : t('Signed-in session (default)')}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  {enabledKeys.length === 0 && (
                    <div className='text-muted-foreground px-3 py-2 text-xs'>
                      {t('Select an API key to run image requests.')}
                    </div>
                  )}
                  {enabledKeys.map((key) => (
                    <SelectItem key={key.id} value={String(key.id)}>
                      {key.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {!selectedKey && enabledKeys.length > 0 && (
                <p className='text-muted-foreground text-xs'>
                  {t('Select an API key to run image requests.')}
                </p>
              )}
            </div>
          )}

          <Tabs
            onValueChange={(value) =>
              value !== null && setMode(value as ImageMode)
            }
            value={mode}
          >
            <TabsList className='group-data-horizontal/tabs:h-auto grid h-auto w-full grid-cols-2 p-1'>
              <TabsTrigger className='h-auto flex-col gap-0.5 py-2' value='generate'>
                <span className='text-sm font-medium'>{t('Image generation')}</span>
                <span className='text-muted-foreground font-mono text-[10px]'>
                  /v1/images/generations
                </span>
              </TabsTrigger>
              <TabsTrigger className='h-auto flex-col gap-0.5 py-2' value='edit'>
                <span className='text-sm font-medium'>{t('Image editing')}</span>
                <span className='text-muted-foreground font-mono text-[10px]'>
                  /v1/images/edits
                </span>
              </TabsTrigger>
            </TabsList>
          </Tabs>

          <div className='space-y-2'>
            <span className='text-sm font-medium'>{t('Image description')}</span>
            <Textarea
              className='min-h-20'
              disabled={isRunning}
              onChange={(event) => setPromptText(event.target.value)}
              placeholder={t('Describe the image you want to generate…')}
              value={promptText}
            />
          </div>

          {mode === 'edit' && (
            <>
              <div className='space-y-2'>
                <span className='text-sm font-medium'>
                  {t('Original image ({{count}}/16)', {
                    count: editImages.length,
                  })}
                </span>
                {editImages.length === 0
                  ? renderUploadArea({ multiple: true, onFiles: addFiles })
                  : null}
                {editImages.length > 0 && (
                  <div className='grid grid-cols-4 gap-2'>
                    {editImages.map((item) => (
                      <div
                        className='border-border/60 group relative overflow-hidden rounded-lg border'
                        key={item.id}
                      >
                        <img
                          alt=''
                          className='aspect-square w-full object-cover'
                          src={item.preview}
                        />
                        <button
                          className='bg-background/80 absolute top-1 right-1 rounded-md border p-1 hover:text-destructive'
                          onClick={() => removeEditImage(item.id)}
                          type='button'
                        >
                          <XIcon className='size-3.5' />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
                <div className='flex gap-2'>
                  <Input
                    className='h-9 flex-1'
                    disabled={isRunning}
                    onChange={(event) => setImageUrlDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault()
                        addImageUrl()
                      }
                    }}
                    placeholder={t('Or paste an image URL, then press Add')}
                    value={imageUrlDraft}
                  />
                  <Button
                    className='h-9'
                    disabled={isRunning || !imageUrlDraft.trim()}
                    onClick={addImageUrl}
                    size='sm'
                    variant='outline'
                  >
                    {t('Add')}
                  </Button>
                </div>
              </div>

              <div className='space-y-2'>
                <span className='text-sm font-medium'>
                  {t('Mask image (optional)')}
                </span>
                {!mask
                  ? renderUploadArea({
                      multiple: false,
                      onFiles: setMaskFromFiles,
                    })
                  : null}
                {mask && (
                  <div className='grid grid-cols-4'>
                    <div className='border-border/60 relative overflow-hidden rounded-lg border'>
                      <img
                        alt=''
                        className='aspect-square w-full object-cover'
                        src={mask.preview}
                      />
                      <button
                        className='bg-background/80 absolute top-1 right-1 rounded-md border p-1 hover:text-destructive'
                        onClick={removeMask}
                        type='button'
                      >
                        <XIcon className='size-3.5' />
                      </button>
                    </div>
                  </div>
                )}
                <div className='flex gap-2'>
                  <Input
                    className='h-9 flex-1'
                    disabled={isRunning}
                    onChange={(event) => setMaskUrlDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault()
                        addMaskUrl()
                      }
                    }}
                    placeholder={t('Optional: paste a mask image URL or upload')}
                    value={maskUrlDraft}
                  />
                  <Button
                    className='h-9'
                    disabled={isRunning || !maskUrlDraft.trim()}
                    onClick={addMaskUrl}
                    size='sm'
                    variant='outline'
                  >
                    {t('Add')}
                  </Button>
                </div>
              </div>
            </>
          )}

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
                  items: SIZE_OPTIONS,
                  label: t('Resolution'),
                  onChange: setSize,
                  value: size,
                })}
                {renderParamSelect({
                  items: QUALITY_OPTIONS,
                  label: t('Quality'),
                  onChange: setQuality,
                  value: quality,
                })}
                <div className='space-y-2'>
                  <span className='text-sm font-medium'>{t('N (count)')}</span>
                  <Input
                    disabled={isRunning}
                    max={IMAGE_EDIT_MAX_IMAGES}
                    min={1}
                    onChange={(event) => {
                      const value = Number(event.target.value)
                      setN(Number.isFinite(value) ? value : 1)
                    }}
                    type='number'
                    value={n}
                  />
                </div>
                {renderParamSelect({
                  items: OUTPUT_FORMAT_OPTIONS,
                  label: t('Output format'),
                  onChange: setOutputFormat,
                  value: outputFormat,
                })}
                {mode === 'edit' && (
                  <>
                    {renderParamSelect({
                      items: BACKGROUND_OPTIONS,
                      label: t('Background mode'),
                      onChange: setBackground,
                      value: background,
                    })}
                    {renderParamSelect({
                      items: MODERATION_OPTIONS,
                      label: t('Content moderation'),
                      onChange: setModeration,
                      value: moderation,
                    })}
                  </>
                )}
              </div>
            </CollapsibleContent>
          </Collapsible>

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
            Run
          </Button>
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
                  {t('Generating…')}
                </div>
              )}
              {result.images.length > 0 && (
                <div
                  className={cn(
                    'grid gap-2',
                    result.images.length > 1 && 'grid-cols-2'
                  )}
                >
                  {result.images.map((src) => (
                    <img
                      alt=''
                      className='border-border/60 max-h-[30rem] w-full rounded-lg border object-contain'
                      key={src}
                      src={src}
                    />
                  ))}
                </div>
              )}
              {!isRunning &&
                result.images.length === 0 &&
                result.phase !== 'error' && (
                  <div className='border-border/60 bg-muted/20 flex h-72 flex-col items-center justify-center gap-2 rounded-xl border border-dashed px-6 text-center'>
                    <ImageIcon className='text-muted-foreground/50 size-10' />
                    {promptText ? (
                      <p className='text-muted-foreground line-clamp-2 max-w-xs text-sm'>
                        {promptText}
                      </p>
                    ) : (
                      <p className='text-muted-foreground/60 text-sm'>
                        {t('Run a request to see the model response here.')}
                      </p>
                    )}
                    <span className='text-muted-foreground/60 font-mono text-xs'>
                      {size === 'auto'
                        ? 'auto'
                        : size.replace('x', ' × ')}
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
