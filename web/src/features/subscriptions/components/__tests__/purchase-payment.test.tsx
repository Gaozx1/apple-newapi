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

import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { paySubscriptionEpay } from '../../api'
import { subscriptionPlanSchema } from '../../types'
import { SubscriptionPurchaseDialog } from '../dialogs/subscription-purchase-dialog'

vi.mock('../../api', () => ({
  paySubscriptionEpay: vi.fn(),
  paySubscriptionStripe: vi.fn(),
  paySubscriptionCreem: vi.fn(),
  paySubscriptionWaffoPancake: vi.fn(),
  paySubscriptionBalance: vi.fn(),
}))
vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({ currency: { quotaPerUnit: 500000 } }),
}))

it('buys a subscription through the selected entry when two methods use Alipay', async () => {
  const user = userEvent.setup()
  vi.mocked(paySubscriptionEpay).mockResolvedValue({
    success: true,
    message: 'success',
    data: {},
    url: 'https://pay.example',
  })
  vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {})
  const plan = subscriptionPlanSchema.parse({
    id: 7,
    title: 'Monthly plan',
    price_amount: 10,
    duration_unit: 'month',
    duration_value: 1,
    quota_reset_period: 'never',
    enabled: true,
    sort_order: 0,
    max_purchase_per_user: 0,
    total_amount: 0,
    allow_balance_pay: false,
  })
  render(
    <SubscriptionPurchaseDialog
      open
      onOpenChange={vi.fn()}
      plan={{ plan }}
      enableOnlineTopUp
      epayMethods={[
        { type: 'alipay', name: 'Alipay legacy' },
        { id: 'pm_second', type: 'alipay', name: 'Alipay 0%' },
      ]}
    />
  )
  await user.click(screen.getByRole('combobox'))
  await user.click(await screen.findByRole('option', { name: 'Alipay 0%' }))
  await user.click(screen.getByRole('button', { name: /^Pay$/ }))
  await waitFor(() =>
    expect(paySubscriptionEpay).toHaveBeenCalledWith({
      plan_id: 7,
      payment_method: 'pm_second',
    })
  )
})
