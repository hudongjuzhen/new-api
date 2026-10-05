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
  createVoice,
  deleteVoice,
  listVoices,
  updateVoice,
  type VoiceListParams,
  type VoiceUpsertDTO,
  type VoiceView,
} from '../api'
import { VoiceEndpointCard } from '../components/voice-endpoint-card'
import { VoiceFormDialog } from '../components/voice-form-dialog'
import { VoiceImportDialog } from '../components/voice-import-dialog'
import { VoiceTableRow } from '../components/voice-table-row'
import { downloadVoicesCsv } from '../lib/voice-export'
import {
  AGE_RANGE_OPTIONS,
  GENDER_OPTIONS,
  VOICE_LANGUAGE_CODES,
  voiceLanguageLabel,
} from '../lib/voice-fields'

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
const VOICE_TABLE_COLUMNS = 7

const voiceListQueryKey = 'zsy-voice-list'

export function VoicePlazaPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keywordDraft, setKeywordDraft] = useState('')
  const [keyword, setKeyword] = useState('')
  const [shelf, setShelf] = useState<ShelfFilter>('all')
  const [gender, setGender] = useState('')
  const [ageRange, setAgeRange] = useState('')
  const [language, setLanguage] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [editing, setEditing] = useState<VoiceView | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<VoiceView | null>(null)

  const filters: VoiceListParams = {
    keyword,
    gender,
    ageRange,
    language,
    enabled: shelf === 'all' ? undefined : shelf === 'on',
  }

  const { data, isLoading } = useQuery({
    queryKey: [voiceListQueryKey, { ...filters, page, pageSize }],
    queryFn: () => listVoices({ ...filters, page, pageSize }),
  })

  const voices = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = data?.totalPages ?? 0

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: [voiceListQueryKey] })

  const createMutation = useMutation({
    mutationFn: (dto: VoiceUpsertDTO) => createVoice(dto),
    onSuccess: () => {
      toast.success(t('Voice created'))
      void refresh()
      setFormOpen(false)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, dto }: { id: number; dto: VoiceUpsertDTO }) =>
      updateVoice(id, dto),
    onSuccess: () => {
      toast.success(t('Voice updated'))
      void refresh()
      setFormOpen(false)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteVoice(id),
    onSuccess: () => {
      toast.success(t('Voice deleted'))
      void refresh()
      setDeleteTarget(null)
      // Deleting the last row of a page would otherwise leave a blank page.
      if (voices.length === 1 && page > 1) setPage(page - 1)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const openCreate = () => {
    setEditing(null)
    setFormOpen(true)
  }

  const openEdit = (voice: VoiceView) => {
    setEditing(voice)
    setFormOpen(true)
  }

  const submitForm = (dto: VoiceUpsertDTO) => {
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
      await downloadVoicesCsv(filters)
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
        <TableCell colSpan={VOICE_TABLE_COLUMNS} className='h-24 text-center'>
          <Loader2 className='mx-auto size-5 animate-spin' />
        </TableCell>
      </TableRow>
    )
  } else if (voices.length === 0) {
    tableBody = (
      <TableRow>
        <TableCell
          colSpan={VOICE_TABLE_COLUMNS}
          className='text-muted-foreground h-24 text-center'
        >
          {t('No voices yet')}
        </TableCell>
      </TableRow>
    )
  } else {
    tableBody = voices.map((voice) => (
      <VoiceTableRow
        key={voice.id}
        voice={voice}
        onEdit={openEdit}
        onDelete={setDeleteTarget}
      />
    ))
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Voice Plaza')}</SectionPageLayout.Title>
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
            {t('New Voice')}
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
                placeholder={t('Search by name or voice_type')}
                aria-label={t('Search by name or voice_type')}
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
                  id='voice-plaza-shelf'
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
                  id='voice-plaza-gender'
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
                  id='voice-plaza-age-range'
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
                value={language}
                onValueChange={(value) => {
                  setLanguage(value ?? '')
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='voice-plaza-language'
                  aria-label={t('Language')}
                  className='w-36'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value=''>{t('All languages')}</SelectItem>
                  {VOICE_LANGUAGE_CODES.map((code) => (
                    <SelectItem key={code} value={code}>
                      {voiceLanguageLabel(code, i18n.language)}
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
                  id='voice-plaza-page-size'
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
                  <TableHead>{t('Voice Name')}</TableHead>
                  <TableHead>{t('Attributes')}</TableHead>
                  <TableHead>{t('Introduction')}</TableHead>
                  <TableHead>{t('Audio Sample')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Sort Order')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>{tableBody}</TableBody>
            </Table>

            <div className='flex flex-wrap items-center justify-between gap-2'>
              <p className='text-muted-foreground text-sm'>
                {t('Total {{count}} voices', { count: total })}
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

            <VoiceEndpointCard />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <VoiceFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        voice={editing}
        onSubmit={submitForm}
        submitting={createMutation.isPending || updateMutation.isPending}
      />

      <VoiceImportDialog
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
            <AlertDialogTitle>{t('Delete Voice')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.',
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
              {t('Delete Voice')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
