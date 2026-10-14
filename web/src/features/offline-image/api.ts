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
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  CreateOfflineImageTaskInput,
  OfflineImageOptions,
  OfflineImageTasksPage,
  OfflineImageTaskQuery,
} from './types'

export async function getOfflineImageOptions(
  group: string
): Promise<OfflineImageOptions> {
  const res = await api.get('/api/offline-image/options', { params: { group } })
  return requireServerSuccess(res.data).data as OfflineImageOptions
}

export async function listOfflineImageTasks(
  query: OfflineImageTaskQuery = {}
): Promise<OfflineImageTasksPage> {
  const res = await api.get('/api/offline-image/tasks', { params: query })
  return requireServerSuccess(res.data).data as OfflineImageTasksPage
}

/**
 * Queues one task. Edit tasks upload their reference image as multipart, so the
 * bytes never pass through a JSON body.
 */
export async function createOfflineImageTask(
  input: CreateOfflineImageTaskInput
): Promise<void> {
  if (input.mode === 'edit') {
    if (!input.reference) {
      throw new Error('edit task requires a reference image')
    }
    const form = new FormData()
    form.append('group', input.group)
    form.append('model', input.model)
    form.append('mode', input.mode)
    form.append('prompt', input.prompt)
    form.append('n', String(input.n))
    if (input.size) form.append('size', input.size)
    if (input.quality) form.append('quality', input.quality)
    form.append('image', input.reference)
    const formRes = await api.post('/api/offline-image/tasks', form)
    requireServerSuccess(formRes.data)
    return
  }

  const jsonRes = await api.post('/api/offline-image/tasks', {
    group: input.group,
    model: input.model,
    mode: input.mode,
    prompt: input.prompt,
    size: input.size ?? '',
    quality: input.quality ?? '',
    n: input.n,
  })
  requireServerSuccess(jsonRes.data)
}

export async function deleteOfflineImageTask(id: number): Promise<void> {
  const res = await api.delete(`/api/offline-image/tasks/${id}`)
  requireServerSuccess(res.data)
}

export function offlineImageUrl(taskId: number, index: number): string {
  return `/api/offline-image/tasks/${taskId}/images/${index}`
}

export function offlineImageReferenceUrl(taskId: number): string {
  return `/api/offline-image/tasks/${taskId}/reference`
}
