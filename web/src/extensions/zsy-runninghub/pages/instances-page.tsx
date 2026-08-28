/*
Copyright (C) 2023-2026 QuantumNous

RunningHub Instances admin page: list, create, edit, delete, keypool refresh.
*/

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, KeyRound } from 'lucide-react'
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
  listInstances,
  createInstance,
  updateInstance,
  deleteInstance,
  refreshKeypool,
  type InstanceView,
  type InstanceCreateDTO,
} from '../api'

const emptyDTO: InstanceCreateDTO = {
  name: '',
  appId: 0,
  channelId: 0,
  enabled: true,
}

function InstanceForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial: InstanceCreateDTO
  onSubmit: (dto: InstanceCreateDTO) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [dto, setDto] = useState<InstanceCreateDTO>(initial)

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
          <Label htmlFor='rh-inst-name'>{t('Instance Name')}</Label>
          <Input
            id='rh-inst-name'
            value={dto.name}
            onChange={(e) => setDto({ ...dto, name: e.target.value })}
            required
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='rh-inst-appid'>App ID</Label>
          <Input
            id='rh-inst-appid'
            type='number'
            value={dto.appId}
            onChange={(e) =>
              setDto({ ...dto, appId: parseInt(e.target.value) || 0 })
            }
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='rh-inst-chid'>{t('Channel')}</Label>
          <Input
            id='rh-inst-chid'
            type='number'
            value={dto.channelId}
            onChange={(e) =>
              setDto({ ...dto, channelId: parseInt(e.target.value) || 0 })
            }
          />
        </div>
        <div className='flex items-end gap-2 pb-1'>
          <label className='flex items-center gap-2'>
            <Switch
              checked={dto.enabled}
              onCheckedChange={(v) =>
                setDto({ ...dto, enabled: v as boolean })
              }
            />
            <span className='text-sm'>{t('Enabled')}</span>
          </label>
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

export function RhInstancesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<InstanceView | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['rh-instances'],
    queryFn: () => listInstances({ page: 1, pageSize: 100 }),
  })

  const createMutation = useMutation({
    mutationFn: (dto: InstanceCreateDTO) => createInstance(dto),
    onSuccess: () => {
      toast.success(t('Instance created'))
      queryClient.invalidateQueries({ queryKey: ['rh-instances'] })
      setDialogOpen(false)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      dto,
    }: {
      id: number
      dto: Partial<InstanceCreateDTO>
    }) => updateInstance(id, dto),
    onSuccess: () => {
      toast.success(t('Instance updated'))
      queryClient.invalidateQueries({ queryKey: ['rh-instances'] })
      setDialogOpen(false)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteInstance(id),
    onSuccess: () => {
      toast.success(t('Instance deleted'))
      queryClient.invalidateQueries({ queryKey: ['rh-instances'] })
      setDeleteId(null)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const refreshMutation = useMutation({
    mutationFn: (id: number) => refreshKeypool(id),
    onSuccess: () => {
      toast.success(t('Keys refreshed'))
      queryClient.invalidateQueries({ queryKey: ['rh-instances'] })
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const handleSubmit = (dto: InstanceCreateDTO) => {
    if (editing) {
      updateMutation.mutate({ id: editing.id, dto })
    } else {
      createMutation.mutate(dto)
    }
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('RunningHub Instances')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          size='sm'
          onClick={() => {
            setEditing(null)
            setDialogOpen(true)
          }}
        >
          <Plus className='size-4' />
          {t('Create Instance')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>{t('Instance Name')}</TableHead>
              <TableHead>{t('Channel')}</TableHead>
              <TableHead>{t('Key Pool Count')}</TableHead>
              <TableHead>{t('Enabled')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className='text-center'>
                  {t('Loading...')}
                </TableCell>
              </TableRow>
            ) : !data?.items?.length ? (
              <TableRow>
                <TableCell colSpan={6} className='text-center'>
                  {t('No data')}
                </TableCell>
              </TableRow>
            ) : (
              data.items.map((inst) => (
                <TableRow key={inst.id}>
                  <TableCell>{inst.id}</TableCell>
                  <TableCell className='font-medium'>{inst.name}</TableCell>
                  <TableCell>
                    {inst.channelName || `#${inst.channelId}`}
                  </TableCell>
                  <TableCell>{inst.keypoolCount}</TableCell>
                  <TableCell>
                    {inst.enabled ? (
                      <Badge>{t('Enabled')}</Badge>
                    ) : (
                      <Badge variant='outline'>{t('Disabled')}</Badge>
                    )}
                  </TableCell>
                  <TableCell className='text-right'>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => refreshMutation.mutate(inst.id)}
                      disabled={refreshMutation.isPending}
                    >
                      <KeyRound className='size-3.5' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => {
                        setEditing(inst)
                        setDialogOpen(true)
                      }}
                    >
                      <Pencil className='size-3.5' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => setDeleteId(inst.id)}
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

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {editing ? t('Edit Instance') : t('Create Instance')}
            </DialogTitle>
          </DialogHeader>
          <InstanceForm
            initial={
              editing
                ? {
                    name: editing.name,
                    appId: editing.appId,
                    channelId: editing.channelId,
                    enabled: editing.enabled,
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
            <AlertDialogTitle>{t('Delete Instance')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Are you sure you want to delete this instance?')}
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
    </SectionPageLayout>
  )
}
