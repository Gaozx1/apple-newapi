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
import { Loader2, SendHorizontal } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

export type TransferFeeMode = 'deduct' | 'extra'

interface UserTransferDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (
    username: string,
    quota: number,
    feeMode: TransferFeeMode
  ) => Promise<boolean>
  availableQuota: number
  transferring: boolean
}

// TRANSFER_FEE_RATE mirrors model.TransferFeeRate on the backend (0.5%).
const TRANSFER_FEE_RATE = 0.005

export function UserTransferDialog(props: UserTransferDialogProps) {
  const { t } = useTranslation()
  const currencyConfig = useSystemConfigStore((state) => state.config.currency)
  const minimumQuota = Math.ceil(
    currencyConfig.quotaPerUnit > 0
      ? currencyConfig.quotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  )
  const minimumAmount = quotaUnitsToDollars(minimumQuota)
  const maximumAmount = quotaUnitsToDollars(props.availableQuota)
  const [username, setUsername] = useState('')
  const [amount, setAmount] = useState(minimumAmount)
  const [feeMode, setFeeMode] = useState<TransferFeeMode>('deduct')

  const transferQuota = parseQuotaFromDollars(amount)
  const fee = transferQuota > 0 ? Math.max(1, Math.ceil(transferQuota * TRANSFER_FEE_RATE)) : 0
  const received = feeMode === 'deduct' ? transferQuota - fee : transferQuota
  const totalCharge = feeMode === 'deduct' ? transferQuota : transferQuota + fee
  const canTransfer =
    username.trim() !== '' &&
    Number.isFinite(amount) &&
    transferQuota >= minimumQuota &&
    totalCharge <= props.availableQuota &&
    received > 0

  useEffect(() => {
    if (props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setUsername('')
      setAmount(minimumAmount)
      setFeeMode('deduct')
    }
  }, [minimumAmount, props.open])

  const handleConfirm = async () => {
    if (!canTransfer) return
    const success = await props.onConfirm(username.trim(), transferQuota, feeMode)
    if (success) {
      props.onOpenChange(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          <SendHorizontal className='h-5 w-5' />
          {t('Transfer to User')}
        </>
      }
      description={t(
        'Transfer balance to another user. A 0.5% fee is charged and burned.'
      )}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      titleClassName='flex items-center gap-2 text-xl font-semibold'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.transferring}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={props.transferring || !canTransfer}
          >
            {props.transferring && (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            )}
            {t('Transfer')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-3 sm:space-y-5 sm:py-4'>
        <div className='space-y-3'>
          <Label
            htmlFor='transfer-username'
            className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
          >
            {t('Recipient Username')}
          </Label>
          <Input
            id='transfer-username'
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder={t('Enter the recipient username')}
            autoComplete='off'
          />
        </div>

        <div className='space-y-3'>
          <Label
            htmlFor='user-transfer-amount'
            className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
          >
            {t('Transfer Amount')}
          </Label>
          <Input
            id='user-transfer-amount'
            type='number'
            value={amount}
            onChange={(e) => setAmount(Number(e.target.value))}
            min={minimumAmount}
            max={maximumAmount}
            step={minimumAmount}
            className='font-mono text-lg'
          />
          <p className='text-muted-foreground text-xs'>
            {t('Minimum:')} {formatQuota(minimumQuota)} · {t('Available')}{' '}
            {formatQuota(props.availableQuota)}
          </p>
        </div>

        <div className='space-y-3'>
          <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
            {t('Fee Mode')}
          </Label>
          <RadioGroup
            value={feeMode}
            onValueChange={(v) => setFeeMode(v as TransferFeeMode)}
            className='space-y-2'
          >
            <Label className='flex cursor-pointer items-center gap-2 text-sm font-normal'>
              <RadioGroupItem value='deduct' />
              {t('Deduct from transfer (recipient receives amount minus fee)')}
            </Label>
            <Label className='flex cursor-pointer items-center gap-2 text-sm font-normal'>
              <RadioGroupItem value='extra' />
              {t('Charge on top (recipient receives the full amount)')}
            </Label>
          </RadioGroup>
        </div>

        {transferQuota >= minimumQuota && (
          <div className='bg-muted/40 space-y-1 rounded-lg border p-3 text-xs'>
            <div className='flex justify-between'>
              <span className='text-muted-foreground'>{t('Fee (0.5%)')}</span>
              <span className='font-mono'>{formatQuota(fee)}</span>
            </div>
            <div className='flex justify-between'>
              <span className='text-muted-foreground'>
                {t('Recipient receives')}
              </span>
              <span className='font-mono'>{formatQuota(received)}</span>
            </div>
            <div className='flex justify-between'>
              <span className='text-muted-foreground'>{t('You pay')}</span>
              <span className='font-mono'>{formatQuota(totalCharge)}</span>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  )
}
