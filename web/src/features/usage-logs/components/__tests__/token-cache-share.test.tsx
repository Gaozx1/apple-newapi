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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'

import { usageLogSchema, type UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { useCommonLogsColumns } from '../columns/common-logs-columns'

function makeLog(other: LogOtherData, promptTokens: number): UsageLog {
  return usageLogSchema.parse({
    id: 1,
    user_id: 2,
    created_at: 1788840000,
    type: 2,
    content: '',
    model_name: 'gpt-image-2',
    prompt_tokens: promptTokens,
    completion_tokens: 10,
    other: JSON.stringify(other),
  })
}

function TokensCell(props: { other: LogOtherData; promptTokens: number }) {
  const table = useReactTable({
    data: [makeLog(props.other, props.promptTokens)],
    columns: useCommonLogsColumns(false, false),
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getAllCells()
    .find((item) => item.column.id === 'prompt_tokens')
  if (!cell) throw new Error('The log must have a tokens column')
  return <>{flexRender(cell.column.columnDef.cell, cell.getContext())}</>
}

const cacheShareCases: Array<{
  name: string
  other: LogOtherData
  promptTokens: number
  expected: string
}> = [
  {
    name: 'the reported prompt already includes cached tokens',
    other: { cache_tokens: 300 },
    promptTokens: 1200,
    expected: 'Cache↓ 300 (25%)',
  },
  {
    name: 'Anthropic usage reports only the uncached input',
    other: { claude: true, cache_tokens: 300 },
    promptTokens: 700,
    expected: 'Cache↓ 300 (30%)',
  },
]

it.each(cacheShareCases)(
  'shows the cache read share next to the cached token count when $name',
  ({ other, promptTokens, expected }) => {
    render(<TokensCell other={other} promptTokens={promptTokens} />)
    expect(screen.getByText(expected)).toBeVisible()
  }
)

it('omits the cache share when the request read nothing from the cache', () => {
  render(
    <TokensCell other={{ cache_creation_tokens: 500 }} promptTokens={1200} />
  )
  expect(screen.queryByText(/Cache/)).not.toBeInTheDocument()
})
