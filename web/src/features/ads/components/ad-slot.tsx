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
import { useTranslation } from 'react-i18next'

import type { AdItem } from '../api'

export interface AdSlotProps {
  ad: AdItem | null
  className?: string
}

/**
 * Renders one advertisement. Admin HTML runs inside a sandboxed iframe so an
 * ad provider's script cannot reach the dashboard session or react to the page.
 * The sandbox allows scripts (ad SDKs need them) but not same-origin access.
 */
export function AdSlot(props: AdSlotProps) {
  const { t } = useTranslation()
  if (props.ad === null) return null

  const wrapperClassName = [
    'overflow-hidden rounded-lg border',
    props.className,
  ]
    .filter(Boolean)
    .join(' ')

  if (props.ad.html.trim() !== '') {
    return (
      <div className={wrapperClassName}>
        <iframe
          title={props.ad.title || t('Advertisement')}
          className='h-40 w-full border-0 bg-transparent'
          sandbox='allow-scripts allow-popups allow-popups-to-escape-sandbox'
          referrerPolicy='no-referrer-when-downgrade'
          srcDoc={`<!doctype html><html><body style="margin:0;display:flex;align-items:center;justify-content:center;min-height:160px;background:transparent">${props.ad.html}</body></html>`}
        />
      </div>
    )
  }

  const image = (
    <img
      src={props.ad.image_url}
      alt={props.ad.title || t('Advertisement')}
      className='h-40 w-full object-cover'
      loading='lazy'
    />
  )

  return (
    <div className={wrapperClassName}>
      {props.ad.link_url.trim() === '' ? (
        image
      ) : (
        <a
          href={props.ad.link_url}
          target='_blank'
          rel='noopener noreferrer sponsored'
          aria-label={props.ad.title || t('Advertisement')}
        >
          {image}
        </a>
      )}
    </div>
  )
}
