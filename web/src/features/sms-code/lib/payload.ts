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
import type { SmsAction } from '../types'

/** Vendor parameters the plugin accepts, in its documented order. */
export interface SmsFields {
  keyWord?: string
  phone?: string
  province?: string
  cardType?: string
  toPhone?: string
  projId?: string
  content?: string
}

const FIELD_NAMES: Record<SmsAction, (keyof SmsFields)[]> = {
  getPhone: ['keyWord', 'phone', 'province', 'cardType'],
  getMsg: ['phone', 'keyWord'],
  send: ['phone', 'toPhone', 'projId', 'content'],
}

/**
 * Keep only the fields the requested model accepts and drop blank ones, so the
 * vendor never receives an empty `phone` that would change its meaning from
 * "random number" to "this number".
 */
export function buildSmsPayload(
  action: SmsAction,
  fields: SmsFields
): Record<string, string> {
  const payload: Record<string, string> = {}
  for (const name of FIELD_NAMES[action]) {
    const value = (fields[name] ?? '').trim()
    if (value === '') continue
    payload[name] = value
  }
  return payload
}
