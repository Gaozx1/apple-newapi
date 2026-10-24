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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { formatCurrencyFromUSD } from '@/lib/currency'

import {
  storeProductFormSchema,
  type StoreProductFormValues,
} from '../lib/form'
import type { StoreProduct } from '../types'

export interface StorePublishDialogProps {
  open: boolean
  saving: boolean
  product: StoreProduct | null
  feePermille: number
  onOpenChange: (open: boolean) => void
  onSubmit: (values: StoreProductFormValues) => void
}

/**
 * Publish a new product or edit one the caller owns. The price is typed in USD
 * and the fee hint shows exactly what the platform keeps per sale.
 */
export function StorePublishDialog(props: StorePublishDialogProps) {
  const { t } = useTranslation()
  const form = useForm<StoreProductFormValues>({
    resolver: zodResolver(storeProductFormSchema),
    defaultValues: { name: '', description: '', price: '' },
  })

  useEffect(() => {
    if (!props.open) return
    if (props.product === null) {
      form.reset({ name: '', description: '', price: '' })
      return
    }
    form.reset({
      name: props.product.name,
      description: props.product.description,
      price: String(props.product.price_amount),
    })
  }, [props.open, props.product, form])

  const priceText = form.watch('price')
  const priceAmount = Number(priceText)
  const feeText =
    Number.isFinite(priceAmount) && priceAmount > 0
      ? formatCurrencyFromUSD(priceAmount * (props.feePermille / 1000))
      : null

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.product === null ? t('Publish product') : t('Edit product')}
      description={t(
        'Buyers pay this price and receive one imported code per purchase.'
      )}
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
            type='submit'
            form='store-publish-form'
            disabled={props.saving}
          >
            {props.saving && <Spinner className='size-3' />}
            {t('Save')}
          </Button>
        </>
      }
    >
      <form
        id='store-publish-form'
        className='grid gap-4'
        onSubmit={form.handleSubmit((values) => props.onSubmit(values))}
      >
        <div className='grid gap-2'>
          <Label htmlFor='store-product-name'>{t('Product name')}</Label>
          <Input id='store-product-name' {...form.register('name')} />
          {form.formState.errors.name !== undefined && (
            <p className='text-destructive text-xs'>
              {t(form.formState.errors.name.message ?? '')}
            </p>
          )}
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-product-description'>{t('Description')}</Label>
          <Textarea
            id='store-product-description'
            rows={4}
            placeholder={t('What the buyer gets, how to redeem the code, …')}
            {...form.register('description')}
          />
          {form.formState.errors.description !== undefined && (
            <p className='text-destructive text-xs'>
              {t(form.formState.errors.description.message ?? '')}
            </p>
          )}
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-product-price'>{t('Price (USD)')}</Label>
          <Input
            id='store-product-price'
            inputMode='decimal'
            placeholder='1.00'
            {...form.register('price')}
          />
          {form.formState.errors.price !== undefined && (
            <p className='text-destructive text-xs'>
              {t(form.formState.errors.price.message ?? '')}
            </p>
          )}
          <p className='text-muted-foreground text-xs'>
            {t('Platform fee per sale')}: {props.feePermille / 10}%
            {feeText !== null && ` · ${feeText}`}
          </p>
        </div>
      </form>
    </Dialog>
  )
}
