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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'

import type { StoreOrder } from '../types'

export interface StoreAfterSalesDialogProps {
  open: boolean
  saving: boolean
  order: StoreOrder | null
  onOpenChange: (open: boolean) => void
  onSubmit: (reason: string) => void
}

/** Open an after-sales case for one purchase: the reason seeds the private thread. */
export function StoreAfterSalesDialog(props: StoreAfterSalesDialogProps) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')

  useEffect(() => {
    if (props.open) setReason('')
  }, [props.open])

  const empty = reason.trim() === ''

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Apply for after-sales')}
      description={
        props.order === null
          ? ''
          : t('Describe the problem with {{name}}', {
              name: props.order.product_name,
            })
      }
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={props.saving || empty}
            onClick={() => props.onSubmit(reason.trim())}
          >
            {props.saving && <Spinner className='size-3' />}
            {t('Submit')}
          </Button>
        </>
      }
    >
      <div className='grid gap-2'>
        <Label htmlFor='after-sales-reason'>{t('What happened')}</Label>
        <Textarea
          id='after-sales-reason'
          rows={4}
          value={reason}
          placeholder={t('For example: the code was already used')}
          onChange={(event) => setReason(event.target.value)}
        />
      </div>
    </Dialog>
  )
}
