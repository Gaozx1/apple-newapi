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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  createOfflineImageTask,
  offlineImageReferenceUrl,
  offlineImageUrl,
} from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

describe('createOfflineImageTask', () => {
  beforeEach(() => {
    vi.mocked(api.post).mockReset()
    vi.mocked(api.post).mockResolvedValue({ data: { success: true } })
  })

  it('sends a text-to-image task as JSON with the requested count', async () => {
    await createOfflineImageTask({
      group: 'default',
      model: 'gpt-image-2',
      mode: 'generate',
      prompt: 'a lighthouse',
      size: '1024x1024',
      quality: 'high',
      n: 3,
    })

    expect(api.post).toHaveBeenCalledWith('/api/offline-image/tasks', {
      group: 'default',
      model: 'gpt-image-2',
      mode: 'generate',
      prompt: 'a lighthouse',
      size: '1024x1024',
      quality: 'high',
      n: 3,
    })
  })

  it('uploads the reference image as multipart for an edit task', async () => {
    const reference = new File([new Uint8Array([1, 2, 3])], 'ref.png', {
      type: 'image/png',
    })

    await createOfflineImageTask({
      group: 'default',
      model: 'gpt-image-2',
      mode: 'edit',
      prompt: 'make it watercolor',
      n: 1,
      reference,
    })

    const [url, body] = vi.mocked(api.post).mock.calls[0]
    expect(url).toBe('/api/offline-image/tasks')
    expect(body).toBeInstanceOf(FormData)
    const form = body as FormData
    expect(form.get('mode')).toBe('edit')
    expect(form.get('prompt')).toBe('make it watercolor')
    expect(form.get('image')).toBe(reference)
  })

  it('refuses to submit an edit task without a reference image', async () => {
    await expect(
      createOfflineImageTask({
        group: 'default',
        model: 'gpt-image-2',
        mode: 'edit',
        prompt: 'make it watercolor',
        n: 1,
        reference: null,
      })
    ).rejects.toThrow('edit task requires a reference image')
    expect(api.post).not.toHaveBeenCalled()
  })

  it('surfaces a failed create response instead of reporting success', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: false, message: 'no quota left' },
    })

    await expect(
      createOfflineImageTask({
        group: 'default',
        model: 'gpt-image-2',
        mode: 'generate',
        prompt: 'a lighthouse',
        n: 1,
      })
    ).rejects.toThrow('no quota left')
  })
})

describe('offline image content urls', () => {
  it('addresses generated images and the reference through the task', () => {
    expect(offlineImageUrl(7, 2)).toBe('/api/offline-image/tasks/7/images/2')
    expect(offlineImageReferenceUrl(7)).toBe(
      '/api/offline-image/tasks/7/reference'
    )
  })
})
