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
import { afterEach, describe, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import type {
  ApiResponse,
  Ticket,
  TicketDetailResponse,
  TicketListResponse,
} from '../types'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')

vi.mock('../api', () => ({
  getAllTickets: vi.fn(),
  getTicket: vi.fn(),
  replyTicket: vi.fn(),
  closeTicket: vi.fn(),
  claimTicket: vi.fn(),
  unclaimTicket: vi.fn(),
  createTicket: vi.fn(),
  getMyTickets: vi.fn(),
}))

const { AdminTickets } = await import('../admin')
const api = await import('../api')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Ticket Management': 'Ticket Management',
        All: 'All',
        Open: 'Open',
        Closed: 'Closed',
        'All claims': 'All claims',
        Unclaimed: 'Unclaimed',
        'Claimed by me': 'Claimed by me',
        'Claimed by {{name}}': 'Claimed by {{name}}',
        'Claim Ticket': 'Claim Ticket',
        'Claiming...': 'Claiming...',
        'Unclaim Ticket': 'Unclaim Ticket',
        'Releasing...': 'Releasing...',
        'Ticket claimed': 'Ticket claimed',
        'Ticket returned to the pending pool':
          'Ticket returned to the pending pool',
        'Failed to claim ticket': 'Failed to claim ticket',
        'Failed to unclaim ticket': 'Failed to unclaim ticket',
        'Claim this ticket to reply to it': 'Claim this ticket to reply to it',
        'No tickets yet': 'No tickets yet',
        Back: 'Back',
        Support: 'Support',
        You: 'You',
        'Created at': 'Created at',
        'Original message': 'Original message',
        'Close Ticket': 'Close Ticket',
        'Ticket closed': 'Ticket closed',
        'Failed to close ticket': 'Failed to close ticket',
        'Send reply': 'Send reply',
        'Sending...': 'Sending...',
        'Type your reply...': 'Type your reply...',
        'This ticket is closed': 'This ticket is closed',
        'Ticket not found': 'Ticket not found',
      },
    },
  },
})

function makeTicket(overrides: Partial<Ticket>): Ticket {
  return {
    id: 1,
    user_id: 7,
    username: 'alice',
    subject: 'Billing question',
    content: 'Why was I charged twice?',
    status: 'open',
    assignee_id: 0,
    assignee_name: '',
    claimed_at: 0,
    created_at: 1756400000,
    updated_at: 1756400000,
    ...overrides,
  }
}

function AdminHarness() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <AdminTickets />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

afterEach(() => {
  useAuthStore.setState(useAuthStore.getInitialState(), true)
})

describe('AdminTickets', () => {
  test('renders tickets from the API with username, subject, and status', async () => {
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: {
        items: [
          makeTicket({ id: 1, username: 'alice', status: 'open' }),
          makeTicket({
            id: 2,
            username: 'bob',
            status: 'closed',
            subject: 'API key not working',
          }),
        ],
        total: 2,
        page: 1,
        size: 20,
      },
    } as ApiResponse<TicketListResponse>)

    render(<AdminHarness />)

    await waitFor(() => {
      expect(api.getAllTickets).toHaveBeenCalledWith(
        1,
        100,
        undefined,
        undefined
      )
    })
    expect(await screen.findByText('Billing question')).toBeInTheDocument()
    expect(screen.getByText(/alice/)).toBeInTheDocument()
    expect(screen.getByText(/bob/)).toBeInTheDocument()
    expect(screen.getByText('API key not working')).toBeInTheDocument()
    expect(screen.getByText('Open', { selector: 'span' })).toBeInTheDocument()
    expect(screen.getByText('Closed', { selector: 'span' })).toBeInTheDocument()
  })

  test('shows the empty state when no tickets exist', async () => {
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, size: 20 },
    } as ApiResponse<TicketListResponse>)

    render(<AdminHarness />)

    expect(await screen.findByText('No tickets yet')).toBeInTheDocument()
  })

  test('re-queries with the selected status when a filter is clicked', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, size: 20 },
    } as ApiResponse<TicketListResponse>)

    render(<AdminHarness />)
    await screen.findByText('No tickets yet')

    await user.click(screen.getByRole('button', { name: 'Closed' }))

    await waitFor(() => {
      expect(api.getAllTickets).toHaveBeenLastCalledWith(
        1,
        100,
        'closed',
        undefined
      )
    })
  })

  test('re-queries the unclaimed pool when that filter is clicked', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, size: 20 },
    } as ApiResponse<TicketListResponse>)

    render(<AdminHarness />)
    await screen.findByText('No tickets yet')

    await user.click(screen.getByRole('button', { name: 'Unclaimed' }))

    await waitFor(() => {
      expect(api.getAllTickets).toHaveBeenLastCalledWith(
        1,
        100,
        undefined,
        'pending'
      )
    })
  })

  test('shows the assignee on a claimed ticket', async () => {
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: {
        items: [
          makeTicket({
            id: 4,
            assignee_id: 9,
            assignee_name: 'admin-bob',
            claimed_at: 1756400100,
          }),
        ],
        total: 1,
        page: 1,
        size: 20,
      },
    } as ApiResponse<TicketListResponse>)

    render(<AdminHarness />)

    expect(await screen.findByText('Claimed by admin-bob')).toBeInTheDocument()
  })

  test('lets the assignee unclaim and close their ticket', async () => {
    useAuthStore.setState(useAuthStore.getInitialState(), true)
    useAuthStore.getState().auth.setUser({ id: 2, username: 'root', role: 100 })
    const user = userEvent.setup()
    const ticket = makeTicket({
      id: 5,
      status: 'open',
      assignee_id: 2,
      assignee_name: 'root',
      claimed_at: 1756400100,
    })
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: { items: [ticket], total: 1, page: 1, size: 20 },
    } as ApiResponse<TicketListResponse>)
    vi.mocked(api.getTicket).mockResolvedValue({
      success: true,
      data: { ticket, replies: [] },
    } as ApiResponse<TicketDetailResponse>)

    render(<AdminHarness />)
    await user.click(await screen.findByText('Billing question'))

    expect(await screen.findByText('Unclaim Ticket')).toBeInTheDocument()
    expect(screen.getByText('Close Ticket')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Unclaim Ticket' }))
    await waitFor(() => {
      expect(api.unclaimTicket).toHaveBeenCalledWith(5)
    })
  })

  test('offers a claim action on the unclaimed detail view', async () => {
    const user = userEvent.setup()
    const ticket = makeTicket({ id: 6, status: 'open' })
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: { items: [ticket], total: 1, page: 1, size: 20 },
    } as ApiResponse<TicketListResponse>)
    vi.mocked(api.getTicket).mockResolvedValue({
      success: true,
      data: { ticket, replies: [] },
    } as ApiResponse<TicketDetailResponse>)

    render(<AdminHarness />)
    await user.click(await screen.findByText('Billing question'))

    await user.click(
      await screen.findByRole('button', { name: 'Claim Ticket' })
    )
    await waitFor(() => {
      expect(api.claimTicket).toHaveBeenCalledWith(6)
    })
  })

  test('asks an admin to claim an unclaimed ticket before handling it', async () => {
    const user = userEvent.setup()
    const ticket = makeTicket({ id: 3, username: 'alice', status: 'open' })
    vi.mocked(api.getAllTickets).mockResolvedValue({
      success: true,
      data: { items: [ticket], total: 1, page: 1, size: 20 },
    } as ApiResponse<TicketListResponse>)
    vi.mocked(api.getTicket).mockResolvedValue({
      success: true,
      data: {
        ticket,
        replies: [
          {
            id: 1,
            ticket_id: 3,
            user_id: 7,
            is_admin: false,
            content: 'Why was I charged twice?',
            created_at: 1756400000,
          },
          {
            id: 2,
            ticket_id: 3,
            user_id: 7,
            is_admin: false,
            content: 'It happened again today',
            created_at: 1756486400,
          },
        ],
      },
    } as ApiResponse<TicketDetailResponse>)

    render(<AdminHarness />)
    await user.click(await screen.findByText('Billing question'))

    expect(
      await screen.findByRole('button', { name: 'Claim Ticket' })
    ).toBeInTheDocument()
    expect(screen.queryByText('Close Ticket')).not.toBeInTheDocument()
    expect(screen.getByText(/alice/)).toBeInTheDocument()
  })
})
