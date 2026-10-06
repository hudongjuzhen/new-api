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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
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

import type { ToneUpsertDTO, ToneView } from '../api'
import {
  parseSceneList,
  toneOptionText,
  type ToneLimits,
  type ToneStandardOption,
} from '../lib/tone-fields'
import {
  createToneFormSchema,
  EMPTY_TONE_FORM,
  toneToFormValues,
  type ToneFormValues,
} from '../lib/tone-form-schema'

interface ToneFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The row being edited, or null when creating a new tone. */
  tone: ToneView | null
  onSubmit: (dto: ToneUpsertDTO) => void
  submitting: boolean
  /** Caps of the live standard; the mirror's when it could not be loaded. */
  limits: ToneLimits
  /** Vocabulary of the live standard, so options match what the API accepts. */
  categories: ToneStandardOption[]
  tones: ToneStandardOption[]
}

const TONE_FORM_ID = 'tone-plaza-form'

/**
 * Create / edit dialog. Nothing here is uploaded: a tone is text, so the whole
 * form is one request.
 *
 * The schema is rebuilt from `limits` because the caps come from the published
 * standard rather than from this file — the dialog must refuse what the API
 * would refuse *as it is deployed now*, not as it was when this component was
 * written.
 */
export function ToneFormDialog(props: ToneFormDialogProps) {
  const { t } = useTranslation()
  const isEdit = props.tone !== null

  const schema = useMemo(
    () => createToneFormSchema(props.limits),
    [props.limits]
  )

  const form = useForm<ToneFormValues>({
    resolver: zodResolver(schema),
    defaultValues: EMPTY_TONE_FORM,
  })

  // The form owns its state once mounted, so each target gets a fresh instance.
  useEffect(() => {
    if (!props.open) return
    form.reset(props.tone ? toneToFormValues(props.tone) : EMPTY_TONE_FORM)
  }, [props.open, props.tone, form, schema])

  const handleSubmit = (values: ToneFormValues) => {
    props.onSubmit({
      name: values.name.trim(),
      description: values.description.trim(),
      prompt: values.prompt.trim(),
      category: values.category,
      tone: values.tone,
      // The tag is stored lower-case, exactly like the backend normalizes it.
      language: values.language.trim().toLowerCase(),
      scenes: parseSceneList(values.scenes),
      sampleInput: values.sampleInput.trim(),
      sampleOutput: values.sampleOutput.trim(),
      enabled: values.enabled,
      sortOrder: values.sortOrder,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={isEdit ? t('Edit Tone') : t('New Tone')}
      description={t(
        'The tone prompt is the instruction downstream models follow.'
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
          <Button type='submit' form={TONE_FORM_ID} disabled={props.submitting}>
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
          id={TONE_FORM_ID}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Tone Name')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('Warm and composed')} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='category'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Category')}</FormLabel>
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
                      {props.categories.map((option) => (
                        <SelectItem
                          key={option.value}
                          value={option.value}
                          title={option.desc || undefined}
                        >
                          {toneOptionText(option, t)}
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
              name='tone'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Tone')}</FormLabel>
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
                      {props.tones.map((option) => (
                        <SelectItem
                          key={option.value}
                          value={option.value}
                          title={option.desc || undefined}
                        >
                          {toneOptionText(option, t)}
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
            name='description'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Introduction')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('What this tone reads like')}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='prompt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Tone Prompt')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={8}
                    placeholder={t('The instruction downstream models follow')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Paste the wording a model should follow. This is the field consumers actually use.'
                  )}
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
              name='scenes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Suitable Scenes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Newsletter, product copy, documentation')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Separate scenes with a comma.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='sampleInput'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sample Input')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={4}
                      placeholder={t('A sentence written plainly')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='sampleOutput'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sample Output')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={4}
                      placeholder={t('The same sentence in this tone')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Optional before / after pair showing what this tone does.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
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
                    {t('Off-shelf tones stay hidden from the public list.')}
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
