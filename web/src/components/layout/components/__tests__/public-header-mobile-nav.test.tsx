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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { PublicHeader } from '../public-header'

let queryClient: QueryClient

async function renderHeader() {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(STATUS_QUERY_KEY, { register_enabled: true })
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

describe('public header mobile navigation dialog', () => {
  it('exposes the toggle as a collapsed dialog trigger', async () => {
    const banner = await renderHeader()

    const toggle = within(banner).getByRole('button', {
      name: 'Toggle navigation menu',
    })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(toggle).toHaveAttribute('aria-controls', 'mobile-nav')
    expect(toggle).toHaveAttribute('aria-haspopup', 'dialog')
  })

  it('marks the overlay as the controlled dialog while open', async () => {
    const banner = await renderHeader()
    const user = userEvent.setup()

    await user.click(
      within(banner).getByRole('button', { name: 'Toggle navigation menu' })
    )

    const dialog = screen.getByRole('dialog', { name: 'Header navigation' })
    expect(dialog).toHaveAttribute('id', 'mobile-nav')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(
      within(banner).getByRole('button', { name: 'Toggle navigation menu' })
    ).toHaveAttribute('aria-expanded', 'true')
  })

  it('closes the mobile dialog when Escape is pressed', async () => {
    const banner = await renderHeader()
    const user = userEvent.setup()

    await user.click(
      within(banner).getByRole('button', { name: 'Toggle navigation menu' })
    )
    await user.keyboard('{Escape}')

    expect(
      within(banner).getByRole('button', { name: 'Toggle navigation menu' })
    ).toHaveAttribute('aria-expanded', 'false')
    expect(document.querySelector('#mobile-nav')).toHaveAttribute(
      'aria-hidden',
      'true'
    )
  })
})
