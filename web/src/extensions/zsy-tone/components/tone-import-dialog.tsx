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
import { Loader2, Upload } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  importTonesCsv,
  type ToneImportMode,
  type ToneImportResult,
} from '../api'

interface ToneImportDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called once an import finished, so the page can refresh its table. */
  onImported: () => void
}

const IMPORT_MODES: ToneImportMode[] = ['upsert', 'create']

const IMPORT_MODE_LABEL: Record<ToneImportMode, string> = {
  upsert: 'Create and update by name',
  create: 'Create only',
}

/**
 * CSV import. The result panel reports what happened per row, because a partly
 * applied file is the normal outcome of a hand-edited spreadsheet: the operator
 * fixes the listed rows and imports the file again.
 */
export function ToneImportDialog(props: ToneImportDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<ToneImportMode>('upsert')
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<ToneImportResult | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // Each opening starts a fresh import, so the reset happens when the dialog
  // closes rather than in an effect that would re-render on open.
  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setFile(null)
      setResult(null)
      setSubmitting(false)
    }
    props.onOpenChange(open)
  }

  const runImport = async () => {
    if (!file) {
      toast.error(t('Choose a CSV file'))
      return
    }
    setSubmitting(true)
    try {
      const imported = await importTonesCsv(file, mode)
      setResult(imported)
      props.onImported()
      toast.success(
        t(
          'Imported: {{created}} created, {{updated}} updated, {{failed}} failed',
          {
            created: imported.created,
            updated: imported.updated,
            failed: imported.failed,
          }
        )
      )
    } catch (error) {
      toast.error((error as Error)?.message || t('Import failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Import Tones')}
      description={t(
        'Upload a CSV exported from this page. name and prompt are required.'
      )}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={submitting}
          >
            {t('Close')}
          </Button>
          <Button type='button' onClick={runImport} disabled={submitting}>
            {submitting ? (
              <Loader2 className='mr-2 size-4 animate-spin' />
            ) : (
              <Upload className='mr-2 size-4' />
            )}
            {submitting ? t('Importing...') : t('Import')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label htmlFor='tone-plaza-import-file'>{t('CSV File')}</Label>
        <Input
          id='tone-plaza-import-file'
          type='file'
          accept='.csv,text/csv'
          disabled={submitting}
          onChange={(event) => {
            setFile(event.target.files?.[0] ?? null)
            setResult(null)
          }}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.'
          )}
        </p>
      </div>

      <div className='space-y-2'>
        <Label htmlFor='tone-plaza-import-mode'>{t('Import Mode')}</Label>
        <Select
          value={mode}
          onValueChange={(value) =>
            setMode((value || 'upsert') as ToneImportMode)
          }
        >
          <SelectTrigger id='tone-plaza-import-mode' className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {IMPORT_MODES.map((item) => (
              <SelectItem key={item} value={item}>
                {t(IMPORT_MODE_LABEL[item])}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {result ? (
        <div className='space-y-2 rounded-md border p-3'>
          <p className='text-sm font-medium'>{t('Import Result')}</p>
          <p className='flex flex-wrap gap-x-3 text-sm'>
            <span>{t('Created: {{count}}', { count: result.created })}</span>
            <span>{t('Updated: {{count}}', { count: result.updated })}</span>
            <span>{t('Failed: {{count}}', { count: result.failed })}</span>
          </p>

          {result.warnings?.length ? (
            <ul className='text-muted-foreground list-inside list-disc text-xs'>
              {result.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          ) : null}

          {result.errors.length > 0 ? (
            <ul
              className='max-h-48 space-y-1 overflow-y-auto text-xs'
              aria-label={t('Failed rows')}
            >
              {result.errors.map((rowError) => (
                <li key={`${rowError.row}-${rowError.message}`}>
                  <span className='font-mono'>
                    {t('Row {{row}}', { row: rowError.row })}
                  </span>
                  {rowError.name ? ` · ${rowError.name}` : ''} —{' '}
                  {rowError.message}
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
    </Dialog>
  )
}
