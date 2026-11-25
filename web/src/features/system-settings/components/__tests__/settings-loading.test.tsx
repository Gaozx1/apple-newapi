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
import { act, render, screen } from '@testing-library/react'
import { lazy, type ReactNode } from 'react'
import { expect, it, vi } from 'vitest'

import { SettingsPage } from '../settings-page'

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ section: 'example' }),
}))
vi.mock('../../hooks/use-system-options', () => ({
  useSystemOptions: () => ({
    data: { data: [] },
    isLoading: false,
    isError: false,
  }),
  getOptionValue: (_: unknown, defaults: unknown) => defaults,
}))
vi.mock('@/components/layout', () => {
  const Slot = (props: { children: ReactNode }) => <div>{props.children}</div>
  return {
    SectionPageLayout: Object.assign(Slot, {
      Title: Slot,
      Actions: Slot,
      Content: Slot,
    }),
  }
})

it('keeps the settings title visible during module loading and passes saved settings to the loaded form', async () => {
  type FormProps = { value: string }
  let resolve!: (module: { default: (props: FormProps) => ReactNode }) => void
  const Form = lazy(
    () =>
      new Promise<{ default: (props: FormProps) => ReactNode }>((done) => {
        resolve = done
      })
  )
  render(
    <SettingsPage
      routePath='/example/$section'
      defaultSection='example'
      defaultSettings={{ value: 'saved-value' }}
      getSectionMeta={() => ({ titleKey: 'Selected settings' })}
      getSectionContent={(_section, settings) => (
        <Form value={settings.value} />
      )}
    />
  )

  expect(screen.getByText('Selected settings')).toBeVisible()
  expect(screen.getByRole('status')).toHaveTextContent('Loading settings...')
  await act(async () => resolve({ default: (props) => <p>{props.value}</p> }))
  expect(await screen.findByText('saved-value')).toBeVisible()
  expect(screen.queryByRole('status')).not.toBeInTheDocument()
})
