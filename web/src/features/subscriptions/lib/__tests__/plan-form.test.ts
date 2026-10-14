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
import { beforeEach, describe, expect, it } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { SubscriptionPlan } from '../../types'
import {
  formValuesToPlanPayload,
  parseModelTokenQuotasJson,
  planToFormValues,
  serializeModelTokenQuotas,
} from '../plan-form'

function makePlan(overrides: Partial<SubscriptionPlan>): SubscriptionPlan {
  return {
    id: 1,
    title: 't',
    price_amount: 2,
    currency: 'USD',
    duration_unit: 'month',
    duration_value: 1,
    quota_reset_period: 'never',
    enabled: true,
    sort_order: 0,
    allow_balance_pay: true,
    allow_wallet_overflow: true,
    max_purchase_per_user: 0,
    total_amount: 0,
    ...overrides,
  }
}

describe('plan-form token buckets', () => {
  it('parses model_token_quotas JSON into editable rows', () => {
    expect(
      parseModelTokenQuotasJson('{"deepseek-chat":100000000,"gpt-x":500}')
    ).toEqual([
      { model: 'deepseek-chat', tokens: 100000000 },
      { model: 'gpt-x', tokens: 500 },
    ])
  })

  it('returns no rows for empty or malformed token bucket JSON', () => {
    expect(parseModelTokenQuotasJson('')).toEqual([])
    expect(parseModelTokenQuotasJson(undefined)).toEqual([])
    expect(parseModelTokenQuotasJson('not-json')).toEqual([])
  })

  it('serializes token rows as raw token counts without quota conversion', () => {
    expect(
      serializeModelTokenQuotas([
        { model: ' deepseek-chat ', tokens: 100000000 },
        { model: '', tokens: 5 },
        { model: 'gpt-x', tokens: 0 },
      ])
    ).toBe('{"deepseek-chat":100000000}')
  })

  it('serializes an empty token row set to an empty string', () => {
    expect(serializeModelTokenQuotas([])).toBe('')
    expect(serializeModelTokenQuotas(undefined)).toBe('')
  })
})

describe('plan-form total_amount -1 round trip', () => {
  it('keeps the -1 no-shared-pool sentinel when loading a plan', () => {
    const values = planToFormValues(makePlan({ total_amount: -1 }))
    expect(values.total_amount).toBe(-1)
  })

  it('sends -1 as-is instead of converting it to quota units', () => {
    const payload = formValuesToPlanPayload({
      ...planToFormValues(
        makePlan({
          total_amount: -1,
          model_token_quotas: '{"deepseek-chat":100000000}',
        })
      ),
    })
    expect(payload.plan.total_amount).toBe(-1)
    expect(payload.plan.model_token_quotas).toBe('{"deepseek-chat":100000000}')
  })
})

describe('plan-form sliding window limits round trip', () => {
  beforeEach(() => {
    // Pin the display currency so the dollar conversions are deterministic.
    useSystemConfigStore
      .getState()
      .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
  })

  it('loads stored window caps as dollars and saves them back as quota units', () => {
    // 500000 quota units = $1 with the default quota-per-unit setting.
    const values = planToFormValues(
      makePlan({ window_5h_amount: 1_000_000, window_7d_amount: 5_000_000 })
    )
    expect(values.window_5h_amount).toBe(2)
    expect(values.window_7d_amount).toBe(10)

    const payload = formValuesToPlanPayload(values)
    expect(payload.plan.window_5h_amount).toBe(1_000_000)
    expect(payload.plan.window_7d_amount).toBe(5_000_000)
  })

  it('keeps 0 for a window the plan does not limit', () => {
    const values = planToFormValues(makePlan({ total_amount: 1_000_000 }))
    expect(values.window_5h_amount).toBe(0)
    expect(values.window_7d_amount).toBe(0)
    expect(formValuesToPlanPayload(values).plan.window_7d_amount).toBe(0)
  })
})
