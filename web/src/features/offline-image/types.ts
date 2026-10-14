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

export const OFFLINE_IMAGE_MODES = ['generate', 'edit'] as const

export type OfflineImageMode = (typeof OFFLINE_IMAGE_MODES)[number]

export const offlineImageTaskSchema = z.object({
  id: z.number(),
  created_at: z.number().default(0),
  user_id: z.number().default(0),
  username: z.string().default(''),
  group: z.string().default(''),
  model: z.string().default(''),
  mode: z.string().default('generate'),
  prompt: z.string().default(''),
  size: z.string().default(''),
  quality: z.string().default(''),
  n: z.number().default(0),
  status: z.string().default('QUEUED'),
  fail_reason: z.string().default(''),
  channel_id: z.number().default(0),
  image_count: z.number().default(0),
  has_reference: z.boolean().default(false),
  submitted_at: z.number().default(0),
  started_at: z.number().default(0),
  finished_at: z.number().default(0),
})

export type OfflineImageTask = z.infer<typeof offlineImageTaskSchema>

export interface OfflineImageTasksPage {
  items: OfflineImageTask[]
  total: number
  page: number
  page_size: number
}

export interface OfflineImageOptions {
  group: string
  models: string[]
  max_n: number
  max_queued: number
  reference_max_mb: number
  prompt_max_length: number
}

export interface CreateOfflineImageTaskInput {
  group: string
  model: string
  mode: OfflineImageMode
  prompt: string
  size?: string
  quality?: string
  n: number
  reference?: File | null
}

export interface OfflineImageTaskQuery {
  p?: number
  page_size?: number
  /** Administrators may list every user's tasks. */
  all?: boolean
}
