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
import { Image as ImageIcon, Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'

import type { VoiceView } from '../api'
import { formatVoiceAudioSize } from '../lib/voice-audio'
import {
  AGE_RANGE_OPTIONS,
  GENDER_OPTIONS,
  voiceLanguageLabel,
  voiceOptionLabelKey,
} from '../lib/voice-fields'

interface VoiceTableRowProps {
  voice: VoiceView
  onEdit: (voice: VoiceView) => void
  onDelete: (voice: VoiceView) => void
}

/** One plaza row: identity, attributes, introduction, audio, shelf state, actions. */
export function VoiceTableRow(props: VoiceTableRowProps) {
  const { t, i18n } = useTranslation()
  const voice = props.voice

  const genderLabelKey = voiceOptionLabelKey(GENDER_OPTIONS, voice.gender)
  const ageLabelKey = voiceOptionLabelKey(AGE_RANGE_OPTIONS, voice.ageRange)

  return (
    <TableRow>
      <TableCell>
        <div className='flex items-center gap-3'>
          {voice.avatarUrl ? (
            <img
              src={voice.avatarUrl}
              alt=''
              loading='lazy'
              className='size-9 shrink-0 rounded-full border object-cover'
            />
          ) : (
            <div
              className='text-muted-foreground flex size-9 shrink-0 items-center justify-center rounded-full border'
              aria-hidden='true'
            >
              <ImageIcon className='size-4' />
            </div>
          )}
          <div className='min-w-0'>
            <div className='font-medium'>{voice.name}</div>
            <code className='text-muted-foreground font-mono text-xs'>
              {voice.voiceType}
            </code>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <div className='flex flex-wrap items-center gap-1'>
          {genderLabelKey ? (
            <Badge variant='outline'>{t(genderLabelKey)}</Badge>
          ) : null}
          {ageLabelKey ? (
            <Badge variant='outline'>{t(ageLabelKey)}</Badge>
          ) : null}
          {voice.language ? (
            <Badge variant='outline'>
              {voiceLanguageLabel(voice.language, i18n.language)}
            </Badge>
          ) : null}
          {genderLabelKey === null &&
          ageLabelKey === null &&
          voice.language === '' ? (
            <span className='text-muted-foreground text-sm'>
              {t('Not specified')}
            </span>
          ) : null}
        </div>
        {voice.scenes.length > 0 ? (
          <div className='mt-1 flex flex-wrap gap-1'>
            {voice.scenes.map((scene) => (
              <Badge key={scene} variant='secondary' className='font-normal'>
                {scene}
              </Badge>
            ))}
          </div>
        ) : null}
      </TableCell>
      <TableCell className='max-w-64 text-sm'>
        <span className='line-clamp-2'>{voice.description || '-'}</span>
      </TableCell>
      <TableCell>
        {voice.audioUrl ? (
          <div className='space-y-1'>
            <audio
              controls
              preload='none'
              src={voice.audioUrl}
              className='h-8 w-48'
              aria-label={t('Audio Sample')}
            />
            {voice.audioSize > 0 ? (
              <div className='text-muted-foreground text-xs'>
                {formatVoiceAudioSize(voice.audioSize)}
              </div>
            ) : null}
          </div>
        ) : (
          <span className='text-muted-foreground text-sm'>
            {t('No audio sample')}
          </span>
        )}
      </TableCell>
      <TableCell>
        <Badge variant={voice.enabled ? 'default' : 'secondary'}>
          {voice.enabled ? t('On Shelf') : t('Off Shelf')}
        </Badge>
      </TableCell>
      <TableCell className='text-sm'>{voice.sortOrder}</TableCell>
      <TableCell className='text-right'>
        <div className='flex justify-end gap-1'>
          <Button
            variant='ghost'
            size='icon'
            onClick={() => props.onEdit(voice)}
            aria-label={t('Edit')}
          >
            <Pencil className='size-4' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            onClick={() => props.onDelete(voice)}
            aria-label={t('Delete')}
          >
            <Trash2 className='size-4' />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
