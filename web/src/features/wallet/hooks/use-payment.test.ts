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
import { act, renderHook } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { requestPayment, requestStripePayment } from '../api'
import { PAYMENT_TYPES } from '../constants'
import { requestPaymentAmount, usePayment } from './use-payment'

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  requestPayment: vi.fn(),
  requestStripePayment: vi.fn(),
}))

vi.mock('../lib', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib')>()),
  submitPaymentForm: vi.fn(),
}))

describe('payment entry routing', () => {
  test('submits the selected Alipay entry ID to the Epay endpoint', async () => {
    vi.mocked(requestPayment).mockResolvedValue({
      message: 'success',
      data: {},
      url: 'https://pay.example',
    })
    const { result } = renderHook(() => usePayment())
    await act(async () => {
      await result.current.processPayment(20, 'alipay', 'pm_second')
    })
    expect(requestPayment).toHaveBeenCalledWith({
      amount: 20,
      payment_method: 'pm_second',
    })
    expect(requestStripePayment).not.toHaveBeenCalled()
  })

  test('keeps Stripe on its native endpoint even with an independent entry ID', async () => {
    vi.mocked(requestStripePayment).mockResolvedValue({
      message: 'success',
      data: { pay_link: 'https://stripe.example' },
    })
    vi.spyOn(window, 'open').mockImplementation(() => null)
    const { result } = renderHook(() => usePayment())
    await act(async () => {
      await result.current.processPayment(20, 'stripe', 'pm_stripe')
    })
    expect(requestStripePayment).toHaveBeenCalledWith({
      amount: 20,
      payment_method: 'stripe',
    })
    expect(requestPayment).not.toHaveBeenCalled()
  })
})

describe('payment amount routing', () => {
  test('uses the dedicated Waffo amount calculator', async () => {
    const calls: string[] = []
    const amount = await requestPaymentAmount(120, PAYMENT_TYPES.WAFFO, {
      regular: async () => {
        calls.push('regular')
        return { success: true, data: '1' }
      },
      stripe: async () => {
        calls.push('stripe')
        return { success: true, data: '2' }
      },
      waffo: async (request) => {
        calls.push(`waffo:${request.amount}`)
        return { success: true, data: '18.75' }
      },
      waffoPancake: async () => {
        calls.push('pancake')
        return { success: true, data: '4' }
      },
    })

    expect(amount).toBe(18.75)
    expect(calls).toEqual(['waffo:120'])
  })
})
