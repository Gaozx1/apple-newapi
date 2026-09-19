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
// Per-model consumption of one subscription. The plan's buckets
// (`model_quotas` / `model_token_quotas`) and the subscription's usage
// (`model_used` / `token_used`) are keyed by model name; either side may miss
// keys, so the rows are the union of both.

export type ModelUsageUnit = 'quota' | 'token'

export interface ModelUsageRow {
  model: string
  /** Consumed amount: quota units, or raw tokens when unit is 'token'. */
  used: number
  /** Bucket size; 0 when the model has no bucket in the plan. */
  limit: number
  /** Unused bucket amount; null when the model has no bucket. */
  remaining: number | null
  /** Share of this subscription's total consumption for the same unit. */
  share: number
}

export function buildModelUsageRows(params: {
  quotas?: Record<string, number> | null
  used?: Record<string, number> | null
}): ModelUsageRow[] {
  const quotas = params.quotas || {}
  const used = params.used || {}
  const models = new Set([...Object.keys(quotas), ...Object.keys(used)])

  let totalUsed = 0
  for (const model of models) {
    totalUsed += usageValue(used[model])
  }

  const rows: ModelUsageRow[] = []
  for (const model of models) {
    const usedAmount = usageValue(used[model])
    const limit = usageValue(quotas[model])
    rows.push({
      model,
      used: usedAmount,
      limit,
      remaining: limit > 0 ? Math.max(0, limit - usedAmount) : null,
      share: totalUsed > 0 ? usedAmount / totalUsed : 0,
    })
  }

  rows.sort((a, b) => b.used - a.used || a.model.localeCompare(b.model))
  return rows
}

// usageValue normalizes a bucket/usage entry: malformed or negative values
// count as no consumption instead of producing a negative row.
function usageValue(value: number | undefined): number {
  const amount = Number(value)
  if (!Number.isFinite(amount) || amount <= 0) return 0
  return amount
}
