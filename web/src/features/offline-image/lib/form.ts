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

import { OFFLINE_IMAGE_MODES } from '../types'

/**
 * Builds the create-task schema. The generation count bound comes from the
 * server (`max_n`), which is the same bound the create endpoint enforces.
 */
export function createOfflineImageFormSchema(maxCount: number) {
  return z.object({
    group: z.string().min(1, 'Please choose a group'),
    model: z.string().min(1, 'Please choose a model'),
    mode: z.enum(OFFLINE_IMAGE_MODES),
    prompt: z
      .string()
      .trim()
      .min(1, 'Prompt is required')
      .max(4000, 'Prompt is too long'),
    size: z.string().trim().max(32, 'Invalid size'),
    quality: z.string().trim().max(32, 'Invalid quality'),
    count: z
      .number()
      .int()
      .min(1, 'At least one image')
      .max(maxCount, 'Requested count exceeds the limit'),
  })
}

export type OfflineImageFormValues = z.infer<
  ReturnType<typeof createOfflineImageFormSchema>
>

export const offlineImageFormDefaults: OfflineImageFormValues = {
  group: '',
  model: '',
  mode: 'generate',
  prompt: '',
  size: '',
  quality: '',
  count: 1,
}
