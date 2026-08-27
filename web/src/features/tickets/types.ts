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
// ============================================================================
// Ticket (工单) Type Definitions
// ============================================================================

/**
 * Generic API response envelope used across the app.
 */
export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

/**
 * Ticket lifecycle status.
 */
export type TicketStatus = 'open' | 'closed'

/**
 * A single ticket owned by a user.
 */
export interface Ticket {
  id: number
  user_id: number
  username: string
  subject: string
  content: string
  status: TicketStatus
  created_at: number
  updated_at: number
}

/**
 * A single message in a ticket thread.
 */
export interface TicketReply {
  id: number
  ticket_id: number
  user_id: number
  is_admin: boolean
  content: string
  created_at: number
}

/**
 * Paginated ticket list response (both self and admin).
 */
export interface TicketListResponse {
  items: Ticket[]
  total: number
  page: number
  size: number
}

/**
 * A single ticket with its full reply thread.
 */
export interface TicketDetailResponse {
  ticket: Ticket
  replies: TicketReply[]
}

/**
 * Create ticket request body.
 */
export interface CreateTicketRequest {
  subject: string
  content: string
}

/**
 * Reply to ticket request body.
 */
export interface ReplyTicketRequest {
  content: string
}
