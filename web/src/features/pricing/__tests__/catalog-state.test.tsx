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
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { expect, it, vi } from 'vitest'

import { Pricing } from '..'
import { usePricingData } from '../hooks/use-pricing-data'

vi.mock('../hooks/use-pricing-data', () => ({ usePricingData: vi.fn() }))
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useSearch: () => ({}),
}))
vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: ReactNode }) => (
    <main>{props.children}</main>
  ),
  PublicPageHero: () => <h1>Model Square</h1>,
}))
vi.mock('../components', () => ({
  LoadingSkeleton: () => <div role='status'>Loading models</div>,
  EmptyState: () => <p>No matching models</p>,
  SearchBar: () => null,
  PricingSidebar: () => null,
  PricingToolbar: () => null,
  ModelCardGrid: () => <p>Available models</p>,
  PricingTable: () => <p>Available models</p>,
}))

function catalogState() {
  return {
    models: [],
    vendors: [],
    groupRatio: {},
    groupRegions: {},
    usableGroup: {},
    endpointMap: {},
    autoGroups: [],
    isLoading: false,
    isFetching: false,
    hasData: false,
    error: new Error('Service unavailable'),
    refetch: vi.fn(),
    priceRate: 1,
    usdExchangeRate: 1,
  }
}

it('shows a recoverable failure instead of an empty catalog when the request fails', async () => {
  const state = catalogState()
  vi.mocked(usePricingData).mockReturnValue(state)
  render(<Pricing />)
  expect(screen.getByText('Failed to load models')).toBeVisible()
  expect(screen.queryByText('No matching models')).not.toBeInTheDocument()
  await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
  expect(state.refetch).toHaveBeenCalledTimes(1)
})

it('keeps the cached catalog usable and disables retry during a background refresh', () => {
  vi.mocked(usePricingData).mockReturnValue({
    ...catalogState(),
    hasData: true,
    isFetching: true,
  })
  render(<Pricing />)
  expect(screen.getByRole('heading', { name: 'Model Square' })).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent('Service unavailable')
  expect(screen.getByRole('button', { name: 'Retry' })).toBeDisabled()
})
