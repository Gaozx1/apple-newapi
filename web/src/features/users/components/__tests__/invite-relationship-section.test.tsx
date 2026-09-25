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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { InviteRelationshipSection } from '../invite-relationship-section'

const previewInviterRebate = vi.fn()
const bindInviter = vi.fn()

vi.mock('../../api', () => ({
  previewInviterRebate: (...args: unknown[]) => previewInviterRebate(...args),
  bindInviter: (...args: unknown[]) => bindInviter(...args),
}))

const preview = {
  success: true,
  data: {
    user_id: 2,
    username: 'invitee',
    current_inviter_id: 0,
    inviter_id: 9,
    inviter_username: 'referrer',
    topup_quota: 1_500_000,
    redemption_quota: 500_000,
    base_quota: 2_000_000,
    rebate_rate: 5,
    rebate_quota: 100_000,
  },
}

function renderSection(onBound = vi.fn()) {
  render(
    <InviteRelationshipSection
      userId={2}
      username='invitee'
      currentInviterId={0}
      onBound={onBound}
    />
  )
  return onBound
}

async function previewThenOpenConfirm() {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Inviter'), 'referrer')
  await user.click(screen.getByRole('button', { name: 'Preview rebate' }))
  await waitFor(() =>
    expect(screen.getByText('Rebate to credit')).toBeInTheDocument()
  )
  await user.click(screen.getByRole('button', { name: 'Bind inviter' }))
  await waitFor(() =>
    expect(screen.getByText('Confirm inviter binding')).toBeInTheDocument()
  )
  return user
}

afterEach(() => {
  cleanup()
  previewInviterRebate.mockReset()
  bindInviter.mockReset()
})

describe('InviteRelationshipSection', () => {
  it('shows the previewed recharge base and rebate before writing anything', async () => {
    previewInviterRebate.mockResolvedValue(preview)
    renderSection()

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Inviter'), 'referrer')
    await user.click(screen.getByRole('button', { name: 'Preview rebate' }))

    await waitFor(() =>
      expect(previewInviterRebate).toHaveBeenCalledWith(2, 'referrer')
    )
    expect(await screen.findByText('Recharge orders')).toBeInTheDocument()
    expect(screen.getByText('Redemption codes')).toBeInTheDocument()
    expect(screen.getByText('5%')).toBeInTheDocument()
    // Preview is read-only: nothing is bound until the admin confirms.
    expect(bindInviter).not.toHaveBeenCalled()
  })

  it('submits the confirmed binding with the historical rebate applied', async () => {
    previewInviterRebate.mockResolvedValue(preview)
    bindInviter.mockResolvedValue({
      success: true,
      data: { inviter_id: 9, inviter_username: 'referrer', rebate_quota: 100_000 },
    })
    const onBound = renderSection()

    const user = await previewThenOpenConfirm()
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() =>
      expect(bindInviter).toHaveBeenCalledWith(2, 'referrer', true)
    )
    await waitFor(() => expect(onBound).toHaveBeenCalled())
  })

  it('drops a stale preview when the inviter identifier changes', async () => {
    previewInviterRebate.mockResolvedValue(preview)
    renderSection()

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Inviter'), 'referrer')
    await user.click(screen.getByRole('button', { name: 'Preview rebate' }))
    await waitFor(() =>
      expect(screen.getByText('Rebate to credit')).toBeInTheDocument()
    )

    await user.type(screen.getByLabelText('Inviter'), '-other')

    // The old amount must not stay confirmable for a different inviter.
    expect(screen.queryByText('Rebate to credit')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Bind inviter' })).toBeDisabled()
  })

  it('skips the rebate when the administrator unchecks it', async () => {
    previewInviterRebate.mockResolvedValue(preview)
    bindInviter.mockResolvedValue({
      success: true,
      data: { inviter_id: 9, inviter_username: 'referrer', rebate_quota: 0 },
    })
    renderSection()

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Inviter'), 'referrer')
    await user.click(screen.getByRole('button', { name: 'Preview rebate' }))
    await waitFor(() =>
      expect(screen.getByText('Rebate to credit')).toBeInTheDocument()
    )

    await user.click(
      screen.getByRole('checkbox', {
        name: 'Also credit the historical rebate to the inviter',
      })
    )
    await user.click(screen.getByRole('button', { name: 'Bind inviter' }))
    await waitFor(() =>
      expect(screen.getByText('Confirm inviter binding')).toBeInTheDocument()
    )
    await user.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() =>
      expect(bindInviter).toHaveBeenCalledWith(2, 'referrer', false)
    )
  })
})
