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
import { Image as ImageIcon, Loader2, Trash2, Upload } from 'lucide-react'
import { useRef, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { uploadPlaygroundImage } from '@/features/playground/api'
import { IMAGE_UPLOAD } from '@/features/playground/constants'

/** Mirrors the backend's image cap (controller.MaxImageUploadBytes). The upload
 * endpoint enforces it; this check only fails fast with better copy. */
const MAX_PORTRAIT_BYTES = 10 * 1024 * 1024

interface AvatarImageFieldProps {
  /** Field id; unique per picture so the label links to this field's URL input. */
  id: string
  /** i18n key of the picture's label (封面图 / 全身照 / 四视图 / 表情图). */
  labelKey: string
  /** Tailwind aspect ratio of the preview box, matching the picture's shape. */
  aspectClassName: string
  /** Stored picture URL; '' means this picture is not set yet. */
  value: string
  onChange: (imageUrl: string) => void
  disabled?: boolean
}

/**
 * The picture picker of the form: one preview beside an upload button and a
 * paste-a-URL input. The persona form renders four of them — the cover (封面图)
 * and the three reference pictures (全身照 / 四视图 / 表情图).
 *
 * The picked file goes to the host's own image store through /api/upload/image —
 * the same endpoint the playground uses — so the picture is served straight from
 * the gateway (`/uploads/images/...`) or from OSS, never from a third party.
 *
 * It is wired with FormItem/FormControl so react-hook-form's FormMessage renders
 * the field error directly under the picker.
 */
export function AvatarImageField(props: AvatarImageFieldProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)

  const busy = uploading || !!props.disabled
  const label = t(props.labelKey)

  const handleFilePicked = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    // Reset so picking the same file again still fires the handler.
    event.target.value = ''
    if (!file) return

    if (!file.type.startsWith('image/')) {
      toast.error(t('Please select image files'))
      return
    }
    if (file.size > MAX_PORTRAIT_BYTES) {
      toast.error(t('Image must be 10MB or smaller'))
      return
    }

    setUploading(true)
    try {
      props.onChange(await uploadPlaygroundImage(file))
      toast.success(t('Image uploaded'))
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Upload failed')
      )
    } finally {
      setUploading(false)
    }
  }

  return (
    <FormItem>
      <FormLabel htmlFor={props.id}>{label}</FormLabel>
      <div className='flex gap-3'>
        <div
          className={`bg-muted/40 flex w-24 shrink-0 items-center justify-center overflow-hidden rounded-md border ${props.aspectClassName}`}
        >
          {props.value ? (
            <img
              src={props.value}
              alt={t('Image preview')}
              className='size-full object-cover'
            />
          ) : (
            <div className='text-muted-foreground flex flex-col items-center gap-1'>
              <ImageIcon aria-hidden='true' className='size-5' />
              <span className='text-xs'>{t('No image')}</span>
            </div>
          )}
        </div>

        <div className='flex min-w-0 flex-1 flex-col gap-2'>
          <input
            ref={fileInputRef}
            type='file'
            accept={IMAGE_UPLOAD.ACCEPT}
            className='hidden'
            disabled={busy}
            // Four pickers share this page, so each file input is named after
            // the picture it fills.
            aria-label={`${t('Upload image')} · ${label}`}
            onChange={handleFilePicked}
          />
          <div className='flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={busy}
              onClick={() => fileInputRef.current?.click()}
            >
              {uploading ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <Upload className='size-4' />
              )}
              {uploading ? t('Uploading...') : t('Upload image')}
            </Button>
            {props.value ? (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                disabled={busy}
                onClick={() => props.onChange('')}
              >
                <Trash2 className='size-4' />
                {t('Remove image')}
              </Button>
            ) : null}
          </div>
          <FormControl>
            <Input
              id={props.id}
              value={props.value}
              placeholder={t('https://… or /uploads/images/…')}
              disabled={busy}
              onChange={(event) => props.onChange(event.target.value)}
            />
          </FormControl>
          <p className='text-muted-foreground text-xs'>
            {t('PNG / JPG / GIF / WebP / BMP, up to 10MB')}
          </p>
        </div>
      </div>
      <FormMessage />
    </FormItem>
  )
}
