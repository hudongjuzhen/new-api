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
  Search,
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
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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

import {
  listPublicApps,
  getPublicAppDetail,
  runApp,
  listMyRhTasks,
  uploadAppMedia,
  getUploadChannelStatus,
  type AppView,
  type SchemaParam,
  type TaskDto,
} from '../api'

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
      mediaPreview = (
        <audio controls src={mediaUrl} className='h-9 w-full' />
      )
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
            uploading || !uploadAvailable ? 'pointer-events-none opacity-60' : ''
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
  if (type === 'image' || type === 'audio' || type === 'video' || type === 'file') {
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

/** Extract result media URLs from a task's RH data payload. */
function extractResultUrls(task: TaskDto): string[] {
  const raw = task.data
  let results: Array<{ url?: unknown; value?: unknown }>
  if (Array.isArray(raw)) {
    results = raw
  } else {
    const nested = (raw as { results?: unknown } | undefined)?.results
    results = Array.isArray(nested) ? (nested as Array<{ url?: unknown; value?: unknown }>) : []
  }
  const out: string[] = []
  for (const it of results) {
    // Only absolute http(s) URLs are renderable media. Anything else (e.g. a
    // relative "/任务超时（1440分钟）" that slipped into data.results[].url)
    // must never become an <img src> — the browser would treat it as a
    // navigation and hit the SPA fallback.
    if (typeof it?.url === 'string' && /^https?:\/\//i.test(it.url)) {
      out.push(it.url)
    } else if (typeof it?.value === 'string' && /^https?:\/\//i.test(it.value)) {
      out.push(it.value)
    }
  }
  if (out.length === 0 && task.result_url && /^https?:\/\//i.test(task.result_url)) {
    out.push(task.result_url)
  }
  return [...new Set(out)]
}

function statusBadge(status: string, t: (k: string) => string) {
  const s = (status || '').toLowerCase()
  if (s === 'success' || s === 'done' || s === 'succeeded') {
    return (
      <Badge className='bg-emerald-500/15 text-emerald-500'>
        {t('Success')}
      </Badge>
    )
  }
  if (s === 'failure' || s === 'failed') {
    return <Badge variant='destructive'>{t('Failed')}</Badge>
  }
  return (
    <Badge variant='outline'>
      <LoaderCircle className='mr-1 size-3 animate-spin' />
      {t('In progress')}
    </Badge>
  )
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
  const selectedToken = enabledKeys.find((k) => k.id === selectedTokenId) ?? null

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
                setUploadingCount((prev) => Math.max(0, prev + (uploading ? 1 : -1)))
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

/**
 * Full-screen image preview for a single generation record. Arrow keys cycle
 * only within that record's own result images — pressing left/right at either
 * end stops (never leaks into the next/previous record). The dialog holds its
 * own index state so it always starts on the clicked image.
 */
function ResultImageLightbox({
  open,
  onOpenChange,
  urls,
  initialIndex,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  urls: string[]
  initialIndex: number
}) {
  const { t } = useTranslation()
  const [index, setIndex] = useState(initialIndex)
  const [wasOpen, setWasOpen] = useState(open)

  // Clamp in render so a stale initialIndex (never expected here) degrades
  // gracefully instead of rendering urls[-1].
  const clamped = Math.min(Math.max(index, 0), urls.length - 1)

  // Restart from the clicked image every time the dialog opens. onOpenChange
  // already mirrors state, so a flip from closed→open resets the index.
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setIndex(initialIndex)
  }

  const prev = () => setIndex((i) => Math.max(0, i - 1))
  const next = () => setIndex((i) => Math.min(urls.length - 1, i + 1))
  const canPrev = clamped > 0
  const canNext = clamped < urls.length - 1

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      prev()
    } else if (e.key === 'ArrowRight') {
      e.preventDefault()
      next()
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className='bg-background/95 max-w-4xl border-border/40 p-3 backdrop-blur-md sm:max-w-4xl'
      >
        <div
          tabIndex={-1}
          onKeyDown={handleKeyDown}
          className='relative flex min-h-[60vh] flex-col outline-none'
        >
          <div className='flex items-center justify-between gap-2 text-xs text-muted-foreground'>
            <span className='flex items-center gap-1.5'>
              <Sparkles className='size-3.5' />
              {t('Preview image')}
            </span>
            <span data-testid='lightbox-counter'>
              {t('{{current}} of {{total}}', {
                current: clamped + 1,
                total: urls.length,
              })}
            </span>
            <div className='flex items-center gap-1'>
              <a
                href={urls[clamped]}
                download
                title={t('Download')}
                aria-label={t('Download')}
                className='text-muted-foreground hover:text-foreground inline-flex items-center justify-center rounded-md p-1 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50'
              >
                <Download className='size-4' />
              </a>
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

          <div className='flex min-h-0 flex-1 items-center justify-center py-3'>
            <img
              src={urls[clamped]}
              alt={t('Preview image')}
              className='max-h-[65vh] max-w-full rounded-md border border-border/40 object-contain'
            />
          </div>

          {urls.length > 1 && (
            <>
              <Button
                type='button'
                variant='outline'
                size='icon'
                title={t('Previous Result')}
                aria-label={t('Previous Result')}
                disabled={!canPrev}
                onClick={prev}
                className='absolute top-1/2 -left-2.5 -translate-y-1/2 border-border/60 bg-background/80 backdrop-blur-sm sm:-left-5'
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
                className='absolute top-1/2 -right-2.5 -translate-y-1/2 border-border/60 bg-background/80 backdrop-blur-sm sm:-right-5'
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

function GenerationRecords({ appId }: { appId: number }) {
  const { t } = useTranslation()
  const tasks = useQuery({
    queryKey: ['rh-my-tasks', appId],
    queryFn: () => listMyRhTasks({ page_size: 20 }),
    refetchInterval: 5000,
  })

  const items = tasks.data?.items ?? []
  // The record currently shown in the lightbox. Index resets to the clicked
  // thumbnail within that record; navigation never crosses record boundaries.
  const [preview, setPreview] = useState<{
    taskId: string
    urls: string[]
    index: number
  } | null>(null)

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
            {items.map((task) => (
              <div key={task.task_id} className='rounded-md border p-2'>
                <div className='flex items-center justify-between gap-2'>
                  <span className='text-muted-foreground truncate font-mono text-[11px]'>
                    {task.task_id}
                  </span>
                  {statusBadge(task.status, t)}
                </div>
                <div className='mt-2 flex gap-1.5 overflow-x-auto'>
                  {extractResultUrls(task).map((url, idx) => (
                    <button
                      key={url}
                      type='button'
                      title={t('Preview image')}
                      onClick={() =>
                        setPreview({
                          taskId: task.task_id,
                          urls: extractResultUrls(task),
                          index: idx,
                        })
                      }
                      className='cursor-pointer'
                    >
                      <img
                        src={url}
                        alt={t('Preview image')}
                        className='h-14 w-14 shrink-0 rounded border object-cover transition-transform hover:scale-105'
                      />
                    </button>
                  ))}
                  {extractResultUrls(task).length === 0 &&
                    task.status?.toLowerCase() === 'success' && (
                      <span className='flex items-center gap-1 text-[11px] text-emerald-500'>
                        <CheckCircle2 className='size-3.5' />
                        {t('Completed')}
                      </span>
                    )}
                  {task.status?.toLowerCase() === 'failure' &&
                    task.fail_reason && (
                      <span className='text-destructive flex items-center gap-1 text-[11px]'>
                        <XCircle className='size-3.5' />
                        {/^https?:\/\//.test(task.fail_reason) ? (
                          <a
                            href={task.fail_reason}
                            target='_blank'
                            rel='noreferrer'
                            className='min-w-0 truncate underline-offset-2 hover:underline'
                          >
                            {task.fail_reason}
                          </a>
                        ) : (
                          <span className='min-w-0 truncate'>
                            {task.fail_reason}
                          </span>
                        )}
                      </span>
                    )}
                </div>
              </div>
            ))}
          </div>
        )}
      </ScrollArea>

      <ResultImageLightbox
        open={preview !== null}
        onOpenChange={(open) => {
          if (!open) setPreview(null)
        }}
        urls={preview?.urls ?? []}
        initialIndex={preview?.index ?? 0}
        key={preview?.taskId ?? 'closed'}
      />
    </div>
  )
}

export function RhPortalPage() {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)

  const apps = useQuery({
    queryKey: ['rh-public-apps', keyword],
    queryFn: () =>
      listPublicApps({ keyword: keyword || undefined, page_size: 100 }),
  })

  const selected = useQuery({
    queryKey: ['rh-public-app-detail', selectedId],
    queryFn: () => getPublicAppDetail(selectedId as number),
    enabled: selectedId !== null,
  })

  const app = selected.data
  const list = apps.data?.items ?? []

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

      <div className='grid gap-4 lg:grid-cols-[280px_1fr]'>
        {/* ---- left: application list menu ---- */}
        <div className='bg-card rounded-xl border p-3'>
          <div className='relative mb-3'>
            <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
            <Input
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              placeholder={t('Search apps...')}
              className='pl-8'
            />
          </div>
          <ScrollArea className='h-[calc(100vh-11rem)]'>
            <div className='space-y-1 pr-1'>
              {list.length === 0 ? (
                <p className='text-muted-foreground px-2 py-4 text-center text-xs'>
                  {t('No apps found')}
                </p>
              ) : (
                list.map((a) => (
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
    </div>
  )
}
