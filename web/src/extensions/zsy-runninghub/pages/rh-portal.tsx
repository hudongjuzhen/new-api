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
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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

import {
  listPublicApps,
  getPublicAppDetail,
  runApp,
  listMyRhTasks,
  type AppView,
  type SchemaParam,
  type TaskDto,
} from '../api'

function fieldKey(p: { nodeId: string; fieldName: string }): string {
  return `${p.nodeId || ''}.${p.fieldName || ''}`
}

/** Render a single schema parameter as the matching control. */
function ParamField({
  param,
  value,
  onChange,
  errors,
}: {
  param: SchemaParam
  value: string
  onChange: (v: string) => void
  errors: Record<string, string>
}) {
  const { t } = useTranslation()
  const type = (param.type || 'text').toLowerCase()
  const requiredMark = param.required ? ' *' : ''
  const err = errors[fieldKey(param)]

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
              {(param.options ?? []).map((o, i) => (
                <SelectItem key={`${o.value}-${i}`} value={o.value || o.label}>
                  {o.label || o.value}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )
      case 'image':
      case 'audio':
      case 'video':
      case 'file':
        return (
          <Input
            value={value ?? ''}
            placeholder={t('Paste a public URL')}
            onChange={(e) => onChange(e.target.value)}
          />
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
  const list = Array.isArray(raw)
    ? raw
    : Array.isArray((raw as any)?.results)
      ? (raw as any).results
      : []
  const out: string[] = []
  for (const it of list) {
    if (typeof it?.url === 'string') out.push(it.url)
    else if (typeof it?.value === 'string' && /^https?:\/\//.test(it.value))
      out.push(it.value)
  }
  if (out.length === 0 && task.result_url) out.push(task.result_url)
  return [...new Set(out)]
}

function statusBadge(status: string, t: (k: string) => string) {
  const s = (status || '').toLowerCase()
  if (s === 'success' || s === 'done' || s === 'succeeded')
    return (
      <Badge className='bg-emerald-500/15 text-emerald-500'>
        {t('Success')}
      </Badge>
    )
  if (s === 'failure' || s === 'failed')
    return <Badge variant='destructive'>{t('Failed')}</Badge>
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

  // Reset the form whenever the selected app changes, pre-filling defaults.
  useEffect(() => {
    const init: Record<string, string> = {}
    for (const p of schema) {
      const k = fieldKey(p)
      if (p.defaultValue !== undefined && p.defaultValue !== null)
        init[k] = p.defaultValue
      else if (p.type === 'boolean') init[k] = 'false'
    }
    setValues(init)
    setErrors({})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.id])

  const submit = useMutation({
    mutationFn: async () => {
      const cleaned: Record<string, string> = {}
      const errs: Record<string, string> = {}
      for (const p of schema) {
        const k = fieldKey(p)
        const raw = (values[k] ?? '').trim()
        if (p.required && raw === '') errs[k] = t('This field is required')
        else cleaned[k] = raw
      }
      setErrors(errs)
      if (Object.keys(errs).length > 0)
        throw new Error(t('Please fix the highlighted fields'))
      await runApp(app.id, cleaned, 'default')
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

  const label = submit.isPending ? t('Submitting...') : t('Run')
  const Icon = submit.isPending ? Loader2 : Play

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
              onChange={(v) =>
                setValues((prev) => ({ ...prev, [fieldKey(p)]: v }))
              }
            />
          ))
        )}
      </div>
      <Button
        className='w-full'
        disabled={submit.isPending}
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
    <div className='flex h-full flex-col overflow-hidden'>
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
  const tasks = useQuery({
    queryKey: ['rh-my-tasks', appId],
    queryFn: () => listMyRhTasks({ page_size: 20 }),
    refetchInterval: 5000,
  })

  const items = tasks.data?.items ?? []

  return (
    <div className='flex h-full flex-col overflow-hidden'>
      <div className='mb-2 flex items-center justify-between'>
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
              <div className={`rounded-md border p-2`}>
                <div className='flex items-center justify-between gap-2'>
                  <span className='text-muted-foreground truncate font-mono text-[11px]'>
                    {task.task_id}
                  </span>
                  {statusBadge(task.status, t)}
                </div>
                <div className='mt-2 flex gap-1.5 overflow-x-auto'>
                  {extractResultUrls(task).map((url, i) => (
                    <a key={i} href={url} target='_blank' rel='noreferrer'>
                      <img
                        src={url}
                        alt=''
                        className='h-14 w-14 shrink-0 rounded border object-cover'
                      />
                    </a>
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
                        <span className='min-w-0 truncate'>
                          {task.fail_reason}
                        </span>
                      </span>
                    )}
                </div>
              </div>
            ))}
          </div>
        )}
      </ScrollArea>
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
          <ScrollArea className='h-[70vh]'>
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
            <div className='bg-card min-h-[60vh] rounded-xl border p-4'>
              <h3 className='mb-3 text-sm font-semibold'>{t('Parameters')}</h3>
              <AppRunForm
                app={app}
                onSubmitted={() => {
                  // records panel polls automatically; nothing else needed here
                }}
              />
            </div>
            {/* intro (center) */}
            <div className='bg-card min-h-[60vh] rounded-xl border p-4'>
              <h3 className='mb-3 text-sm font-semibold'>
                {t('About this app')}
              </h3>
              <AppIntro app={app} />
            </div>
            {/* generation records (right) */}
            <div className='bg-card min-h-[60vh] rounded-xl border p-4 md:col-span-2 xl:col-span-1'>
              <GenerationRecords appId={app.id} />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
