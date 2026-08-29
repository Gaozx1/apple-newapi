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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ShoppingBag } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

import { getOAuthCallPlans, purchaseOAuthCallPlan } from './api'

export function CallPackagesCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['oauth-call-plans'],
    queryFn: getOAuthCallPlans,
  })

  const purchaseMutation = useMutation({
    mutationFn: (planId: number) => purchaseOAuthCallPlan(planId),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Call package purchased'))
        queryClient.invalidateQueries({ queryKey: ['oauth-call-plans'] })
      } else {
        toast.error(res.message || t('Failed to purchase call package'))
      }
    },
    onError: (err: unknown) => {
      const msg =
        err instanceof Error
          ? err.message
          : t('Failed to purchase call package')
      toast.error(msg)
    },
  })

  const response = data?.data
  const plans = response?.plans ?? []
  const balance = response?.balance ?? 0

  if (isLoading) {
    return <Skeleton className='h-32 w-full rounded-xl' />
  }

  if (plans.length === 0) {
    return null
  }

  return (
    <Card data-card-hover='false'>
      <CardHeader>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <CardTitle>{t('Call Packages')}</CardTitle>
          <p className='text-muted-foreground text-xs'>
            {t('Remaining calls')}:{' '}
            <span className='text-foreground font-semibold tabular-nums'>
              {balance}
            </span>
          </p>
        </div>
      </CardHeader>
      <CardContent className='space-y-3'>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Prepaid calls are consumed for API calls beyond the free daily tier, before the wallet is charged.'
          )}
        </p>
        <div className='grid gap-3 sm:grid-cols-2'>
          {plans.map((plan) => (
            <div
              key={plan.id}
              className='bg-muted/20 flex flex-col gap-2 rounded-lg border p-3'
            >
              <div className='flex items-center justify-between gap-2'>
                <span className='truncate text-sm font-medium'>
                  {plan.name}
                </span>
                <span className='font-mono text-sm font-semibold'>
                  ${Number(plan.price_amount || 0).toFixed(2)}
                </span>
              </div>
              <div className='text-muted-foreground text-xs'>
                {plan.call_count.toLocaleString()} {t('calls')}
                {plan.validity_days > 0
                  ? ` · ${t('Valid for {{days}} days', { days: plan.validity_days })}`
                  : ` · ${t('Never expires')}`}
              </div>
              <Button
                size='sm'
                className='mt-auto w-full sm:w-auto'
                disabled={purchaseMutation.isPending}
                onClick={() => purchaseMutation.mutate(plan.id)}
              >
                <ShoppingBag className='mr-1 size-3.5' />
                {purchaseMutation.isPending ? t('Purchasing...') : t('Buy')}
              </Button>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
