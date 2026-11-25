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
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, Ban, CircleAlert, Shield } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { PublicLayout } from '@/components/layout'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Pagination } from '@/components/ui/pagination'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'
import { api } from '@/lib/api'
import { formatNumber, formatTimestampToDate } from '@/lib/format'

type PublicBan = {
  account: string
  source: 'manual' | 'jailbreak' | 'legacy'
  reason?: string
  banned_at?: number
}

type BanPage = { items: PublicBan[]; has_more: boolean }

export function UserBanBoard() {
  const { t, i18n } = useTranslation()
  const [page, setPage] = useState(1)
  const { data, isPending, isError, isFetching, refetch } = useQuery({
    queryKey: ['public-user-bans', page],
    queryFn: async ({ signal }) => {
      const { data: response } = await api.get<{
        success: boolean
        data?: BanPage
      }>('/api/user-bans', {
        params: { p: page },
        signal,
        skipErrorHandler: true,
      })
      if (!response.success || !response.data) {
        throw new Error('Unable to load account bans')
      }
      return response.data
    },
    retry: false,
  })
  const sourceLabels = {
    manual: t('Manual ban'),
    jailbreak: t('Automatic ban'),
    legacy: t('Historical ban'),
  }

  return (
    <PublicLayout>
      <div className='mx-auto max-w-6xl space-y-7 pb-8'>
        <section className='site-ban-cover px-5 py-8 sm:px-10 sm:py-11'>
          <div className='flex items-center gap-4 sm:gap-6'>
            <div className='border-border/95 text-primary grid size-12 shrink-0 place-items-center rounded-xl border sm:size-16'>
              <Shield
                aria-hidden
                className='size-6 sm:size-8'
                strokeWidth={1.5}
              />
            </div>
            <div>
              <h1 className='site-display text-3xl font-semibold tracking-tight sm:text-5xl'>
                {t('Ban Board')}
              </h1>
              <p className='text-muted-foreground mt-2 max-w-xl text-sm leading-relaxed sm:text-base'>
                {t('Active account bans, ordered by most recent action.')}
              </p>
            </div>
          </div>
        </section>

        <section
          aria-label={t('Ban Board')}
          aria-busy={isFetching}
          aria-live='polite'
        >
          {isPending && (
            <div className='grid gap-4 md:grid-cols-2'>
              <span className='sr-only'>{t('Loading...')}</span>
              {Array.from({ length: 6 }, (_, index) => (
                <Skeleton
                  key={index}
                  aria-hidden
                  className='h-36 rounded-2xl'
                />
              ))}
            </div>
          )}
          {isError && (
            <div className='space-y-3'>
              <Alert variant='destructive' className='px-4 py-3'>
                <CircleAlert aria-hidden />
                <AlertTitle>{t('Unable to load account bans')}</AlertTitle>
              </Alert>
              <Button
                variant='outline'
                disabled={isFetching}
                onClick={() => void refetch()}
              >
                {t('Try again')}
              </Button>
            </div>
          )}
          {!isPending && !isError && !data.items.length && (
            <EmptyState
              icon={Ban}
              title={t('No active account bans')}
              bordered
            />
          )}
          {!isPending && !isError && data.items.length > 0 && (
            <ul className='studio-ban-list'>
              {data.items.map((ban) => (
                <li key={ban.account} className='min-w-0'>
                  <Card className='studio-ban-entry bg-card h-full border shadow-none ring-0'>
                    <CardContent className='space-y-4 px-4 sm:px-6'>
                      <div className='flex items-start justify-between gap-3'>
                        <bdi className='text-foreground min-w-0 font-mono text-base font-semibold wrap-anywhere'>
                          {ban.account}
                        </bdi>
                        <Badge variant='secondary' className='h-auto shrink-0'>
                          {sourceLabels[ban.source] ?? sourceLabels.legacy}
                        </Badge>
                      </div>
                      <p
                        className='text-sm leading-relaxed wrap-anywhere whitespace-pre-line'
                        dir='auto'
                      >
                        {ban.source === 'jailbreak'
                          ? t('Anti-jailbreak policy triggered')
                          : ban.reason || t('No recorded reason')}
                      </p>
                      {ban.banned_at ? (
                        <time
                          className='text-muted-foreground block border-t pt-3 text-xs tabular-nums'
                          dateTime={new Date(
                            ban.banned_at * 1000
                          ).toISOString()}
                        >
                          {formatTimestampToDate(ban.banned_at)}
                        </time>
                      ) : null}
                    </CardContent>
                  </Card>
                </li>
              ))}
            </ul>
          )}
        </section>

        {(page > 1 || data?.has_more) && (
          <Pagination
            aria-label={t('Pagination')}
            className='flex flex-wrap items-center justify-between gap-2 border-t pt-4'
          >
            <Button
              variant='outline'
              disabled={page === 1}
              onClick={() => setPage(page - 1)}
            >
              <ArrowLeft aria-hidden data-icon='inline-start' />
              {t('Previous')}
            </Button>
            <span
              className='text-muted-foreground text-sm tabular-nums'
              aria-live='polite'
            >
              {t('Page {{page}}', {
                page: formatNumber(page, toIntlLocale(i18n.language)),
              })}
            </span>
            <Button
              variant='outline'
              disabled={!data?.has_more}
              onClick={() => setPage(page + 1)}
            >
              {t('Next')}
              <ArrowRight aria-hidden data-icon='inline-end' />
            </Button>
          </Pagination>
        )}
      </div>
    </PublicLayout>
  )
}
