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
import { describe, expect, it } from 'vitest'

import { SubscriptionWindowUsage } from '../subscription-window-usage'

const fiveHour = {
  limit: 1_000_000,
  used: 250_000,
  remaining: 750_000,
  reset_at: 1_700_003_600,
}

describe('SubscriptionWindowUsage', () => {
  it('renders nothing when the plan limits no window', () => {
    const { container } = render(<SubscriptionWindowUsage />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows each limited window with usage, remaining quota and reset time', () => {
    render(
      <SubscriptionWindowUsage
        window5h={fiveHour}
        window7d={{ ...fiveHour, limit: 5_000_000, used: 1_000_000 }}
      />
    )

    const fiveHourRow = screen.getByText('5-hour limit').parentElement
    expect(fiveHourRow).toHaveTextContent('0.5')
    expect(fiveHourRow).toHaveTextContent('2')
    expect(fiveHourRow).toHaveTextContent('Remaining')
    expect(fiveHourRow).toHaveTextContent('Next reset')

    expect(screen.getByText('7-day limit').parentElement).toHaveTextContent(
      '10'
    )
  })

  it('omits a row for a window the plan does not limit', () => {
    render(<SubscriptionWindowUsage window7d={fiveHour} />)

    expect(screen.queryByText('5-hour limit')).not.toBeInTheDocument()
    expect(screen.getByText('7-day limit')).toBeInTheDocument()
  })
})
