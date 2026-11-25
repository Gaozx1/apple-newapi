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
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { getHomePageContent } from '../../api'
import { useHomePageContent } from '../use-home-page-content'

vi.mock('../../api', () => ({ getHomePageContent: vi.fn() }))

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

let client: QueryClient

function wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

beforeEach(() => {
  localStorage.clear()
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  client.clear()
  localStorage.clear()
})

it.each(['', '# Cached homepage'])(
  'renders cached content %j while the fresh request is pending',
  async (content) => {
    localStorage.setItem('home_page_content', content)
    const response = deferred<Awaited<ReturnType<typeof getHomePageContent>>>()
    vi.mocked(getHomePageContent).mockReturnValue(response.promise)
    const { result } = renderHook(useHomePageContent, { wrapper })

    expect(result.current).toEqual({ content, isLoaded: true, isUrl: false })
    await act(async () =>
      response.resolve({ success: true, data: 'https://example.com/home' })
    )
    await waitFor(() =>
      expect(result.current.content).toBe('https://example.com/home')
    )
    expect(result.current.isUrl).toBe(true)
  }
)

it('reuses the same pending homepage request for concurrent consumers', async () => {
  const response = deferred<Awaited<ReturnType<typeof getHomePageContent>>>()
  vi.mocked(getHomePageContent).mockReturnValue(response.promise)
  const first = renderHook(useHomePageContent, { wrapper })
  const second = renderHook(useHomePageContent, { wrapper })
  expect(first.result.current.isLoaded).toBe(false)
  expect(getHomePageContent).toHaveBeenCalledTimes(1)

  await act(async () => response.resolve({ success: true, data: '' }))
  await waitFor(() => expect(second.result.current.isLoaded).toBe(true))
  expect(localStorage.getItem('home_page_content')).toBe('')
})

it('retains cached content after an unsuccessful refresh', async () => {
  localStorage.setItem('home_page_content', '# Cached homepage')
  vi.mocked(getHomePageContent).mockResolvedValue({
    success: false,
    message: 'Temporarily unavailable',
  })
  const { result } = renderHook(useHomePageContent, { wrapper })
  await waitFor(() =>
    expect(client.getQueryState(['home-page-content'])?.status).toBe('error')
  )
  expect(result.current.content).toBe('# Cached homepage')
  expect(result.current.isLoaded).toBe(true)
  expect(localStorage.getItem('home_page_content')).toBe('# Cached homepage')
})

it('loads the homepage when browser storage is unavailable', async () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('Storage blocked')
  })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('Storage blocked')
  })
  vi.mocked(getHomePageContent).mockResolvedValue({
    success: true,
    data: '# Fresh homepage',
  })
  const { result } = renderHook(useHomePageContent, { wrapper })
  await waitFor(() =>
    expect(result.current).toEqual({
      content: '# Fresh homepage',
      isLoaded: true,
      isUrl: false,
    })
  )
})
