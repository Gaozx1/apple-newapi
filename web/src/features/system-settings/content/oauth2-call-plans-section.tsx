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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  adminCreateOAuthCallPlan,
  adminDeleteOAuthCallPlan,
  adminListOAuthCallPlans,
  adminUpdateOAuthCallPlan,
  type OAuthCallPlan,
} from '@/features/oauth2/api'

import { SettingsSection } from '../components/settings-section'

function emptyPlan(): OAuthCallPlan {
  return {
    id: 0,
    name: '',
    call_count: 1000,
    price_amount: 1,
    validity_days: 0,
    enabled: true,
    sort_order: 0,
  }
}

function PlanRowEditor(props: {
  row: OAuthCallPlan
  saving: boolean
  isSavingRow: boolean
  onSave: (row: OAuthCallPlan) => void
  onDelete: ((row: OAuthCallPlan) => void) | null
}) {
  const { t } = useTranslation()
  const [values, setValues] = useState<OAuthCallPlan>(props.row)

  const update = (patch: Partial<OAuthCallPlan>) => {
    setValues((current) => ({ ...current, ...patch }))
  }

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Input
          className='h-8 w-44 min-w-[140px] flex-1'
          value={values.name}
          placeholder={t('Package name')}
          onChange={(e) => update({ name: e.target.value })}
        />
        <Input
          className='h-8 w-28'
          type='number'
          min={1}
          value={values.call_count}
          aria-label={t('Call count')}
          onChange={(e) =>
            update({ call_count: Number.parseInt(e.target.value, 10) || 0 })
          }
        />
        <div className='flex items-center gap-1'>
          <Input
            className='h-8 w-20'
            type='number'
            min={0}
            step='0.01'
            value={values.price_amount}
            aria-label={t('Price')}
            onChange={(e) =>
              update({ price_amount: Number.parseFloat(e.target.value) || 0 })
            }
          />
          <span className='text-muted-foreground text-xs'>$</span>
        </div>
        <div className='flex items-center gap-1'>
          <Input
            className='h-8 w-16'
            type='number'
            min={0}
            value={values.validity_days}
            aria-label={t('Validity (days)')}
            onChange={(e) =>
              update({
                validity_days: Number.parseInt(e.target.value, 10) || 0,
              })
            }
          />
          <span className='text-muted-foreground text-xs'>{t('days')}</span>
        </div>
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

export function OAuth2CallPlansSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [savingId, setSavingId] = useState<number | 'new' | null>(null)
  const [draft, setDraft] = useState<OAuthCallPlan | null>(null)

  const plansQuery = useQuery({
    queryKey: ['oauth-admin-call-plans'],
    queryFn: adminListOAuthCallPlans,
  })

  const plans = plansQuery.data?.data || []

  const handleSave = async (row: OAuthCallPlan) => {
    const payload: Partial<OAuthCallPlan> = {
      name: row.name,
      call_count: Number(row.call_count) || 0,
      price_amount: Number(row.price_amount) || 0,
      validity_days: Number(row.validity_days) || 0,
      enabled: row.enabled,
      sort_order: Number(row.sort_order) || 0,
    }
    setSavingId(row.id || 'new')
    try {
      if (row.id > 0) {
        const res = await adminUpdateOAuthCallPlan(row.id, payload)
        if (!res.success) {
          toast.error(res.message || t('Update failed'))
          return
        }
        toast.success(t('Update succeeded'))
      } else {
        const res = await adminCreateOAuthCallPlan(payload)
        if (!res.success || !res.data) {
          toast.error(res.message || t('Create failed'))
          return
        }
        toast.success(t('Create succeeded'))
        setDraft(null)
      }
      await queryClient.invalidateQueries({
        queryKey: ['oauth-admin-call-plans'],
      })
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setSavingId(null)
    }
  }

  const handleDelete = async (row: OAuthCallPlan) => {
    if (row.id <= 0) return
    try {
      const res = await adminDeleteOAuthCallPlan(row.id)
      if (!res.success) {
        toast.error(res.message || t('Delete failed'))
        return
      }
      toast.success(t('Delete succeeded'))
      await queryClient.invalidateQueries({
        queryKey: ['oauth-admin-call-plans'],
      })
    } catch {
      toast.error(t('Request failed'))
    }
  }

  return (
    <SettingsSection title={t('OAuth 2.0 Call Packages')}>
      <div className='space-y-3'>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Users buy these call packages with their wallet balance. Package calls are consumed for API calls beyond the free daily tier, before the wallet is charged.'
          )}
        </p>

        {plansQuery.isLoading ? (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        ) : (
          <div className='space-y-3'>
            {plans.map((row) => (
              <PlanRowEditor
                key={row.id}
                row={row}
                saving={savingId !== null}
                isSavingRow={savingId === row.id}
                onSave={handleSave}
                onDelete={handleDelete}
              />
            ))}
            {draft && (
              <div className='space-y-2'>
                <p className='text-muted-foreground text-xs'>
                  {t('Fill in the new package and press Save to create it.')}
                </p>
                <PlanRowEditor
                  row={draft}
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
          onClick={() => setDraft(emptyPlan())}
        >
          <Plus className='mr-1 h-3.5 w-3.5' />
          {t('Add Call Package')}
        </Button>
      </div>
    </SettingsSection>
  )
}
