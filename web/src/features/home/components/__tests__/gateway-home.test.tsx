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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { STATUS_QUERY_KEY, type StatusData } from '@/lib/status-query'

import { Hero } from '../sections/hero'

// Decorative brand artwork is outside the navigation and clipboard contracts.
vi.mock('@lobehub/icons/es/CherryStudio/components/Color.js', () => ({
  default: () => null,
}))

let client: QueryClient

afterEach(() => {
  cleanup()
  client?.clear()
})

async function renderGateway(status: StatusData, isAuthenticated = false) {
  client = new QueryClient({
    defaultOptions: { queries: { enabled: false, retry: false } },
  })
  client.setQueryData(STATUS_QUERY_KEY, status)
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => <Hero isAuthenticated={isAuthenticated} />,
    }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return screen.findByRole('heading', { name: 'Model Access' })
}

it('sends visitors to sign-in for gated models and honours disabled registration and documentation', async () => {
  await renderGateway({
    register_enabled: false,
    HeaderNavModules: JSON.stringify({
      pricing: { enabled: true, requireAuth: true },
      docs: false,
    }),
  })
  const modelLink = new URL(
    screen.getByRole<HTMLAnchorElement>('button', { name: 'View models' }).href,
    window.location.origin
  )
  expect(modelLink.pathname).toBe('/sign-in')
  expect(modelLink.searchParams.get('redirect')).toBe('/pricing')
  expect(screen.getByRole('button', { name: 'Sign in' })).toHaveAttribute(
    'href',
    '/sign-in'
  )
  expect(screen.queryByRole('button', { name: 'Docs' })).not.toBeInTheDocument()
})

it('copies the advertised address, uses configured documentation, and respects hidden modules', async () => {
  const user = userEvent.setup()
  const writeText = vi.spyOn(navigator.clipboard, 'writeText')
  const address = 'https://api.example.com/edge/'
  await renderGateway(
    {
      docs_link: 'https://docs.example.com/start',
      api_info: [{ url: address }],
      HeaderNavModules: JSON.stringify({ pricing: false, console: true }),
    },
    true
  )
  expect(screen.getByText(address)).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Copy API URL' }))
  expect(writeText).toHaveBeenCalledWith(address)
  expect(screen.getByRole('button', { name: 'Docs' })).toHaveAttribute(
    'href',
    'https://docs.example.com/start'
  )
  expect(
    screen.getByRole('button', { name: 'Go to Dashboard' })
  ).toHaveAttribute('href', '/dashboard')
  expect(
    screen.queryByRole('button', { name: 'View models' })
  ).not.toBeInTheDocument()
})
