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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

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

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const tierPriceSchema = z.object({
  ImageTierPriceEnabled: z.boolean(),
  ImageTierPrice1K: z.coerce.number().min(0),
  ImageTierPrice2K: z.coerce.number().min(0),
  ImageTierPrice4K: z.coerce.number().min(0),
  ImageTierPriceBase: z.coerce.number().min(0),
})

type TierPriceFormValues = z.infer<typeof tierPriceSchema>

type ImageTierPricingSectionProps = {
  defaultValues: TierPriceFormValues
}

export function ImageTierPricingSection({
  defaultValues,
}: ImageTierPricingSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<TierPriceFormValues>({
    resolver: zodResolver(tierPriceSchema) as never,
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: TierPriceFormValues) => {
    const payload = {
      enabled: values.ImageTierPriceEnabled,
      price_1k: values.ImageTierPrice1K,
      price_2k: values.ImageTierPrice2K,
      price_4k: values.ImageTierPrice4K,
      base_price: values.ImageTierPriceBase,
    }
    await updateOption.mutateAsync({
      key: 'ImageTierPrice',
      value: JSON.stringify(payload),
    })
  }

  return (
    <SettingsSection title={t('Image resolution pricing')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save image tier pricing'
          />
          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='ImageTierPriceEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable image tier billing')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Image generations and edits are billed a flat USD per call by the ACTUAL generated image resolution: 1K (longest edge ≤ 1280px), 2K (≤ 2560px), 4K (> 2560px). Overrides the model flat price and upstream token billing for image endpoints. Sizes that cannot be classified use the base price below (or the model flat price when it is 0).'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </SettingsSwitchItem>
              )}
            />

            <div className='grid gap-4 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='ImageTierPrice1K'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>1K ($/call)</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={0}
                        step='0.001'
                        onChange={(e) =>
                          field.onChange(Number.parseFloat(e.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='ImageTierPrice2K'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>2K ($/call)</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={0}
                        step='0.001'
                        onChange={(e) =>
                          field.onChange(Number.parseFloat(e.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='ImageTierPrice4K'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>4K ($/call)</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={0}
                        step='0.001'
                        onChange={(e) =>
                          field.onChange(Number.parseFloat(e.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='ImageTierPriceBase'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Base price for unclassified sizes')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Used when the actual output size cannot be read (e.g. URL-only responses). 0 falls back to the model flat price for such requests.'
                    )}
                  </FormDescription>
                  <FormControl>
                    <Input
                      {...field}
                      type='number'
                      min={0}
                      step='0.001'
                      onChange={(e) =>
                        field.onChange(Number.parseFloat(e.target.value) || 0)
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
