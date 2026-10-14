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
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { ProfileLevelCard } from '../components/profile-level-card'
import type { UserLevelStatus, UserProfile } from '../types'

function buildProfile(overrides: Partial<UserProfile> = {}): UserProfile {
  return {
    id: 1,
    username: 'alice',
    display_name: 'Alice',
    role: 1,
    group: 'default',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    status: 1,
    aff_count: 0,
    aff_quota: 0,
    aff_history_quota: 0,
    created_time: 0,
    ...overrides,
  }
}

const reachedLevel: UserLevelStatus = {
  enabled: true,
  has_level: true,
  number: 2,
  name: 'Gold',
  threshold: 100,
  has_next: true,
  next_number: 3,
  next_name: 'Platinum',
  next_threshold: 300,
}

describe('profile level card', () => {
  test('shows the reached level, the cumulative recharge and the next threshold', () => {
    render(
      <ProfileLevelCard
        profile={buildProfile({
          recharge_amount: 140,
          user_level: reachedLevel,
        })}
      />
    )

    expect(screen.getByText('Lv.2 · Gold')).toBeInTheDocument()
    expect(screen.getByText('$140')).toBeInTheDocument()
    expect(screen.getByText('Next level: Platinum')).toBeInTheDocument()
    expect(
      screen.getByRole('progressbar', { name: 'Level progress' })
    ).toBeInTheDocument()
    expect(screen.getByText('$160')).toBeInTheDocument()
    expect(screen.getByText('Threshold: $300')).toBeInTheDocument()
  })

  test('reports no level yet and the full distance to the first level', () => {
    render(
      <ProfileLevelCard
        profile={buildProfile({
          recharge_amount: 0,
          user_level: {
            ...reachedLevel,
            has_level: false,
            number: 0,
            name: '',
            threshold: 0,
            next_number: 1,
            next_name: 'Gold',
          },
        })}
      />
    )

    expect(screen.getByText('No level yet')).toBeInTheDocument()
    expect(screen.getByText('Next level: Gold')).toBeInTheDocument()
    expect(screen.queryByText('Lv.1 · Gold')).toBe(null)
  })

  test('stays hidden while no level ladder is enabled', () => {
    const { container } = render(
      <ProfileLevelCard
        profile={buildProfile({
          recharge_amount: 150,
          user_level: { ...reachedLevel, enabled: false },
        })}
      />
    )

    expect(container).toBeEmptyDOMElement()
  })
})
