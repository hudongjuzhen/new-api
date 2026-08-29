/*
Copyright (C) 2023-2026 QuantumNous

RunningHub Apps admin page: list, create, edit, delete.
*/

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  listApps,
  createApp,
  updateApp,
  deleteApp,
  syncAppsFromChannel,
  type AppView,
  type AppCreateDTO,
} from '../api'

const emptyDTO: AppCreateDTO = {
  name: '',
  slug: '',
  kind: 'aic_app',
  upstreamId: '',
  description: '',
  coverUrl: '',
  published: true,
  adminOnly: false,
  paramSchema: [],
  perCallBilling: false,
  fixedQuotaPerCall: 0,
  modelBaseRateRatio: 1.0,
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

  const set = <K extends keyof AppCreateDTO>(k: K, v: AppCreateDTO[K]) =>
    setDto((prev) => ({ ...prev, [k]: v }))

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(dto)
      }}
      className='space-y-4'
    >
      <div className='grid grid-cols-2 gap-4'>
        <div className='space-y-1.5'>
          <Label htmlFor='rh-app-name'>{t('App Name')}</Label>
          <Input
            id='rh-app-name'
            value={dto.name}
            onChange={(e) => set('name', e.target.value)}
            required
          />
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
          <Label htmlFor='rh-app-kind'>{t('Kind')}</Label>
          <Select value={dto.kind} onValueChange={(v) => set('kind', v)}>
            <SelectTrigger id='rh-app-kind'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='aic_app'>{t('AIC App')}</SelectItem>
              <SelectItem value='workflow'>{t('Workflow')}</SelectItem>
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
      <div className='flex justify-end gap-2 pt-2'>
        <Button type='button' variant='outline' onClick={onCancel}>
          {t('Cancel')}
        </Button>
        <Button type='submit'>{t('Save')}</Button>
      </div>
    </form>
  )
}

export function RhAppsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<AppView | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)

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
        <SectionPageLayout.Title>{t('RunningHub Apps')}</SectionPageLayout.Title>
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
                <TableHead>{t('Per-Call Billing')}</TableHead>
                <TableHead>{t('Rate Ratio')}</TableHead>
                <TableHead>{t('Published')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className='text-center'>
                    {t('Loading...')}
                  </TableCell>
                </TableRow>
              ) : !data?.items?.length ? (
                <TableRow>
                  <TableCell colSpan={8} className='text-center'>
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
                    <TableCell>
                      {app.perCallBilling ? (
                        <Badge>{t('Per-Call Billing')}</Badge>
                      ) : (
                        <Badge variant='outline'>{t('Dynamic')}</Badge>
                      )}
                    </TableCell>
                    <TableCell>{app.modelBaseRateRatio}</TableCell>
                    <TableCell>
                      {app.published ? (
                        <Badge>{t('Published')}</Badge>
                      ) : (
                        <Badge variant='outline'>{t('Draft')}</Badge>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
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
        <DialogContent className='max-w-2xl'>
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
                  }
                : emptyDTO
            }
            onSubmit={handleSubmit}
            onCancel={() => setDialogOpen(false)}
          />
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
