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
import { Loader2, Ticket } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import { Label } from '@/components/ui/label'
import { handleServerError } from '@/lib/handle-server-error'

import { useResetCard as submitResetCard } from '../../api'
import { formatTimestamp } from '../../lib'
import type { ResetCard, UserSubscriptionRecord } from '../../types'

interface UseResetCardDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  card: ResetCard | null
  subscriptions: UserSubscriptionRecord[]
  onSuccess: () => void | Promise<void>
}

export function UseResetCardDialog(props: UseResetCardDialogProps) {
  const { t } = useTranslation()
  const [selectedSubscriptionId, setSelectedSubscriptionId] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const card = props.card
  const targetSubscriptionId = card?.target_subscription_id || 0
  const isBound = targetSubscriptionId > 0
  const boundSubscriptionActive = props.subscriptions.some(
    (record) => record.subscription.id === targetSubscriptionId
  )
  const selectableSubscriptions = isBound
    ? props.subscriptions.filter(
        (record) => record.subscription.id === targetSubscriptionId
      )
    : props.subscriptions
  const options = selectableSubscriptions.map((record) => {
    const subscription = record.subscription
    return {
      value: String(subscription.id),
      label: `#${subscription.id} · ${formatTimestamp(subscription.start_time)} → ${formatTimestamp(subscription.end_time)}`,
    }
  })
  const canSubmit =
    !!card &&
    selectedSubscriptionId !== '' &&
    !submitting &&
    (!isBound || boundSubscriptionActive)

  useEffect(() => {
    if (!props.open) return
    setSelectedSubscriptionId(
      targetSubscriptionId > 0 ? String(targetSubscriptionId) : ''
    )
  }, [props.open, targetSubscriptionId])

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setSelectedSubscriptionId('')
    }
    props.onOpenChange(open)
  }

  const handleConfirm = async () => {
    if (!card || !selectedSubscriptionId) return
    setSubmitting(true)
    try {
      const res = await submitResetCard({
        card_key: card.card_key,
        user_subscription_id: Number(selectedSubscriptionId),
      })
      if (res.success) {
        toast.success(t('Reset card used successfully'))
        setSelectedSubscriptionId('')
        await props.onSuccess()
        props.onOpenChange(false)
      } else {
        handleServerError(res, t('Failed to use reset card'))
      }
    } catch (error) {
      handleServerError(error, t('Failed to use reset card'))
    } finally {
      setSubmitting(false)
    }
  }

  if (!card) return null

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={
        <>
          <Ticket className='h-5 w-5' />
          {t('Use Reset Card')}
        </>
      }
      description={t(
        'This card will reset the used quota of the selected active subscription.'
      )}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={!canSubmit}>
            {submitting && <Loader2 className='h-4 w-4 animate-spin' />}
            {t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='bg-muted/50 space-y-2 rounded-lg border p-3 text-sm'>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground'>{t('Card Key')}</span>
            <code className='min-w-0 truncate font-mono text-xs'>
              {card.card_key}
            </code>
          </div>
          {card.plan_title && (
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>{t('Plan')}</span>
              <span className='max-w-[200px] truncate font-medium'>
                {card.plan_title}
              </span>
            </div>
          )}
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground'>{t('Expires at')}</span>
            <span>
              {card.expires_at === 0
                ? t('Never expires')
                : formatTimestamp(card.expires_at)}
            </span>
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='use-reset-card-subscription'>
            {t('Select a subscription')}
          </Label>
          <Combobox
            id='use-reset-card-subscription'
            options={options}
            value={selectedSubscriptionId}
            onValueChange={(value) => {
              if (value !== null) setSelectedSubscriptionId(value)
            }}
            placeholder={t('Select a subscription')}
            className='w-full'
            disabled={submitting}
          />
          {isBound && (
            <p className='text-muted-foreground text-xs'>
              {t('This card can only reset subscription #{{id}}', {
                id: targetSubscriptionId,
              })}
            </p>
          )}
          {isBound && !boundSubscriptionActive && (
            <p className='text-destructive text-xs'>
              {t('The subscription bound to this card is not active.')}
            </p>
          )}
          {!isBound && options.length === 0 && (
            <p className='text-muted-foreground text-xs'>
              {t('No active subscriptions available.')}
            </p>
          )}
        </div>
      </div>
    </Dialog>
  )
}
