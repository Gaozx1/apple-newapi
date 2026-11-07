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
import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { AuthLayout } from '../auth-layout'

let queryClient: QueryClient

async function renderAuthLayout() {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(STATUS_QUERY_KEY, {})
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => (
        <QueryClientProvider client={queryClient}>
          <AuthLayout>
            <form aria-label='sign-in-form' />
          </AuthLayout>
        </QueryClientProvider>
      ),
    }),
    history: createMemoryHistory({ initialEntries: ['/sign-in'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
}

beforeEach(() => {
  window.localStorage.clear()
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  useSystemConfigStore.setState({
    config: {
      systemName: 'Example Gateway',
      logo: '/logo.png',
      currency: useSystemConfigStore.getInitialState().config.currency,
    },
    loading: false,
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: '' },
  })
})

afterEach(() => {
  queryClient?.clear()
  window.localStorage.clear()
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
})

describe('auth layout split panel', () => {
  it('renders the sign-in form as the page main content', async () => {
    await renderAuthLayout()

    const main = await screen.findByRole('main')
    expect(main).toContainElement(
      screen.getByRole('form', { name: 'sign-in-form' })
    )
  })

  it('keeps a home link below the form card', async () => {
    await renderAuthLayout()

    const homeLink = await screen.findByRole('link', { name: 'Back to Home' })
    expect(homeLink).toHaveAttribute('href', '/')
  })

  it('shows the configured system name in the brand area', async () => {
    await renderAuthLayout()

    expect(await screen.findAllByText('Example Gateway')).not.toHaveLength(0)
  })
})
