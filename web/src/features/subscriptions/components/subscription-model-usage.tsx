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
import { useTranslation } from 'react-i18next'

import { formatPercent, formatQuota } from '@/lib/format'

import { buildModelUsageRows, type ModelUsageUnit } from '../lib/model-usage'
import type { UserSubscriptionRecord } from '../types'

interface Props {
  record: UserSubscriptionRecord
}

/**
 * Per-model consumption of one subscription: each row shows used/limit, the
 * remaining bucket and the model's share of this subscription's consumption
 * in the same unit. Only plans with per-model buckets have this data.
 */
export function SubscriptionModelUsage(props: Props) {
  const { t } = useTranslation()
  const quotas = props.record.model_quotas
  const tokenQuotas = props.record.model_token_quotas
  const hasQuotaBuckets = Boolean(quotas && Object.keys(quotas).length > 0)
  const hasTokenBuckets = Boolean(
    tokenQuotas && Object.keys(tokenQuotas).length > 0
  )

  if (!hasQuotaBuckets && !hasTokenBuckets) return null

  return (
    <div className='mt-2 space-y-2'>
      {hasQuotaBuckets && (
        <ModelUsageList
          title={t('Per-model quota buckets')}
          unit='quota'
          quotas={quotas}
          used={props.record.model_used}
        />
      )}
      {hasTokenBuckets && (
        <ModelUsageList
          title={t('Per-model token buckets')}
          unit='token'
          quotas={tokenQuotas}
          used={props.record.token_used}
        />
      )}
    </div>
  )
}

function ModelUsageList(props: {
  title: string
  unit: ModelUsageUnit
  quotas?: Record<string, number>
  used?: Record<string, number>
}) {
  const { t } = useTranslation()
  const rows = buildModelUsageRows({ quotas: props.quotas, used: props.used })
  const showTokens = props.unit === 'token'
  const unitSuffix = showTokens ? ` ${t('tokens')}` : ''
  const formatAmount = (value: number) =>
    showTokens ? value.toLocaleString() : formatQuota(value)

  return (
    <div className='space-y-1'>
      <div className='text-muted-foreground font-medium'>{props.title}</div>
      {rows.map((row) => {
        const usedLabel = formatAmount(row.used)
        const amountLabel =
          row.limit > 0 ? `${usedLabel}/${formatAmount(row.limit)}` : usedLabel
        return (
          <div
            key={row.model}
            className='flex flex-wrap items-baseline justify-between gap-x-2 gap-y-0.5 text-xs'
          >
            <span className='min-w-0 truncate font-medium'>{row.model}</span>
            <span className='text-muted-foreground shrink-0 tabular-nums'>
              {amountLabel}
              {unitSuffix}
              {row.remaining !== null && (
                <>
                  {' · '}
                  {t('Remaining')} {formatAmount(row.remaining)}
                </>
              )}
              {' · '}
              {t('Share')} {formatPercent(row.share * 100)}
            </span>
          </div>
        )
      })}
    </div>
  )
}
