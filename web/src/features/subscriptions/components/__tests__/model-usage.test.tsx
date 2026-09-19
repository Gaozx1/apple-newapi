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
import { render, screen } from '@testing-library/react'
import { afterEach, beforeAll, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { UserSubscriptionRecord } from '../../types'
import { UserSubscriptionsDialog } from '../dialogs/user-subscriptions-dialog'
import { SubscriptionModelUsage } from '../subscription-model-usage'

// Base UI's Sheet animations are not implemented by jsdom.
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, 'getAnimations', {
    configurable: true,
    value: () => [],
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

function buildRecord(
  overrides: Partial<UserSubscriptionRecord> = {}
): UserSubscriptionRecord {
  return {
    subscription: {
      id: 1,
      user_id: 7,
      plan_id: 3,
      status: 'active',
      start_time: 1,
      end_time: 2,
      amount_total: 1000,
      amount_used: 400,
    },
    ...overrides,
  }
}

describe('SubscriptionModelUsage', () => {
  test('each token bucket row shows used/limit, remaining and its share', () => {
    render(
      <SubscriptionModelUsage
        record={buildRecord({
          model_token_quotas: { 'deepseek-chat': 1000, 'gpt-x': 500 },
          token_used: { 'deepseek-chat': 750, 'gpt-x': 250 },
        })}
      />
    )

    expect(screen.getByText('Per-model token buckets')).toBeInTheDocument()
    // Token buckets are raw counts, so they render verbatim: 750/1,000 with
    // 250 remaining is 75% of this subscription's token consumption.
    expect(screen.getByText('deepseek-chat').parentElement).toHaveTextContent(
      '750/1,000 tokens · Remaining 250 · Share 75%'
    )
    expect(screen.getByText('gpt-x').parentElement).toHaveTextContent(
      '250/500 tokens · Remaining 250 · Share 25%'
    )
  })

  test('a quota bucket row omits the token unit and reports its share', () => {
    render(
      <SubscriptionModelUsage
        record={buildRecord({
          model_quotas: { 'model-a': 100, 'model-b': 300 },
          model_used: { 'model-a': 100, 'model-b': 300 },
        })}
      />
    )

    const row = screen.getByText('model-a').parentElement
    expect(row).toHaveTextContent('Share 25%')
    expect(row).not.toHaveTextContent('tokens')
    expect(screen.getByText('model-b').parentElement).toHaveTextContent(
      'Share 75%'
    )
    expect(
      screen.queryByText('Per-model token buckets')
    ).not.toBeInTheDocument()
  })

  test('renders nothing when the plan has no per-model buckets', () => {
    const { container } = render(
      <SubscriptionModelUsage record={buildRecord()} />
    )

    expect(container).toBeEmptyDOMElement()
  })
})

describe('UserSubscriptionsDialog per-model usage', () => {
  test('shows each user subscription model share next to its total quota', async () => {
    // The admin endpoint ships usage only for plans with per-model buckets, so
    // the dialog must surface it without an extra request.
    vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
      if (url === '/api/subscription/admin/plans') {
        return {
          data: {
            success: true,
            data: [
              {
                plan: {
                  id: 3,
                  title: 'Bucket Plan',
                  price_amount: 10,
                  duration_unit: 'month',
                  duration_value: 1,
                  quota_reset_period: 'never',
                  enabled: true,
                  sort_order: 0,
                  max_purchase_per_user: 0,
                  total_amount: 1000,
                },
              },
            ],
          },
        }
      }
      if (url === '/api/subscription/admin/users/7/subscriptions') {
        return {
          data: {
            success: true,
            data: [
              buildRecord({
                model_quotas: { 'model-a': 100, 'model-b': 300 },
                model_used: { 'model-a': 25, 'model-b': 75 },
              }),
            ],
          },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    })

    render(
      <UserSubscriptionsDialog
        open
        onOpenChange={() => undefined}
        user={{ id: 7, username: 'bucket-user' }}
      />
    )

    await screen.findByText('Bucket Plan')
    expect(screen.getByText('model-a').parentElement).toHaveTextContent(
      'Share 25%'
    )
    expect(screen.getByText('model-b').parentElement).toHaveTextContent(
      'Share 75%'
    )
  })
})
