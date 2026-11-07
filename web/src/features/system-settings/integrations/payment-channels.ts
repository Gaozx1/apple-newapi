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
import { useMemo } from 'react'
import * as z from 'zod'

import { useSystemOptions } from '../hooks/use-system-options'

export const paymentChannelSchema = z
  .object({
    id: z.string().regex(/^[a-zA-Z0-9_-]{1,32}$/),
    name: z.string().trim().min(1).max(120),
    address: z.string().trim(),
    merchant_id: z.string().trim().max(100),
    key: z.string().max(512).default(''),
    key_configured: z.boolean().default(false),
    enabled: z.boolean(),
  })
  .superRefine((channel, ctx) => {
    if (channel.address) {
      try {
        const url = new URL(channel.address)
        if (
          !['http:', 'https:'].includes(url.protocol) ||
          url.username ||
          url.password ||
          url.search ||
          url.hash
        ) {
          throw new Error('invalid URL')
        }
      } catch {
        ctx.addIssue({
          code: 'custom',
          path: ['address'],
          message: 'Invalid payment gateway address',
        })
      }
    }
    if (channel.enabled) {
      for (const field of ['address', 'merchant_id'] as const) {
        if (!channel[field]) {
          ctx.addIssue({ code: 'custom', path: [field], message: 'Required' })
        }
      }
      if (!channel.key && !channel.key_configured) {
        ctx.addIssue({ code: 'custom', path: ['key'], message: 'Required' })
      }
    }
  })

export type PaymentChannel = z.output<typeof paymentChannelSchema>

export function usePaymentChannels() {
  const query = useSystemOptions()
  const channels = useMemo(() => {
    const value = query.data?.data.find(
      (option) => option.key === 'PaymentChannels'
    )?.value
    if (!value) return []
    // Invalid saved configuration must be shown as an error, not silently
    // replaced with an empty list that could erase channels on the next save.
    return z.array(paymentChannelSchema).parse(JSON.parse(value))
  }, [query.data])
  return { query, channels }
}
