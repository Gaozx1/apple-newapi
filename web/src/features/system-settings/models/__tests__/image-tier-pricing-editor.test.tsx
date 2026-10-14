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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { ImageTierPricingEditor } from '../image-tier-pricing-editor'

function renderEditor() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ImageTierPricingEditor />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('image tier pricing editor rows', () => {
  test('labels each tier price field with its own resolution and keeps the model name in its own row', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: [
          {
            key: 'ImageTierPrice',
            value:
              '{"gpt-image-1":{"enabled":true,"price_1k":0.03,"price_2k":0.04,"price_4k":0.04}}',
          },
        ],
      },
    } as never)

    renderEditor()

    const modelName = await screen.findByPlaceholderText('Model name')
    expect(modelName).toHaveValue('gpt-image-1')

    for (const [tier, price] of [
      ['1K', '0.03'],
      ['2K', '0.04'],
      ['4K', '0.04'],
    ] as const) {
      const field = screen.getByRole('spinbutton', {
        name: new RegExp(`^${tier}`),
      })
      expect(field).toHaveValue(Number(price))
      // Each price keeps its own labeled field instead of sharing a
      // cramped inline row with the model name input.
      expect(field.closest('div')).toHaveTextContent(new RegExp(`^${tier}`))
    }

    expect(screen.getByRole('switch', { name: 'Enabled' })).toBeChecked()

    fireEvent.change(modelName, { target: { value: 'gpt-image-2' } })
    await waitFor(() => expect(modelName).toHaveValue('gpt-image-2'))
  })
})
