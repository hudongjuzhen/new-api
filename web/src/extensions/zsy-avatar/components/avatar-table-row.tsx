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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'

import type { AvatarView } from '../api'
import {
  AGE_RANGE_OPTIONS,
  AVATAR_IMAGE_FIELDS,
  avatarOptionLabelKey,
  GENDER_OPTIONS,
  RACE_OPTIONS,
  type AvatarImageFieldName,
} from '../lib/avatar-fields'

interface AvatarTableRowProps {
  avatar: AvatarView
  onEdit: (avatar: AvatarView) => void
  onDelete: (avatar: AvatarView) => void
}

/**
 * One plaza row: the persona's pictures and name, demographic attributes,
 * introduction, the linked voice with its sample, shelf state and actions.
 */
export function AvatarTableRow(props: AvatarTableRowProps) {
  const { t } = useTranslation()
  const avatar = props.avatar
  // A picture hosted elsewhere can be unreachable (the provider's CDN may be
  // blocked for the visitor): fall back to the placeholder instead of leaving a
  // broken image in the row. Tracked per picture, so one dead reference photo
  // never hides the cover.
  const [brokenImages, setBrokenImages] = useState<
    Partial<Record<AvatarImageFieldName, boolean>>
  >({})

  const pictureOf = (name: AvatarImageFieldName) =>
    brokenImages[name] ? '' : avatar[name]

  // The three reference pictures (全身照 / 四视图 / 表情图) this persona carries,
  // shown beside the cover so an operator sees at a glance which rows still miss
  // one without opening every form.
  const referenceImages = AVATAR_IMAGE_FIELDS.filter(
    (image) => image.name !== 'imageUrl' && pictureOf(image.name) !== ''
  )

  const genderLabelKey = avatarOptionLabelKey(GENDER_OPTIONS, avatar.gender)
  const ageLabelKey = avatarOptionLabelKey(AGE_RANGE_OPTIONS, avatar.ageRange)
  const raceLabelKey = avatarOptionLabelKey(RACE_OPTIONS, avatar.race)
  const hasAttributes =
    genderLabelKey !== null || ageLabelKey !== null || raceLabelKey !== null
  const coverUrl = pictureOf('imageUrl')

  return (
    <TableRow>
      <TableCell>
        <div className='flex items-center gap-3'>
          {coverUrl ? (
            <img
              src={coverUrl}
              alt=''
              loading='lazy'
              className='h-12 w-9 shrink-0 rounded-md border object-cover'
              onError={() =>
                setBrokenImages((previous) => ({
                  ...previous,
                  imageUrl: true,
                }))
              }
            />
          ) : (
            <div
              className='text-muted-foreground flex h-12 w-9 shrink-0 items-center justify-center rounded-md border'
              aria-hidden='true'
            >
              <ImageIcon className='size-4' />
            </div>
          )}
          <div className='min-w-0'>
            <div className='font-medium'>{avatar.name}</div>
            <code className='text-muted-foreground font-mono text-xs'>
              {avatar.voiceId || t('No voice linked')}
            </code>
            {referenceImages.length > 0 ? (
              <div className='mt-1 flex gap-1'>
                {referenceImages.map((image) => (
                  <img
                    key={image.name}
                    src={pictureOf(image.name)}
                    alt={t(image.labelKey)}
                    title={t(image.labelKey)}
                    loading='lazy'
                    className='size-8 rounded border object-cover'
                    onError={() =>
                      setBrokenImages((previous) => ({
                        ...previous,
                        [image.name]: true,
                      }))
                    }
                  />
                ))}
              </div>
            ) : null}
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
          {raceLabelKey ? (
            <Badge variant='outline'>{t(raceLabelKey)}</Badge>
          ) : null}
          {hasAttributes ? null : (
            <span className='text-muted-foreground text-sm'>
              {t('Not specified')}
            </span>
          )}
        </div>
        {avatar.scenes.length > 0 ? (
          <div className='mt-1 flex flex-wrap gap-1'>
            {avatar.scenes.map((scene) => (
              <Badge key={scene} variant='secondary' className='font-normal'>
                {scene}
              </Badge>
            ))}
          </div>
        ) : null}
      </TableCell>
      <TableCell className='max-w-64 text-sm'>
        <span className='line-clamp-2'>{avatar.description || '-'}</span>
      </TableCell>
      <TableCell>
        {avatar.voiceSampleUrl ? (
          <div className='space-y-1'>
            <audio
              controls
              preload='none'
              src={avatar.voiceSampleUrl}
              className='h-8 w-48'
              aria-label={t('Voice Sample')}
            />
            <div className='text-muted-foreground truncate text-xs'>
              {avatar.voiceName || avatar.voiceId}
            </div>
          </div>
        ) : (
          <span className='text-muted-foreground text-sm'>
            {avatar.voiceId
              ? t('No sample audio for this voice')
              : t('No voice linked')}
          </span>
        )}
      </TableCell>
      <TableCell>
        <Badge variant={avatar.enabled ? 'default' : 'secondary'}>
          {avatar.enabled ? t('On Shelf') : t('Off Shelf')}
        </Badge>
      </TableCell>
      <TableCell className='text-sm'>{avatar.sortOrder}</TableCell>
      <TableCell className='text-right'>
        <div className='flex justify-end gap-1'>
          <Button
            variant='ghost'
            size='icon'
            onClick={() => props.onEdit(avatar)}
            aria-label={t('Edit')}
          >
            <Pencil className='size-4' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            onClick={() => props.onDelete(avatar)}
            aria-label={t('Delete')}
          >
            <Trash2 className='size-4' />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
