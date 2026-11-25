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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { ModelFingerprint } from '..'

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

let client: QueryClient
const results = {
  checked_at: '2026-11-24T10:00:00Z',
  reference_built_at: '2026-11-01T10:00:00Z',
  results: [
    {
      group: 'standard',
      model: 'model-alpha',
      reference: '',
      verdict: 'match',
      failed_samples: 0,
    },
    {
      group: 'premium',
      model: 'model-beta',
      reference: '',
      verdict: 'mismatch',
      failed_samples: 0,
    },
  ],
}

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  useAuthStore.getState().auth.setUser({ id: 42, username: 'reader', role: 1 })
})
afterEach(() => {
  client.clear()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
})

function renderResults() {
  render(
    <QueryClientProvider client={client}>
      <ModelFingerprint />
    </QueryClientProvider>
  )
}

const candidateAnalysis = {
  decision: 'scored',
  prediction: 'candidate-beta',
  prediction_name: 'Candidate Beta',
  family_prediction_name: 'Beta',
  probability: 0.12,
  used_outputs: 3,
  diagnostics: [],
  results: [
    {
      model: 'advertised-alpha',
      display_name: 'Advertised Alpha',
      family_name: 'Alpha',
      score: -12.3,
      probability: 0.0059,
    },
    {
      model: 'candidate-beta',
      display_name: 'Candidate Beta',
      family_name: 'Beta',
      score: -0.42,
      probability: 0.9941,
    },
  ],
}

it('binds card and detail confidence to the predicted candidate and keeps each comparison probability separate', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        ...results,
        results: [
          {
            ...results.results[1],
            model: 'advertised-alpha',
            analysis: candidateAnalysis,
          },
        ],
      },
    },
  })
  renderResults()
  const heading = await screen.findByRole('heading', {
    name: 'advertised-alpha',
  })
  const card = heading.closest('li')
  if (!card) throw new Error('Missing result card')
  expect(within(card).getByText('Candidate Beta')).toBeVisible()
  expect(within(card).getByText('Candidate confidence')).toBeVisible()
  expect(within(card).getByText('99.41%')).toBeVisible()
  expect(within(card).getByText('Possible mismatch')).toBeVisible()
  expect(
    within(card).getByRole('progressbar', {
      name: 'Confidence in Candidate Beta',
    })
  ).toHaveAttribute('aria-valuenow', '99.41')
  expect(within(card).queryByText('12%')).not.toBeInTheDocument()

  await userEvent
    .setup()
    .click(within(card).getByRole('button', { name: 'Details' }))
  const dialog = await screen.findByRole('dialog')
  expect(
    within(dialog).getByRole('progressbar', {
      name: 'Confidence in Candidate Beta',
    })
  ).toHaveAttribute('aria-valuenow', '99.41')
  expect(
    within(dialog).getByRole('columnheader', { name: 'Candidate confidence' })
  ).toBeVisible()
  const table = within(dialog).getByRole('table')
  expect(
    within(table).getByRole('row', { name: /Candidate Beta/ })
  ).toHaveTextContent('99.41%')
  expect(
    within(table).getByRole('row', { name: /Advertised Alpha/ })
  ).toHaveTextContent('0.59%')
})

it.each([
  {
    name: 'candidate probability is missing',
    decision: 'scored',
    prediction: 'candidate-beta',
    probability: null,
  },
  {
    name: 'only partial samples are available',
    decision: 'partial',
    prediction: 'candidate-beta',
    probability: null,
  },
  {
    name: 'no candidate was predicted',
    decision: 'not_model',
    prediction: '',
    probability: 0.9941,
  },
])(
  'does not substitute a score or unrelated probability when $name',
  async ({ decision, prediction, probability }) => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          ...results,
          results: [
            {
              ...results.results[1],
              verdict: 'inconclusive',
              analysis: {
                ...candidateAnalysis,
                decision,
                prediction,
                results: candidateAnalysis.results.map((candidate) => ({
                  ...candidate,
                  probability,
                })),
              },
            },
          ],
        },
      },
    })
    renderResults()
    const heading = await screen.findByRole('heading', { name: 'model-beta' })
    const card = heading.closest('li')
    if (!card) throw new Error('Missing result card')
    expect(within(card).getByText('Candidate confidence')).toBeVisible()
    expect(within(card).queryByRole('progressbar')).not.toBeInTheDocument()
    expect(within(card).queryByText('12%')).not.toBeInTheDocument()
    expect(within(card).queryByText('0%')).not.toBeInTheDocument()
    expect(within(card).getAllByText('—').length).toBeGreaterThan(0)
  }
)

it('shows loading instead of an empty report while the first request is pending', () => {
  vi.spyOn(api, 'get').mockReturnValue(new Promise(() => {}))
  renderResults()
  expect(screen.getByRole('status')).toHaveTextContent('Loading...')
  expect(
    screen.queryByText('No fingerprint results yet.')
  ).not.toBeInTheDocument()
})

it('recovers a failed results request with Retry', async () => {
  vi.spyOn(api, 'get')
    .mockRejectedValueOnce(new Error('Service unavailable'))
    .mockResolvedValueOnce({ data: { success: true, data: results } })
  renderResults()
  expect(await screen.findByText('Service unavailable')).toBeVisible()
  await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
  expect(
    await screen.findByRole('heading', { name: 'model-alpha' })
  ).toBeVisible()
  expect(screen.queryByText('Service unavailable')).not.toBeInTheDocument()
})

it('combines group search and verdict filters and clears them without losing report details', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: results },
  })
  renderResults()
  await screen.findByRole('heading', { name: 'model-alpha' })
  const user = userEvent.setup()
  await user.type(screen.getByRole('searchbox'), 'PREMIUM')
  expect(
    screen.queryByRole('heading', { name: 'model-alpha' })
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Status'), {
    target: { value: 'match' },
  })
  expect(screen.getByText('No results found')).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Clear filters' }))
  expect(screen.getByRole('heading', { name: 'model-alpha' })).toBeVisible()
  const beta = screen.getByRole('heading', { name: 'model-beta' }).closest('li')
  if (!beta) throw new Error('Missing result card')
  await user.click(within(beta).getByRole('button', { name: 'Details' }))
  await waitFor(() =>
    expect(screen.getByRole('dialog')).toHaveTextContent('model-beta')
  )
})

it('keeps target fields focused while editing and sends only target fields when saving', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  const target = { group: 'standard', model: 'model-alpha', reference: '' }
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        schedule: { enabled: false, minutes: 10 },
        catalog: { standard: ['model-alpha'] },
        targets: [target],
        max_targets: 20,
        base_url: '/v1',
        last_run: null,
      },
    },
  })
  const put = vi.spyOn(api, 'put').mockResolvedValue({
    data: {
      success: true,
      data: [{ ...target, reference: 'canonical-alpha' }],
    },
  })
  renderResults()
  const reference = await screen.findByLabelText('Reference model')
  const user = userEvent.setup()
  await user.type(reference, 'canonical-alpha')
  expect(reference).toHaveFocus()
  expect(reference).toHaveValue('canonical-alpha')
  await user.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/model-fingerprint', {
      schedule: { enabled: false, minutes: 10 },
      targets: [{ ...target, reference: 'canonical-alpha' }],
    })
  )
})

it('runs only the selected saved group, preserves other result cards, and can switch back to all groups', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        schedule: { enabled: true, minutes: 10 },
        catalog: {
          standard: ['model-alpha'],
          premium: ['model-beta'],
          unsaved: ['model-gamma'],
        },
        targets: results.results.map(({ group, model, reference }) => ({
          group,
          model,
          reference,
        })),
        max_targets: 8,
        base_url: '/v1',
        last_run: results,
      },
    },
  })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: results } })
  renderResults()
  await screen.findByRole('heading', { name: 'model-alpha' })
  const selector = screen.getByLabelText('Group to test')
  expect(
    within(selector).queryByRole('option', { name: 'unsaved' })
  ).not.toBeInTheDocument()
  fireEvent.change(selector, { target: { value: 'premium' } })
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Test selected group' }))
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      '/api/model-fingerprint/run',
      { group: 'premium' },
      { timeout: 520_000 }
    )
  )
  expect(screen.getByRole('heading', { name: 'model-alpha' })).toBeVisible()
  expect(screen.getByRole('heading', { name: 'model-beta' })).toBeVisible()
  fireEvent.change(selector, { target: { value: '' } })
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Run fingerprint check' }))
  await waitFor(() =>
    expect(post).toHaveBeenLastCalledWith(
      '/api/model-fingerprint/run',
      {},
      { timeout: 520_000 }
    )
  )
})

it('keeps group testing disabled until target edits are saved', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        schedule: { enabled: false, minutes: 10 },
        catalog: { standard: ['model-alpha'] },
        targets: [{ group: 'standard', model: 'model-alpha', reference: '' }],
        max_targets: 8,
        base_url: '/v1',
        last_run: null,
      },
    },
  })
  renderResults()
  const reference = await screen.findByLabelText('Reference model')
  fireEvent.change(screen.getByLabelText('Group to test'), {
    target: { value: 'standard' },
  })
  await userEvent.setup().type(reference, 'canonical-alpha')
  expect(
    screen.getByRole('button', { name: 'Test selected group' })
  ).toBeDisabled()
})
