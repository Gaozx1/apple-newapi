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
import { fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { UserBanBoard } from '..'

vi.mock('@/components/layout', () => ({
  PublicLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}))

const pageResponse = (account: string, hasMore = false) => ({
  data: {
    success: true,
    data: {
      items: [
        {
          account,
          source: 'manual',
          reason: '滥用服务器资源',
          banned_at: 1795420000,
        },
      ],
      has_more: hasMore,
    },
  },
})

function renderBoard() {
  const client = new QueryClient({
    defaultOptions: { queries: { gcTime: 0, retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <UserBanBoard />
    </QueryClientProvider>
  )
}

afterEach(() => vi.restoreAllMocks())

describe('Ban Board', () => {
  it('shows the public reason and requests the adjacent page', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(pageResponse('account-a', true))
      .mockResolvedValueOnce(pageResponse('account-b'))
    renderBoard()

    expect(await screen.findByText('account-a')).toBeVisible()
    expect(screen.getByText('滥用服务器资源')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))

    expect(await screen.findByText('account-b')).toBeVisible()
    expect(get).toHaveBeenLastCalledWith(
      '/api/user-bans',
      expect.objectContaining({ params: { p: 2 } })
    )
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  it('retries a failed request for the current page', async () => {
    vi.spyOn(api, 'get')
      .mockRejectedValueOnce(new Error('service unavailable'))
      .mockResolvedValueOnce(pageResponse('account-a'))
    renderBoard()

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Unable to load account bans'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByText('account-a')).toBeVisible()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
