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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { MultiSelect } from '@/components/multi-select'
import { PasswordInput } from '@/components/password-input'
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
import { getGroups } from '@/features/users/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { SecuritySettings, UpdateOptionRequest } from '../types'

type ReviewSettings = Pick<
  SecuritySettings,
  | 'jailbreak_setting.enabled'
  | 'jailbreak_setting.base_url'
  | 'jailbreak_setting.api_key'
  | 'jailbreak_setting.blacklist_groups'
  | 'jailbreak_setting.warning_limit'
  | 'JailbreakKeyConfigured'
>

export function ReviewSettingsSection(props: {
  defaultValues: ReviewSettings
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const groups = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
    staleTime: 5 * 60 * 1000,
  })
  const schema = z
    .object({
      JailbreakKeyConfigured: z.boolean(),
      jailbreak_setting: z.object({
        enabled: z.boolean(),
        base_url: z.string().trim(),
        api_key: z.string().trim(),
        blacklist_groups: z.array(z.string().trim().min(1)),
        warning_limit: z.number().int().min(0).max(100),
      }),
    })
    .superRefine((values, ctx) => {
      const review = values.jailbreak_setting
      if (review.base_url) {
        try {
          const url = new URL(review.base_url)
          if (
            !['https:', 'http:'].includes(url.protocol) ||
            url.username ||
            url.password ||
            url.search ||
            url.hash
          ) {
            throw new Error('invalid URL')
          }
        } catch {
          ctx.addIssue({
            code: 'custom',
            path: ['jailbreak_setting', 'base_url'],
            message: t('Enter a valid review service Base URL'),
          })
        }
      }
      if (review.enabled && !review.base_url) {
        ctx.addIssue({
          code: 'custom',
          path: ['jailbreak_setting', 'base_url'],
          message: t(
            'Configure the review service Base URL and API key before enabling review'
          ),
        })
      }
      if (review.enabled && !review.api_key && !values.JailbreakKeyConfigured) {
        ctx.addIssue({
          code: 'custom',
          path: ['jailbreak_setting', 'api_key'],
          message: t(
            'Configure the review service Base URL and API key before enabling review'
          ),
        })
      }
    })
  type FormValues = z.infer<typeof schema>
  const initialValues = useMemo(
    () => ({
      JailbreakKeyConfigured: props.defaultValues.JailbreakKeyConfigured,
      jailbreak_setting: {
        enabled: props.defaultValues['jailbreak_setting.enabled'],
        base_url: props.defaultValues['jailbreak_setting.base_url'],
        api_key: '',
        blacklist_groups:
          props.defaultValues['jailbreak_setting.blacklist_groups'],
        warning_limit: props.defaultValues['jailbreak_setting.warning_limit'],
      },
    }),
    [props.defaultValues]
  )
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: initialValues,
  })
  const isSaving = form.formState.isSubmitting || updateOption.isPending
  useResetForm(
    form,
    isSaving || form.formState.isDirty ? undefined : initialValues
  )

  const onSubmit = async (values: FormValues) => {
    const review = values.jailbreak_setting
    const baseline = form.formState.defaultValues?.jailbreak_setting
    const updates: UpdateOptionRequest[] = []
    if (!review.enabled && baseline?.enabled !== review.enabled) {
      updates.push({ key: 'jailbreak_setting.enabled', value: false })
    }
    if (review.base_url !== baseline?.base_url) {
      updates.push({
        key: 'jailbreak_setting.base_url',
        value: review.base_url,
      })
    }
    if (review.api_key) {
      updates.push({ key: 'jailbreak_setting.api_key', value: review.api_key })
    }
    if (
      JSON.stringify(review.blacklist_groups) !==
      JSON.stringify(baseline?.blacklist_groups)
    ) {
      updates.push({
        key: 'jailbreak_setting.blacklist_groups',
        value: JSON.stringify(review.blacklist_groups),
      })
    }
    if (review.warning_limit !== baseline?.warning_limit) {
      updates.push({
        key: 'jailbreak_setting.warning_limit',
        value: String(review.warning_limit),
      })
    }
    if (review.enabled && baseline?.enabled !== review.enabled) {
      updates.push({ key: 'jailbreak_setting.enabled', value: true })
    }
    if (!updates.length) {
      toast.info(t('No changes to save'))
      return
    }
    try {
      for (const update of updates) await updateOption.mutateAsync(update)
    } catch {
      return
    }
    form.reset({
      ...values,
      JailbreakKeyConfigured:
        values.JailbreakKeyConfigured || Boolean(review.api_key),
      jailbreak_setting: { ...review, api_key: '' },
    })
  }

  return (
    <SettingsSection title={t('Review Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={isSaving}
          />
          <FormDirtyIndicator isDirty={form.formState.isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='jailbreak_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Anti-Jailbreak Guard')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Review requests across all channels asynchronously. Confirmed attacks disable the account and create a jailbreak record.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={isSaving}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <FormField
              control={form.control}
              name='jailbreak_setting.base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder='https://38.55.153.114'
                      disabled={isSaving}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='jailbreak_setting.api_key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Judge API Key')}</FormLabel>
                  <FormControl>
                    <PasswordInput
                      {...field}
                      autoComplete='new-password'
                      placeholder={t('Enter new token to update')}
                      disabled={isSaving}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave blank to keep the existing key')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='jailbreak_setting.warning_limit'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Warning Count Before Ban')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='number'
                      min={0}
                      max={100}
                      step={1}
                      onChange={(event) =>
                        field.onChange(
                          Number.parseInt(event.target.value, 10) || 0
                        )
                      }
                      disabled={isSaving}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Confirmed attacks within this count only send a warning email; the account is banned after that. 0 bans on the first confirmed attack.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='jailbreak_setting.blacklist_groups'
              render={({ field }) => (
                <FormItem data-settings-form-span='full'>
                  <FormLabel>{t('Review blacklist groups')}</FormLabel>
                  <FormControl>
                    <MultiSelect
                      options={(groups.data?.data ?? []).map((group) => ({
                        label: group,
                        value: group,
                      }))}
                      selected={field.value}
                      onChange={field.onChange}
                      allowCreate
                      placeholder={t('Review blacklist groups')}
                      disabled={isSaving}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Users in these groups are exempt from review. You can select multiple groups.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
