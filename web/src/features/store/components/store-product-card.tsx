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
import { ShoppingBag } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'
import { formatCurrencyFromUSD } from '@/lib/currency'

import type { StoreProduct } from '../types'

export interface StoreProductCardProps {
  product: StoreProduct
  buying: boolean
  onBuy: (product: StoreProduct) => void
}

/** One listing: price, remaining stock and the buy action. */
export function StoreProductCard(props: StoreProductCardProps) {
  const { t } = useTranslation()
  const product = props.product
  const soldOut = product.stock <= 0
  const disabled = soldOut || product.is_owner || props.buying

  return (
    <Card className='flex h-full flex-col'>
      <CardHeader className='gap-2'>
        <div className='flex items-start justify-between gap-2'>
          <CardTitle className='text-base break-words'>
            {product.name}
          </CardTitle>
          {soldOut ? (
            <Badge variant='destructive'>{t('Sold out')}</Badge>
          ) : (
            <Badge variant='secondary'>
              {t('{{count}} in stock', { count: product.stock })}
            </Badge>
          )}
        </div>
        {product.description !== '' && (
          <p className='text-muted-foreground line-clamp-3 text-xs break-words whitespace-pre-wrap'>
            {product.description}
          </p>
        )}
      </CardHeader>
      <CardContent className='flex-1'>
        <div className='flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
          <span className='text-muted-foreground'>
            {t('Seller')}: {product.username || `#${product.user_id}`}
          </span>
          <span className='text-muted-foreground'>
            {t('Sold')}: {product.sold_count}
          </span>
        </div>
      </CardContent>
      <CardFooter className='flex items-center justify-between gap-2'>
        <span className='font-mono text-sm'>
          {formatCurrencyFromUSD(product.price_amount)}
        </span>
        <Button
          size='sm'
          disabled={disabled}
          onClick={() => props.onBuy(product)}
        >
          {props.buying ? (
            <Spinner className='size-3' />
          ) : (
            <ShoppingBag className='size-3' aria-hidden='true' />
          )}
          {product.is_owner ? t('Your product') : t('Buy now')}
        </Button>
      </CardFooter>
    </Card>
  )
}
