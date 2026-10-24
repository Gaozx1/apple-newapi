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
import { Pencil, Plus, Trash2, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
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

import type { StoreProduct } from '../types'

export interface StoreMyProductsProps {
  products: StoreProduct[]
  onPublish: () => void
  onEdit: (product: StoreProduct) => void
  onImport: (product: StoreProduct) => void
  onToggleStatus: (product: StoreProduct) => void
  onDelete: (product: StoreProduct) => void
}

/** The caller's own listings with their stock and lifecycle actions. */
export function StoreMyProducts(props: StoreMyProductsProps) {
  const { t } = useTranslation()

  return (
    <div className='grid gap-3'>
      <div className='flex justify-end'>
        <Button size='sm' onClick={props.onPublish}>
          <Plus className='size-3' aria-hidden='true' />
          {t('Publish product')}
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Product name')}</TableHead>
            <TableHead>{t('Price (USD)')}</TableHead>
            <TableHead>{t('Stock')}</TableHead>
            <TableHead>{t('Sold')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead className='text-end'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.products.map((product) => (
            <TableRow key={product.id}>
              <TableCell className='max-w-56 truncate'>
                {product.name}
              </TableCell>
              <TableCell className='font-mono'>
                {formatCurrencyFromUSD(product.price_amount)}
              </TableCell>
              <TableCell>{product.stock}</TableCell>
              <TableCell>{product.sold_count}</TableCell>
              <TableCell>
                {product.status === 1 ? (
                  <Badge variant='secondary'>{t('On sale')}</Badge>
                ) : (
                  <Badge variant='outline'>{t('Off shelf')}</Badge>
                )}
              </TableCell>
              <TableCell>
                <div className='flex items-center justify-end gap-1'>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => props.onImport(product)}
                  >
                    <Upload className='size-3' aria-hidden='true' />
                    {t('Import codes')}
                  </Button>
                  <Button
                    size='icon'
                    variant='ghost'
                    aria-label={t('Edit product')}
                    onClick={() => props.onEdit(product)}
                  >
                    <Pencil className='size-3' aria-hidden='true' />
                  </Button>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => props.onToggleStatus(product)}
                  >
                    {product.status === 1 ? t('Take down') : t('Put on sale')}
                  </Button>
                  <Button
                    size='icon'
                    variant='ghost'
                    aria-label={t('Delete')}
                    onClick={() => props.onDelete(product)}
                  >
                    <Trash2 className='size-3' aria-hidden='true' />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {props.products.length === 0 && (
        <p className='text-muted-foreground text-sm'>{t('No products yet')}</p>
      )}
    </div>
  )
}
