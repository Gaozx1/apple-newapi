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
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import type * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
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

import { paymentChannelSchema, type PaymentChannel } from './payment-channels'

type Props = {
  channel: PaymentChannel
  editing: boolean
  pending: boolean
  onClose: () => void
  onSave: (channel: PaymentChannel) => void
}

export function PaymentChannelDialog(props: Props) {
  const { t } = useTranslation()
  const form = useForm<
    z.input<typeof paymentChannelSchema>,
    unknown,
    PaymentChannel
  >({
    resolver: zodResolver(paymentChannelSchema),
    defaultValues: props.channel,
  })
  const fields = [
    { name: 'id', label: t('Channel ID') },
    { name: 'name', label: t('Name') },
    { name: 'address', label: t('Gateway address') },
    { name: 'merchant_id', label: t('Merchant ID') },
    { name: 'key', label: t('Merchant key') },
  ] as const

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !props.pending) props.onClose()
      }}
      title={
        props.editing ? t('Edit payment channel') : t('Add payment channel')
      }
      description={t(
        'Supports Epay-compatible gateways for WeChat Pay, Alipay, USDT and custom payment methods.'
      )}
      contentClassName='sm:max-w-xl'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={props.pending}
            onClick={props.onClose}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='payment-channel-form'
            disabled={props.pending}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='payment-channel-form'
          onSubmit={form.handleSubmit((values) => props.onSave(values))}
        >
          <FieldGroup>
            {fields.map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{item.label}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        disabled={
                          props.pending || (item.name === 'id' && props.editing)
                        }
                        type={item.name === 'key' ? 'password' : 'text'}
                        autoComplete={
                          item.name === 'key' ? 'new-password' : 'off'
                        }
                      />
                    </FormControl>
                    {item.name === 'key' && (
                      <FormDescription>
                        {t('Leave the key blank to keep the saved key.')}
                      </FormDescription>
                    )}
                    {item.name === 'address' && (
                      <FormDescription>
                        {t('Use the gateway base address, without submit.php.')}
                      </FormDescription>
                    )}
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between'>
                  <FormLabel>{t('Enabled')}</FormLabel>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      disabled={props.pending}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </FieldGroup>
        </form>
      </Form>
    </Dialog>
  )
}
