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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import type { SubscriptionPlan, PlanPayload } from '../types'

export function getPlanFormSchema(t: TFunction) {
  return z.object({
    title: z.string().min(1, t('Please enter plan title')),
    subtitle: z.string().optional(),
    price_amount: z.coerce.number().min(0, t('Please enter amount')),
    duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
    duration_value: z.coerce.number().min(1),
    custom_seconds: z.coerce.number().min(0).optional(),
    quota_reset_period: z.enum([
      'never',
      'daily',
      'weekly',
      'monthly',
      'custom',
    ]),
    quota_reset_custom_seconds: z.coerce.number().min(0).optional(),
    enabled: z.boolean(),
    sort_order: z.coerce.number(),
    allow_balance_pay: z.boolean(),
    allow_wallet_overflow: z.boolean(),
    max_purchase_per_user: z.coerce.number().min(0),
    total_amount: z.coerce.number().min(0),
    model_quotas: z.array(
      z.object({
        model: z.string().min(1, t('Please enter the model name')),
        quota: z.coerce.number().min(0),
      })
    ),
    usable_groups: z.array(z.string()),
    upgrade_group: z.string().optional(),
    downgrade_group: z.string().optional(),
    stripe_price_id: z.string().optional(),
    creem_product_id: z.string().optional(),
    waffo_pancake_product_id: z.string().optional(),
  })
}

export type PlanFormValues = z.infer<ReturnType<typeof getPlanFormSchema>>

export const PLAN_FORM_DEFAULTS: PlanFormValues = {
  title: '',
  subtitle: '',
  price_amount: 0,
  duration_unit: 'month',
  duration_value: 1,
  custom_seconds: 0,
  quota_reset_period: 'never',
  quota_reset_custom_seconds: 0,
  enabled: true,
  sort_order: 0,
  allow_balance_pay: true,
  allow_wallet_overflow: true,
  max_purchase_per_user: 0,
  total_amount: 0,
  model_quotas: [],
  usable_groups: [],
  upgrade_group: '',
  downgrade_group: '',
  stripe_price_id: '',
  creem_product_id: '',
  waffo_pancake_product_id: '',
}

// parseModelQuotasJson converts the plan's model_quotas JSON string into
// editable form rows. Entries are returned in the JSON's own order.
export function parseModelQuotasJson(
  raw: string | undefined
): { model: string; quota: number }[] {
  if (!raw || raw.trim() === '') return []
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    return Object.entries(parsed)
      .filter(([model, quota]) => model && Number(quota) > 0)
      .map(([model, quota]) => ({
        model,
        quota: quotaUnitsToDollars(Number(quota)),
      }))
  } catch {
    return []
  }
}

// serializeModelQuotas converts form rows back into the plan's JSON string.
// Rows with an empty model name or non-positive quota are dropped; an empty
// result serializes to '' (single shared pool).
export function serializeModelQuotas(
  rows: { model: string; quota: number }[] | undefined
): string {
  const quotas: Record<string, number> = {}
  for (const row of rows || []) {
    const model = (row.model || '').trim()
    const quota = parseQuotaFromDollars(Number(row.quota || 0))
    if (model && quota > 0) quotas[model] = quota
  }
  if (Object.keys(quotas).length === 0) return ''
  return JSON.stringify(quotas)
}

export function planToFormValues(plan: SubscriptionPlan): PlanFormValues {
  return {
    title: plan.title || '',
    subtitle: plan.subtitle || '',
    price_amount: Number(plan.price_amount || 0),
    duration_unit: plan.duration_unit || 'month',
    duration_value: Number(plan.duration_value || 1),
    custom_seconds: Number(plan.custom_seconds || 0),
    quota_reset_period: plan.quota_reset_period || 'never',
    quota_reset_custom_seconds: Number(plan.quota_reset_custom_seconds || 0),
    enabled: plan.enabled !== false,
    sort_order: Number(plan.sort_order || 0),
    allow_balance_pay: plan.allow_balance_pay !== false,
    allow_wallet_overflow: plan.allow_wallet_overflow !== false,
    max_purchase_per_user: Number(plan.max_purchase_per_user || 0),
    total_amount: quotaUnitsToDollars(Number(plan.total_amount || 0)),
    model_quotas: parseModelQuotasJson(plan.model_quotas),
    usable_groups: (plan.usable_groups || '')
      .split(',')
      .map((g) => g.trim())
      .filter(Boolean),
    upgrade_group: plan.upgrade_group || '',
    downgrade_group: plan.downgrade_group || '',
    stripe_price_id: plan.stripe_price_id || '',
    creem_product_id: plan.creem_product_id || '',
    waffo_pancake_product_id: plan.waffo_pancake_product_id || '',
  }
}

export function formValuesToPlanPayload(values: PlanFormValues): PlanPayload {
  return {
    plan: {
      ...values,
      price_amount: Number(values.price_amount || 0),
      currency: 'USD',
      duration_value: Number(values.duration_value || 0),
      custom_seconds: Number(values.custom_seconds || 0),
      quota_reset_period: values.quota_reset_period || 'never',
      quota_reset_custom_seconds:
        values.quota_reset_period === 'custom'
          ? Number(values.quota_reset_custom_seconds || 0)
          : 0,
      sort_order: Number(values.sort_order || 0),
      max_purchase_per_user: Number(values.max_purchase_per_user || 0),
      total_amount: parseQuotaFromDollars(Number(values.total_amount || 0)),
      model_quotas: serializeModelQuotas(values.model_quotas),
      usable_groups: (values.usable_groups || []).join(','),
      upgrade_group: values.upgrade_group || '',
      downgrade_group: values.downgrade_group || '',
    },
  }
}
