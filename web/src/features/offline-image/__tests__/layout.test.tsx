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
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { OfflineImage } from '../index'

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return { ...actual, api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() } }
})

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <OfflineImage />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

describe('OfflineImage layout', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.get).mockImplementation((async (url: string) => {
      if (url.startsWith('/api/offline-image/options')) {
        return {
          data: {
            success: true,
            data: {
              models: [],
              max_n: 4,
              max_queued: 10,
              reference_max_mb: 10,
              prompt_max_length: 4000,
            },
          },
        }
      }
      return {
        data: {
          success: true,
          data: { items: [], total: 0, page: 1, page_size: 10 },
        },
      }
    }) as never)
  })

  // The shell around the dashboard is overflow-hidden, so the page has to live
  // in the layout's scrolling content area; otherwise the list below the create
  // form is clipped with no way to scroll to it.
  it('keeps form and task list inside one scrollable content area', async () => {
    renderPage()

    const main = document.querySelector('main')
    expect(main).not.toBeNull()

    const scroller = main?.querySelector('.overflow-auto')
    expect(scroller).toBeTruthy()
    expect(scroller).toContainElement(
      await screen.findByRole('button', { name: 'Create task' })
    )
    expect(scroller).toContainElement(
      screen.getByRole('heading', { name: 'Tasks' })
    )
  })
})
