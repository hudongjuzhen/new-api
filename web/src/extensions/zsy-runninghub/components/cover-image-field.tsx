/*
Copyright (C) 2023-2026 QuantumNous

RunningHub app cover picker: uploads an image (or accepts a pasted URL) and
reports the stored URL to the app form.

The picked file goes to the host's own image store through /api/upload/image —
the same endpoint the playground uses — because a cover is served straight from
the gateway (`/uploads/...`) or from OSS, never from RunningHub.
*/

import { Image as ImageIcon, Loader2, Trash2, Upload } from 'lucide-react'
import { useRef, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { IMAGE_UPLOAD } from '@/features/playground/constants'

/** Mirrors the backend's image cap (controller.MaxImageUploadBytes). The
 * upload endpoint enforces it; this check only fails fast with better copy. */
const MAX_COVER_IMAGE_BYTES = 10 * 1024 * 1024

export function CoverImageField(props: {
  /** Stored cover URL; '' means the app has no cover yet. */
  value: string
  onChange: (coverUrl: string) => void
  /** Uploads one picked file and resolves with its stored URL. */
  uploadImage: (file: File) => Promise<string>
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)

  const busy = uploading || !!props.disabled

  const handleFilePicked = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    // Reset so picking the same file again still fires onChange.
    event.target.value = ''
    if (!file) return

    if (!file.type.startsWith('image/')) {
      toast.error(t('Please select image files'))
      return
    }
    if (file.size > MAX_COVER_IMAGE_BYTES) {
      toast.error(t('Cover image must be 10MB or smaller'))
      return
    }

    setUploading(true)
    try {
      props.onChange(await props.uploadImage(file))
      toast.success(t('Cover image uploaded'))
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Image upload failed')
      )
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className='space-y-1.5'>
      <Label htmlFor='rh-app-cover-url'>{t('Cover Image')}</Label>
      <div className='flex gap-3'>
        <div className='bg-muted/40 flex aspect-video w-40 shrink-0 items-center justify-center overflow-hidden rounded-md border'>
          {props.value ? (
            <img
              src={props.value}
              alt={t('Cover Image')}
              className='size-full object-cover'
            />
          ) : (
            <div className='text-muted-foreground flex flex-col items-center gap-1'>
              <ImageIcon aria-hidden='true' className='size-5' />
              <span className='text-xs'>{t('No cover image')}</span>
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
            aria-label={t('Upload image')}
            onChange={handleFilePicked}
          />
          <div className='flex items-center gap-2'>
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
            {props.value && (
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
            )}
          </div>
          <Input
            id='rh-app-cover-url'
            value={props.value}
            placeholder={t('Paste an image URL')}
            onChange={(e) => props.onChange(e.target.value)}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Upload an image or paste an image URL')}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t('PNG / JPG / GIF / WebP / BMP, up to 10MB')}
          </p>
        </div>
      </div>
    </div>
  )
}
