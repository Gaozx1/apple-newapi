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

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { RankingPeriod } from '../types'

const PERIODS: { id: RankingPeriod; labelKey: string }[] = [
  { id: 'today', labelKey: 'Today' },
  { id: 'week', labelKey: 'Week' },
  { id: 'month', labelKey: 'Month' },
  { id: 'year', labelKey: 'Year' },
]

type RankingsHeroProps = {
  period: RankingPeriod
  onPeriodChange: (period: RankingPeriod) => void
}

/**
 * Period selector for the rankings page. The page title and subtitle are
 * rendered by `PublicPageHero`; this component only owns the tab strip.
 */
export function RankingsHero(props: RankingsHeroProps) {
  const { t } = useTranslation()

  return (
    <Tabs
      value={props.period}
      onValueChange={(value) => {
        if (typeof value === 'string') {
          props.onPeriodChange(value as RankingPeriod)
        }
      }}
    >
      <TabsList
        variant='line'
        activateOnFocus
        aria-label={t('Period')}
        className='border-border/60 h-auto w-full justify-start border-b pb-0'
      >
        {PERIODS.map((period) => (
          <TabsTrigger
            key={period.id}
            value={period.id}
            className='h-9 flex-none px-3 text-sm'
          >
            {t(period.labelKey)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}
