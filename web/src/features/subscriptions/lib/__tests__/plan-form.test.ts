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
import { describe, expect, it } from 'vitest'

import {
  formValuesToPlanPayload,
  parseModelTokenQuotasJson,
  planToFormValues,
  serializeModelTokenQuotas,
} from '../plan-form'
import type { SubscriptionPlan } from '../../types'

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
