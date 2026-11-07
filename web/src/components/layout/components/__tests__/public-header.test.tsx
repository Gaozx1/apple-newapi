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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { PublicHeader } from '../public-header'

let queryClient: QueryClient

async function renderHeader(status: Record<string, unknown>) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(STATUS_QUERY_KEY, status)
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => (
        <QueryClientProvider client={queryClient}>
          <PublicHeader
            showThemeSwitch={false}
            showLanguageSwitcher={false}
            showNotifications={false}
          />
        </QueryClientProvider>
      ),
    }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
  return screen.findByRole('banner')
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

describe('public header auth actions', () => {
  it('shows a register entry next to sign in when registration is open', async () => {
    const banner = await renderHeader({ register_enabled: true })

    const register = within(banner).getByRole('button', { name: 'Sign up' })
    expect(register).toHaveAttribute('href', '/sign-up')
    expect(
      within(banner).getByRole('button', { name: 'Sign in' })
    ).toHaveAttribute('href', '/sign-in')
  })

  it('hides the register entry but keeps sign in when registration is closed', async () => {
    const banner = await renderHeader({ register_enabled: false })

    expect(
      within(banner).queryByRole('button', { name: 'Sign up' })
    ).not.toBeInTheDocument()
    expect(
      within(banner).getByRole('button', { name: 'Sign in' })
    ).toBeInTheDocument()
  })

  it('shows the register entry in the mobile menu as well', async () => {
    const banner = await renderHeader({ register_enabled: true })
    const user = userEvent.setup()

    await user.click(
      within(banner).getByRole('button', { name: 'Toggle navigation menu' })
    )

    const mobileRegister = screen.getByRole('link', { name: 'Sign up' })
    expect(mobileRegister).toHaveAttribute('href', '/sign-up')
  })

  it('does not show anonymous auth actions to signed-in users', async () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'alice',
      role: 1,
    })
    const banner = await renderHeader({ register_enabled: true })

    await waitFor(() =>
      expect(
        within(banner).queryByRole('button', { name: 'Sign up' })
      ).toBeNull()
    )
    expect(within(banner).queryByRole('button', { name: 'Sign in' })).toBeNull()
  })
})
