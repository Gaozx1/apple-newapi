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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ComponentProps } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { ReviewSettingsSection } from '../review-settings-section'

type Values = ComponentProps<typeof ReviewSettingsSection>['defaultValues']
const defaults: Values = {
  'jailbreak_setting.enabled': false,
  'jailbreak_setting.base_url': '',
  'jailbreak_setting.api_key': '',
  'jailbreak_setting.blacklist_groups': [],
  'jailbreak_setting.warning_limit': 3,
  JailbreakKeyConfigured: false,
}

function Fixture(props: { values: Values }) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <SettingsPageProvider actionsContainer={actions}>
      <div ref={setActions} />
      <ReviewSettingsSection defaultValues={props.values} />
    </SettingsPageProvider>
  )
}

function renderSettings(values: Partial<Values> = {}) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: ['staff', 'trusted'] },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Fixture values={{ ...defaults, ...values }} />
    </QueryClientProvider>
  )
  return userEvent.setup()
}

afterEach(() => vi.restoreAllMocks())

describe('system review settings', () => {
  it('saves credentials and multiple exempt groups before enabling review', async () => {
    const request = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = renderSettings()
    await user.type(
      screen.getByRole('textbox', { name: 'Base URL' }),
      'https://review.example'
    )
    await user.type(screen.getByLabelText('Judge API Key'), 'new-key')
    await user.click(
      screen.getByRole('combobox', { name: 'Review blacklist groups' })
    )
    await user.paste('staff,trusted')
    await user.keyboard('{Escape}')
    await user.click(
      screen.getByRole('switch', { name: 'Anti-Jailbreak Guard' })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(4))
    expect(request.mock.calls.map((call) => call[1])).toEqual([
      { key: 'jailbreak_setting.base_url', value: 'https://review.example' },
      { key: 'jailbreak_setting.api_key', value: 'new-key' },
      {
        key: 'jailbreak_setting.blacklist_groups',
        value: '["staff","trusted"]',
      },
      { key: 'jailbreak_setting.enabled', value: true },
    ])
    await waitFor(() =>
      expect(screen.getByLabelText('Judge API Key')).toHaveValue('')
    )
  })

  it('keeps the saved key when changing exempt groups with a blank key input', async () => {
    const request = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = renderSettings({
      'jailbreak_setting.enabled': true,
      'jailbreak_setting.base_url': 'https://review.example',
      JailbreakKeyConfigured: true,
    })
    expect(screen.getByLabelText('Judge API Key')).toHaveAttribute(
      'type',
      'password'
    )
    await user.click(
      screen.getByRole('combobox', { name: 'Review blacklist groups' })
    )
    await user.paste('staff,trusted')
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(1))
    expect(request.mock.calls[0][1]).toEqual({
      key: 'jailbreak_setting.blacklist_groups',
      value: '["staff","trusted"]',
    })
  })

  it('requires a key before enabling review for the first time', async () => {
    const request = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = renderSettings({
      'jailbreak_setting.base_url': 'https://review.example',
    })
    await user.click(
      screen.getByRole('switch', { name: 'Anti-Jailbreak Guard' })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(
      await screen.findByText(
        'Configure the review service Base URL and API key before enabling review'
      )
    ).toBeVisible()
    expect(request).not.toHaveBeenCalled()
  })

  it('retains the key draft and does not enable review when credential saving fails', async () => {
    const request = vi.spyOn(api, 'put').mockResolvedValue({
      data: { success: false, message: 'Storage unavailable' },
    })
    const user = renderSettings({
      'jailbreak_setting.base_url': 'https://review.example',
    })
    await user.type(screen.getByLabelText('Judge API Key'), 'draft-key')
    await user.click(
      screen.getByRole('switch', { name: 'Anti-Jailbreak Guard' })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(screen.getByLabelText('Judge API Key')).toHaveValue('draft-key')
    expect(request.mock.calls[0][1]).toEqual({
      key: 'jailbreak_setting.api_key',
      value: 'draft-key',
    })
  })
})
