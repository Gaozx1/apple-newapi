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
import { cn } from '@/lib/utils'

export interface PublicPageHeroProps {
  eyebrow?: React.ReactNode
  title: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  breadcrumb?: React.ReactNode
  children?: React.ReactNode
  align?: 'center' | 'left'
  size?: 'default' | 'lg'
  className?: string
}

export function PublicPageHero(props: PublicPageHeroProps) {
  const align = props.align ?? 'center'
  const size = props.size ?? 'default'

  return (
    <section
      className={cn(
        'relative isolate overflow-hidden px-6 pt-[8rem] pb-12 md:pt-[9.5rem] md:pb-16',
        props.className
      )}
    >
      <div
        aria-hidden
        className='aurora-bg pointer-events-none absolute inset-0 -z-10'
      />
      <div
        aria-hidden
        className='public-grid pointer-events-none absolute inset-0 -z-10'
      />
      <div
        aria-hidden
        className='pointer-events-none absolute -top-40 left-1/2 -z-10 h-[520px] w-[820px] -translate-x-1/2 rounded-full bg-[radial-gradient(closest-side,oklch(0.62_0.23_275/0.30),transparent)] blur-3xl'
      />

      <div className='mx-auto w-full max-w-6xl'>
        {props.breadcrumb ? (
          <div className='mb-5'>{props.breadcrumb}</div>
        ) : null}
        <div
          className={
            align === 'center' ? 'mx-auto max-w-3xl text-center' : 'max-w-3xl'
          }
        >
          {props.eyebrow ? (
            <span className='border-primary/25 bg-primary/[0.07] text-primary inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-[11px] font-semibold tracking-[0.14em] uppercase backdrop-blur-sm'>
              {props.eyebrow}
            </span>
          ) : null}
          <h1
            className={cn(
              'text-balance font-bold tracking-[-0.03em]',
              size === 'lg'
                ? 'mt-4 text-[clamp(2.25rem,4.5vw,3.75rem)] leading-[1.05]'
                : 'mt-4 text-[clamp(2rem,4vw,3.25rem)] leading-[1.06]'
            )}
          >
            {props.title}
          </h1>
          {props.description ? (
            <p
              className={cn(
                'text-muted-foreground mt-4 text-sm leading-relaxed md:text-base',
                align === 'center' ? 'mx-auto max-w-2xl' : 'max-w-2xl'
              )}
            >
              {props.description}
            </p>
          ) : null}
          {props.actions ? (
            <div
              className={cn(
                'mt-6 flex flex-wrap gap-3',
                align === 'center' ? 'justify-center' : ''
              )}
            >
              {props.actions}
            </div>
          ) : null}
          {props.children ? <div className='mt-8'>{props.children}</div> : null}
        </div>
      </div>
    </section>
  )
}
