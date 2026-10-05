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
import { zodResolver } from '@hookform/resolvers/zod'
import { Image as ImageIcon, Loader2, Upload, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { uploadVoiceAudio, type VoiceUpsertDTO, type VoiceView } from '../api'
import {
  formatVoiceAudioSize,
  validateVoiceAudioFile,
  VOICE_AUDIO_ACCEPT,
} from '../lib/voice-audio'
import {
  AGE_RANGE_OPTIONS,
  GENDER_OPTIONS,
  parseSceneList,
} from '../lib/voice-fields'
import {
  EMPTY_VOICE_FORM,
  voiceFormSchema,
  voiceToFormValues,
  type VoiceFormValues,
} from '../lib/voice-form-schema'

interface VoiceFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The row being edited, or null when creating a new voice. */
  voice: VoiceView | null
  onSubmit: (dto: VoiceUpsertDTO) => void
  submitting: boolean
}

const VOICE_FORM_ID = 'voice-plaza-form'

/**
 * Create / edit dialog. The sample audio is uploaded on pick (so the admin sees
 * the real URL and can preview it) and only the resulting URL is submitted with
 * the form.
 */
export function VoiceFormDialog(props: VoiceFormDialogProps) {
  const { t } = useTranslation()
  const isEdit = props.voice !== null
  const [uploading, setUploading] = useState(false)

  const form = useForm<VoiceFormValues>({
    resolver: zodResolver(voiceFormSchema),
    defaultValues: EMPTY_VOICE_FORM,
  })

  // The form owns its state once mounted, so each target gets a fresh instance.
  useEffect(() => {
    if (!props.open) return
    form.reset(props.voice ? voiceToFormValues(props.voice) : EMPTY_VOICE_FORM)
  }, [props.open, props.voice, form])

  const audioUrl = useWatch({ control: form.control, name: 'audioUrl' })
  const audioName = useWatch({ control: form.control, name: 'audioName' })
  const audioSize = useWatch({ control: form.control, name: 'audioSize' })
  const busy = props.submitting || uploading

  const pickAudio = async (file: File | undefined) => {
    if (!file) return
    const failure = validateVoiceAudioFile(file)
    if (failure) {
      toast.error(t(failure))
      return
    }
    setUploading(true)
    try {
      const uploaded = await uploadVoiceAudio(file)
      form.setValue('audioUrl', uploaded.url)
      form.setValue('audioName', uploaded.originalName || uploaded.filename)
      form.setValue('audioSize', uploaded.size)
      toast.success(t('Audio uploaded'))
    } catch (error) {
      toast.error((error as Error)?.message || t('Upload failed'))
    } finally {
      setUploading(false)
    }
  }

  const handleSubmit = (values: VoiceFormValues) => {
    props.onSubmit({
      name: values.name.trim(),
      description: values.description.trim(),
      voiceType: values.voiceType.trim(),
      gender: values.gender,
      ageRange: values.ageRange,
      // The tag is stored lower-case, exactly like the backend normalizes it.
      language: values.language.trim().toLowerCase(),
      scenes: parseSceneList(values.scenes),
      avatarUrl: values.avatarUrl.trim(),
      audioUrl: values.audioUrl,
      audioName: values.audioName,
      audioSize: values.audioSize,
      enabled: values.enabled,
      sortOrder: values.sortOrder,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={isEdit ? t('Edit Voice') : t('New Voice')}
      description={t(
        'The voice_type is the value a TTS request sends to the upstream provider.'
      )}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={busy}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={VOICE_FORM_ID} disabled={busy}>
            {props.submitting ? (
              <Loader2 className='mr-2 size-4 animate-spin' />
            ) : null}
            {props.submitting ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={VOICE_FORM_ID}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Voice Name')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('Sweet Female Voice')} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='voiceType'
            render={({ field }) => (
              <FormItem>
                <FormLabel>voice_type</FormLabel>
                <FormControl>
                  <Input
                    className='font-mono'
                    placeholder='zh_female_shuangkuaisisi_moon_bigtts'
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='description'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Introduction')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('What this voice sounds like')}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='gender'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Gender')}</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={(value) => field.onChange(value ?? '')}
                  >
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {GENDER_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.labelKey)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='ageRange'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Age Range')}</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={(value) => field.onChange(value ?? '')}
                  >
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {AGE_RANGE_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.labelKey)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='scenes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Suitable Scenes')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('Customer service, audiobook, live stream')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Separate scenes with a comma; at most 8 scenes.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='language'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Language')}</FormLabel>
                  <FormControl>
                    <Input
                      className='font-mono'
                      placeholder='zh / en / pt-br'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Short lower-case language tag (zh, en, ja…).')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='avatarUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Avatar')}</FormLabel>
                  <div className='flex items-center gap-2'>
                    {field.value ? (
                      <img
                        src={field.value}
                        alt={t('Avatar preview')}
                        loading='lazy'
                        className='size-10 shrink-0 rounded-full border object-cover'
                      />
                    ) : (
                      <div className='text-muted-foreground flex size-10 shrink-0 items-center justify-center rounded-full border'>
                        <ImageIcon className='size-4' aria-hidden='true' />
                      </div>
                    )}
                    <FormControl>
                      <Input
                        placeholder={t('https://… or /uploads/voices/…')}
                        {...field}
                      />
                    </FormControl>
                  </div>
                  <FormDescription>
                    {t('Portrait shown next to the voice; optional.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='voice-plaza-audio'>{t('Audio Sample')}</Label>
            <div className='flex flex-wrap items-center gap-2'>
              <Input
                id='voice-plaza-audio'
                type='file'
                accept={VOICE_AUDIO_ACCEPT}
                disabled={busy}
                onChange={(event) => {
                  const file = event.target.files?.[0]
                  // Allow re-picking the same file after a failed upload.
                  event.target.value = ''
                  void pickAudio(file)
                }}
                className='max-w-full'
              />
              {uploading ? (
                <span className='text-muted-foreground flex items-center gap-2 text-sm'>
                  <Loader2 className='size-4 animate-spin' />
                  {t('Uploading...')}
                </span>
              ) : null}
              {audioUrl && !uploading ? (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => {
                    form.setValue('audioUrl', '')
                    form.setValue('audioName', '')
                    form.setValue('audioSize', 0)
                  }}
                >
                  <X className='size-4' />
                  {t('Remove Audio')}
                </Button>
              ) : null}
            </div>
            <p className='text-muted-foreground text-sm'>
              {t(
                'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB'
              )}
            </p>
            {audioUrl ? (
              <div className='space-y-2 rounded-md border p-3'>
                <audio
                  controls
                  preload='none'
                  src={audioUrl}
                  className='w-full'
                  aria-label={t('Audio Sample')}
                />
                <p className='text-muted-foreground truncate text-xs'>
                  {audioName || t('No audio sample')}
                  {audioSize > 0 ? ` · ${formatVoiceAudioSize(audioSize)}` : ''}
                </p>
              </div>
            ) : (
              <p className='text-muted-foreground flex items-center gap-2 text-sm'>
                <Upload className='size-4' />
                {t('No audio sample')}
              </p>
            )}
            <p className='text-muted-foreground text-xs'>
              {t('Sample audio is optional; a voice without one still works.')}
            </p>
          </div>

          <div className='flex flex-wrap items-end gap-6'>
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('On Shelf')}</FormLabel>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={(checked) =>
                        field.onChange(checked as boolean)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Off-shelf voices stay hidden from the public list.')}
                  </FormDescription>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='sortOrder'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sort Order')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      className='w-32'
                      value={String(field.value)}
                      onChange={(event) => {
                        const next = event.target.valueAsNumber
                        field.onChange(
                          Number.isNaN(next) ? 0 : Math.trunc(next)
                        )
                      }}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Smaller numbers come first.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </form>
      </Form>
    </Dialog>
  )
}
