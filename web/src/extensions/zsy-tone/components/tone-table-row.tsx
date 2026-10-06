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
import { Columns2, Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'

import type { ToneView } from '../api'
import { toneLanguageLabel, toneValueText, type ToneStandardOption } from '../lib/tone-fields'
import { hasToneSample } from '../lib/tone-form-schema'

interface ToneTableRowProps {
  tone: ToneView
  /** Vocabulary of the live standard, so a translated label is always used. */
  categories: ToneStandardOption[]
  tones: ToneStandardOption[]
  onEdit: (tone: ToneView) => void
  onDelete: (tone: ToneView) => void
  onShowSample: (tone: ToneView) => void
}

/**
 * One plaza row: identity, vocabulary, introduction, prompt, shelf state, actions.
 *
 * The sample button is absent — not disabled — when a row carries no pair at all,
 * which is the same rule the voice plaza applies to a voice with no audio: a
 * control that can never do anything reads as broken.
 */
export function ToneTableRow(props: ToneTableRowProps) {
  const { t, i18n } = useTranslation()
  const tone = props.tone

  const categoryText = toneValueText(props.categories, tone.category, t)
  const toneText = toneValueText(props.tones, tone.tone, t)
  const hasVocabulary = categoryText !== '' || toneText !== '' || tone.language !== ''

  return (
    <TableRow>
      <TableCell>
        <div className='min-w-0 space-y-1'>
          <div className='font-medium'>{tone.name}</div>
          {categoryText ? (
            <Badge variant='outline'>{categoryText}</Badge>
          ) : (
            <span className='text-muted-foreground text-xs'>
              {t('Uncategorized')}
            </span>
          )}
        </div>
      </TableCell>
      <TableCell>
        <div className='flex flex-wrap items-center gap-1'>
          {toneText ? <Badge variant='outline'>{toneText}</Badge> : null}
          {tone.language ? (
            <Badge variant='outline'>
              {toneLanguageLabel(tone.language, i18n.language)}
            </Badge>
          ) : null}
          {!hasVocabulary ? (
            <span className='text-muted-foreground text-sm'>
              {t('Not specified')}
            </span>
          ) : null}
        </div>
        {tone.scenes.length > 0 ? (
          <div className='mt-1 flex flex-wrap gap-1'>
            {tone.scenes.map((scene) => (
              <Badge key={scene} variant='secondary' className='font-normal'>
                {scene}
              </Badge>
            ))}
          </div>
        ) : null}
      </TableCell>
      <TableCell className='max-w-56 text-sm'>
        <span className='line-clamp-2'>{tone.description || '-'}</span>
      </TableCell>
      <TableCell className='max-w-72 text-sm'>
        <span className='line-clamp-2'>{tone.prompt || '-'}</span>
      </TableCell>
      <TableCell>
        <Badge variant={tone.enabled ? 'default' : 'secondary'}>
          {tone.enabled ? t('On Shelf') : t('Off Shelf')}
        </Badge>
      </TableCell>
      <TableCell className='text-sm'>{tone.sortOrder}</TableCell>
      <TableCell className='text-right'>
        <div className='flex justify-end gap-1'>
          {hasToneSample(tone) ? (
            <Button
              variant='ghost'
              size='icon'
              onClick={() => props.onShowSample(tone)}
              aria-label={t('Sample')}
              title={t('Sample')}
            >
              <Columns2 className='size-4' />
            </Button>
          ) : null}
          <Button
            variant='ghost'
            size='icon'
            onClick={() => props.onEdit(tone)}
            aria-label={t('Edit')}
          >
            <Pencil className='size-4' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            onClick={() => props.onDelete(tone)}
            aria-label={t('Delete')}
          >
            <Trash2 className='size-4' />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
