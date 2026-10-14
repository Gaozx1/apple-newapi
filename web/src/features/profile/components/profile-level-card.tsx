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
import { Crown } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Progress } from '@/components/ui/progress'
import { TitledCard } from '@/components/ui/titled-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatCurrencyFromUSD } from '@/lib/currency'

import type { UserProfile } from '../types'

// ============================================================================
// User Level Card
//
// Shows the ladder rung the user reached from their cumulative recharge amount,
// the amount itself, and how far the next rung is. The ladder and its names are
// configured by the administrator; the card stays hidden while no ladder is
// active.
// ============================================================================

interface ProfileLevelCardProps {
  profile: UserProfile | null
}

export function ProfileLevelCard(props: ProfileLevelCardProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const level = props.profile?.user_level

  if (!level?.enabled) return null

  const rechargeAmount = Number(props.profile?.recharge_amount || 0)
  const formatAmount = (amount: number) =>
    formatCurrencyFromUSD(amount, { locale })

  const currentLabel = level.has_level
    ? `Lv.${level.number} · ${level.name}`
    : t('No level yet')

  const span = level.next_threshold - level.threshold
  const progress = level.has_next
    ? Math.min(
        100,
        Math.max(
          0,
          span > 0 ? ((rechargeAmount - level.threshold) / span) * 100 : 100
        )
      )
    : 100

  return (
    <TitledCard
      title={t('Level')}
      description={t('Based on your cumulative recharge amount')}
      icon={<Crown className='h-4 w-4' />}
      iconTone='warning'
      disableHoverEffect
    >
      <div className='space-y-3'>
        <div className='flex flex-wrap items-center gap-2'>
          <StatusBadge label={currentLabel} variant='info' copyable={false} />
        </div>

        <div className='grid grid-cols-2 gap-3'>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
              {t('Cumulative recharge')}
            </div>
            <div className='mt-1 truncate font-mono text-lg font-bold tabular-nums'>
              {formatAmount(rechargeAmount)}
            </div>
          </div>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
              {t('Level')}
            </div>
            <div className='mt-1 truncate text-lg font-bold'>
              {level.has_level ? level.name : '—'}
            </div>
          </div>
        </div>

        {level.has_next ? (
          <div className='space-y-1.5'>
            <div className='text-muted-foreground flex flex-wrap items-center justify-between gap-2 text-xs'>
              <span>
                {t('Next level: {{name}}', { name: level.next_name })}
              </span>
              <span className='font-mono tabular-nums'>
                {formatAmount(
                  Math.max(0, level.next_threshold - rechargeAmount)
                )}
              </span>
            </div>
            <Progress value={progress} aria-label={t('Level progress')} />
            <div className='text-muted-foreground text-xs'>
              {t('Threshold')}: {formatAmount(level.next_threshold)}
            </div>
          </div>
        ) : (
          <p className='text-muted-foreground text-xs'>
            {t('You have reached the highest level')}
          </p>
        )}
      </div>
    </TitledCard>
  )
}
