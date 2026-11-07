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
import { Link } from '@tanstack/react-router'
import { ArrowRight, KeyRound, ShieldCheck, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'

interface OpenPlatformProps {
  className?: string
  isAuthenticated?: boolean
}

// Billing tiers mirror the backend `oauthTierPrice` in controller/oauth2.go:
// first 150 calls/day free, 151-200 $0.001/call, 201-300 $0.002/call, 301+ $0.003.
const TIERS = [
  {
    range: '1 - 150',
    price: '$0',
    free: true,
    labelKey: 'Free of charge',
  },
  { range: '151 - 200', price: '$0.001', free: false, labelKey: 'per call' },
  { range: '201 - 300', price: '$0.002', free: false, labelKey: 'per call' },
  { range: '301+', price: '$0.003', free: false, labelKey: 'per call' },
]

export function OpenPlatform(props: OpenPlatformProps) {
  const { t } = useTranslation()

  return (
    <section className='relative z-10 px-6 py-20 md:py-24'>
      <AnimateInView>
        <div className='border-border/60 bg-card shadow-card mx-auto max-w-6xl overflow-hidden rounded-3xl border'>
          <div className='grid gap-10 p-8 md:grid-cols-2 md:p-12'>
            {/* Left: heading + CTA */}
            <div className='flex flex-col items-start justify-center gap-5'>
              <div className='border-primary/25 bg-primary/10 text-primary inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium'>
                <KeyRound className='size-3.5' />
                {t('Open Platform')}
              </div>

              <h2 className='text-2xl leading-tight font-semibold tracking-[-0.02em] md:text-4xl'>
                {t('OAuth 2.0 Open Platform')}
              </h2>

              <p className='text-muted-foreground max-w-md text-sm leading-relaxed md:text-base'>
                {t(
                  'Let third-party applications sign in with the standard OAuth 2.0 flow and call the API on behalf of users — with a free daily allowance and transparent tiered pricing.'
                )}
              </p>

              <div className='text-muted-foreground flex flex-wrap items-center gap-4 text-xs'>
                <span className='inline-flex items-center gap-1.5'>
                  <ShieldCheck className='text-success size-3.5' />
                  {t('Standard OAuth 2.0 flow')}
                </span>
                <span className='inline-flex items-center gap-1.5'>
                  <Wallet className='text-success size-3.5' />
                  {t('Pay from wallet balance')}
                </span>
              </div>

              <Button
                size='lg'
                className='group mt-1 h-11 rounded-xl px-6 text-sm font-medium shadow-xs'
                render={
                  <Link
                    to={props.isAuthenticated ? '/oauth2-clients' : '/sign-up'}
                  />
                }
              >
                {props.isAuthenticated
                  ? t('Open OAuth 2.0 Apps')
                  : t('Start Building')}
                <ArrowRight className='ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
              </Button>
            </div>

            {/* Right: pricing tiers */}
            <div className='flex flex-col justify-center gap-3'>
              <div className='grid grid-cols-2 gap-3'>
                {TIERS.map((tier) => (
                  <div
                    key={tier.range}
                    className={`rounded-xl border p-4 transition-colors ${
                      tier.free
                        ? 'border-success/40 bg-success/10'
                        : 'bg-background/60 border-border/60 hover:border-primary/40'
                    }`}
                  >
                    <p className='text-muted-foreground text-[11px] font-medium tracking-wider uppercase'>
                      {t('Calls / day')}
                    </p>
                    <p className='mt-1 font-mono text-lg font-bold'>
                      {tier.range}
                    </p>
                    <p
                      className={`mt-1 text-2xl font-bold tracking-tight ${
                        tier.free ? 'text-success' : 'text-primary'
                      }`}
                    >
                      {tier.price}
                    </p>
                    <p className='text-muted-foreground mt-0.5 text-xs'>
                      {t(tier.labelKey)}
                    </p>
                  </div>
                ))}
              </div>
              <p className='text-muted-foreground text-center text-xs'>
                {t('Per user, resets daily')}
              </p>
            </div>
          </div>
        </div>
      </AnimateInView>
    </section>
  )
}
