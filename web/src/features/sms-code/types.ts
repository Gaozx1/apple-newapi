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
/** The three d1jiema models this gateway serves. */
export type SmsAction = 'getPhone' | 'getMsg' | 'send'

/** One poll result kept in the reading history. */
export interface SmsPollEntry {
  at: number
  text: string
  pending: boolean
}

/**
 * The JSON envelope the d1jiema plugin renders on its native routes. A vendor
 * refusal answers `code: "error"`; a reading that has not received the message
 * yet answers `code: "pending"` (the gateway refunds it), so callers must
 * inspect `code` instead of trusting the HTTP status.
 */
export interface SmsCallResult {
  code: 'ok' | 'error' | 'pending'
  operation?: string
  task_id?: string
  phone?: string
  message?: string
  pending?: boolean
  raw?: string
  reason?: string
}
