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
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { OfflineImageThumbnail } from '../offline-image-thumbnail'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn() },
}))

const createObjectURL = vi.fn(() => 'blob:offline-image')
const revokeObjectURL = vi.fn()

function renderThumbnail(props: { taskId: number; index?: number }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <OfflineImageThumbnail taskId={props.taskId} index={props.index} />
    </QueryClientProvider>
  )
}

describe('OfflineImageThumbnail', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    createObjectURL.mockClear()
    revokeObjectURL.mockClear()
    Object.assign(URL, { createObjectURL, revokeObjectURL })
  })

  it('fetches the stored image through the authenticated client', async () => {
    const blob = new Blob([new Uint8Array([1, 2, 3])], { type: 'image/png' })
    vi.mocked(api.get).mockResolvedValue({ data: blob })

    renderThumbnail({ taskId: 5, index: 0 })

    // A bare <img src> request cannot carry the dashboard token, which made the
    // live page show nothing even though the task had succeeded.
    const image = await screen.findByRole('img', { name: 'Generated image 1' })
    expect(api.get).toHaveBeenCalledWith(
      '/api/offline-image/tasks/5/images/0',
      expect.objectContaining({ responseType: 'blob' })
    )
    expect(image).toHaveAttribute('src', 'blob:offline-image')
  })

  it('loads the reference image from the task reference endpoint', async () => {
    const blob = new Blob([new Uint8Array([9])], { type: 'image/jpeg' })
    vi.mocked(api.get).mockResolvedValue({ data: blob })

    renderThumbnail({ taskId: 7 })

    await screen.findByRole('img', { name: 'Reference image' })
    expect(api.get).toHaveBeenCalledWith(
      '/api/offline-image/tasks/7/reference',
      expect.objectContaining({ responseType: 'blob' })
    )
  })

  it('shows a placeholder instead of a broken image when the fetch fails', async () => {
    vi.mocked(api.get).mockRejectedValue(new Error('401'))

    renderThumbnail({ taskId: 5, index: 1 })

    expect(
      await screen.findByRole('img', { name: 'Generated image is loading' })
    ).toBeVisible()
    expect(createObjectURL).not.toHaveBeenCalled()
  })
})
