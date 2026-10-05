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
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { useState } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'

import { testOSSConnection } from '../api'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const createOSSSchema = (t: (key: string) => string) =>
  z
    .object({
      oss_setting: z.object({
        enabled: z.boolean(),
        endpoint: z.string(),
        bucket: z.string(),
        access_key_id: z.string(),
        access_key_secret: z.string(),
        path_prefix: z.string(),
        custom_domain: z.string(),
        use_ssl: z.boolean(),
      }),
    })
    .superRefine((data, ctx) => {
      if (!data.oss_setting.enabled) return

      const requiredFields = [
        { name: 'endpoint', value: data.oss_setting.endpoint },
        { name: 'bucket', value: data.oss_setting.bucket },
        { name: 'access_key_id', value: data.oss_setting.access_key_id },
      ] as const

      for (const field of requiredFields) {
        if (field.value.trim() !== '') continue
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['oss_setting', field.name],
          message: t('Required when Aliyun OSS is enabled'),
        })
      }
    })

type OSSFormValues = z.infer<ReturnType<typeof createOSSSchema>>

export type OSSSettingsDefaults = {
  oss_setting: {
    enabled: boolean
    endpoint: string
    bucket: string
    access_key_id: string
    access_key_secret: string
    path_prefix: string
    custom_domain: string
    use_ssl: boolean
  }
}

type OSSTestState = {
  loading: boolean
  ok: boolean | null
  message: string | null
}

export function OSSSettingsSection(props: {
  defaultValues: OSSSettingsDefaults
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const ossSchema = createOSSSchema(t)

  const [testState, setTestState] = useState<OSSTestState>({
    loading: false,
    ok: null,
    message: null,
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<OSSFormValues>({
      resolver: zodResolver(ossSchema) as Resolver<
        OSSFormValues,
        unknown,
        OSSFormValues
      >,
      defaultValues: props.defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          if (value === undefined || value === null) continue
          await updateOption.mutateAsync({
            key,
            value: typeof value === 'string' ? value.trim() : String(value),
          })
        }
      },
    })

  const enabled = form.watch('oss_setting.enabled')

  const handleTestConnection = async () => {
    setTestState({ loading: true, ok: null, message: null })
    try {
      const res = await testOSSConnection()
      if (res.success) {
        setTestState({ loading: false, ok: true, message: null })
        return
      }
      setTestState({
        loading: false,
        ok: false,
        message: res.message || t('Connection failed'),
      })
    } catch (error) {
      setTestState({
        loading: false,
        ok: false,
        message:
          error instanceof Error ? error.message : t('Connection failed'),
      })
    }
  }

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <SettingsSection title={t('Aliyun OSS Object Storage')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit} autoComplete='off'>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={updateOption.isPending || isSubmitting}
              isResetDisabled={!isDirty}
              saveLabel='Save OSS settings'
            />
            <FormDirtyIndicator isDirty={isDirty} />

            <FormField
              control={form.control}
              name='oss_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable Aliyun OSS')}</FormLabel>
                    <FormDescription>
                      {t(
                        'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={updateOption.isPending || isSubmitting}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            {enabled ? (
              <>
                <FormField
                  control={form.control}
                  name='oss_setting.endpoint'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('OSS Endpoint')}</FormLabel>
                      <FormControl>
                        <Input
                          autoComplete='off'
                          placeholder={t('oss-cn-hangzhou.aliyuncs.com')}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oss_setting.bucket'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Bucket')}</FormLabel>
                      <FormControl>
                        <Input
                          autoComplete='off'
                          placeholder={t('my-bucket')}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Name of the OSS bucket that stores uploaded files')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oss_setting.access_key_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('AccessKey ID')}</FormLabel>
                      <FormControl>
                        <Input
                          autoComplete='off'
                          placeholder={t('Enter AccessKey ID')}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Alibaba Cloud AccessKey ID used to sign uploads')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oss_setting.access_key_secret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('AccessKey Secret')}</FormLabel>
                      <FormControl>
                        <Input
                          autoComplete='off'
                          type='password'
                          placeholder={t('Enter new secret to update')}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Leave blank to keep the existing credential')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oss_setting.path_prefix'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Object Key Prefix')}</FormLabel>
                      <FormControl>
                        <Input
                          autoComplete='off'
                          placeholder={t('uploads')}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Directory prefix inside the bucket. Leave blank to write to the bucket root.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oss_setting.custom_domain'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Custom Domain')}</FormLabel>
                      <FormControl>
                        <Input
                          autoComplete='off'
                          placeholder={t('https://cdn.example.com')}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oss_setting.use_ssl'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Use HTTPS')}</FormLabel>
                        <FormDescription>
                          {t('Access OSS over HTTPS')}
                        </FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </SettingsSwitchItem>
                  )}
                />

                <SettingsControlGroup>
                  <div className='flex flex-wrap items-center gap-2'>
                    <Button
                      type='button'
                      variant='secondary'
                      onClick={handleTestConnection}
                      disabled={testState.loading || updateOption.isPending}
                    >
                      {testState.loading ? (
                        <Loader2 className='me-2 size-4 animate-spin' />
                      ) : null}
                      {t('Test Connection')}
                    </Button>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Uploads and removes a probe object using the saved configuration, so save your changes first.'
                      )}
                    </p>
                  </div>

                  {testState.ok === true ? (
                    <Alert
                      variant='default'
                      className='flex items-center gap-2'
                    >
                      <CheckCircle2 className='size-4 text-green-600' />
                      <div>
                        <AlertTitle>{t('Connection successful')}</AlertTitle>
                        <AlertDescription>
                          {t(
                            'The configured bucket accepts uploads and deletes.'
                          )}
                        </AlertDescription>
                      </div>
                    </Alert>
                  ) : null}

                  {testState.ok === false && testState.message ? (
                    <Alert
                      variant='destructive'
                      className='flex items-center gap-2'
                    >
                      <XCircle className='size-4' />
                      <div>
                        <AlertTitle>{t('Connection failed')}</AlertTitle>
                        <AlertDescription>{testState.message}</AlertDescription>
                      </div>
                    </Alert>
                  ) : null}
                </SettingsControlGroup>
              </>
            ) : null}
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
