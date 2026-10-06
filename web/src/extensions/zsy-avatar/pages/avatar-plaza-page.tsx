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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Loader2, Plus, Search, Upload } from 'lucide-react'
import { useState, type ReactNode } from 'react'
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
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import {
  createAvatar,
  deleteAvatar,
  listAvatars,
  updateAvatar,
  type AvatarListParams,
  type AvatarUpsertDTO,
  type AvatarView,
} from '../api'
import { AvatarEndpointCard } from '../components/avatar-endpoint-card'
import { AvatarFormDialog } from '../components/avatar-form-dialog'
import { AvatarImportDialog } from '../components/avatar-import-dialog'
import { AvatarTableRow } from '../components/avatar-table-row'
import { downloadAvatarsCsv } from '../lib/avatar-export'
import {
  AGE_RANGE_OPTIONS,
  GENDER_OPTIONS,
  RACE_OPTIONS,
} from '../lib/avatar-fields'

/** `all` maps to "no filter", so both shelf states show up side by side. */
type ShelfFilter = 'all' | 'on' | 'off'

const SHELF_FILTERS: ShelfFilter[] = ['all', 'on', 'off']

const SHELF_FILTER_LABEL: Record<ShelfFilter, string> = {
  all: 'All',
  on: 'On Shelf',
  off: 'Off Shelf',
}

const PAGE_SIZE_CHOICES = ['10', '20', '50', '100']

/** Columns of the plaza table, used by the loading and empty states. */
const AVATAR_TABLE_COLUMNS = 7

const avatarListQueryKey = 'zsy-avatar-list'

export function AvatarPlazaPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keywordDraft, setKeywordDraft] = useState('')
  const [keyword, setKeyword] = useState('')
  const [shelf, setShelf] = useState<ShelfFilter>('all')
  const [gender, setGender] = useState('')
  const [ageRange, setAgeRange] = useState('')
  const [race, setRace] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [editing, setEditing] = useState<AvatarView | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AvatarView | null>(null)

  const filters: AvatarListParams = {
    keyword,
    gender,
    ageRange,
    race,
    enabled: shelf === 'all' ? undefined : shelf === 'on',
  }

  const { data, isLoading } = useQuery({
    queryKey: [avatarListQueryKey, { ...filters, page, pageSize }],
    queryFn: () => listAvatars({ ...filters, page, pageSize }),
    // The plaza itself can change without this page acting (a CSV import run from
    // the server, another admin): opening the page always asks the server again
    // instead of trusting a cached answer.
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: true,
  })

  const avatars = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = data?.totalPages ?? 0

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: [avatarListQueryKey] })

  const createMutation = useMutation({
    mutationFn: (dto: AvatarUpsertDTO) => createAvatar(dto),
    onSuccess: () => {
      toast.success(t('Avatar created'))
      void refresh()
      setFormOpen(false)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, dto }: { id: number; dto: AvatarUpsertDTO }) =>
      updateAvatar(id, dto),
    onSuccess: () => {
      toast.success(t('Avatar updated'))
      void refresh()
      setFormOpen(false)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteAvatar(id),
    onSuccess: () => {
      toast.success(t('Avatar deleted'))
      void refresh()
      setDeleteTarget(null)
      // Deleting the last row of a page would otherwise leave a blank page.
      if (avatars.length === 1 && page > 1) setPage(page - 1)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const openCreate = () => {
    setEditing(null)
    setFormOpen(true)
  }

  const openEdit = (avatar: AvatarView) => {
    setEditing(avatar)
    setFormOpen(true)
  }

  const submitForm = (dto: AvatarUpsertDTO) => {
    if (editing) {
      updateMutation.mutate({ id: editing.id, dto })
      return
    }
    createMutation.mutate(dto)
  }

  const applySearch = () => {
    setKeyword(keywordDraft.trim())
    setPage(1)
  }

  // The export carries the same filters as the table, so what the operator sees
  // is what the file contains.
  const runExport = async () => {
    setExporting(true)
    try {
      await downloadAvatarsCsv(filters)
      toast.success(t('Export started'))
    } catch (error) {
      toast.error((error as Error)?.message || t('Export failed'))
    } finally {
      setExporting(false)
    }
  }

  let tableBody: ReactNode
  if (isLoading) {
    tableBody = (
      <TableRow>
        <TableCell colSpan={AVATAR_TABLE_COLUMNS} className='h-24 text-center'>
          <Loader2 className='mx-auto size-5 animate-spin' />
        </TableCell>
      </TableRow>
    )
  } else if (avatars.length === 0) {
    tableBody = (
      <TableRow>
        <TableCell
          colSpan={AVATAR_TABLE_COLUMNS}
          className='text-muted-foreground h-24 text-center'
        >
          {t('No avatars yet')}
        </TableCell>
      </TableRow>
    )
  } else {
    tableBody = avatars.map((avatar) => (
      <AvatarTableRow
        key={avatar.id}
        avatar={avatar}
        onEdit={openEdit}
        onDelete={setDeleteTarget}
      />
    ))
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Avatar Plaza')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            size='sm'
            variant='outline'
            onClick={() => setImportOpen(true)}
          >
            <Upload className='size-4' />
            {t('Import')}
          </Button>
          <Button
            size='sm'
            variant='outline'
            disabled={exporting}
            onClick={() => void runExport()}
          >
            {exporting ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <Download className='size-4' />
            )}
            {t('Export')}
          </Button>
          <Button size='sm' onClick={openCreate}>
            <Plus className='size-4' />
            {t('New Avatar')}
          </Button>
        </SectionPageLayout.Actions>

        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <div className='flex flex-wrap items-center gap-2'>
              <Input
                value={keywordDraft}
                onChange={(event) => setKeywordDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') applySearch()
                }}
                placeholder={t('Search by name or voice id')}
                aria-label={t('Search by name or voice id')}
                className='w-full sm:w-64'
              />
              <Button variant='outline' size='sm' onClick={applySearch}>
                <Search className='size-4' />
                {t('Search')}
              </Button>
              <Select
                value={shelf}
                onValueChange={(value) => {
                  setShelf((value || 'all') as ShelfFilter)
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='avatar-plaza-shelf'
                  aria-label={t('Shelf Status')}
                  className='w-36'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SHELF_FILTERS.map((filter) => (
                    <SelectItem key={filter} value={filter}>
                      {t(SHELF_FILTER_LABEL[filter])}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={gender}
                onValueChange={(value) => {
                  setGender(value ?? '')
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='avatar-plaza-gender'
                  aria-label={t('Gender')}
                  className='w-32'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {GENDER_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={ageRange}
                onValueChange={(value) => {
                  setAgeRange(value ?? '')
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='avatar-plaza-age-range'
                  aria-label={t('Age Range')}
                  className='w-32'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {AGE_RANGE_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={race}
                onValueChange={(value) => {
                  setRace(value ?? '')
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='avatar-plaza-race'
                  aria-label={t('Race')}
                  className='w-40'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {RACE_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={String(pageSize)}
                onValueChange={(value) => {
                  setPageSize(Number(value) || 20)
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='avatar-plaza-page-size'
                  aria-label={t('Page size')}
                  className='w-32'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAGE_SIZE_CHOICES.map((size) => (
                    <SelectItem key={size} value={size}>
                      {t('{{count}} per page', { count: size })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Avatar Name')}</TableHead>
                  <TableHead>{t('Attributes')}</TableHead>
                  <TableHead>{t('Introduction')}</TableHead>
                  <TableHead>{t('Voice Sample')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Sort Order')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>{tableBody}</TableBody>
            </Table>

            <div className='flex flex-wrap items-center justify-between gap-2'>
              <p className='text-muted-foreground text-sm'>
                {t('Total {{count}} avatars', { count: total })}
              </p>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage(page - 1)}
                >
                  {t('Previous')}
                </Button>
                <span className='text-sm'>
                  {t('Page {{page}} / {{total}}', {
                    page: data?.page ?? page,
                    total: Math.max(totalPages, 1),
                  })}
                </span>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={totalPages === 0 || page >= totalPages}
                  onClick={() => setPage(page + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            </div>

            <AvatarEndpointCard />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <AvatarFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        avatar={editing}
        onSubmit={submitForm}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <AvatarImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImported={() => void refresh()}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Avatar')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.',
                { name: deleteTarget?.name ?? '' }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
              }}
            >
              {t('Delete Avatar')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
