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
import { PhoneIncoming } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { useSmsCall } from '../hooks/use-sms-call'
import {
  SMS_CARD_TYPES,
  smsPhoneFormDefaults,
  smsPhoneFormSchema,
  type SmsPhoneFormValues,
} from '../lib/form'
import { buildSmsPayload } from '../lib/payload'
import { SmsResultCard } from './sms-result-card'

export interface SmsPhonePanelProps {
  apiKey: string
  keyLoading: boolean
  onTaken: (phone: string, keyWord: string) => void
}

/**
 * getPhone: take a number for a project keyword. The keyword travels with the
 * panel because the later getMsg call must use the same one.
 */
export function SmsPhonePanel(props: SmsPhonePanelProps) {
  const { t } = useTranslation()
  const call = useSmsCall('getPhone', props.apiKey)
  const form = useForm<SmsPhoneFormValues>({
    resolver: zodResolver(smsPhoneFormSchema),
    defaultValues: smsPhoneFormDefaults,
  })

  const ready = props.apiKey !== '' && !props.keyLoading
  const errors = form.formState.errors

  const onSubmit = form.handleSubmit(async (values) => {
    const next = await call.run(buildSmsPayload('getPhone', values))
    if (next === null || next.code !== 'ok') return
    if (next.phone !== undefined && next.phone !== '') {
      props.onTaken(next.phone, values.keyWord.trim())
      toast.success(t('Number ready'))
    }
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <PhoneIncoming className='size-4' aria-hidden='true' />
          {t('Take a number')}
        </CardTitle>
        <CardDescription>
          {t(
            'Fill in the SMS keyword so the number only receives the messages of that project.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className='grid gap-4'>
          <div className='grid gap-2'>
            <Label htmlFor='sms-phone-keyword'>{t('SMS keyword')}</Label>
            <Input
              id='sms-phone-keyword'
              placeholder={t('For example 毛竹')}
              {...form.register('keyWord')}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Use the name inside the brackets of the message, such as 毛竹.'
              )}
            </p>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='sms-phone-number'>{t('Specific number')}</Label>
            <Input
              id='sms-phone-number'
              inputMode='numeric'
              placeholder={t('Leave empty for a random number')}
              {...form.register('phone')}
            />
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='grid gap-2'>
              <Label htmlFor='sms-phone-province'>{t('Province')}</Label>
              <Input
                id='sms-phone-province'
                placeholder={t('Optional')}
                {...form.register('province')}
              />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='sms-phone-card-type'>{t('Card type')}</Label>
              <Select
                value={form.watch('cardType')}
                onValueChange={(value) => {
                  if (value === null) return
                  form.setValue(
                    'cardType',
                    value as SmsPhoneFormValues['cardType'],
                    {
                      shouldValidate: true,
                    }
                  )
                }}
              >
                <SelectTrigger
                  id='sms-phone-card-type'
                  aria-label={t('Card type')}
                  className='w-full'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SMS_CARD_TYPES.map((cardType) => (
                    <SelectItem key={cardType} value={cardType}>
                      {cardType}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {errors.cardType !== undefined && (
            <p className='text-destructive text-xs'>
              {t(errors.cardType.message ?? '')}
            </p>
          )}

          <Button type='submit' disabled={!ready || call.busy}>
            {call.busy ? t('Taking a number...') : t('Take number')}
          </Button>

          {call.error !== null && (
            <p className='text-destructive text-sm break-all'>{call.error}</p>
          )}
          <SmsResultCard result={call.result} />
        </form>
      </CardContent>
    </Card>
  )
}
