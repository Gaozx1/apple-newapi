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
import { Fingerprint, ShieldCheck, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

type AuthLayoutProps = {
  children: React.ReactNode
}

function AuthBrand(props: { compact?: boolean }) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()
  const logoSize = props.compact ? 'size-8' : 'size-9'

  return (
    <Link
      to='/'
      className='inline-flex items-center gap-2.5 transition-opacity hover:opacity-80'
    >
      <div
        className={cn(
          'border-border/60 bg-card relative shrink-0 overflow-hidden rounded-xl border',
          logoSize
        )}
      >
        {loading ? (
          <Skeleton className='absolute inset-0 rounded-xl' />
        ) : (
          <img src={logo} alt={t('Logo')} className='size-full object-cover' />
        )}
      </div>
      {loading ? (
        <Skeleton className='h-5 w-24' />
      ) : (
        <span
          className={cn(
            'font-semibold tracking-tight',
            props.compact ? 'text-sm' : 'text-base'
          )}
        >
          {systemName}
        </span>
      )}
    </Link>
  )
}

export function AuthLayout(props: AuthLayoutProps) {
  const { t } = useTranslation()

  const capabilities = [
    { key: 'oauth', icon: ShieldCheck, label: t('OAuth 2.0') },
    { key: 'passkey', icon: Fingerprint, label: t('Passkey') },
    { key: 'billing', icon: Wallet, label: t('Billing') },
  ]

  return (
    <div className='bg-background relative min-h-svh overflow-hidden'>
      <div
        aria-hidden
        className='auth-bg pointer-events-none absolute inset-0 -z-10 opacity-70'
      />
      <div className='relative mx-auto grid min-h-svh w-full max-w-[1200px] lg:grid-cols-[minmax(0,1fr)_minmax(0,480px)]'>
        <aside className='hidden flex-col justify-between p-10 lg:flex xl:p-14'>
          <AuthBrand />

          <div className='max-w-md'>
            <span className='border-border/60 bg-card/60 text-muted-foreground inline-flex items-center gap-2 rounded-full border px-3 py-1 text-[11px] font-medium tracking-[0.14em] uppercase backdrop-blur-sm'>
              {t('Secure & Reliable')}
            </span>
            <h1 className='mt-5 text-[clamp(2.25rem,3.2vw,3rem)] leading-[1.08] font-semibold tracking-[-0.02em]'>
              {t('Unified API Gateway for')}
              <br />
              <span className='text-gradient-brand'>
                {t('Vast Range of AI Models')}
              </span>
            </h1>
            <ul className='mt-8 space-y-3'>
              {capabilities.map((capability) => (
                <li
                  key={capability.key}
                  className='text-muted-foreground flex items-center gap-3 text-sm'
                >
                  <span className='border-border/60 bg-card/70 text-foreground inline-flex size-8 shrink-0 items-center justify-center rounded-lg border'>
                    <capability.icon
                      aria-hidden
                      className='size-4'
                      strokeWidth={1.75}
                    />
                  </span>
                  {capability.label}
                </li>
              ))}
            </ul>
          </div>

          <p className='text-muted-foreground/70 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs'>
            <span>{t('Session')}</span>
            <span aria-hidden>·</span>
            <span>{t('Audit Logs')}</span>
            <span aria-hidden>·</span>
            <span>{t('Security')}</span>
          </p>
        </aside>

        <main className='flex min-h-svh flex-col justify-center px-4 py-10 sm:px-8 lg:py-14'>
          <div className='mb-8 lg:hidden'>
            <AuthBrand compact />
          </div>
          <div className='border-border/60 bg-card/70 shadow-dialog mx-auto w-full max-w-[400px] rounded-2xl border p-6 backdrop-blur-xl sm:p-8'>
            {props.children}
          </div>
          <div className='text-muted-foreground mx-auto mt-6 w-full max-w-[400px] text-center text-xs'>
            <Link to='/' className='hover:text-foreground transition-colors'>
              {t('Back to Home')}
            </Link>
          </div>
        </main>
      </div>
    </div>
  )
}
