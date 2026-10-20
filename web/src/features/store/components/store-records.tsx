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
import dayjs from 'dayjs'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatCurrencyFromUSD } from '@/lib/currency'

import type { StoreOrder } from '../types'

export interface StoreRecordsProps {
  orders: StoreOrder[]
  mode: 'purchases' | 'sales'
  onAfterSales?: (order: StoreOrder) => void
}

/** Purchase history (with codes) and sales history (with fees). */
export function StoreRecords(props: StoreRecordsProps) {
  const { t } = useTranslation()
  const sales = props.mode === 'sales'
  const showAfterSales = !sales && props.onAfterSales !== undefined

  return (
    <div className='grid gap-3'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Product name')}</TableHead>
            <TableHead>{t('Amount')}</TableHead>
            {sales ? (
              <>
                <TableHead>{t('Platform fee')}</TableHead>
                <TableHead>{t('Income')}</TableHead>
              </>
            ) : (
              <TableHead>{t('Your code')}</TableHead>
            )}
            <TableHead>{t('Time')}</TableHead>
            {showAfterSales && <TableHead>{t('Actions')}</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.orders.map((order) => (
            <TableRow key={order.id}>
              <TableCell className='max-w-56 truncate'>
                {order.product_name}
              </TableCell>
              <TableCell className='font-mono'>
                {formatCurrencyFromUSD(order.price_amount)}
              </TableCell>
              {sales ? (
                <>
                  <TableCell className='font-mono'>
                    {formatCurrencyFromUSD(order.fee_amount)}
                  </TableCell>
                  <TableCell className='font-mono'>
                    {formatCurrencyFromUSD(order.income_amount)}
                  </TableCell>
                </>
              ) : (
                <TableCell>
                  <span className='flex items-center gap-1'>
                    <code className='font-mono text-xs break-all'>
                      {order.code}
                    </code>
                    <CopyButton
                      value={order.code}
                      aria-label={t('Copy code')}
                      className='size-6 shrink-0'
                    />
                  </span>
                </TableCell>
              )}
              <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                {dayjs.unix(order.created_at).format('YYYY-MM-DD HH:mm')}
              </TableCell>
              {showAfterSales && (
                <TableCell>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={() => props.onAfterSales?.(order)}
                  >
                    {t('After-sales')}
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {props.orders.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {sales ? t('No sales yet') : t('No purchases yet')}
        </p>
      )}
    </div>
  )
}
