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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { HeroTerminalDemo } from '../hero-terminal-demo'

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('hero terminal demo tabs', () => {
  it('keeps the initial example readable until playback is started', () => {
    vi.useFakeTimers()
    render(<HeroTerminalDemo />)

    expect(screen.getByRole('button', { name: 'Start' })).toBeVisible()
    act(() => vi.advanceTimersByTime(10000))
    expect(screen.getByRole('tab', { name: 'Chat' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  it('lets keyboard users select the next protocol with an arrow key', async () => {
    const user = userEvent.setup()
    render(<HeroTerminalDemo />)
    const chat = screen.getByRole('tab', { name: 'Chat' })
    chat.focus()

    await user.keyboard('{ArrowRight}')

    expect(screen.getByRole('tab', { name: 'Responses' })).toHaveFocus()
    expect(screen.getByRole('tab', { name: 'Responses' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('tabpanel')).toHaveTextContent('/v1/responses')
  })

  it('marks the visible endpoint as the selected tab', () => {
    render(<HeroTerminalDemo />)

    expect(screen.getByRole('tablist')).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Chat' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('tab', { name: 'Claude' })).toHaveAttribute(
      'aria-selected',
      'false'
    )
  })

  it('copies the endpoint of the selected protocol', async () => {
    const user = userEvent.setup()
    render(<HeroTerminalDemo />)

    await user.click(screen.getByRole('tab', { name: 'Claude' }))
    await user.click(screen.getByRole('button', { name: 'Copy to clipboard' }))

    expect(await navigator.clipboard.readText()).toBe('/v1/messages')
    expect(screen.getByRole('button', { name: 'Copied' })).toBeVisible()
  })

  it('switches the demo when another tab is selected', async () => {
    const user = userEvent.setup()
    render(<HeroTerminalDemo />)

    await user.click(screen.getByRole('tab', { name: 'Claude' }))

    await waitFor(() =>
      expect(screen.getByRole('tab', { name: 'Claude' })).toHaveAttribute(
        'aria-selected',
        'true'
      )
    )
    expect(await screen.findByText('/v1/messages')).toBeVisible()
  })

  it('stops cycling the demos while playback is paused', () => {
    vi.useFakeTimers()
    vi.stubGlobal('matchMedia', (query: string): MediaQueryList => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }))
    render(<HeroTerminalDemo />)

    fireEvent.click(screen.getByRole('button', { name: 'Start' }))
    const pause = screen.getByRole('button', { name: 'Stop' })
    fireEvent.click(pause)
    expect(screen.getByRole('button', { name: 'Start' })).toBeVisible()

    // Crossing the cycle interval must not advance the carousel while paused.
    act(() => vi.advanceTimersByTime(5000))
    expect(screen.getByRole('tab', { name: 'Chat' })).toHaveAttribute(
      'aria-selected',
      'true'
    )

    fireEvent.click(screen.getByRole('button', { name: 'Start' }))
    act(() => vi.advanceTimersByTime(5000))
    expect(screen.getByRole('button', { name: 'Stop' })).toBeVisible()
  })
})
