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

import { formatQuota } from '@/lib/format'

import type { SubscriptionWindowUsage as WindowUsage } from '../types'

interface Props {
  /** Rolling 5-hour and 7-day windows of one subscription; either may be absent. */
  window5h?: WindowUsage
  window7d?: WindowUsage
}

/**
 * The subscription's rolling window limits: each row shows used/limit, the
 * remaining quota and when the oldest usage leaves the window. Only plans that
 * cap a window have this data.
 */
export function SubscriptionWindowUsage(props: Props) {
  const { t } = useTranslation()
  const rows: { key: string; label: string; usage: WindowUsage }[] = []
  if (props.window5h) {
    rows.push({ key: '5h', label: t('5-hour limit'), usage: props.window5h })
  }
  if (props.window7d) {
    rows.push({ key: '7d', label: t('7-day limit'), usage: props.window7d })
  }

  if (rows.length === 0) return null

  return (
    <div className='mt-2 space-y-1'>
      <div className='text-muted-foreground font-medium'>
        {t('Sliding window limits')}
      </div>
      {rows.map((row) => (
        <div
          key={row.key}
          className='flex flex-wrap items-baseline justify-between gap-x-2 gap-y-0.5 text-xs'
        >
          <span className='min-w-0 truncate font-medium'>{row.label}</span>
          <span className='text-muted-foreground shrink-0 tabular-nums'>
            {formatQuota(row.usage.used)}/{formatQuota(row.usage.limit)}
            {' · '}
            {t('Remaining')} {formatQuota(row.usage.remaining)}
            {row.usage.reset_at > 0 && (
              <>
                {' · '}
                {t('Next reset')}:{' '}
                {new Date(row.usage.reset_at * 1000).toLocaleString()}
              </>
            )}
          </span>
        </div>
      ))}
    </div>
  )
}
