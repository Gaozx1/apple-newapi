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
import { Send } from 'lucide-react'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

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
import { Textarea } from '@/components/ui/textarea'

import { useSmsCall } from '../hooks/use-sms-call'
import { smsSendFormSchema, type SmsSendFormValues } from '../lib/form'
import { buildSmsPayload } from '../lib/payload'
import { SmsResultCard } from './sms-result-card'

export interface SmsSendPanelProps {
  apiKey: string
  keyLoading: boolean
  takenNumber: string
}

/**
 * send: send an SMS from a number taken on this page. The vendor refuses
 * messages to personal numbers, and its answer is shown verbatim.
 */
export function SmsSendPanel(props: SmsSendPanelProps) {
  const { t } = useTranslation()
  const call = useSmsCall('send', props.apiKey)
  const form = useForm<SmsSendFormValues>({
    resolver: zodResolver(smsSendFormSchema),
    defaultValues: {
      phone: props.takenNumber,
      toPhone: '',
      content: '',
      projId: '',
    },
  })

  const ready = props.apiKey !== '' && !props.keyLoading
  const errors = form.formState.errors

  useEffect(() => {
    if (props.takenNumber === '') return
    form.setValue('phone', props.takenNumber, { shouldValidate: false })
  }, [props.takenNumber, form])

  const onSubmit = form.handleSubmit(async (values) => {
    await call.run(buildSmsPayload('send', values))
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Send className='size-4' aria-hidden='true' />
          {t('Send a message')}
        </CardTitle>
        <CardDescription>
          {t(
            'Only platform numbers accept messages; sending to a personal number is rejected by the vendor.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className='grid gap-4'>
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='grid gap-2'>
              <Label htmlFor='sms-send-from'>{t('Number')}</Label>
              <Input
                id='sms-send-from'
                inputMode='numeric'
                {...form.register('phone')}
              />
              {errors.phone !== undefined && (
                <p className='text-destructive text-xs'>
                  {t(errors.phone.message ?? '')}
                </p>
              )}
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='sms-send-to'>{t('Recipient')}</Label>
              <Input
                id='sms-send-to'
                inputMode='numeric'
                placeholder={t('Service number to send to')}
                {...form.register('toPhone')}
              />
              {errors.toPhone !== undefined && (
                <p className='text-destructive text-xs'>
                  {t(errors.toPhone.message ?? '')}
                </p>
              )}
            </div>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='sms-send-content'>{t('Content')}</Label>
            <Textarea
              id='sms-send-content'
              rows={3}
              placeholder={t('Message text')}
              {...form.register('content')}
            />
            {errors.content !== undefined && (
              <p className='text-destructive text-xs'>
                {t(errors.content.message ?? '')}
              </p>
            )}
          </div>

          <div className='grid gap-2 sm:max-w-xs'>
            <Label htmlFor='sms-send-project'>{t('Project ID')}</Label>
            <Input
              id='sms-send-project'
              placeholder={t('Optional')}
              {...form.register('projId')}
            />
          </div>

          <Button type='submit' disabled={!ready || call.busy}>
            {call.busy ? t('Sending...') : t('Send')}
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
