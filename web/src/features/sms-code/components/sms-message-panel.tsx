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
import dayjs from 'dayjs'
import { MessageSquareCode } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
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
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import { useSmsCall } from '../hooks/use-sms-call'
import {
  SMS_HISTORY_LIMIT,
  SMS_POLL_INTERVAL_MS,
  smsMessageFormSchema,
  type SmsMessageFormValues,
} from '../lib/form'
import { buildSmsPayload } from '../lib/payload'
import type { SmsCallResult, SmsPollEntry } from '../types'
import { SmsResultCard } from './sms-result-card'

export interface SmsMessagePanelProps {
  apiKey: string
  keyLoading: boolean
  takenNumber: string
  takenKeyWord: string
}

/** Whether the poller should keep going after one reading call. */
type SmsQueryOutcome = 'pending' | 'arrived' | 'stopped'

/**
 * getMsg: read the message a number received. Only the reading that actually
 * returns the message is billed, so automatic polling keeps waiting until the
 * message arrives or the reader stops it.
 */
export function SmsMessagePanel(props: SmsMessagePanelProps) {
  const { t } = useTranslation()
  const { busy, result, error, run } = useSmsCall('getMsg', props.apiKey)
  const [polling, setPolling] = useState(false)
  const [history, setHistory] = useState<SmsPollEntry[]>([])
  const inFlight = useRef(false)
  const form = useForm<SmsMessageFormValues>({
    resolver: zodResolver(smsMessageFormSchema),
    defaultValues: { phone: props.takenNumber, keyWord: props.takenKeyWord },
  })

  const ready = props.apiKey !== '' && !props.keyLoading

  // A freshly taken number replaces the fields: reading must use the same
  // number and keyword the taking call used.
  useEffect(() => {
    if (props.takenNumber === '') return
    form.setValue('phone', props.takenNumber, { shouldValidate: false })
  }, [props.takenNumber, form])

  useEffect(() => {
    if (props.takenKeyWord === '') return
    form.setValue('keyWord', props.takenKeyWord, { shouldValidate: false })
  }, [props.takenKeyWord, form])

  const appendHistory = useCallback((next: SmsCallResult) => {
    const text = (next.raw ?? next.message ?? '').trim()
    setHistory((previous) => {
      const head = previous[0]
      if (head !== undefined && head.text === text) return previous
      const entry: SmsPollEntry = {
        at: Date.now(),
        text,
        pending: next.pending === true,
      }
      return [entry, ...previous].slice(0, SMS_HISTORY_LIMIT)
    })
  }, [])

  const queryOnce = useCallback(async (): Promise<SmsQueryOutcome> => {
    if (props.apiKey === '') return 'stopped'
    // A tick that arrives while a call is still running waits for the next one
    // instead of starting a second billed request.
    if (inFlight.current) return 'pending'
    const parsed = smsMessageFormSchema.safeParse(form.getValues())
    if (!parsed.success) {
      void form.trigger()
      return 'stopped'
    }
    inFlight.current = true
    try {
      const next = await run(buildSmsPayload('getMsg', parsed.data))
      if (next === null || next.code === 'error') return 'stopped'
      appendHistory(next)
      return next.code === 'pending' ? 'pending' : 'arrived'
    } finally {
      inFlight.current = false
    }
  }, [appendHistory, form, props.apiKey, run])

  useEffect(() => {
    if (!polling) return
    let cancelled = false
    const tick = async () => {
      const outcome = await queryOnce()
      if (cancelled) return
      if (outcome !== 'pending') setPolling(false)
    }
    void tick()
    const timer = setInterval(() => {
      void tick()
    }, SMS_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [polling, queryOnce])

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <MessageSquareCode className='size-4' aria-hidden='true' />
          {t('Read the message')}
        </CardTitle>
        <CardDescription>
          {t(
            'Only the reading that returns the message is billed, so polling can wait until the code arrives.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className='grid gap-4'
          onSubmit={form.handleSubmit(() => {
            void queryOnce()
          })}
        >
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='grid gap-2'>
              <Label htmlFor='sms-message-number'>{t('Number')}</Label>
              <Input
                id='sms-message-number'
                inputMode='numeric'
                {...form.register('phone')}
              />
              {form.formState.errors.phone !== undefined && (
                <p className='text-destructive text-xs'>
                  {t(form.formState.errors.phone.message ?? '')}
                </p>
              )}
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='sms-message-keyword'>{t('SMS keyword')}</Label>
              <Input
                id='sms-message-keyword'
                placeholder={t('For example 毛竹')}
                {...form.register('keyWord')}
              />
              {form.formState.errors.keyWord !== undefined && (
                <p className='text-destructive text-xs'>
                  {t(form.formState.errors.keyWord.message ?? '')}
                </p>
              )}
            </div>
          </div>

          <div className='flex flex-wrap items-center gap-4'>
            <Button type='submit' disabled={!ready || busy}>
              {busy ? t('Reading...') : t('Read once')}
            </Button>
            <div className='flex items-center gap-2'>
              <Switch
                id='sms-message-poll'
                checked={polling}
                disabled={!ready}
                onCheckedChange={(checked) => setPolling(checked)}
                aria-label={t('Auto poll')}
              />
              <Label htmlFor='sms-message-poll'>{t('Auto poll')}</Label>
            </div>
            {polling && (
              <span className='text-muted-foreground flex items-center gap-2 text-xs'>
                <Spinner className='size-3' />
                {t('Polling every {{seconds}} seconds', {
                  seconds: SMS_POLL_INTERVAL_MS / 1000,
                })}
              </span>
            )}
          </div>

          {error !== null && (
            <p className='text-destructive text-sm break-all'>{error}</p>
          )}
          <SmsResultCard result={result} />

          {history.length > 0 && (
            <div className='grid gap-1'>
              <p className='text-muted-foreground text-xs'>{t('History')}</p>
              <ul className='grid gap-1'>
                {history.map((entry) => (
                  <li
                    key={`${entry.at}-${entry.text}`}
                    className='flex gap-2 text-xs break-all'
                  >
                    <span className='text-muted-foreground shrink-0'>
                      {dayjs(entry.at).format('HH:mm:ss')}
                    </span>
                    <span>{entry.text}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </form>
      </CardContent>
    </Card>
  )
}
