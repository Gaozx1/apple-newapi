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
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import { drawLottery, getLotteryStatus } from './api'
import type { LotteryPrizeType, LotteryRecord } from './types'

function LotteryPrizeCard({ label, weight }: { label: string; weight: number }) {
  const { t } = useTranslation()
  return (
    <div className='bg-muted/40 flex items-center justify-between gap-2 rounded-lg px-3 py-2'>
      <span className='text-sm font-medium'>{t(label)}</span>
      <span className='text-muted-foreground text-sm tabular-nums'>
        {weight}%
      </span>
    </div>
  )
}

function LotteryResultBanner({
  result,
}: {
  result: {
    prize_type: LotteryPrizeType
    prize_label: string
    prize_value: number
    rebate_rate: number
    cost_quota: number
    free_draw: boolean
  } | null
}) {
  const { t } = useTranslation()
  if (!result) return null
  const isWin = result.prize_type !== 'thanks'
  return (
    <Card
      data-card-hover='false'
      className={isWin ? 'border-success/50' : 'border-muted'}
    >
      <CardContent className='p-4'>
        <div className='flex items-center gap-3'>
          <span
            className={
              isWin
                ? 'text-success text-lg font-semibold'
                : 'text-muted-foreground text-lg font-semibold'
            }
          >
            {t(result.prize_label)}
          </span>
        </div>
        {result.prize_type === 'balance' && (
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('You won')} {formatQuota(result.prize_value)}
          </p>
        )}
        {result.prize_type === 'coupon' && (
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Recharge rebate coupon')} {result.rebate_rate}%
          </p>
        )}
        {result.free_draw && (
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Used a free draw chance')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

export function Lottery() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['lottery-status'],
    queryFn: getLotteryStatus,
  })
  const status = data?.data
  const drawMutation = useMutation({
    mutationFn: () => drawLottery(),
    onSuccess: (res) => {
      if (res.success && res.data) {
        queryClient.invalidateQueries({ queryKey: ['lottery-status'] })
        toast.success(t('Draw successful'))
      } else {
        toast.error(res.message || t('Draw failed'))
      }
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : t('Draw failed')
      toast.error(msg)
    },
  })

  if (isLoading) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Lottery')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <Skeleton className='h-40 w-full rounded-xl' />
          <Skeleton className='mt-4 h-60 w-full rounded-xl' />
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  if (!status?.enabled) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Lottery')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <Card data-card-hover='false'>
            <CardContent className='p-6 text-center text-muted-foreground'>
              {t('Lottery is not enabled')}
            </CardContent>
          </Card>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  const costQuota = status.cost_quota || 0
  const chances = status.chances || 0
  const canDraw = chances > 0 || costQuota > 0

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Lottery')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
        <Card data-card-hover='false'>
          <CardHeader>
            <CardTitle>{t('Lottery Draw')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='text-muted-foreground flex flex-wrap gap-x-6 gap-y-1 text-sm'>
              <span>
                {t('Free draw chances')}:{' '}
                <span className='font-semibold text-foreground'>{chances}</span>
              </span>
              <span>
                {t('Draw cost')}:{' '}
                <span className='font-semibold text-foreground'>
                  {formatQuota(costQuota)}
                </span>
              </span>
              <span>
                {t('Unused coupons')}:{' '}
                <span className='font-semibold text-foreground'>
                  {status.coupon_count || 0}
                </span>
              </span>
            </div>

            <LotteryResultBanner result={drawMutation.data?.data ?? null} />

            <Button
              onClick={() => drawMutation.mutate()}
              disabled={!canDraw || drawMutation.isPending}
              className='w-full sm:w-auto'
            >
              {drawMutation.isPending ? t('Drawing...') : t('Draw')}
            </Button>
          </CardContent>
        </Card>

        <Card data-card-hover='false'>
          <CardHeader>
            <CardTitle>{t('Prize Table')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
              <LotteryPrizeCard label='$10 Balance' weight={0.1} />
              <LotteryPrizeCard label='$7 Balance' weight={3} />
              <LotteryPrizeCard label='$5 Balance' weight={5.5} />
              <LotteryPrizeCard label='$3 Balance' weight={6} />
              <LotteryPrizeCard label='$1 Balance' weight={6.5} />
              <LotteryPrizeCard label='5% Recharge Coupon' weight={7} />
              <LotteryPrizeCard label='3% Recharge Coupon' weight={9.4} />
              <LotteryPrizeCard label='2% Recharge Coupon' weight={13} />
              <LotteryPrizeCard label='1% Recharge Coupon' weight={16.5} />
              <LotteryPrizeCard label='Thanks for participating' weight={33} />
            </div>
          </CardContent>
        </Card>

        <Card data-card-hover='false'>
          <CardHeader>
            <CardTitle>{t('Recent Draws')}</CardTitle>
          </CardHeader>
          <CardContent>
            {status.records.length === 0 ? (
              <p className='text-muted-foreground text-sm'>{t('No records yet')}</p>
            ) : (
              <div className='divide-y'>
                {status.records.map((record: LotteryRecord) => (
                  <div
                    key={record.id}
                    className='flex items-center justify-between py-2 text-sm'
                  >
                    <span className='font-medium'>{t(record.prize_label)}</span>
                    <span className='text-muted-foreground tabular-nums'>
                      {new Date(record.created_at * 1000).toLocaleString()}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
