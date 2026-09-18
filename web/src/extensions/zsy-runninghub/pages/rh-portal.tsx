/*
Copyright (C) 2023-2026 QuantumNous

RunningHub user-side application center.

Three-column layout when an app is selected:
  - left   : dynamic parameter form (driven by the app's ParamSchema)
  - center : application introduction (cover + description)
  - right  : generation records (the current user's RunningHub tasks)

The whole page is available to any authenticated user and follows the host's
style / i18n conventions (all copy goes through t()).
*/

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Loader2,
  Play,
  RefreshCw,
  Sparkles,
  Inbox,
  LoaderCircle,
  CheckCircle2,
  XCircle,
  UploadCloud,
  Trash2,
  ChevronLeft,
  ChevronRight,
  Download,
  X,
  Clock,
  Ban,
  File as FileIcon,
  FileArchive,
  FileText,
  Film,
  Music,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getApiKeys } from '@/features/keys/api'
import { formatLogQuota } from '@/lib/format'

import {
  listPublicApps,
  getPublicAppDetail,
  runApp,
  listMyRhTasks,
  uploadAppMedia,
  getUploadChannelStatus,
  listCategories,
  cancelRhTask,
  fetchTaskResultContent,
  type AppView,
  type SchemaParam,
  type TaskDto,
} from '../api'
import {
  extractResults,
  isDownloadOnlyKind,
  type RhResultItem,
  type RhResultKind,
} from '../lib/result-media'
import { rhCancelKind, rhStatusKey } from '../lib/task-status'
import { ApiExamples } from '../components/api-examples'

function fieldKey(p: { nodeId: string; fieldName: string }): string {
  return `${p.nodeId || ''}.${p.fieldName || ''}`
}

/** MIME accept attribute for each media parameter type. */
const MEDIA_ACCEPT: Record<string, string> = {
  image: 'image/*',
  audio: 'audio/*',
  video: 'video/*',
}

/**
 * Upload-capable renderer for image/audio/video parameters.
 *
 * Apps no longer bind a channel: both the submit and the upload paths route to
 * the RunningHub site the app's `site` field declares (cn → 国内站, intl →
 * 国际站). The backend picks the first enabled channel of the site's type; the
 * portal only checks that such a channel exists (uploadAvailable) before
 * offering the dropzone.
 *
 * File upload is the input for media parameters, so no separate URL input is
 * rendered here — pasting a public URL manually is not needed when uploads are
 * available and avoids confusing the user with a second input on the same
 * field. The uploaded fileName is what the upstream nodeInfoList fieldValue
 * expects; the fetchable media URL (RH-hosted after an upload) is shown as a
 * live preview below.
 */
function MediaParamField({
  param,
  onChange,
  errors,
  site,
  uploadAvailable,
  onUploadingChange,
}: {
  param: SchemaParam
  onChange: (v: string) => void
  errors: Record<string, string>
  site: string
  uploadAvailable: boolean
  onUploadingChange: (uploading: boolean) => void
}) {
  const { t } = useTranslation()
  const type = (param.type || 'file').toLowerCase()
  const err = errors[fieldKey(param)]
  const [uploading, setUploading] = useState(false)
  const [uploaded, setUploaded] = useState<{
    fileName: string
    url: string
  } | null>(null)

  // Reset local upload preview whenever the field identity changes (e.g. the
  // selected app changed and this component instance got reused).
  useEffect(() => {
    setUploaded(null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [param])

  const handleFile = async (file: File) => {
    if (!file) return
    if (!uploadAvailable) {
      toast.error(t('This site has no available channel for upload'))
      return
    }
    setUploading(true)
    onUploadingChange(true)
    try {
      const result = await uploadAppMedia(site, file)
      setUploaded(result)
      onChange(result.fileName)
    } catch (e: unknown) {
      toast.error((e as Error)?.message || t('Upload failed'))
    } finally {
      setUploading(false)
      onUploadingChange(false)
    }
  }

  const removeUpload = () => {
    setUploaded(null)
    onChange('')
  }

  const hasFile = uploaded !== null
  // The media preview only ever reflects an uploaded file now: removes the
  // "two inputs per media field" confusion (the duplicate-label symptom came
  // from the separate paste-URL row sharing the field with the dropzone).
  const mediaUrl = hasFile ? uploaded.url : ''

  let mediaPreview: ReactNode | null = null
  if (mediaUrl) {
    if (type === 'image') {
      mediaPreview = (
        <img
          src={mediaUrl}
          alt={hasFile ? uploaded.fileName : ''}
          className='h-24 w-full rounded-md border object-cover'
        />
      )
    } else if (type === 'audio') {
      mediaPreview = <audio controls src={mediaUrl} className='h-9 w-full' />
    } else if (type === 'video') {
      mediaPreview = (
        <video
          controls
          src={mediaUrl}
          className='max-h-44 w-full rounded-md border'
        />
      )
    }
  }

  return (
    <div className='space-y-1.5'>
      <Label className='text-xs'>
        {param.label || param.fieldName}
        {param.required ? ' *' : ''}
      </Label>

      {mediaPreview && (
        <div className='space-y-1.5'>
          {mediaPreview}
          {hasFile && (
            <div className='flex items-center justify-between gap-2'>
              <a
                href={uploaded.url}
                target='_blank'
                rel='noreferrer'
                className='text-muted-foreground hover:text-foreground truncate font-mono text-[11px] underline-offset-2 hover:underline'
              >
                {uploaded.fileName}
              </a>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                title={t('Remove')}
                onClick={removeUpload}
              >
                <Trash2 className='size-3.5' />
              </Button>
            </div>
          )}
        </div>
      )}

      {!hasFile && (
        <label
          className={`border-border/60 bg-background hover:border-primary/50 hover:bg-primary/5 flex cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border border-dashed px-3 py-4 text-center transition-colors ${
            uploading || !uploadAvailable
              ? 'pointer-events-none opacity-60'
              : ''
          }`}
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault()
            const file = e.dataTransfer.files?.[0]
            if (file) void handleFile(file)
          }}
        >
          {uploading ? (
            <Loader2 className='text-muted-foreground size-4 animate-spin' />
          ) : (
            <UploadCloud className='text-muted-foreground size-4' />
          )}
          <span className='text-muted-foreground text-xs'>
            {uploadAvailable
              ? t('Click to upload')
              : t('This site has no available channel for upload')}
          </span>
          <input
            accept={MEDIA_ACCEPT[type]}
            className='hidden'
            type='file'
            disabled={uploading || !uploadAvailable}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) void handleFile(file)
              e.target.value = ''
            }}
          />
        </label>
      )}

      {err && <p className='text-destructive text-xs'>{err}</p>}
    </div>
  )
}

/** Render a single schema parameter as the matching control. */
function ParamField({
  param,
  value,
  onChange,
  errors,
  site,
  uploadAvailable,
  onUploadingChange,
}: {
  param: SchemaParam
  value: string
  onChange: (v: string) => void
  errors: Record<string, string>
  site: string
  uploadAvailable: boolean
  onUploadingChange: (uploading: boolean) => void
}) {
  const { t } = useTranslation()
  const type = (param.type || 'text').toLowerCase()
  const requiredMark = param.required ? ' *' : ''
  const err = errors[fieldKey(param)]

  // Media params render their own Label inside MediaParamField; returning here
  // skips the generic label wrapper below so the title never shows twice.
  if (
    type === 'image' ||
    type === 'audio' ||
    type === 'video' ||
    type === 'file'
  ) {
    return (
      <MediaParamField
        param={param}
        onChange={onChange}
        errors={errors}
        site={site}
        uploadAvailable={uploadAvailable}
        onUploadingChange={onUploadingChange}
      />
    )
  }

  const control = (() => {
    switch (type) {
      case 'textarea':
        return (
          <Textarea
            value={value ?? ''}
            placeholder={param.placeholder}
            onChange={(e) => onChange(e.target.value)}
            rows={4}
          />
        )
      case 'number':
      case 'int':
      case 'integer':
      case 'float':
      case 'duration':
      case 'seconds':
        return (
          <Input
            type='number'
            value={value ?? ''}
            min={param.min}
            max={param.max}
            placeholder={param.placeholder}
            onChange={(e) => onChange(e.target.value)}
          />
        )
      case 'select':
      case 'radio':
      case 'enum':
        return (
          <Select
            value={value ?? ''}
            onValueChange={(v) => {
              if (v != null) onChange(v)
            }}
          >
            <SelectTrigger>
              <SelectValue placeholder={param.placeholder || t('Select...')} />
            </SelectTrigger>
            <SelectContent>
              {(param.options ?? []).map((o) => (
                <SelectItem
                  key={`${o.value}-${o.label}`}
                  value={o.value || o.label}
                >
                  {o.label || o.value}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )
      case 'boolean':
      case 'bool':
      case 'checkbox':
        return (
          <div className='flex items-center gap-2'>
            <Switch
              checked={value === 'true'}
              onCheckedChange={(v) => onChange(String(v))}
            />
            <span className='text-muted-foreground text-xs'>
              {t('Enabled')}
            </span>
          </div>
        )
      default:
        return (
          <Input
            value={value ?? ''}
            placeholder={param.placeholder}
            onChange={(e) => onChange(e.target.value)}
          />
        )
    }
  })()

  return (
    <div className='space-y-1.5'>
      <Label className='text-xs'>
        {param.label || param.fieldName}
        {requiredMark}
      </Label>
      {control}
      {err && <p className='text-destructive text-xs'>{err}</p>}
    </div>
  )
}

/** Human label of a result kind (keys stay literal so i18n scanning finds them). */
function resultKindLabel(
  kind: RhResultKind,
  t: (key: string) => string
): string {
  switch (kind) {
    case 'image':
      return t('Image')
    case 'video':
      return t('Video')
    case 'audio':
      return t('Audio')
    case 'text':
      return t('Text')
    case 'archive':
      return t('Archive')
    default:
      return t('File')
  }
}

/** Type icon of a result kind, returned as an element (never as a component). */
function resultKindIconElement(
  kind: RhResultKind,
  className: string
): ReactNode {
  switch (kind) {
    case 'video':
      return <Film className={className} />
    case 'audio':
      return <Music className={className} />
    case 'text':
      return <FileText className={className} />
    case 'archive':
      return <FileArchive className={className} />
    default:
      return <FileIcon className={className} />
  }
}

/**
 * One result entry inside a generation record.
 *
 * Images and videos get a real thumbnail; audio and text open the inline
 * preview; archives and unknown binaries download directly, because there is
 * nothing useful to render for them in the browser.
 *
 * Exported for the result-rendering regression tests.
 */
export function ResultTile({
  item,
  onOpen,
}: {
  item: RhResultItem
  onOpen: () => void
}) {
  const { t } = useTranslation()
  const label = resultKindLabel(item.kind, t)

  if (item.kind === 'image' && item.url) {
    return (
      <button
        type='button'
        title={t('Preview image')}
        aria-label={t('Preview image')}
        onClick={onOpen}
        className='cursor-pointer'
      >
        <img
          src={item.url}
          alt={t('Preview image')}
          className='h-14 w-14 shrink-0 rounded border object-cover transition-transform hover:scale-105'
        />
      </button>
    )
  }

  if (item.kind === 'video' && item.url) {
    // No inline <video> in the list: a metadata preload still fires one request
    // per record, and the upstream storage may not answer with a ranged one. The
    // play badge plus the dialog keeps the list cheap.
    return (
      <button
        type='button'
        title={t('Preview video')}
        aria-label={t('Preview video')}
        onClick={onOpen}
        className='text-muted-foreground hover:text-foreground relative flex size-14 shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded border'
      >
        <Film className='size-4' />
        <span className='w-full truncate px-0.5 text-center text-[9px] leading-none'>
          {label}
        </span>
        <span className='absolute right-1 bottom-1 flex items-center justify-center'>
          <Play className='size-3 fill-current' />
        </span>
      </button>
    )
  }

  const iconElement = resultKindIconElement(item.kind, 'size-4')

  if (isDownloadOnlyKind(item.kind)) {
    return (
      <a
        href={item.url}
        download={item.fileName || undefined}
        target='_blank'
        rel='noreferrer'
        title={t('Download file')}
        aria-label={`${t('Download file')}: ${item.fileName || label}`}
        className='text-muted-foreground hover:text-foreground flex size-14 shrink-0 flex-col items-center justify-center gap-0.5 rounded border'
      >
        {iconElement}
        <span className='w-full truncate px-0.5 text-center text-[9px] leading-none'>
          {label}
        </span>
      </a>
    )
  }

  const previewTitle =
    item.kind === 'audio' ? t('Preview audio') : t('Preview text')
  return (
    <button
      type='button'
      title={previewTitle}
      aria-label={previewTitle}
      onClick={onOpen}
      className='text-muted-foreground hover:text-foreground flex size-14 shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded border'
    >
      {iconElement}
      <span className='w-full truncate px-0.5 text-center text-[9px] leading-none'>
        {label}
      </span>
    </button>
  )
}

/**
 * Inline text preview. RunningHub returns some results as text: either inline
 * in `results[].text`, or as a .txt/.json/.csv file that the gateway fetches on
 * the browser's behalf (the upstream storage sends no CORS headers).
 */
function ResultTextPreview({
  taskId,
  item,
}: {
  taskId: string
  item: RhResultItem
}) {
  const { t } = useTranslation()
  const needsFetch = !item.text && Boolean(item.url)
  const content = useQuery({
    queryKey: ['rh-result-content', taskId, item.url ?? ''],
    queryFn: () => fetchTaskResultContent(taskId, item.url as string),
    enabled: needsFetch,
    staleTime: Infinity,
    retry: false,
  })

  if (needsFetch && content.isLoading) {
    return (
      <div className='text-muted-foreground flex items-center gap-2 text-sm'>
        <Loader2 className='size-4 animate-spin' />
        {t('Loading preview…')}
      </div>
    )
  }
  if (needsFetch && content.isError) {
    return (
      <div className='space-y-2 text-sm'>
        <p className='text-destructive'>{t('Failed to load preview')}</p>
        <a
          href={item.url}
          download={item.fileName || undefined}
          target='_blank'
          rel='noreferrer'
          className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1 underline-offset-2 hover:underline'
        >
          <Download className='size-3.5' />
          {t('Download file')}
        </a>
      </div>
    )
  }

  const text = item.text ?? content.data?.content ?? ''
  return (
    <div className='flex min-h-0 flex-1 flex-col gap-2'>
      <pre className='bg-muted/30 max-h-[60vh] overflow-auto rounded-md border p-3 text-xs whitespace-pre-wrap'>
        {text}
      </pre>
      {content.data?.truncated && (
        <p className='text-muted-foreground text-xs'>
          {t('Showing the first part of the file only.')}
        </p>
      )}
    </div>
  )
}

/**
 * Full-screen result preview for one generation record. Arrow keys cycle only
 * within that record's own results — pressing left/right at either end stops
 * (never leaks into the next/previous record). The dialog holds its own index
 * state so it always starts on the clicked tile.
 *
 * Exported for the result-rendering regression tests.
 */
export function ResultPreviewDialog({
  open,
  onOpenChange,
  items,
  taskId,
  initialIndex,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  items: RhResultItem[]
  taskId: string
  initialIndex: number
}) {
  const { t } = useTranslation()
  const [index, setIndex] = useState(initialIndex)
  const [wasOpen, setWasOpen] = useState(open)

  // Clamp in render so a stale initialIndex (never expected here) degrades
  // gracefully instead of rendering items[-1].
  const clamped = Math.min(Math.max(index, 0), items.length - 1)
  const current = items[clamped]

  // Restart from the clicked tile every time the dialog opens. onOpenChange
  // already mirrors state, so a flip from closed→open resets the index.
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setIndex(initialIndex)
  }

  const prev = () => setIndex((i) => Math.max(0, i - 1))
  const next = () => setIndex((i) => Math.min(items.length - 1, i + 1))
  const canPrev = clamped > 0
  const canNext = clamped < items.length - 1

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      prev()
    } else if (e.key === 'ArrowRight') {
      e.preventDefault()
      next()
    }
  }

  const title = current ? resultKindLabel(current.kind, t) : t('Preview')

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className='bg-background/95 border-border/40 max-w-4xl p-3 backdrop-blur-md sm:max-w-4xl'
      >
        <div
          tabIndex={-1}
          onKeyDown={handleKeyDown}
          className='relative flex min-h-[60vh] flex-col outline-none'
        >
          <div className='text-muted-foreground flex items-center justify-between gap-2 text-xs'>
            <span className='flex min-w-0 items-center gap-1.5'>
              <Sparkles className='size-3.5 shrink-0' />
              <span className='truncate'>{current?.fileName || title}</span>
            </span>
            <span data-testid='lightbox-counter'>
              {t('{{current}} of {{total}}', {
                current: clamped + 1,
                total: items.length,
              })}
            </span>
            <div className='flex items-center gap-1'>
              {current?.url && (
                <a
                  href={current.url}
                  download={current.fileName || undefined}
                  title={t('Download')}
                  aria-label={t('Download')}
                  className='text-muted-foreground hover:text-foreground focus-visible:ring-ring/50 inline-flex items-center justify-center rounded-md p-1 transition-colors outline-none focus-visible:ring-2'
                >
                  <Download className='size-4' />
                </a>
              )}
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                title={t('Close')}
                onClick={() => onOpenChange(false)}
              >
                <X className='size-4' />
              </Button>
            </div>
          </div>

          <div className='flex min-h-0 flex-1 flex-col items-center justify-center py-3'>
            {current?.kind === 'image' && current.url && (
              <img
                src={current.url}
                alt={title}
                className='border-border/40 max-h-[65vh] max-w-full rounded-md border object-contain'
              />
            )}
            {current?.kind === 'video' && current.url && (
              <video
                src={current.url}
                controls
                autoPlay
                playsInline
                className='border-border/40 max-h-[65vh] w-full rounded-md border'
              />
            )}
            {current?.kind === 'audio' && current.url && (
              <div className='w-full space-y-2'>
                <Music className='text-muted-foreground mx-auto size-8' />
                <audio src={current.url} controls autoPlay className='w-full' />
              </div>
            )}
            {current?.kind === 'text' && (
              <div className='flex min-h-0 w-full flex-1 flex-col'>
                <ResultTextPreview taskId={taskId} item={current} />
              </div>
            )}
            {current && isDownloadOnlyKind(current.kind) && (
              <div className='flex flex-col items-center gap-3 text-center'>
                {resultKindIconElement(
                  current.kind,
                  'size-10 text-muted-foreground'
                )}
                <p className='text-sm break-all'>{current.fileName || title}</p>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'This file type cannot be previewed. Download it to open.'
                  )}
                </p>
                <a
                  href={current.url}
                  download={current.fileName || undefined}
                  target='_blank'
                  rel='noreferrer'
                  className={buttonVariants({ variant: 'outline', size: 'sm' })}
                >
                  <Download className='size-4' />
                  {t('Download file')}
                </a>
              </div>
            )}
          </div>

          {items.length > 1 && (
            <>
              <Button
                type='button'
                variant='outline'
                size='icon'
                title={t('Previous Result')}
                aria-label={t('Previous Result')}
                disabled={!canPrev}
                onClick={prev}
                className='border-border/60 bg-background/80 absolute top-1/2 -left-2.5 -translate-y-1/2 backdrop-blur-sm sm:-left-5'
              >
                <ChevronLeft className='size-5' />
              </Button>
              <Button
                type='button'
                variant='outline'
                size='icon'
                title={t('Next Result')}
                aria-label={t('Next Result')}
                disabled={!canNext}
                onClick={next}
                className='border-border/60 bg-background/80 absolute top-1/2 -right-2.5 -translate-y-1/2 backdrop-blur-sm sm:-right-5'
              >
                <ChevronRight className='size-5' />
              </Button>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

function statusBadge(status: string, t: (k: string) => string) {
  switch (rhStatusKey(status)) {
    case 'Success':
      return (
        <Badge className='bg-emerald-500/15 text-emerald-500'>
          {t('Success')}
        </Badge>
      )
    case 'Failed':
      return <Badge variant='destructive'>{t('Failed')}</Badge>
    case 'Queued':
      // Waiting for a free slot on the site's channels: nothing has reached
      // RunningHub yet (see the backend queue).
      return (
        <Badge variant='secondary'>
          <Clock className='mr-1 size-3' />
          {t('Queued')}
        </Badge>
      )
    default:
      return (
        <Badge variant='outline'>
          <LoaderCircle className='mr-1 size-3 animate-spin' />
          {t('In progress')}
        </Badge>
      )
  }
}

/**
 * Failure message shown for a failed generation record: the RH payload's
 * `data.errorMessage` takes precedence when present (it carries the precise
 * upstream failure message, e.g. "Task not found, please check the task ID"),
 * otherwise `task.fail_reason` is used. In extreme cases the raw `data` can
 * itself be a bare error string (e.g. "/任务超时（1440分钟）"), so prefer the
 * string payload over the sanitized task fields. Only absolute URLs are
 * rendered as links — anything else is plain truncated text (a relative path
 * like "/任务超时（1440分钟）" must never become an <a href> navigation).
 */
function taskFailMessage(task: TaskDto): string {
  const rawData = task.data
  const nested =
    rawData && typeof rawData === 'object'
      ? (rawData as { errorMessage?: unknown })
      : null
  if (
    nested &&
    typeof nested.errorMessage === 'string' &&
    nested.errorMessage.trim() !== ''
  ) {
    return nested.errorMessage
  }
  if (typeof rawData === 'string' && rawData.trim() !== '') {
    return rawData
  }
  return task.fail_reason || ''
}

function FailMessageText({ message }: { message: string }) {
  if (/^https?:\/\//.test(message)) {
    return (
      <a
        href={message}
        target='_blank'
        rel='noreferrer'
        className='min-w-0 truncate underline-offset-2 hover:underline'
      >
        {message}
      </a>
    )
  }
  return <span className='min-w-0 truncate'>{message}</span>
}

function AppRunForm({
  app,
  onSubmitted,
}: {
  app: AppView
  onSubmitted: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const schema = app.paramSchema ?? []
  const [values, setValues] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [uploadingCount, setUploadingCount] = useState(0)
  const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null)

  // The current user's API keys, so the run can be billed against a chosen
  // token instead of the dashboard session default (TokenId=0). Only enabled
  // tokens are offered; a token with a positive quota (or unlimited) is
  // required for the pre-consume step to pass.
  const { data: keysData } = useQuery({
    queryKey: ['rh-my-api-keys'],
    queryFn: () => getApiKeys({ p: 1, size: 100 }),
  })
  const enabledKeys = (keysData?.data?.items ?? []).filter(
    (k) => k.status === 1
  )
  const selectedToken =
    enabledKeys.find((k) => k.id === selectedTokenId) ?? null

  // Media uploads route through the RunningHub channel pool of the app's
  // site (cn → 国内站 61, intl → 国际站 62); the app itself no longer binds a
  // channel. One shared check gates every media dropzone on the form.
  const appSite = app.site ?? ''
  const { data: uploadChannel } = useQuery({
    queryKey: ['rh-upload-channel', appSite],
    queryFn: () => getUploadChannelStatus(appSite),
    staleTime: 60_000,
  })
  const uploadAvailable = uploadChannel?.available ?? false

  // Reset the form whenever the selected app changes, pre-filling defaults.
  useEffect(() => {
    const init: Record<string, string> = {}
    for (const p of schema) {
      const k = fieldKey(p)
      if (p.defaultValue !== undefined && p.defaultValue !== null) {
        init[k] = p.defaultValue
      } else if (p.type === 'boolean') {
        init[k] = 'false'
      }
    }
    setValues(init)
    setErrors({})
    setUploadingCount(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.id])

  const submit = useMutation({
    mutationFn: async () => {
      if (uploadingCount > 0) {
        throw new Error(t('Please wait for the upload to finish'))
      }
      const cleaned: Record<string, string> = {}
      const errs: Record<string, string> = {}
      for (const p of schema) {
        const k = fieldKey(p)
        const raw = (values[k] ?? '').trim()
        if (p.required && raw === '') {
          errs[k] = t('This field is required')
        } else {
          cleaned[k] = raw
        }
      }
      setErrors(errs)
      if (Object.keys(errs).length > 0) {
        throw new Error(t('Please fix the highlighted fields'))
      }
      await runApp(app.id, cleaned, { tokenId: selectedTokenId || undefined })
    },
    onError: (e) => {
      toast.error(String((e as Error)?.message ?? e))
    },
    onSuccess: () => {
      toast.success(t('Task submitted'))
      onSubmitted()
      void queryClient.invalidateQueries({ queryKey: ['rh-my-tasks'] })
    },
  })

  let label = t('Run')
  let Icon = Play
  if (submit.isPending) {
    label = t('Submitting...')
    Icon = Loader2
  } else if (uploadingCount > 0) {
    label = t('Uploading...')
    Icon = Loader2
  }

  return (
    <div className='flex h-full flex-col space-y-4'>
      <div className='flex-1 space-y-3'>
        {schema.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('This application has no configurable parameters yet.')}
          </p>
        ) : (
          schema.map((p) => (
            <ParamField
              key={fieldKey(p)}
              param={p}
              value={values[fieldKey(p)] ?? ''}
              errors={errors}
              site={appSite}
              uploadAvailable={uploadAvailable}
              onUploadingChange={(uploading) =>
                setUploadingCount((prev) =>
                  Math.max(0, prev + (uploading ? 1 : -1))
                )
              }
              onChange={(v) =>
                setValues((prev) => ({ ...prev, [fieldKey(p)]: v }))
              }
            />
          ))
        )}
      </div>
      <div className='space-y-1.5'>
        <Label htmlFor='rh-run-token'>{t('Billing API Key')}</Label>
        <Select
          value={selectedToken ? String(selectedToken.id) : ''}
          onValueChange={(v) => {
            if (v == null || v === '') {
              setSelectedTokenId(null)
            } else {
              const id = Number(v)
              if (enabledKeys.some((k) => k.id === id)) {
                setSelectedTokenId(id)
              }
            }
          }}
        >
          <SelectTrigger id='rh-run-token'>
            <SelectValue>
              {selectedToken
                ? `${selectedToken.name}${
                    selectedToken.unlimited_quota ? ` (${t('Unlimited')})` : ''
                  }`
                : t('Select an API key to bill this run')}
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
        <p className='text-muted-foreground text-xs'>
          {t(
            'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.'
          )}
        </p>
      </div>
      <Button
        className='w-full'
        disabled={submit.isPending || uploadingCount > 0 || !selectedToken}
        onClick={() => submit.mutate()}
      >
        <Icon className='size-4' />
        {label}
      </Button>
    </div>
  )
}

function AppIntro({ app }: { app: AppView }) {
  const { t } = useTranslation()
  return (
    <div className='relative flex h-full min-h-0 flex-col overflow-hidden'>
      <ScrollArea className='min-h-0 flex-1'>
        <div className='space-y-3'>
          {app.coverUrl ? (
            <img
              src={app.coverUrl}
              alt={app.name}
              className='aspect-video w-full rounded-md border object-cover'
            />
          ) : (
            <div className='bg-muted/40 flex aspect-video w-full items-center justify-center rounded-md border'>
              <Sparkles className='text-muted-foreground size-6' />
            </div>
          )}
          <h2 className='text-lg leading-tight font-semibold'>{app.name}</h2>
          <p className='text-muted-foreground text-sm whitespace-pre-wrap'>
            {app.description || t('No description')}
          </p>
          {app.kind && (
            <Badge variant='outline'>
              {app.kind === 'ai_app' ? t('AIC App') : app.kind}
            </Badge>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}

function GenerationRecords({ appId }: { appId: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const tasks = useQuery({
    queryKey: ['rh-my-tasks', appId],
    queryFn: () => listMyRhTasks({ page_size: 20 }),
    refetchInterval: 5000,
  })

  const items = tasks.data?.items ?? []
  // The record currently shown in the preview dialog. The index resets to the
  // clicked tile within that record; navigation never crosses record boundaries.
  const [preview, setPreview] = useState<{
    taskId: string
    items: RhResultItem[]
    index: number
  } | null>(null)
  // The record awaiting cancel confirmation (queued or running).
  const [cancelTarget, setCancelTarget] = useState<TaskDto | null>(null)

  const cancel = useMutation({
    mutationFn: (taskId: string) => cancelRhTask(taskId),
    onSuccess: () => {
      toast.success(t('Task cancelled, the quota has been refunded'))
      setCancelTarget(null)
      void queryClient.invalidateQueries({ queryKey: ['rh-my-tasks'] })
    },
    onError: (error: unknown) => {
      toast.error(String((error as Error)?.message ?? error))
    },
  })

  return (
    <div className='flex h-full flex-col overflow-hidden'>
      <div className='mb-2 shrink-0 items-center justify-between gap-2 sm:flex'>
        <span className='text-sm font-medium'>{t('Generation Records')}</span>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          title={t('Refresh')}
          onClick={() => void tasks.refetch()}
        >
          <RefreshCw
            className={tasks.isFetching ? 'size-3.5 animate-spin' : 'size-3.5'}
          />
        </Button>
      </div>
      <ScrollArea className='min-h-0 flex-1'>
        {items.length === 0 ? (
          <div className='text-muted-foreground flex h-full flex-col items-center justify-center gap-2 py-8 text-center text-xs'>
            <Inbox className='size-6' />
            <p>{t('No generation records yet')}</p>
          </div>
        ) : (
          <div className='space-y-2'>
            {items.map((task) => {
              const results = extractResults(task)
              return (
                <div key={task.task_id} className='rounded-md border p-2'>
                  <div className='flex items-center justify-between gap-2'>
                    <span className='text-muted-foreground truncate font-mono text-[11px]'>
                      {task.task_id}
                    </span>
                    <span className='flex shrink-0 items-center gap-1.5'>
                      {task.quota > 0 && (
                        <span className='border-border/60 bg-muted/30 inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] leading-none font-medium tabular-nums'>
                          {t('Deducted')} {formatLogQuota(task.quota)}
                          {task.billing_source === 'subscription' && (
                            <span className='text-muted-foreground flex items-center gap-0.5 text-[10px]'>
                              ·{t('Subscription')}
                            </span>
                          )}
                        </span>
                      )}
                      {statusBadge(task.status, t)}
                      {rhCancelKind(task.status) !== null && (
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon-sm'
                          title={t('Cancel task')}
                          aria-label={t('Cancel task')}
                          disabled={cancel.isPending}
                          onClick={() => setCancelTarget(task)}
                        >
                          <Ban className='size-3.5' />
                        </Button>
                      )}
                    </span>
                  </div>
                  <div className='mt-2 flex gap-1.5 overflow-x-auto'>
                    {results.map((item, idx) => (
                      <ResultTile
                        key={item.key}
                        item={item}
                        onOpen={() =>
                          setPreview({
                            taskId: task.task_id,
                            items: results,
                            index: idx,
                          })
                        }
                      />
                    ))}
                    {results.length === 0 &&
                      task.status?.toLowerCase() === 'success' && (
                        <span className='flex items-center gap-1 text-[11px] text-emerald-500'>
                          <CheckCircle2 className='size-3.5' />
                          {t('Completed')}
                        </span>
                      )}
                    {task.status?.toLowerCase() === 'failure' &&
                      taskFailMessage(task) && (
                        <span className='text-destructive flex items-center gap-1 text-[11px]'>
                          <XCircle className='size-3.5' />
                          <FailMessageText message={taskFailMessage(task)} />
                        </span>
                      )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </ScrollArea>

      <ResultPreviewDialog
        open={preview !== null}
        onOpenChange={(open) => {
          if (!open) setPreview(null)
        }}
        items={preview?.items ?? []}
        taskId={preview?.taskId ?? ''}
        initialIndex={preview?.index ?? 0}
        key={preview?.taskId ?? 'closed'}
      />

      <AlertDialog
        open={cancelTarget !== null}
        onOpenChange={(open) => {
          if (!open) setCancelTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Cancel this task?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {rhCancelKind(cancelTarget?.status ?? '') === 'queued'
                ? t(
                    'This task is still waiting for a free slot. Cancelling it removes it from the queue and refunds the charge.'
                  )
                : t(
                    'The run will be stopped on RunningHub and the charge refunded. This cannot be undone.'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cancel.isPending}>
              {t('Keep it running')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={cancel.isPending}
              onClick={(event) => {
                event.preventDefault()
                if (cancelTarget) cancel.mutate(cancelTarget.task_id)
              }}
            >
              {cancel.isPending ? (
                <Loader2 className='mr-1 size-3.5 animate-spin' />
              ) : null}
              {t('Cancel task')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export function RhPortalPage() {
  const { t } = useTranslation()
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [activeCategory, setActiveCategory] = useState<number | null>(null)

  const apps = useQuery({
    queryKey: ['rh-public-apps'],
    queryFn: () => listPublicApps({ page_size: 100 }),
  })

  const categories = useQuery({
    queryKey: ['rh-app-categories'],
    queryFn: () => listCategories(),
  })

  const selected = useQuery({
    queryKey: ['rh-public-app-detail', selectedId],
    queryFn: () => getPublicAppDetail(selectedId as number),
    enabled: selectedId !== null,
  })

  const app = selected.data
  const list = apps.data?.items ?? []
  const catList = categories.data ?? []

  const filtered =
    activeCategory == null
      ? list
      : list.filter((a) => a.categoryId === activeCategory)

  const handleCategoryClick = (catId: number | null) => {
    setActiveCategory(catId)
    // If the currently selected app no longer belongs to the picked category,
    // clear the selection so the right pane doesn't show a stale app.
    const stillVisible =
      activeCategory == null ||
      (selectedId != null &&
        list.some((a) => a.id === selectedId && a.categoryId === catId))
    if (!stillVisible) setSelectedId(null)
  }

  return (
    <div className='container mx-auto max-w-[1400px] space-y-4 px-4 py-6'>
      <div>
        <h1 className='text-xl font-semibold'>{t('RunningHub App Center')}</h1>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Browse RunningHub applications, fill in the parameters and generate.'
          )}
        </p>
      </div>

      <div className='grid gap-4 lg:grid-cols-[180px_300px_1fr]'>
        {/* ---- left: categories ---- */}
        <div className='bg-card flex h-[calc(100vh-11rem)] min-h-0 flex-col rounded-xl border p-3'>
          <div className='mb-2 px-1 text-sm font-semibold'>
            {t('Categories')}
          </div>
          <ScrollArea className='min-h-0 flex-1'>
            <div className='space-y-1 pr-1'>
              <button
                type='button'
                onClick={() => handleCategoryClick(null)}
                className={`w-full rounded-md px-3 py-2 text-left transition-colors ${
                  activeCategory == null
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted'
                }`}
              >
                <div className='flex items-center justify-between gap-2'>
                  <span className='truncate text-sm font-medium'>
                    {t('All Apps')}
                  </span>
                  <span className='shrink-0 text-xs opacity-70'>
                    {list.length}
                  </span>
                </div>
              </button>
              {catList.map((cat) => (
                <button
                  key={cat.id}
                  type='button'
                  onClick={() => handleCategoryClick(cat.id)}
                  className={`w-full rounded-md px-3 py-2 text-left transition-colors ${
                    activeCategory === cat.id
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-muted'
                  }`}
                >
                  <div className='flex items-center justify-between gap-2'>
                    <span className='truncate text-sm font-medium'>
                      {cat.name}
                    </span>
                    <span className='shrink-0 text-xs opacity-70'>
                      {cat.appCount ?? 0}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          </ScrollArea>
        </div>

        {/* ---- middle: application list for the picked category ---- */}
        <div className='bg-card flex h-[calc(100vh-11rem)] min-h-0 flex-col rounded-xl border p-3'>
          <div className='mb-2 px-1 text-sm font-semibold'>
            {t('Apps in category')}
          </div>
          <ScrollArea className='min-h-0 flex-1'>
            <div className='space-y-1 pr-1'>
              {filtered.length === 0 ? (
                <p className='text-muted-foreground px-2 py-4 text-center text-xs'>
                  {t('No apps found')}
                </p>
              ) : (
                filtered.map((a) => (
                  <button
                    key={a.id}
                    type='button'
                    onClick={() => setSelectedId(a.id)}
                    className={`w-full rounded-md px-3 py-2 text-left transition-colors ${
                      selectedId === a.id
                        ? 'bg-primary text-primary-foreground'
                        : 'text-muted-foreground hover:bg-muted'
                    }`}
                  >
                    <div className='text-sm font-medium'>{a.name}</div>
                    <div className='truncate text-xs opacity-70'>
                      {a.description || t('No description')}
                    </div>
                  </button>
                ))
              )}
            </div>
          </ScrollArea>
        </div>

        {/* ---- right: main view ---- */}
        {!app ? (
          <div className='bg-card text-muted-foreground flex min-h-[60vh] items-center justify-center rounded-xl border text-center text-sm'>
            {apps.isLoading ? (
              <Loader2 className='size-6 animate-spin' />
            ) : (
              <div className='space-y-2'>
                <Sparkles className='mx-auto size-8' />
                <p>{t('Select an application from the left to start.')}</p>
              </div>
            )}
          </div>
        ) : (
          <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
            {/* form (left) */}
            <div className='bg-card flex h-[calc(100vh-11rem)] min-h-0 flex-col rounded-xl border p-4'>
              <h3 className='mb-3 text-sm font-semibold'>{t('Parameters')}</h3>
              <div className='min-h-0 flex-1'>
                <AppRunForm
                  app={app}
                  onSubmitted={() => {
                    // records panel polls automatically; nothing else needed here
                  }}
                />
              </div>
            </div>
            {/* intro (center) */}
            <div className='bg-card flex h-[calc(100vh-11rem)] min-h-0 flex-col rounded-xl border p-4'>
              <h3 className='mb-3 text-sm font-semibold'>
                {t('About this app')}
              </h3>
              <div className='min-h-0 flex-1'>
                <AppIntro app={app} />
              </div>
            </div>
            {/* generation records (right) */}
            <div className='bg-card flex h-[calc(100vh-11rem)] min-h-0 flex-col rounded-xl border p-4 md:col-span-2 xl:col-span-1'>
              <GenerationRecords appId={app.id} />
            </div>
          </div>
        )}
      </div>

      {/* ---- third-party integration: how to call this app from code ----
          Rendered full width under the app introduction so the samples stay
          readable (the intro column is only a third of the page). */}
      {app && <ApiExamples app={app} />}
    </div>
  )
}
