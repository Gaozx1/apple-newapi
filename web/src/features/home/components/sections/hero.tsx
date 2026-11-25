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
import CherryStudioColor from '@lobehub/icons/es/CherryStudio/components/Color.js'
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  ArrowUpRight,
  BookOpen,
  Cable,
  KeyRound,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { useApiInfo } from '@/features/dashboard/hooks/use-status-data'
import { useStatus } from '@/hooks/use-status'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'

export function Hero(props: { isAuthenticated: boolean }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { items: addresses } = useApiInfo()
  const modules = parseHeaderNavModulesFromStatus(status)
  const docsUrl =
    (typeof status?.docs_link === 'string' && status.docs_link.trim()) ||
    'https://docs.newapi.pro'
  const serverAddress =
    (typeof status?.server_address === 'string' &&
      status.server_address.trim()) ||
    window.location.origin
  const address =
    addresses.find((item) => item.url?.trim())?.url || serverAddress
  const guestPath = status?.register_enabled === false ? '/sign-in' : '/sign-up'
  const guestLabel =
    status?.register_enabled === false ? t('Sign in') : t('Get Started')
  const entryPath = props.isAuthenticated ? '/dashboard' : guestPath
  const docsLink = /^https?:\/\//i.test(docsUrl) ? (
    <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
  ) : (
    <Link to={docsUrl} />
  )

  return (
    <div className='gateway-frame'>
      <section className='gateway-intro'>
        <div className='gateway-intro-copy'>
          <h1>{t('Model Access')}</h1>
          <p>
            {t(
              'Connect through OpenAI, Claude, Gemini, and other compatible API routes'
            )}
          </p>
          <div className='flex flex-wrap gap-2.5'>
            {modules.pricing.enabled && (
              <Button
                size='lg'
                render={
                  modules.pricing.requireAuth && !props.isAuthenticated ? (
                    <Link to='/sign-in' search={{ redirect: '/pricing' }} />
                  ) : (
                    <Link to='/pricing' />
                  )
                }
              >
                {t('View models')}{' '}
                <ArrowRight data-icon='inline-end' aria-hidden />
              </Button>
            )}
            {modules.console && (
              <Button
                variant='outline'
                size='lg'
                render={<Link to={entryPath} />}
              >
                {props.isAuthenticated ? t('Go to Dashboard') : guestLabel}
              </Button>
            )}
          </div>
        </div>
        <div className='gateway-connect'>
          <div className='gateway-connect-heading'>
            <Cable aria-hidden />
            <h2>{t('API Access')}</h2>
          </div>
          <div className='gateway-endpoint'>
            <div>
              <span>{t('Server Address')}</span>
              <code>{address}</code>
            </div>
            <CopyButton value={address} tooltip={t('Copy API URL')} />
          </div>
          {modules.docs && (
            <Button
              variant='ghost'
              className='gateway-resource'
              render={docsLink}
            >
              <BookOpen aria-hidden /> {t('Docs')} <ArrowUpRight aria-hidden />
            </Button>
          )}
          <Button
            variant='ghost'
            className='gateway-resource'
            render={
              <Link
                to={props.isAuthenticated ? '/oauth2-clients' : guestPath}
              />
            }
          >
            <KeyRound aria-hidden /> {t('Open Platform')}{' '}
            <ArrowRight aria-hidden />
          </Button>
        </div>
      </section>
      <section className='studio-integrations'>
        <div>
          <h2>{t('Supported Applications')}</h2>
          <p>
            {t(
              'Supports one-click configuration and perfectly adapts to NewAPI multi-protocol configuration.'
            )}
          </p>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            render={
              <a
                href='https://cherry-ai.com'
                target='_blank'
                rel='noopener noreferrer'
              />
            }
          >
            <CherryStudioColor size={20} />
            Cherry Studio
          </Button>
          <Button
            variant='outline'
            render={
              <a
                href='https://ccswitch.io'
                target='_blank'
                rel='noopener noreferrer'
              />
            }
          >
            CC Switch
          </Button>
        </div>
      </section>
    </div>
  )
}
