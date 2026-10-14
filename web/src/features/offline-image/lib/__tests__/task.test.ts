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
  isOfflineImageTaskActive,
  offlineImageStatusLabel,
  offlineImageStatusVariant,
} from '../task'

describe('offline image task status presentation', () => {
  it.each([
    ['PENDING', 'Queued', 'secondary'],
    ['QUEUED', 'Queued', 'secondary'],
    ['RUNNING', 'Generating', 'warning'],
    ['SUCCESS', 'Completed', 'default'],
    ['FAILURE', 'Failed', 'destructive'],
  ])('renders %s as %s', (status, label, variant) => {
    expect(offlineImageStatusLabel(status)).toBe(label)
    expect(offlineImageStatusVariant(status)).toBe(variant)
  })

  it('falls back to a queued label for an unknown status', () => {
    expect(offlineImageStatusLabel('WHATEVER')).toBe('Queued')
    expect(offlineImageStatusVariant('WHATEVER')).toBe('secondary')
  })
})

describe('isOfflineImageTaskActive', () => {
  it.each(['PENDING', 'QUEUED', 'RUNNING'])(
    'keeps polling while the task is %s',
    (status) => {
      expect(isOfflineImageTaskActive({ status })).toBe(true)
    }
  )

  it.each(['SUCCESS', 'FAILURE'])(
    'stops polling once the task is %s',
    (status) => {
      expect(isOfflineImageTaskActive({ status })).toBe(false)
    }
  )
})
