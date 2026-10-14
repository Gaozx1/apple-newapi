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
import { z } from 'zod'

/**
 * One anti-jailbreak verdict as recorded by the channel guard. The user content
 * and the classifier's raw reply are kept so administrators can review a ban.
 */
export const jailbreakRecordSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  username: z.string().default(''),
  channel_id: z.number(),
  channel_name: z.string().default(''),
  model_name: z.string().default(''),
  request_id: z.string().default(''),
  token_id: z.number().default(0),
  token_name: z.string().default(''),
  judge_model: z.string().default(''),
  reason: z.string().default(''),
  content: z.string().default(''),
  raw_verdict: z.string().default(''),
  // banned / already_banned / skipped_staff
  action: z.string().default('banned'),
  banned_at: z.number().default(0),
  created_at: z.number().default(0),
})

export type JailbreakRecord = z.infer<typeof jailbreakRecordSchema>

export interface GetJailbreakRecordsParams {
  p?: number
  page_size?: number
  keyword?: string
  user_id?: number
  channel_id?: number
}

export interface GetJailbreakRecordsResponse {
  success: boolean
  message?: string
  data?: {
    items: JailbreakRecord[]
    total: number
    page: number
    page_size: number
  }
}
