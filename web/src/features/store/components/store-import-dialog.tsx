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

import { countImportCodes } from '../lib/import-codes'
import type { StoreProduct } from '../types'

export interface StoreImportDialogProps {
  open: boolean
  saving: boolean
  product: StoreProduct | null
  maxCodes: number
  onOpenChange: (open: boolean) => void
  onSubmit: (codes: string) => void
}

/** Bulk-import codes for one product, one code per line. */
export function StoreImportDialog(props: StoreImportDialogProps) {
  const { t } = useTranslation()
  const [codes, setCodes] = useState('')

  useEffect(() => {
    if (props.open) setCodes('')
  }, [props.open])

  const counted = countImportCodes(codes)
  const tooMany = counted > props.maxCodes
  const empty = counted === 0

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Import codes')}
      description={
        props.product === null
          ? ''
          : t('One code per line, for {{name}}', { name: props.product.name })
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
            disabled={props.saving || empty || tooMany}
            onClick={() => props.onSubmit(codes)}
          >
            {props.saving && <Spinner className='size-3' />}
            {t('Import')}
          </Button>
        </>
      }
    >
      <div className='grid gap-2'>
        <Label htmlFor='store-codes'>{t('Codes')}</Label>
        <Textarea
          id='store-codes'
          rows={10}
          className='font-mono text-xs'
          placeholder={'CARD-0001\nCARD-0002\nCARD-0003'}
          value={codes}
          onChange={(event) => setCodes(event.target.value)}
        />
        <p
          className={
            tooMany
              ? 'text-destructive text-xs'
              : 'text-muted-foreground text-xs'
          }
        >
          {t('{{count}} codes detected', { count: counted })} ·{' '}
          {t('up to {{max}} per import', { max: props.maxCodes })}
        </p>
      </div>
    </Dialog>
  )
}
