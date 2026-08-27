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
import { api } from '@/lib/api'

import type {
  ApiResponse,
  CreateTicketRequest,
  ReplyTicketRequest,
  Ticket,
  TicketDetailResponse,
  TicketListResponse,
} from './types'

// ============================================================================
// Ticket (工单) APIs
// ============================================================================

/**
 * Get the current user's tickets (paginated).
 */
export async function getMyTickets(
  page = 1,
  pageSize = 20
): Promise<ApiResponse<TicketListResponse>> {
  const res = await api.get('/api/user/tickets', {
    params: { page, page_size: pageSize },
  })
  return res.data
}

/**
 * Admin: get all tickets (paginated), optionally filtered by status.
 */
export async function getAllTickets(
  page = 1,
  pageSize = 20,
  status?: string
): Promise<ApiResponse<TicketListResponse>> {
  const params: Record<string, unknown> = { page, page_size: pageSize }
  if (status) params.status = status
  const res = await api.get('/api/user/admin-tickets', { params })
  return res.data
}

/**
 * Create a new ticket as the current user.
 */
export async function createTicket(
  data: CreateTicketRequest
): Promise<ApiResponse<Ticket>> {
  const res = await api.post('/api/user/tickets', data)
  return res.data
}

/**
 * Get a single ticket with its reply thread. Works for both self and admin;
 * the backend enforces ownership for non-admins.
 */
export async function getTicket(
  id: number
): Promise<ApiResponse<TicketDetailResponse>> {
  const res = await api.get(`/api/user/tickets/${id}`)
  return res.data
}

/**
 * Reply to a ticket (user or admin).
 */
export async function replyTicket(
  id: number,
  data: ReplyTicketRequest
): Promise<ApiResponse> {
  const res = await api.post(`/api/user/tickets/${id}/reply`, data)
  return res.data
}

/**
 * Admin: close a ticket.
 */
export async function closeTicket(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/user/tickets/${id}/close`)
  return res.data
}
