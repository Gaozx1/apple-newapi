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
import { describe, expect, test } from 'vitest'

import { buildModelUsageRows } from '../model-usage'

describe('buildModelUsageRows', () => {
  test('shares come from the models the subscription actually consumed', () => {
    const rows = buildModelUsageRows({
      quotas: { 'model-a': 100, 'model-b': 200, 'model-c': 50 },
      used: { 'model-a': 60, 'model-b': 20 },
    })

    expect(rows.map((row) => row.model)).toEqual([
      'model-a',
      'model-b',
      'model-c',
    ])
    expect(rows[0]).toMatchObject({
      used: 60,
      limit: 100,
      remaining: 40,
      share: 0.75,
    })
    expect(rows[1]).toMatchObject({
      used: 20,
      limit: 200,
      remaining: 180,
      share: 0.25,
    })
    // An untouched bucket is listed with a zero share so the plan's full
    // allocation stays visible.
    expect(rows[2]).toMatchObject({
      used: 0,
      limit: 50,
      remaining: 50,
      share: 0,
    })
  })

  test('an overspent bucket floors remaining at zero instead of going negative', () => {
    const rows = buildModelUsageRows({
      quotas: { 'model-a': 100 },
      used: { 'model-a': 130 },
    })

    expect(rows[0]).toMatchObject({ used: 130, limit: 100, remaining: 0 })
    expect(rows[0].share).toBe(1)
  })

  test('a model without a bucket keeps a null remaining and full share', () => {
    const rows = buildModelUsageRows({
      quotas: { 'model-a': 100 },
      used: { 'model-a': 10, 'model-x': 30 },
    })

    const unbucketed = rows.find((row) => row.model === 'model-x')
    expect(unbucketed).toMatchObject({ used: 30, limit: 0, remaining: null })
    expect(unbucketed?.share).toBe(0.75)
  })

  test('an empty denominator yields zero shares and no NaN', () => {
    const rows = buildModelUsageRows({ quotas: { 'model-a': 100 }, used: {} })

    expect(rows).toHaveLength(1)
    expect(rows[0].share).toBe(0)
    expect(rows[0].remaining).toBe(100)
  })

  test('malformed and negative entries are treated as no consumption', () => {
    const rows = buildModelUsageRows({
      quotas: { 'model-a': 100, 'model-b': Number.NaN, 'model-c': -5 },
      used: { 'model-a': 40, 'model-b': -10 },
    })

    expect(rows.find((row) => row.model === 'model-a')).toMatchObject({
      used: 40,
      limit: 100,
      remaining: 60,
      share: 1,
    })
    expect(rows.find((row) => row.model === 'model-b')).toMatchObject({
      used: 0,
      limit: 0,
      remaining: null,
      share: 0,
    })
    expect(rows.find((row) => row.model === 'model-c')).toMatchObject({
      used: 0,
      limit: 0,
      remaining: null,
    })
  })

  test('missing inputs produce no rows', () => {
    expect(buildModelUsageRows({})).toEqual([])
    expect(buildModelUsageRows({ quotas: null, used: null })).toEqual([])
    expect(buildModelUsageRows({ quotas: {}, used: {} })).toEqual([])
  })
})
