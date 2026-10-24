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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { formatCurrencyFromUSD } from '@/lib/currency'

import type { StoreOrder } from '../types'

export interface StoreCodeDialogProps {
  order: StoreOrder | null
  onOpenChange: (open: boolean) => void
}

/** Shown right after a purchase, and readable again from "My purchases". */
export function StoreCodeDialog(props: StoreCodeDialogProps) {
  const { t } = useTranslation()
  const order = props.order

  return (
    <Dialog
      open={order !== null}
      onOpenChange={props.onOpenChange}
      title={t('Purchase complete')}
      description={
        order === null
          ? ''
          : t('You paid {{price}} for {{name}}', {
              price: formatCurrencyFromUSD(order.price_amount),
              name: order.product_name,
            })
      }
      footer={
        <Button type='button' onClick={() => props.onOpenChange(false)}>
          {t('Done')}
        </Button>
      }
    >
      {order !== null && (
        <div className='grid gap-3'>
          <div className='grid gap-1'>
            <p className='text-muted-foreground text-xs'>{t('Your code')}</p>
            <div className='bg-muted/40 flex items-center justify-between gap-2 rounded-lg border p-3'>
              <code
                data-testid='store-code'
                className='font-mono text-sm break-all select-all'
              >
                {order.code}
              </code>
              <CopyButton
                value={order.code}
                aria-label={t('Copy code')}
                className='size-7 shrink-0'
              />
            </div>
          </div>
          <p className='text-muted-foreground text-xs'>
            {t('Order number')}:{' '}
            <span className='font-mono'>{order.order_no}</span>
          </p>
          <p className='text-muted-foreground text-xs'>
            {t('You can read this code again under "My purchases".')}
          </p>
        </div>
      )}
    </Dialog>
  )
}
