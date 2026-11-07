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
import { CherryStudio } from '@lobehub/icons'
import { Link } from '@tanstack/react-router'
import { ArrowRight, BookOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

import { HeroTerminalDemo } from '../hero-terminal-demo'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

// Stylized three-dots indicator representing "More"
const MoreIcon = () => (
  <svg
    className='text-muted-foreground/60 group-hover:text-foreground size-6 shrink-0 transition-colors'
    viewBox='0 0 24 24'
    fill='none'
    xmlns='http://www.w3.org/2000/svg'
  >
    <circle cx='6' cy='12' r='2' fill='currentColor' />
    <circle cx='12' cy='12' r='2' fill='currentColor' />
    <circle cx='18' cy='12' r='2' fill='currentColor' />
  </svg>
)

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'

  const renderDocsButton = () => {
    const isExternal = docsUrl.startsWith('http')
    if (isExternal) {
      return (
        <Button
          variant='outline'
          className='group border-border/60 hover:border-border hover:bg-muted/50 inline-flex h-11 items-center gap-1.5 rounded-xl px-5 text-sm font-medium shadow-xs'
          render={
            <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
          }
        >
          <BookOpen className='text-muted-foreground/80 group-hover:text-foreground size-4 transition-colors duration-200' />
          <span>{t('Docs')}</span>
        </Button>
      )
    }
    return (
      <Button
        variant='outline'
        className='group border-border/60 hover:border-border hover:bg-muted/50 inline-flex h-11 items-center gap-1.5 rounded-xl px-5 text-sm font-medium shadow-xs'
        render={<Link to={docsUrl} />}
      >
        <BookOpen className='text-muted-foreground/80 group-hover:text-foreground size-4 transition-colors duration-200' />
        <span>{t('Docs')}</span>
      </Button>
    )
  }

  return (
    <section className='relative isolate z-10 overflow-hidden px-6 pt-[7.5rem] pb-[5.5rem] md:pt-[9rem] md:pb-[7rem]'>
      <div
        aria-hidden
        className='public-hero-glow pointer-events-none absolute inset-0 -z-10'
      />
      <div
        aria-hidden
        className='public-grid pointer-events-none absolute inset-0 -z-10'
      />

      <div className='mx-auto grid max-w-6xl grid-cols-1 items-start gap-12 lg:grid-cols-12 lg:gap-8'>
        {/* Left Column: Title, description, action buttons and application support */}
        <div className='flex flex-col items-start text-left lg:col-span-6'>
          {/* Top Pill Badge — links to the OAuth 2.0 open platform */}
          <Link
            to={props.isAuthenticated ? '/oauth2-clients' : '/sign-up'}
            className='landing-animate-fade-up group border-primary/25 bg-primary/8 text-primary hover:border-primary/40 hover:bg-primary/10 mb-5 inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[11px] font-medium opacity-0 shadow-xs transition-colors'
            style={{ animationDelay: '0ms' }}
          >
            <span className='relative flex size-1.5'>
              <span className='bg-primary absolute inline-flex h-full w-full animate-ping rounded-full opacity-75' />
              <span className='bg-primary relative inline-flex size-1.5 rounded-full' />
            </span>
            <span>
              {t('OAuth 2.0 Open Platform is live')}
              <span className='text-primary/70'>
                {' '}
                · {t('First 150 API calls free every day')}
              </span>
            </span>
            <ArrowRight className='size-3 transition-transform duration-200 group-hover:translate-x-0.5' />
          </Link>

          <h1
            className='landing-animate-fade-up text-[clamp(2.5rem,5vw,3.75rem)] leading-[1.05] font-semibold tracking-[-0.03em]'
            style={{ animationDelay: '60ms' }}
          >
            {t('Unified API Gateway for')}
            <br />
            <span className='text-gradient-brand'>
              {t('Vast Range of AI Models')}
            </span>
          </h1>
          <p
            className='landing-animate-fade-up text-muted-foreground mt-5 max-w-xl text-base leading-relaxed opacity-0 md:text-lg'
            style={{ animationDelay: '120ms' }}
          >
            {t(
              'Access a vast selection of models via a standard, unified API protocol. Power AI applications, manage digital assets, and connect the Future.'
            )}
          </p>

          <div
            className='landing-animate-fade-up mt-8 flex flex-wrap items-center gap-3 opacity-0'
            style={{ animationDelay: '180ms' }}
          >
            {props.isAuthenticated ? (
              <>
                <Button
                  className='group h-11 rounded-xl px-5 text-sm font-medium shadow-xs'
                  render={<Link to='/dashboard' />}
                >
                  {t('Go to Dashboard')}
                  <ArrowRight className='ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                </Button>
                {renderDocsButton()}
              </>
            ) : (
              <>
                <Button
                  className='group h-11 rounded-xl px-5 text-sm font-medium shadow-xs'
                  render={<Link to='/sign-up' />}
                >
                  {t('Get Started')}
                  <ArrowRight className='ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                </Button>
                <Button
                  variant='outline'
                  className='border-border/60 hover:border-border hover:bg-muted/50 h-11 rounded-xl px-5 text-sm font-medium shadow-xs'
                  render={<Link to='/pricing' />}
                >
                  {t('View Pricing')}
                </Button>
                {renderDocsButton()}
              </>
            )}
          </div>

          {/* Supported Apps (参考图二样式，进行卡片化和信息扩充设计，增加视觉高度) */}
          <div
            className='landing-animate-fade-up mt-10 w-full max-w-xl opacity-0'
            style={{ animationDelay: '240ms' }}
          >
            <div className='mb-4 flex flex-col gap-1'>
              <span className='text-muted-foreground/50 text-[10px] font-bold tracking-[0.15em] uppercase'>
                {t('Supported Applications')}
              </span>
              <p className='text-muted-foreground/60 text-xs leading-relaxed'>
                {t(
                  'Supports one-click configuration and perfectly adapts to NewAPI multi-protocol configuration.'
                )}
              </p>
            </div>
            <div className='flex flex-wrap items-center gap-3'>
              {/* Cherry Studio */}
              <a
                href='https://cherry-ai.com'
                target='_blank'
                rel='noopener noreferrer'
                className='group border-border/60 bg-card/60 text-foreground/80 hover:border-primary/30 hover:text-foreground flex items-center gap-3 rounded-xl border px-4 py-2.5 text-sm font-medium shadow-xs transition-colors duration-300'
              >
                <CherryStudio.Color size={24} className='shrink-0' />
                <span>Cherry Studio</span>
              </a>

              {/* CC Switch */}
              <a
                href='https://ccswitch.io'
                target='_blank'
                rel='noopener noreferrer'
                className='group border-border/60 bg-card/60 text-foreground/80 hover:border-primary/30 hover:text-foreground flex items-center gap-3 rounded-xl border px-4 py-2.5 text-sm font-medium shadow-xs transition-colors duration-300'
              >
                <img
                  src='https://ccswitch.io/favicon.png'
                  alt='CC Switch'
                  className='size-6 shrink-0 rounded-md object-contain'
                  onError={(e) => {
                    // Fallback to a styled text avatar if the remote favicon fails to load in sandbox or local environments
                    e.currentTarget.style.display = 'none'
                    const fallback = e.currentTarget.nextSibling as HTMLElement
                    if (fallback) fallback.style.display = 'flex'
                  }}
                />
                <span
                  style={{ display: 'none' }}
                  className='bg-primary/10 text-primary size-6 shrink-0 items-center justify-center rounded-md text-[10px] font-bold'
                >
                  CC
                </span>
                <span>CC Switch</span>
              </a>

              {/* "更多" */}
              <div className='group border-border/60 bg-card/60 text-foreground/55 hover:border-primary/30 hover:text-foreground flex cursor-default items-center gap-2.5 rounded-xl border px-4 py-2.5 text-sm font-medium shadow-xs transition-colors duration-300'>
                <MoreIcon />
                <span>{t('More Apps')}</span>
              </div>
            </div>
          </div>
        </div>

        {/* Right Column: Hero Terminal API Demo */}
        <div
          className='landing-animate-fade-up flex w-full justify-center opacity-0 lg:col-span-6'
          style={{ animationDelay: '320ms' }}
        >
          <HeroTerminalDemo className='mt-8 lg:mt-0' />
        </div>
      </div>
    </section>
  )
}
