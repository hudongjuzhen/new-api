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
import { Badge } from '@/components/ui/badge'
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
  createTone,
  deleteTone,
  fetchToneStandard,
  listTones,
  updateTone,
  type ToneListParams,
  type ToneUpsertDTO,
  type ToneView,
} from '../api'
import { ToneEndpointCard } from '../components/tone-endpoint-card'
import { ToneFormDialog } from '../components/tone-form-dialog'
import { ToneImportDialog } from '../components/tone-import-dialog'
import { ToneSampleDialog } from '../components/tone-sample-dialog'
import { ToneTableRow } from '../components/tone-table-row'
import { downloadTonesCsv } from '../lib/tone-export'
import {
  normalizeToneStandard,
  TONE_LANGUAGE_CODES,
  TONE_STANDARD_FALLBACK,
  toneLanguageLabel,
  toneOptionText,
  type ToneStandardOption,
} from '../lib/tone-fields'

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
const TONE_TABLE_COLUMNS = 7

const toneListQueryKey = 'zsy-tone-list'
const toneStandardQueryKey = 'zsy-tone-standard'

export function TonePlazaPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keywordDraft, setKeywordDraft] = useState('')
  const [keyword, setKeyword] = useState('')
  const [shelf, setShelf] = useState<ShelfFilter>('all')
  const [category, setCategory] = useState('')
  const [tone, setTone] = useState('')
  const [language, setLanguage] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [editing, setEditing] = useState<ToneView | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ToneView | null>(null)
  const [sampleTarget, setSampleTarget] = useState<ToneView | null>(null)

  /*
   * The published standard, fetched once per mount. Every option list and every
   * cap on this screen comes from it, so adding a category on the backend shows
   * up here without a frontend release.
   *
   * A failure is swallowed into the mirror on purpose: the standard is an
   * enhancement, and an operator who came here to edit one prompt should not be
   * stopped by a red toast about an endpoint they never asked for. The mirror
   * carries the same vocabulary and caps, and the version chip says which one is
   * in force (hover), so the fallback is visible without being noisy.
   */
  const { data: loadedStandard } = useQuery({
    queryKey: [toneStandardQueryKey],
    queryFn: async () => {
      try {
        return normalizeToneStandard(await fetchToneStandard())
      } catch {
        return TONE_STANDARD_FALLBACK
      }
    },
    // The standard is the one cacheable public endpoint (`Cache-Control:
    // public, max-age=3600`): its content changes only when a version ships,
    // unlike the catalog, which is mutable data. Matching that hour here means
    // the page does not re-ask while an operator works.
    staleTime: 60 * 60 * 1000,
  })
  const standard = loadedStandard ?? TONE_STANDARD_FALLBACK

  /*
   * Filters treat the empty vocabulary value as "no filter" (the leading「All」
   * item), which is what an operator reaches for; the empty value stays
   * selectable in the form, where "uncategorized" is a real state of a row.
   */
  const categoryChoices = standard.categories.filter(
    (option) => option.value !== ''
  )
  const toneChoices = standard.tones.filter((option) => option.value !== '')

  const filters: ToneListParams = {
    keyword,
    category,
    tone,
    language,
    enabled: shelf === 'all' ? undefined : shelf === 'on',
  }

  const { data, isLoading } = useQuery({
    queryKey: [toneListQueryKey, { ...filters, page, pageSize }],
    queryFn: () => listTones({ ...filters, page, pageSize }),
    // The plaza itself can change without this page acting (a CSV import run from
    // the server, another admin, a seeding script), so opening the page always
    // asks the server again instead of trusting a cached answer.
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: true,
  })

  const tones = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = data?.totalPages ?? 0

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: [toneListQueryKey] })

  const createMutation = useMutation({
    mutationFn: (dto: ToneUpsertDTO) => createTone(dto),
    onSuccess: () => {
      toast.success(t('Tone created'))
      void refresh()
      setFormOpen(false)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, dto }: { id: number; dto: ToneUpsertDTO }) =>
      updateTone(id, dto),
    onSuccess: () => {
      toast.success(t('Tone updated'))
      void refresh()
      setFormOpen(false)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteTone(id),
    onSuccess: () => {
      toast.success(t('Tone deleted'))
      void refresh()
      setDeleteTarget(null)
      // Deleting the last row of a page would otherwise leave a blank page.
      if (tones.length === 1 && page > 1) setPage(page - 1)
    },
    onError: (error: unknown) => {
      toast.error((error as Error)?.message || t('Request failed'))
    },
  })

  const openCreate = () => {
    setEditing(null)
    setFormOpen(true)
  }

  const openEdit = (target: ToneView) => {
    setEditing(target)
    setFormOpen(true)
  }

  const submitForm = (dto: ToneUpsertDTO) => {
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
      await downloadTonesCsv(filters)
      toast.success(t('Export started'))
    } catch (error) {
      toast.error((error as Error)?.message || t('Export failed'))
    } finally {
      setExporting(false)
    }
  }

  /** A filter dropdown over one axis of the published vocabulary. */
  const filterSelect = (props: {
    id: string
    label: string
    value: string
    onChange: (value: string) => void
    options: ToneStandardOption[]
    width: string
  }) => (
    <Select
      value={props.value}
      onValueChange={(value) => {
        props.onChange(value ?? '')
        setPage(1)
      }}
    >
      <SelectTrigger id={props.id} aria-label={props.label} className={props.width}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value=''>{t('All')}</SelectItem>
        {props.options.map((option) => (
          <SelectItem
            key={option.value}
            value={option.value}
            title={option.desc || undefined}
          >
            {/* The label comes from the standard, translated where we have it. */}
            {toneOptionText(option, t)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  let tableBody: ReactNode
  if (isLoading) {
    tableBody = (
      <TableRow>
        <TableCell colSpan={TONE_TABLE_COLUMNS} className='h-24 text-center'>
          <Loader2 className='mx-auto size-5 animate-spin' />
        </TableCell>
      </TableRow>
    )
  } else if (tones.length === 0) {
    tableBody = (
      <TableRow>
        <TableCell
          colSpan={TONE_TABLE_COLUMNS}
          className='text-muted-foreground h-24 text-center'
        >
          {t('No tones yet')}
        </TableCell>
      </TableRow>
    )
  } else {
    tableBody = tones.map((row) => (
      <ToneTableRow
        key={row.id}
        tone={row}
        categories={standard.categories}
        tones={standard.tones}
        onEdit={openEdit}
        onDelete={setDeleteTarget}
        onShowSample={setSampleTarget}
      />
    ))
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Tone Plaza')}</SectionPageLayout.Title>
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
            {t('New Tone')}
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
                placeholder={t('Search by name, prompt or sample')}
                aria-label={t('Search by name, prompt or sample')}
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
                  id='tone-plaza-shelf'
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
              {filterSelect({
                id: 'tone-plaza-category',
                label: t('Category'),
                value: category,
                onChange: setCategory,
                options: categoryChoices,
                width: 'w-36',
              })}
              {filterSelect({
                id: 'tone-plaza-tone',
                label: t('Tone'),
                value: tone,
                onChange: setTone,
                options: toneChoices,
                width: 'w-32',
              })}
              <Select
                value={language}
                onValueChange={(value) => {
                  setLanguage(value ?? '')
                  setPage(1)
                }}
              >
                <SelectTrigger
                  id='tone-plaza-language'
                  aria-label={t('Language')}
                  className='w-36'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value=''>{t('All languages')}</SelectItem>
                  {TONE_LANGUAGE_CODES.map((code) => (
                    <SelectItem key={code} value={code}>
                      {toneLanguageLabel(code, i18n.language)}
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
                  id='tone-plaza-page-size'
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
              {/*
                The standard is a versioned contract, so which version is in
                force is shown rather than implied. The tooltip distinguishes the
                live endpoint from the built-in mirror — a silent fallback would
                otherwise be indistinguishable from a stale backend.
              */}
              <Badge
                variant='outline'
                className='font-normal'
                title={
                  standard.source === 'server'
                    ? t('Loaded from the published standard endpoint.')
                    : t('Built-in mirror: the standard endpoint is unreachable.')
                }
              >
                {t('Tone Standard')} v{standard.version}
              </Badge>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Tone Name')}</TableHead>
                  <TableHead>{t('Tone')}</TableHead>
                  <TableHead>{t('Introduction')}</TableHead>
                  <TableHead>{t('Tone Prompt')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Sort Order')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>{tableBody}</TableBody>
            </Table>

            <div className='flex flex-wrap items-center justify-between gap-2'>
              <p className='text-muted-foreground text-sm'>
                {t('Total {{count}} tones', { count: total })}
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

            <ToneEndpointCard standard={standard} />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ToneFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        tone={editing}
        onSubmit={submitForm}
        submitting={createMutation.isPending || updateMutation.isPending}
        limits={standard.limits}
        categories={standard.categories}
        tones={standard.tones}
      />

      <ToneImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImported={() => void refresh()}
      />

      <ToneSampleDialog
        open={sampleTarget !== null}
        tone={sampleTarget}
        examplePair={standard.examplePair}
        onOpenChange={(open) => {
          if (!open) setSampleTarget(null)
        }}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Tone')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Delete "{{name}}"? This cannot be undone.', {
                name: deleteTarget?.name ?? '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
              }}
            >
              {t('Delete Tone')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
