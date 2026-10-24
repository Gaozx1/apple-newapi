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
import { Flag } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import type { StoreChannelOffer } from '../types'

export interface StoreChannelCardProps {
  offer: StoreChannelOffer
  onReport: (offer: StoreChannelOffer) => void
}

/** One user-uploaded upstream: models, seller, health and the buy action. */
export function StoreChannelCard(props: StoreChannelCardProps) {
  const { t } = useTranslation()
  const offer = props.offer
  const failureRate =
    offer.request_count > 0
      ? Math.round((offer.error_count / offer.request_count) * 100)
      : 0
  const unhealthy = offer.request_count > 0 && failureRate >= 80

  return (
    <Card className='flex h-full flex-col'>
      <CardHeader className='gap-2'>
        <div className='flex items-start justify-between gap-2'>
          <CardTitle className='text-base break-words'>{offer.name}</CardTitle>
          <Badge variant='secondary'>
            {t('Ratio {{ratio}}', { ratio: offer.ratio })}
          </Badge>
        </div>
        {offer.description !== '' && (
          <p className='text-muted-foreground line-clamp-3 text-xs break-words whitespace-pre-wrap'>
            {offer.description}
          </p>
        )}
      </CardHeader>
      <CardContent className='flex flex-1 flex-col gap-2'>
        <p className='text-xs break-all'>
          <span className='text-muted-foreground'>{t('Models')}: </span>
          <span className='font-mono'>{offer.models}</span>
        </p>
        <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
          <span>
            {t('Seller')}: {offer.username || `#${offer.user_id}`}
          </span>
          <span>{t('Bought by {{count}}', { count: offer.sold_count })}</span>
          {offer.request_count > 0 && (
            <span className={unhealthy ? 'text-destructive' : undefined}>
              {t('Failure rate')}: {failureRate}% ({offer.error_count}/
              {offer.request_count})
            </span>
          )}
        </div>
      </CardContent>
      <CardFooter className='flex items-center justify-between gap-2'>
        <Button
          size='sm'
          variant='outline'
          onClick={() => props.onReport(offer)}
        >
          <Flag className='size-3' aria-hidden='true' />
          {t('Report')}
        </Button>
        <span className='text-muted-foreground text-right text-xs'>
          {t('Usable by selecting the {{group}} group', {
            group: offer.group_name,
          })}
        </span>
      </CardFooter>
    </Card>
  )
}
