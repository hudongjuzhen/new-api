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
import { Loader2 } from 'lucide-react'
import { useEffect } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import type { AvatarUpsertDTO, AvatarView } from '../api'
import {
  AGE_RANGE_OPTIONS,
  AVATAR_IMAGE_FIELDS,
  avatarFormSchema,
  avatarToFormValues,
  EMPTY_AVATAR_FORM,
  GENDER_OPTIONS,
  parseSceneList,
  RACE_OPTIONS,
  type AvatarFormValues,
} from '../lib/avatar-fields'
import { AvatarImageField } from './avatar-image-field'

interface AvatarFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The row being edited, or null when creating a new persona. */
  avatar: AvatarView | null
  onSubmit: (dto: AvatarUpsertDTO) => void
  submitting: boolean
}

const AVATAR_FORM_ID = 'avatar-plaza-form'

/**
 * Create / edit dialog. The picture is uploaded on pick (so the admin sees the
 * real URL and the preview) and only the resulting URL is submitted with the
 * form; the voice sample is never part of the payload — it follows the voice
 * named by `voiceId`.
 */
export function AvatarFormDialog(props: AvatarFormDialogProps) {
  const { t } = useTranslation()
  const isEdit = props.avatar !== null

  const form = useForm<AvatarFormValues>({
    resolver: zodResolver(avatarFormSchema),
    defaultValues: EMPTY_AVATAR_FORM,
  })

  // The form owns its state once mounted, so each target gets a fresh instance.
  useEffect(() => {
    if (!props.open) return
    form.reset(
      props.avatar ? avatarToFormValues(props.avatar) : EMPTY_AVATAR_FORM
    )
  }, [props.open, props.avatar, form])

  const voiceId = useWatch({ control: form.control, name: 'voiceId' })

  const handleSubmit = (values: AvatarFormValues) => {
    props.onSubmit({
      name: values.name.trim(),
      description: values.description.trim(),
      imageUrl: values.imageUrl.trim(),
      fullBodyUrl: values.fullBodyUrl.trim(),
      fourViewUrl: values.fourViewUrl.trim(),
      expressionUrl: values.expressionUrl.trim(),
      gender: values.gender,
      ageRange: values.ageRange,
      race: values.race,
      scenes: parseSceneList(values.scenes),
      voiceId: values.voiceId.trim(),
      enabled: values.enabled,
      sortOrder: values.sortOrder,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={isEdit ? t('Edit Avatar') : t('New Avatar')}
      description={t(
        'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.'
      )}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.submitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form={AVATAR_FORM_ID}
            disabled={props.submitting}
          >
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
          id={AVATAR_FORM_ID}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Avatar Name')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('Customer service girl')} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            {AVATAR_IMAGE_FIELDS.map((image) => (
              <FormField
                key={image.name}
                control={form.control}
                name={image.name}
                render={({ field }) => (
                  <AvatarImageField
                    id={`avatar-plaza-${image.name}`}
                    labelKey={image.labelKey}
                    aspectClassName={image.aspectClassName}
                    value={field.value}
                    onChange={field.onChange}
                    disabled={props.submitting}
                  />
                )}
              />
            ))}
          </div>

          <FormField
            control={form.control}
            name='description'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Introduction')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('What this persona looks like and does')}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-3'>
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

            <FormField
              control={form.control}
              name='race'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Race')}</FormLabel>
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
                      {RACE_OPTIONS.map((option) => (
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

          <FormField
            control={form.control}
            name='voiceId'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Voice ID')}</FormLabel>
                <FormControl>
                  <Input
                    className='font-mono'
                    placeholder='zh_female_vv_uranus_bigtts'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {isEdit && props.avatar && voiceId.trim() !== '' ? (
            <div className='space-y-2 rounded-md border p-3'>
              {props.avatar.voiceSampleUrl ? (
                <>
                  <p className='text-sm font-medium'>
                    {t('Voice Sample')}
                    {props.avatar.voiceName
                      ? ` · ${props.avatar.voiceName}`
                      : ''}
                  </p>
                  <audio
                    controls
                    preload='none'
                    src={props.avatar.voiceSampleUrl}
                    className='w-full'
                    aria-label={t('Voice Sample')}
                  />
                </>
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'This voice id is not in the Voice Plaza, so no sample audio is available.'
                  )}
                </p>
              )}
            </div>
          ) : null}

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
                    {t('Off-shelf personas stay hidden from the public list.')}
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
