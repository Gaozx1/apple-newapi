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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { grantLotteryChances } from '../../api'

interface GrantLotteryChancesDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  username: string
  onSuccess?: () => void
}

export function GrantLotteryChancesDialog({
  open,
  onOpenChange,
  userId,
  username,
  onSuccess,
}: GrantLotteryChancesDialogProps) {
  const { t } = useTranslation()
  const [count, setCount] = useState('1')
  const [submitting, setSubmitting] = useState(false)

  const handleGrant = async () => {
    const parsed = Number.parseInt(count, 10)
    if (!Number.isInteger(parsed) || parsed <= 0) {
      toast.error(t('Please enter a valid positive number'))
      return
    }
    setSubmitting(true)
    try {
      const res = await grantLotteryChances(userId, parsed)
      if (res.success) {
        toast.success(t('Lottery chances granted'))
        onOpenChange(false)
        onSuccess?.()
      } else {
        toast.error(res.message || t('Failed to grant lottery chances'))
      }
    } catch {
      toast.error(t('Failed to grant lottery chances'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Grant Lottery Chances')}</DialogTitle>
          <DialogDescription>
            {t('Grant free lottery draw chances to {{username}}', {
              username,
            })}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-2 py-2'>
          <label className='text-sm font-medium' htmlFor='lottery-chances'>
            {t('Number of chances')}
          </label>
          <Input
            id='lottery-chances'
            type='number'
            min={1}
            value={count}
            onChange={(e) => setCount(e.target.value)}
            disabled={submitting}
          />
        </div>

        <DialogFooter>
          <Button
            variant='ghost'
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleGrant} disabled={submitting}>
            {submitting ? t('Granting...') : t('Grant')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
