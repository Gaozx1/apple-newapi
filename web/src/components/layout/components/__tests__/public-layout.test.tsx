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
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { PublicLayout } from '../public-layout'

let queryClient: QueryClient

async function renderLayout(children: React.ReactNode) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(STATUS_QUERY_KEY, { register_enabled: true })
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
}

beforeEach(() => {
  window.localStorage.clear()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  useSystemConfigStore.setState({ loading: false })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: '' },
  })
})

afterEach(() => {
  queryClient?.clear()
  window.localStorage.clear()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
})

describe('public layout structure', () => {
  it('marks the main landmark with the skip-link target id', async () => {
    await renderLayout(
      <PublicLayout>
        <p>Body</p>
      </PublicLayout>
    )

    const main = await screen.findByRole('main')
    expect(main).toHaveAttribute('id', 'content')
  })

  it('renders the site footer by default when the main container is used', async () => {
    await renderLayout(
      <PublicLayout>
        <p>Body</p>
      </PublicLayout>
    )

    expect(await screen.findByRole('contentinfo')).toBeVisible()
  })

  it('keeps the main landmark but omits the footer in bare hero mode', async () => {
    await renderLayout(
      <PublicLayout showMainContainer={false}>
        <p>Body</p>
      </PublicLayout>
    )

    expect(await screen.findByRole('main')).toHaveAttribute('id', 'content')
    await waitFor(() =>
      expect(screen.queryByRole('contentinfo')).not.toBeInTheDocument()
    )
  })

  it('renders the footer in bare mode when explicitly requested', async () => {
    await renderLayout(
      <PublicLayout showMainContainer={false} showFooter>
        <p>Body</p>
      </PublicLayout>
    )

    expect(await screen.findByRole('contentinfo')).toBeVisible()
  })
})
