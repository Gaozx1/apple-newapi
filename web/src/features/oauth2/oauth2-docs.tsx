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
import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

function CodeBlock(props: { code: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    const ok = await copyToClipboard(props.code)
    if (ok) {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    }
  }

  return (
    <div className='group relative overflow-x-auto rounded-lg border bg-muted/40'>
      <Button
        variant='ghost'
        size='icon-sm'
        className='absolute top-1.5 right-1.5 opacity-0 transition-opacity group-hover:opacity-100'
        onClick={handleCopy}
        aria-label={t('Copy')}
      >
        {copied ? (
          <Check className='size-3.5 text-emerald-500' />
        ) : (
          <Copy className='size-3.5' />
        )}
      </Button>
      <pre className='p-3 pr-10 text-xs leading-relaxed'>
        <code>{props.code}</code>
      </pre>
    </div>
  )
}

// Billing tiers: first 50 calls/day free, 51-100 $0.001/call, 101-200
// $0.002/call, 201+ $0.003/call. Must stay in sync with the backend
// `oauthTierPrice` in controller/oauth2.go.
const BILLING_TIERS = [
  { range: '1 - 50', price: '$0', free: true },
  { range: '51 - 100', price: '$0.001' },
  { range: '101 - 200', price: '$0.002' },
  { range: '201+', price: '$0.003' },
]

/**
 * Compact pricing tier strip, reusable on the apps tab and inside the docs.
 */
export function PricingTiers() {
  const { t } = useTranslation()

  return (
    <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
      {BILLING_TIERS.map((tier) => (
        <div
          key={tier.range}
          className={`rounded-lg border px-3 py-2 transition-colors ${
            tier.free
              ? 'border-emerald-500/30 bg-emerald-500/5'
              : 'border-border/40 bg-muted/20'
          }`}
        >
          <p className='text-muted-foreground text-[10px] tracking-wider uppercase'>
            {t('Calls / day')}
          </p>
          <p className='mt-0.5 font-mono text-xs font-semibold'>
            {tier.range}
          </p>
          <p
            className={`mt-0.5 text-xs font-medium ${
              tier.free
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-foreground'
            }`}
          >
            {tier.free ? t('Free of charge') : tier.price}
          </p>
        </div>
      ))}
    </div>
  )
}

export function OAuth2Docs() {
  const { t } = useTranslation()

  return (
    <div className='space-y-8'>
      {/* Billing — placed first so pricing is visible without scrolling far */}
      <section className='space-y-3'>
        <div>
          <h3 className='text-base font-semibold'>{t('Billing')}</h3>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Calls are counted per user per day (reset daily). The first 50 calls are free; beyond that each call is charged to the user wallet by tier.'
            )}
          </p>
        </div>
        <PricingTiers />
        <p className='text-muted-foreground text-xs'>
          {t(
            'When the wallet balance is insufficient, the API returns HTTP 402 with error "insufficient_quota".'
          )}
        </p>
      </section>

      {/* Integration flow */}
      <section className='space-y-4'>
        <div>
          <h3 className='text-base font-semibold'>{t('Integration Guide')}</h3>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Connect your application in four steps using the standard OAuth 2.0 authorization code flow.'
            )}
          </p>
        </div>

        <div className='space-y-5'>
          <div className='space-y-2'>
            <div className='flex items-center gap-2'>
              <Badge variant='outline' className='font-mono text-[11px]'>
                1
              </Badge>
              <span className='text-sm font-medium'>{t('Create an app')}</span>
            </div>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Register your application in the "My Apps" tab to obtain a client_id and client_secret. The secret is shown only once — store it securely.'
              )}
            </p>
          </div>

          <div className='space-y-2'>
            <div className='flex items-center gap-2'>
              <Badge variant='outline' className='font-mono text-[11px]'>
                2
              </Badge>
              <span className='text-sm font-medium'>
                {t('Redirect the user to the authorization page')}
              </span>
            </div>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Send the user to the authorize endpoint. After consent, they are redirected back to your redirect_uri with a one-time code.'
              )}
            </p>
            <CodeBlock
              code={`GET https://your-site.com/oauth2/authorize
    ?client_id=YOUR_CLIENT_ID
    &redirect_uri=YOUR_REDIRECT_URI
    &scope=profile
    &state=RANDOM_STATE`}
            />
          </div>

          <div className='space-y-2'>
            <div className='flex items-center gap-2'>
              <Badge variant='outline' className='font-mono text-[11px]'>
                3
              </Badge>
              <span className='text-sm font-medium'>
                {t('Exchange the code for an access token')}
              </span>
            </div>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Exchange the one-time code for a bearer token. The token is valid for 2 hours.'
              )}
            </p>
            <CodeBlock
              code={`curl -X POST https://your-site.com/api/oauth2/token \\
  -d grant_type=authorization_code \\
  -d code=THE_CODE \\
  -d redirect_uri=YOUR_REDIRECT_URI \\
  -d client_id=YOUR_CLIENT_ID \\
  -d client_secret=YOUR_CLIENT_SECRET

# Response
{
  "access_token": "...",
  "token_type": "Bearer",
  "expires_in": 7200,
  "scope": "profile"
}`}
            />
          </div>

          <div className='space-y-2'>
            <div className='flex items-center gap-2'>
              <Badge variant='outline' className='font-mono text-[11px]'>
                4
              </Badge>
              <span className='text-sm font-medium'>
                {t('Call the user info endpoint')}
              </span>
            </div>
            <CodeBlock
              code={`curl https://your-site.com/api/oauth2/userinfo \\
  -H "Authorization: Bearer ACCESS_TOKEN"

# Response
{
  "sub": 123,
  "username": "alice",
  "email": "alice@example.com",
  "role": 1
}`}
            />
          </div>
        </div>
      </section>

      {/* Notes */}
      <section className='space-y-2'>
        <h3 className='text-base font-semibold'>{t('Notes')}</h3>
        <ul className='text-muted-foreground list-disc space-y-1.5 pl-5 text-sm'>
          <li>
            {t(
              'Authorization codes are single-use and expire after 10 minutes.'
            )}
          </li>
          <li>
            {t(
              'Access tokens are bearer credentials — keep them secret and never expose them in client-side code.'
            )}
          </li>
          <li>
            {t(
              'Scopes are optional. Leave the client scopes empty to allow any scope request.'
            )}
          </li>
        </ul>
      </section>
    </div>
  )
}
