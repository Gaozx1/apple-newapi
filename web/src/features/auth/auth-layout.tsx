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

  return (
    <div className='site-auth studio-access'>
      <main className='studio-access-main'>
        <div className='studio-access-brand'>
          <AuthBrand />
        </div>
        <div className='site-auth-form studio-access-form'>
          {props.children}
        </div>
        <Link to='/' className='studio-access-back'>
          {t('Back to Home')}
        </Link>
      </main>
    </div>
  )
}
