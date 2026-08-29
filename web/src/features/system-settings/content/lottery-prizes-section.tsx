import { useQuery, useQueryClient } from '@tanstack/react-query'
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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  adminCreateLotteryPrize,
  adminDeleteLotteryPrize,
  adminListLotteryPrizes,
  adminUpdateLotteryPrize,
} from '@/features/lottery/api'
import type { LotteryPrize, LotteryPrizeType } from '@/features/lottery/types'
import { getAdminPlans } from '@/features/subscriptions/api'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import { SettingsSection } from '../components/settings-section'

const PRIZE_TYPE_OPTIONS: { value: LotteryPrizeType; labelKey: string }[] = [
  { value: 'balance', labelKey: 'Balance top-up' },
  { value: 'coupon', labelKey: 'Recharge rebate coupon' },
  { value: 'subscription', labelKey: 'Subscription' },
  { value: 'thanks', labelKey: 'Thanks for participating' },
]

type EditablePrize = LotteryPrize & { quotaDisplay: number }

function toEditable(prize: LotteryPrize): EditablePrize {
  return {
    ...prize,
    quotaDisplay: quotaUnitsToDollars(Number(prize.quota || 0)),
  }
}

function emptyPrize(): EditablePrize {
  return {
    id: 0,
    type: 'balance',
    label: '',
    weight: 0,
    quota: 0,
    rebate_rate: 0,
    plan_id: 0,
    enabled: true,
    sort_order: 0,
    quotaDisplay: 0,
  }
}

function PrizeRowEditor(props: {
  row: EditablePrize
  plans: { id: number; title: string }[]
  saving: boolean
  isSavingRow: boolean
  onSave: (row: EditablePrize) => void
  onDelete: ((row: EditablePrize) => void) | null
}) {
  const { t } = useTranslation()
  const [values, setValues] = useState<EditablePrize>(props.row)

  const update = (patch: Partial<EditablePrize>) => {
    setValues((current) => ({ ...current, ...patch }))
  }

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Input
          className='h-8 w-40 min-w-[140px] flex-1'
          value={values.label}
          placeholder={t('Prize name')}
          onChange={(e) => update({ label: e.target.value })}
        />
        <Select
          items={PRIZE_TYPE_OPTIONS.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          }))}
          value={values.type}
          onValueChange={(v) => update({ type: v as LotteryPrizeType })}
        >
          <SelectTrigger className='h-8 w-36'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {PRIZE_TYPE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {t(option.labelKey)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        <div className='flex items-center gap-1'>
          <Input
            className='h-8 w-20'
            type='number'
            min={0}
            step='0.01'
            value={values.weight}
            aria-label={t('Probability weight')}
            onChange={(e) =>
              update({ weight: Number.parseFloat(e.target.value) || 0 })
            }
          />
          <span className='text-muted-foreground text-xs'>%</span>
        </div>
        {values.type === 'balance' && (
          <Input
            className='h-8 w-28'
            type='number'
            min={0}
            step='0.01'
            value={values.quotaDisplay}
            aria-label={t('Balance amount')}
            onChange={(e) =>
              update({ quotaDisplay: Number.parseFloat(e.target.value) || 0 })
            }
          />
        )}
        {values.type === 'coupon' && (
          <div className='flex items-center gap-1'>
            <Input
              className='h-8 w-16'
              type='number'
              min={0}
              max={100}
              step='0.1'
              value={values.rebate_rate}
              aria-label={t('Rebate rate')}
              onChange={(e) =>
                update({ rebate_rate: Number.parseFloat(e.target.value) || 0 })
              }
            />
            <span className='text-muted-foreground text-xs'>%</span>
          </div>
        )}
        {values.type === 'subscription' && (
          <Select
            items={props.plans.map((plan) => ({
              value: String(plan.id),
              label: plan.title,
            }))}
            value={values.plan_id ? String(values.plan_id) : ''}
            onValueChange={(v) => update({ plan_id: Number(v) || 0 })}
          >
            <SelectTrigger className='h-8 w-44'>
              <SelectValue placeholder={t('Select a subscription plan')} />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {props.plans.map((plan) => (
                  <SelectItem key={plan.id} value={String(plan.id)}>
                    {plan.title}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        )}
        <Input
          className='h-8 w-16'
          type='number'
          min={0}
          value={values.sort_order}
          aria-label={t('Sort Order')}
          onChange={(e) =>
            update({ sort_order: Number.parseInt(e.target.value, 10) || 0 })
          }
        />
        <Switch
          checked={values.enabled}
          onCheckedChange={(checked) => update({ enabled: checked })}
          aria-label={t('Enabled Status')}
        />
        <div className='ml-auto flex items-center gap-1.5'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={props.saving}
            onClick={() => props.onSave(values)}
          >
            {props.isSavingRow ? t('Saving...') : t('Save')}
          </Button>
          {props.onDelete && (
            <Button
              type='button'
              size='icon'
              variant='ghost'
              className='h-8 w-8'
              aria-label={t('Delete')}
              disabled={props.saving}
              onClick={() => props.onDelete?.(values)}
            >
              <Trash2 className='h-4 w-4' />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

export function LotteryPrizesSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [savingId, setSavingId] = useState<number | 'new' | null>(null)
  const [draft, setDraft] = useState<EditablePrize | null>(null)
  const [plans, setPlans] = useState<{ id: number; title: string }[]>([])

  const prizesQuery = useQuery({
    queryKey: ['lottery-admin-prizes'],
    queryFn: adminListLotteryPrizes,
  })

  const prizes: EditablePrize[] = useMemo(
    () => (prizesQuery.data?.data || []).map(toEditable),
    [prizesQuery.data]
  )

  useEffect(() => {
    let cancelled = false
    getAdminPlans()
      .then((res) => {
        if (cancelled) return
        if (res.success) {
          setPlans(
            (res.data || []).map((record) => ({
              id: record.plan.id,
              title: record.plan.title || `#${record.plan.id}`,
            }))
          )
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  const buildPayload = (row: EditablePrize): Partial<LotteryPrize> => ({
    type: row.type,
    label: row.label,
    weight: Number(row.weight) || 0,
    quota:
      row.type === 'balance'
        ? parseQuotaFromDollars(Number(row.quotaDisplay) || 0)
        : 0,
    rebate_rate: row.type === 'coupon' ? Number(row.rebate_rate) || 0 : 0,
    plan_id: row.type === 'subscription' ? Number(row.plan_id) || 0 : 0,
    enabled: row.enabled,
    sort_order: Number(row.sort_order) || 0,
  })

  const handleSave = async (row: EditablePrize) => {
    setSavingId(row.id || 'new')
    try {
      if (row.id > 0) {
        const res = await adminUpdateLotteryPrize(row.id, buildPayload(row))
        if (!res.success) {
          toast.error(res.message || t('Update failed'))
          return
        }
        toast.success(t('Update succeeded'))
      } else {
        const res = await adminCreateLotteryPrize(buildPayload(row))
        if (!res.success || !res.data) {
          toast.error(res.message || t('Create failed'))
          return
        }
        toast.success(t('Create succeeded'))
        setDraft(null)
      }
      await queryClient.invalidateQueries({
        queryKey: ['lottery-admin-prizes'],
      })
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setSavingId(null)
    }
  }

  const handleDelete = async (row: EditablePrize) => {
    if (row.id <= 0) return
    try {
      const res = await adminDeleteLotteryPrize(row.id)
      if (!res.success) {
        toast.error(res.message || t('Delete failed'))
        return
      }
      toast.success(t('Delete succeeded'))
      await queryClient.invalidateQueries({
        queryKey: ['lottery-admin-prizes'],
      })
    } catch {
      toast.error(t('Request failed'))
    }
  }

  const totalWeight = prizes
    .filter((row) => row.enabled)
    .reduce((sum, row) => sum + (Number(row.weight) || 0), 0)

  return (
    <SettingsSection title={t('Prize Table')}>
      <div className='space-y-3'>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Probabilities are relative weights across all enabled prizes. Changes apply to new draws immediately.'
          )}
          {prizes.length > 0 && (
            <>
              {' · '}
              {t('Enabled weight total')}: {Number(totalWeight.toFixed(2))}
            </>
          )}
        </p>

        {prizesQuery.isLoading ? (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        ) : (
          <div className='space-y-3'>
            {prizes.map((row) => (
              <PrizeRowEditor
                key={row.id}
                row={row}
                plans={plans}
                saving={savingId !== null}
                isSavingRow={savingId === row.id}
                onSave={handleSave}
                onDelete={handleDelete}
              />
            ))}
            {draft && (
              <div className='space-y-2'>
                <p className='text-muted-foreground text-xs'>
                  {t('Fill in the new prize and press Save to create it.')}
                </p>
                <PrizeRowEditor
                  row={draft}
                  plans={plans}
                  saving={savingId !== null}
                  isSavingRow={savingId === 'new'}
                  onSave={handleSave}
                  onDelete={null}
                />
              </div>
            )}
          </div>
        )}

        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={savingId !== null || draft !== null}
          onClick={() => setDraft(emptyPrize())}
        >
          <Plus className='mr-1 h-3.5 w-3.5' />
          {t('Add Prize')}
        </Button>
      </div>
    </SettingsSection>
  )
}
