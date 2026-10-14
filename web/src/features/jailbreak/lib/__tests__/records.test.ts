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
import { describe, expect, it } from 'vitest'

import {
  actionLabelKey,
  contentPreview,
  jailbreakActionVariant,
} from '../records'

describe('jailbreak record display helpers', () => {
  it('maps the stored action to its label key and badge variant', () => {
    expect(actionLabelKey('banned')).toBe('Account banned')
    expect(jailbreakActionVariant('banned')).toBe('danger')

    expect(actionLabelKey('already_banned')).toBe('Account already banned')
    expect(jailbreakActionVariant('already_banned')).toBe('warning')

    expect(actionLabelKey('skipped_staff')).toBe('Administrator, not banned')
    expect(jailbreakActionVariant('skipped_staff')).toBe('neutral')
  })

  it('keeps an unknown action readable instead of showing a raw value', () => {
    expect(actionLabelKey('something_new')).toBe('Recorded')
    expect(jailbreakActionVariant('something_new')).toBe('neutral')
  })

  it('collapses whitespace and truncates long user content for the table', () => {
    expect(contentPreview('  line one\n\nline   two  ')).toBe(
      'line one line two'
    )
    expect(contentPreview('x'.repeat(200), 10)).toBe('xxxxxxxxxx…')
    expect(contentPreview('')).toBe('')
  })
})
