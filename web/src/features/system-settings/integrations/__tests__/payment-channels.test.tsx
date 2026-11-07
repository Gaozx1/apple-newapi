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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { updateSystemOption } from '../../api'
import { PaymentChannelDialog } from '../payment-channel-dialog'
import { paymentChannelSchema, type PaymentChannel } from '../payment-channels'
import { PaymentChannelsEditor } from '../payment-channels-editor'
import { PaymentMethodDialog } from '../payment-method-dialog'

const channel: PaymentChannel = {
  id: 'gmpay',
  name: 'GMPay USDT',
  address: 'http://pay.example/payments/epay/v1/order/create-transaction',
  merchant_id: '1001',
  key: '',
  key_configured: true,
  enabled: true,
}

vi.mock('../../api', () => ({ updateSystemOption: vi.fn() }))
vi.mock('../../hooks/use-system-options', () => ({
  useSystemOptions: () => ({
    data: {
      data: [{ key: 'PaymentChannels', value: JSON.stringify([channel]) }],
    },
    isLoading: false,
    isError: false,
  }),
}))

describe('payment channel configuration', () => {
  it('keeps a saved merchant key when an edited channel leaves the key blank', async () => {
    const user = userEvent.setup()
    const save = vi.fn()
    render(
      <PaymentChannelDialog
        channel={channel}
        editing
        pending={false}
        onClose={vi.fn()}
        onSave={save}
      />
    )
    expect(screen.getByLabelText('Channel ID')).toBeDisabled()
    expect(screen.getByLabelText('Merchant key')).toHaveAttribute(
      'type',
      'password'
    )
    await user.clear(screen.getByLabelText('Name'))
    await user.type(screen.getByLabelText('Name'), 'USDT processor')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'gmpay',
          name: 'USDT processor',
          key: '',
          key_configured: true,
        })
      )
    )
  })

  it('rejects enabled channels with missing credentials and non-HTTP gateway URLs', () => {
    expect(
      paymentChannelSchema.safeParse({ ...channel, key_configured: false })
        .success
    ).toBe(false)
    expect(
      paymentChannelSchema.safeParse({
        ...channel,
        address: 'file:///etc/passwd',
      }).success
    ).toBe(false)
    expect(
      paymentChannelSchema.safeParse({
        ...channel,
        address: 'https://user:password@pay.example',
      }).success
    ).toBe(false)
    expect(
      paymentChannelSchema.safeParse({
        ...channel,
        enabled: false,
        key_configured: false,
      }).success
    ).toBe(true)
  })

  it('keeps the editor open after a failed channel save', async () => {
    const user = userEvent.setup()
    vi.mocked(updateSystemOption).mockResolvedValueOnce({
      success: false,
      message: 'Save failed',
    })
    const client = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    render(
      <QueryClientProvider client={client}>
        <PaymentChannelsEditor />
      </QueryClientProvider>
    )
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(updateSystemOption).toHaveBeenCalled())
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    )
    expect(
      screen.getByRole('dialog', { name: 'Edit payment channel' })
    ).toBeVisible()
  })

  it('saves an independently selected channel and gateway type without losing existing method fields', async () => {
    const user = userEvent.setup()
    const save = vi.fn()
    render(
      <PaymentMethodDialog
        open
        onOpenChange={vi.fn()}
        onSave={save}
        channels={[channel]}
        editData={{
          name: 'USDT',
          type: 'usdt-button',
          color: '#123456',
          min_topup: '50',
        }}
      />
    )
    await user.selectOptions(screen.getByLabelText('Payment channel'), 'gmpay')
    await user.type(
      screen.getByLabelText('Gateway payment type (optional)'),
      'usdt.tron'
    )
    await user.clear(screen.getByLabelText('Minimum top-up (optional)'))
    await user.click(screen.getByRole('button', { name: 'Update' }))
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({
          type: 'usdt-button',
          channel_id: 'gmpay',
          gateway_type: 'usdt.tron',
          color: '#123456',
        })
      )
    )
    expect(save.mock.calls[0][0].min_topup).toBeUndefined()
  })

  it('keeps native Stripe methods bound to their existing provider settings', () => {
    render(
      <PaymentMethodDialog
        open
        onOpenChange={vi.fn()}
        onSave={vi.fn()}
        channels={[channel]}
        editData={{ name: 'Stripe', type: 'stripe' }}
      />
    )
    expect(screen.getByLabelText('Payment channel')).toBeDisabled()
    expect(
      screen.queryByLabelText('Gateway payment type (optional)')
    ).not.toBeInTheDocument()
  })
})
