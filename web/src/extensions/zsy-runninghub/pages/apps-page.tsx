/*
Copyright (C) 2023-2026 QuantumNous

RunningHub Apps admin page: list, create, edit, delete.

The app form uses a two-column layout — basic info on the left, the
parameter-template editor on the right (with a one-click fetch that calls
RunningHub's apiCallDemo through the selected channel).
*/

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Plus,
  Pencil,
  Trash2,
  RefreshCw,
  Wand2,
  Loader2,
  X,
  KeyRound,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
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
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import {
  listApps,
  createApp,
  updateApp,
  deleteApp,
  syncAppsFromChannel,
  parseCurlRequest,
  getRhChannels,
  getAppKeypool,
  refreshAppKeypool,
  type AppView,
  type AppCreateDTO,
  type SchemaParam,
  type AppKeypoolResult,
} from '../api'

const TYPE_CHOICES = [
  'text',
  'textarea',
  'number',
  'image',
  'audio',
  'video',
  'select',
]

const emptyDTO: AppCreateDTO = {
  name: '',
  slug: '',
  kind: 'ai_app',
  upstreamId: '',
  description: '',
  coverUrl: '',
  published: true,
  adminOnly: false,
  paramSchema: [],
  perCallBilling: false,
  fixedQuotaPerCall: 0,
  modelBaseRateRatio: 1.0,
  channelId: 0,
}

const CURL_EXAMPLE = `curl --location --request POST 'https://www.runninghub.cn/openapi/v2/run/ai-app/2051268528824700930' \\
--header "Content-Type: application/json" \\
--header "Authorization: Bearer \${RUNNINGHUB_API_KEY}" \\
--data-raw '{
  "nodeInfoList": [
    {
      "nodeId": "642",
      "fieldName": "image",
      "fieldValue": "4cec23f9fef05cb20ca8b045fdf6daee81016b3d69e3dcc0b99430e1610fa3d3.jpg",
      "description": "image"
    },
    {
      "nodeId": "641",
      "fieldName": "text",
      "fieldValue": "8K高清，人像精修，去除脸上的痘印，专业人像摄影，肤色干净通透，均匀细腻无瑕疵，奶油肌质感，专业影棚精修，柔和影棚光，高清细节，增强头发质感纹理，整个人看起来非常有气色，清晰可见的发丝，超高清的服装，整体色调与原图保持一致",
      "description": "提示词"
    },
    {
      "nodeId": "755",
      "fieldName": "value",
      "fieldValue": "2048",
      "description": "尺寸"
    }
  ],
  "instanceType": "default",
  "usePersonalQueue": "false"
}'`

function parseOptions(raw: string): SchemaParam['options'] {
  try {
    const arr = JSON.parse(raw)
    if (!Array.isArray(arr)) return []
    return arr.map((o) =>
      typeof o === 'string'
        ? { label: o, value: o }
        : { label: o?.label ?? o?.value, value: o?.value ?? o?.label }
    )
  } catch {
    return []
  }
}

function AppForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial: AppCreateDTO
  onSubmit: (dto: AppCreateDTO) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [dto, setDto] = useState<AppCreateDTO>(initial)
  const [curlInput, setCurlInput] = useState('')
  const [fetching, setFetching] = useState(false)

  const set = <K extends keyof AppCreateDTO>(k: K, v: AppCreateDTO[K]) =>
    setDto((prev) => ({ ...prev, [k]: v }))

  const setParam = (i: number, patch: Partial<SchemaParam>) =>
    setDto((prev) => {
      const next = prev.paramSchema.map((p, idx) =>
        idx === i ? { ...p, ...patch } : p
      )
      return { ...prev, paramSchema: next }
    })

  const removeParam = (i: number) =>
    setDto((prev) => ({
      ...prev,
      paramSchema: prev.paramSchema.filter((_, idx) => idx !== i),
    }))

  const addParam = () =>
    setDto((prev) => ({
      ...prev,
      paramSchema: [
        ...prev.paramSchema,
        { nodeId: '', fieldName: '', label: '', type: 'text', required: true },
      ],
    }))

  const handleFetch = async () => {
    if (!curlInput.trim()) {
      toast.error(t('Please paste a RunningHub request example first'))
      return
    }
    setFetching(true)
    try {
      const res = await parseCurlRequest(curlInput.trim())
      const data = res.data
      if (!res.success || !data) {
        toast.error(res.message || t('Failed to parse the request example'))
        return
      }
      const schema = data.schema ?? []
      const errors = data.schemaErrors ?? []
      setDto((prev) => ({
        ...prev,
        kind: data.kind || prev.kind,
        upstreamId: data.upstreamId || prev.upstreamId,
        name: prev.name.trim() || data.appName || '',
        paramSchema: schema,
      }))
      if (errors.length > 0) {
        toast.warning(t('Request example parsed, but some fields were skipped'))
      } else {
        toast.success(t('Request example parsed'))
      }
    } catch (e) {
      toast.error(String((e as Error)?.message ?? e))
    } finally {
      setFetching(false)
    }
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(dto)
      }}
      className='space-y-4'
    >
      <div className='grid grid-cols-1 gap-6 lg:grid-cols-2'>
        {/* ---------- LEFT: basic info ---------- */}
        <div className='space-y-4'>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-name'>{t('App Name')}</Label>
            <Input
              id='rh-app-name'
              value={dto.name}
              onChange={(e) => set('name', e.target.value)}
              required
            />
          </div>
          <div className='grid grid-cols-2 gap-4'>
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-kind'>{t('Kind')}</Label>
              <Select
                value={dto.kind}
                onValueChange={(v) => {
                  if (v != null) set('kind', v)
                }}
              >
                <SelectTrigger id='rh-app-kind'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='ai_app'>{t('AIC App')}</SelectItem>
                  <SelectItem value='workflow'>{t('Workflow')}</SelectItem>
                  <SelectItem value='model'>{t('Model')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-upstream'>{t('Upstream ID')}</Label>
              <Input
                id='rh-app-upstream'
                value={dto.upstreamId}
                onChange={(e) => set('upstreamId', e.target.value)}
                required
              />
            </div>
          </div>
          <div className='grid grid-cols-2 gap-4'>
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-ratio'>{t('Model Base Rate Ratio')}</Label>
              <Input
                id='rh-app-ratio'
                type='number'
                step='0.01'
                min='0.01'
                value={dto.modelBaseRateRatio}
                onChange={(e) =>
                  set('modelBaseRateRatio', parseFloat(e.target.value) || 1.0)
                }
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-fixed'>{t('Fixed Quota')}</Label>
              <Input
                id='rh-app-fixed'
                type='number'
                min='0'
                value={dto.fixedQuotaPerCall}
                onChange={(e) =>
                  set('fixedQuotaPerCall', parseInt(e.target.value) || 0)
                }
                disabled={!dto.perCallBilling}
              />
            </div>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-slug'>{t('Slug')}</Label>
            <Input
              id='rh-app-slug'
              value={dto.slug}
              onChange={(e) => set('slug', e.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-desc'>{t('Description')}</Label>
            <Input
              id='rh-app-desc'
              value={dto.description}
              onChange={(e) => set('description', e.target.value)}
            />
          </div>
          <div className='flex items-center gap-6'>
            <label className='flex items-center gap-2'>
              <Switch
                checked={dto.published}
                onCheckedChange={(v) => set('published', v as boolean)}
              />
              <span className='text-sm'>{t('Published')}</span>
            </label>
            <label className='flex items-center gap-2'>
              <Switch
                checked={dto.adminOnly}
                onCheckedChange={(v) => set('adminOnly', v as boolean)}
              />
              <span className='text-sm'>{t('Admin Only')}</span>
            </label>
            <label className='flex items-center gap-2'>
              <Switch
                checked={dto.perCallBilling}
                onCheckedChange={(v) => set('perCallBilling', v as boolean)}
              />
              <span className='text-sm'>{t('Per-Call Billing')}</span>
            </label>
          </div>
        </div>

        {/* ---------- RIGHT: request example + parameter template ---------- */}
        <div className='space-y-3 rounded-md border p-3'>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-curl'>{t('Request Example')}</Label>
            <textarea
              id='rh-app-curl'
              className='text-muted-foreground field-sizing-content max-h-72 min-h-40 w-full rounded-md border bg-transparent px-2 py-1 font-mono text-xs'
              placeholder={t('Paste a RunningHub curl request example…')}
              value={curlInput}
              onChange={(e) => setCurlInput(e.target.value)}
            />
          </div>
          <div className='flex items-center justify-between gap-2'>
            <Button
              type='button'
              size='sm'
              variant='ghost'
              title={t('Fill with an example')}
              onClick={() => setCurlInput(CURL_EXAMPLE)}
            >
              {t('Use example')}
            </Button>
            <Button
              type='button'
              size='sm'
              onClick={handleFetch}
              disabled={fetching}
            >
              {fetching ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <Wand2 className='size-4' />
              )}
              {t('Fetch Template')}
            </Button>
          </div>
          <div className='flex items-center justify-between'>
            <div className='text-sm font-medium'>{t('Parameter Template')}</div>
          </div>
          {dto.paramSchema.length === 0 ? (
            <p className='text-muted-foreground text-sm'>
              {t(
                'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.'
              )}
            </p>
          ) : (
            dto.paramSchema.map((p, i) => (
              <div key={i} className='space-y-2 rounded-md border p-2'>
                <div className='grid grid-cols-[1fr_140px] gap-2'>
                  <Input
                    value={p.label}
                    placeholder={t('Label')}
                    onChange={(e) => setParam(i, { label: e.target.value })}
                  />
                  <Select
                    value={p.type}
                    onValueChange={(v) => {
                      if (v != null) setParam(i, { type: v })
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {TYPE_CHOICES.map((c) => (
                        <SelectItem key={c} value={c}>
                          {c}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='grid grid-cols-[1fr_140px] gap-2'>
                  <Input
                    value={p.fieldName}
                    placeholder={t('Field name')}
                    onChange={(e) => setParam(i, { fieldName: e.target.value })}
                  />
                  <label className='flex items-center gap-2'>
                    <Switch
                      checked={!!p.required}
                      onCheckedChange={(v) => setParam(i, { required: !!v })}
                    />
                    <span className='text-xs'>{t('Required')}</span>
                  </label>
                </div>
                {p.type === 'select' && (
                  <textarea
                    className='min-h-16 w-full rounded-md border bg-transparent px-2 py-1 font-mono text-xs'
                    placeholder={t(
                      'Options as JSON, e.g. [{"label":"1:1","value":"1:1"}]'
                    )}
                    value={JSON.stringify(p.options ?? [])}
                    onChange={(e) =>
                      setParam(i, { options: parseOptions(e.target.value) })
                    }
                  />
                )}
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground text-xs'>
                    {p.nodeId ? `nodeId=${p.nodeId}` : ''}
                  </span>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    onClick={() => removeParam(i)}
                  >
                    <X className='size-3.5' />
                  </Button>
                </div>
              </div>
            ))
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={addParam}
            className='w-full'
          >
            <Plus className='size-4' />
            {t('Add Parameter')}
          </Button>
        </div>
      </div>

      <div className='flex justify-end gap-2 pt-2'>
        <Button type='button' variant='outline' onClick={onCancel}>
          {t('Cancel')}
        </Button>
        <Button type='submit'>{t('Save')}</Button>
      </div>
    </form>
  )
}

function KeypoolContent({
  data,
  loading,
  refreshing,
  onRefresh,
}: {
  data: AppKeypoolResult | undefined
  loading: boolean
  refreshing: boolean
  onRefresh: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div className='text-muted-foreground text-sm'>
          {data
            ? data.channelId
              ? `${data.channelName || `#${data.channelId}`} · ${data.enabled}/${data.total} ${t('keys enabled')}`
              : t('No channel bound — keypool cannot sync')
            : ''}
        </div>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={onRefresh}
          disabled={refreshing || loading}
        >
          {refreshing ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <RefreshCw className='size-4' />
          )}
          {t('Refresh Keypool')}
        </Button>
      </div>
      {loading ? (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      ) : !data || data.keys.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('No keys in this app keypool yet')}
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Key')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('In-flight')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.keys.map((k) => (
              <TableRow key={k.id}>
                <TableCell className='font-mono text-xs'>{k.key}</TableCell>
                <TableCell>
                  {k.enabled ? (
                    <Badge>{t('Enabled')}</Badge>
                  ) : (
                    <Badge variant='outline'>{t('Disabled')}</Badge>
                  )}
                </TableCell>
                <TableCell className='text-right'>{k.occupancy}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

export function RhAppsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<AppView | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)

  const { data: channels } = useQuery({
    queryKey: ['rh-channels'],
    queryFn: getRhChannels,
  })
  const channelNameById = (channels ?? []).reduce<Record<number, string>>(
    (m, ch) => {
      m[ch.id] = ch.name
      return m
    },
    {}
  )

  const { data, isLoading } = useQuery({
    queryKey: ['rh-apps'],
    queryFn: () => listApps({ page: 1, pageSize: 100 }),
  })

  const createMutation = useMutation({
    mutationFn: (dto: AppCreateDTO) => createApp(dto),
    onSuccess: () => {
      toast.success(t('App created'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
      setDialogOpen(false)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, dto }: { id: number; dto: AppCreateDTO }) =>
      updateApp(id, dto),
    onSuccess: () => {
      toast.success(t('App updated'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
      setDialogOpen(false)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteApp(id),
    onSuccess: () => {
      toast.success(t('App deleted'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
      setDeleteId(null)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const syncMutation = useMutation({
    mutationFn: (channelId: number) => syncAppsFromChannel(channelId),
    onSuccess: () => {
      toast.success(t('Synced from channel'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const [keypoolAppId, setKeypoolAppId] = useState<number | null>(null)

  const keypoolQuery = useQuery({
    queryKey: ['rh-app-keypool', keypoolAppId],
    queryFn: () => getAppKeypool(keypoolAppId as number),
    enabled: keypoolAppId !== null,
  })

  const refreshKeypoolMutation = useMutation({
    mutationFn: (id: number) => refreshAppKeypool(id),
    onSuccess: () => {
      toast.success(t('Keypool refreshed'))
      queryClient.invalidateQueries({
        queryKey: ['rh-app-keypool', keypoolAppId],
      })
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const openCreate = () => {
    setEditing(null)
    setDialogOpen(true)
  }

  const openEdit = (app: AppView) => {
    setEditing(app)
    setDialogOpen(true)
  }

  const handleSubmit = (dto: AppCreateDTO) => {
    if (editing) {
      updateMutation.mutate({ id: editing.id, dto })
    } else {
      createMutation.mutate(dto)
    }
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('RunningHub Apps')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            size='sm'
            onClick={() => syncMutation.mutate(0)}
            disabled={syncMutation.isPending}
          >
            <RefreshCw className='size-4' />
            {t('Sync from Channel')}
          </Button>
          <Button size='sm' onClick={openCreate}>
            <Plus className='size-4' />
            {t('Create App')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>{t('App Name')}</TableHead>
                <TableHead>{t('Kind')}</TableHead>
                <TableHead>{t('Upstream ID')}</TableHead>
                <TableHead>{t('Channel')}</TableHead>
                <TableHead>{t('Per-Call Billing')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={7} className='text-center'>
                    {t('Loading...')}
                  </TableCell>
                </TableRow>
              ) : !data?.items?.length ? (
                <TableRow>
                  <TableCell colSpan={7} className='text-center'>
                    {t('No data')}
                  </TableCell>
                </TableRow>
              ) : (
                data.items.map((app) => (
                  <TableRow key={app.id}>
                    <TableCell>{app.id}</TableCell>
                    <TableCell className='font-medium'>{app.name}</TableCell>
                    <TableCell>{app.kind}</TableCell>
                    <TableCell className='font-mono text-xs'>
                      {app.upstreamId}
                    </TableCell>
                    <TableCell className='text-xs'>
                      {app.channelId
                        ? (channelNameById[app.channelId] ??
                          `#${app.channelId}`)
                        : '—'}
                    </TableCell>
                    <TableCell>
                      {app.perCallBilling ? (
                        <Badge>{t('Per-Call Billing')}</Badge>
                      ) : (
                        <Badge variant='outline'>{t('Dynamic')}</Badge>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        title={t('Keypool')}
                        onClick={() => setKeypoolAppId(app.id)}
                      >
                        <KeyRound className='size-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => openEdit(app)}
                      >
                        <Pencil className='size-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => setDeleteId(app.id)}
                      >
                        <Trash2 className='size-3.5' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-h-[85vh] w-[80vw] max-w-[80vw] overflow-y-auto sm:max-w-[80vw]'>
          <DialogHeader>
            <DialogTitle>
              {editing ? t('Edit App') : t('Create App')}
            </DialogTitle>
          </DialogHeader>
          <AppForm
            initial={
              editing
                ? {
                    name: editing.name,
                    slug: editing.slug,
                    kind: editing.kind,
                    upstreamId: editing.upstreamId,
                    description: editing.description,
                    coverUrl: editing.coverUrl,
                    published: editing.published,
                    adminOnly: editing.adminOnly,
                    paramSchema: editing.paramSchema,
                    perCallBilling: editing.perCallBilling,
                    fixedQuotaPerCall: editing.fixedQuotaPerCall,
                    modelBaseRateRatio: editing.modelBaseRateRatio,
                    channelId: editing.channelId ?? 0,
                  }
                : emptyDTO
            }
            onSubmit={handleSubmit}
            onCancel={() => setDialogOpen(false)}
          />
        </DialogContent>
      </Dialog>

      <Dialog
        open={keypoolAppId !== null}
        onOpenChange={(v) => !v && setKeypoolAppId(null)}
      >
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('App Keypool')}</DialogTitle>
          </DialogHeader>
          {keypoolAppId !== null && (
            <KeypoolContent
              data={keypoolQuery.data}
              loading={keypoolQuery.isLoading}
              refreshing={refreshKeypoolMutation.isPending}
              onRefresh={() => refreshKeypoolMutation.mutate(keypoolAppId)}
            />
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteId !== null}
        onOpenChange={(v) => !v && setDeleteId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete App')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Are you sure you want to delete this app?')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
