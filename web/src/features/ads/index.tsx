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
import { useMutation, useQuery } from '@tanstack/react-query'
import { Coins, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  getAdConfig,
  getNextAd,
  recordAdView,
  type AdItem,
  type AdViewResult,
} from './api'
import { AdSlot } from './components/ad-slot'

const AD_VIEW_SECONDS = 10

/** Rewarded ads page: watch, then claim the configured balance reward. */
export function AdsPage() {
  const { t } = useTranslation()
  const [ad, setAd] = useState<AdItem | null>(null)
  const [remainingSeconds, setRemainingSeconds] = useState(0)
  const [lastReward, setLastReward] = useState<AdViewResult | null>(null)

  const configQuery = useQuery({
    queryKey: ['ads', 'config'],
    queryFn: getAdConfig,
  })

  const nextAdQuery = useQuery({
    queryKey: ['ads', 'next-reward'],
    queryFn: async () => (await getNextAd('reward')).ad,
    enabled: configQuery.data?.enabled === true,
  })

  const currentAd = ad ?? nextAdQuery.data ?? null

  const viewMutation = useMutation({
    mutationFn: (adId: number) => recordAdView(adId, 'reward'),
    onSuccess: (result) => {
      setLastReward(result)
      toast.success(
        t('Reward credited: {{amount}}', {
          amount: formatCurrencyFromUSD(result.reward_amount),
        })
      )
      setAd(null)
      setRemainingSeconds(0)
      void nextAdQuery.refetch()
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Failed to claim reward'))),
  })

  const startTimer = () => {
    if (currentAd === null || remainingSeconds > 0) return
    setRemainingSeconds(AD_VIEW_SECONDS)
    const timer = window.setInterval(() => {
      setRemainingSeconds((current) => {
        if (current <= 1) {
          window.clearInterval(timer)
          return 0
        }
        return current - 1
      })
    }, 1000)
  }

  const config = configQuery.data
  const loading = configQuery.isLoading || nextAdQuery.isLoading

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Watch ads')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-4'>
          <Card>
            <CardHeader className='gap-2'>
              <CardTitle className='flex items-center gap-2'>
                <Coins className='size-4' aria-hidden='true' />
                {t('Watch an ad, earn balance')}
              </CardTitle>
              <p className='text-muted-foreground text-sm'>
                {config === undefined
                  ? ''
                  : t('Each completed view credits {{amount}}.', {
                      amount: formatCurrencyFromUSD(config.reward_amount),
                    })}
              </p>
            </CardHeader>
            <CardContent className='grid gap-4'>
              {loading && <Spinner className='size-4' />}

              {!loading && config?.enabled !== true && (
                <p className='text-muted-foreground text-sm'>
                  {t('Ads are not enabled by the administrator yet.')}
                </p>
              )}

              {!loading && config?.enabled === true && currentAd === null && (
                <p className='text-muted-foreground text-sm'>
                  {t('No ads available right now. Please come back later.')}
                </p>
              )}

              {currentAd !== null && (
                <>
                  <AdSlot ad={currentAd} />
                  <div className='flex flex-wrap items-center gap-3'>
                    <Button
                      type='button'
                      variant='outline'
                      onClick={() => {
                        setAd(null)
                        setRemainingSeconds(0)
                        void nextAdQuery.refetch()
                      }}
                    >
                      <RefreshCw className='size-4' aria-hidden='true' />
                      {t('Next ad')}
                    </Button>
                    <Button
                      type='button'
                      disabled={viewMutation.isPending || remainingSeconds > 0}
                      onClick={() => {
                        if (remainingSeconds === 0) {
                          startTimer()
                          return
                        }
                        viewMutation.mutate(currentAd.id)
                      }}
                    >
                      {viewMutation.isPending && <Spinner className='size-3' />}
                      {remainingSeconds === 0 && t('Watch ad')}
                      {remainingSeconds > 0 &&
                        t('Claim in {{seconds}}s', {
                          seconds: remainingSeconds,
                        })}
                      {remainingSeconds < 0 && t('Claim reward')}
                    </Button>
                    <span className='text-muted-foreground text-xs'>
                      {t(
                        'The reward can be claimed after the countdown finishes.'
                      )}
                    </span>
                  </div>
                </>
              )}

              {lastReward !== null && (
                <p className='text-muted-foreground text-sm'>
                  {t('Last reward: {{amount}} · views today: {{count}}', {
                    amount: formatCurrencyFromUSD(lastReward.reward_amount),
                    count: lastReward.views_today,
                  })}
                </p>
              )}
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
