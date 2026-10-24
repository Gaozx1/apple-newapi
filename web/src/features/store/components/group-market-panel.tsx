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
import { Flag, ShoppingBag } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  claimStoreGroup,
  listMarketGroups,
  unclaimStoreGroup,
  type MarketGroup,
} from '../api'

export interface GroupMarketPanelProps {
  onReport: (group: MarketGroup) => void
}

/**
 * 分组市场: the only place a user group becomes usable by others. Claiming a
 * group is what puts its models into the buyer's model square and lets their
 * tokens route through its channels.
 */
export function GroupMarketPanel(props: GroupMarketPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [claimTarget, setClaimTarget] = useState<MarketGroup | null>(null)
  const [unclaimTarget, setUnclaimTarget] = useState<MarketGroup | null>(null)

  const groupsQuery = useQuery({
    queryKey: ['store-market-groups'],
    queryFn: listMarketGroups,
  })

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['store-market-groups'] })
    void queryClient.invalidateQueries({ queryKey: ['store-groups'] })
  }

  const claimMutation = useMutation({
    mutationFn: (group: MarketGroup) => claimStoreGroup(group.id),
    onSuccess: () => {
      toast.success(t('Claimed, the group is now in your model square'))
      setClaimTarget(null)
      refresh()
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Claim failed'))),
  })

  const unclaimMutation = useMutation({
    mutationFn: (group: MarketGroup) => unclaimStoreGroup(group.id),
    onSuccess: () => {
      toast.success(t('Claim removed'))
      setUnclaimTarget(null)
      refresh()
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Claim failed'))),
  })

  const groups = groupsQuery.data ?? []

  return (
    <div className='grid gap-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Claiming a group adds its models to your model square and lets your tokens call it, billed at the group multiplier.'
          )}
        </p>
      </div>

      {groupsQuery.isLoading && <LoadingState />}
      {!groupsQuery.isLoading && groups.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No groups on the market yet')}
        </p>
      )}

      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
        {groups.map((group) => (
          <Card key={group.id} className='flex h-full flex-col'>
            <CardHeader className='gap-2'>
              <div className='flex items-start justify-between gap-2'>
                <CardTitle className='text-base break-words'>
                  {group.public_name}
                </CardTitle>
                <Badge variant='secondary'>
                  {t('Ratio {{ratio}}', { ratio: group.ratio })}
                </Badge>
              </div>
              {group.remark !== '' && (
                <p className='text-muted-foreground line-clamp-3 text-xs break-words whitespace-pre-wrap'>
                  {group.remark}
                </p>
              )}
            </CardHeader>
            <CardContent className='flex flex-1 flex-col gap-2'>
              <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
                <span>
                  {t('Seller')}: {group.username || `#${group.user_id}`}
                </span>
                <span>{group.claimed ? t('Claimed') : t('Not claimed')}</span>
              </div>
            </CardContent>
            <div className='flex items-center justify-between gap-2 px-6 pb-4'>
              <Button
                size='sm'
                variant='outline'
                onClick={() => props.onReport(group)}
              >
                <Flag className='size-3' aria-hidden='true' />
                {t('Report')}
              </Button>
              {group.is_owner && (
                <Badge variant='outline'>{t('Your group')}</Badge>
              )}
              {!group.is_owner && group.claimed && (
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => setUnclaimTarget(group)}
                >
                  {t('Remove claim')}
                </Button>
              )}
              {!group.is_owner && !group.claimed && (
                <Button size='sm' onClick={() => setClaimTarget(group)}>
                  <ShoppingBag className='size-3' aria-hidden='true' />
                  {t('Claim group')}
                </Button>
              )}
            </div>
          </Card>
        ))}
      </div>

      <ConfirmDialog
        open={claimTarget !== null}
        title={t('Claim group')}
        desc={
          claimTarget === null
            ? ''
            : t(
                'Claim {{name}} at ratio {{ratio}}? Its models appear in your model square and your tokens can call it.',
                {
                  name: claimTarget.public_name,
                  ratio: claimTarget.ratio,
                }
              )
        }
        confirmText={t('Claim')}
        isLoading={claimMutation.isPending}
        handleConfirm={() => {
          if (claimTarget !== null) claimMutation.mutate(claimTarget)
        }}
        onOpenChange={(open) => {
          if (!open) setClaimTarget(null)
        }}
      />

      <ConfirmDialog
        open={unclaimTarget !== null}
        title={t('Remove claim')}
        desc={
          unclaimTarget === null
            ? ''
            : t('Remove {{name}} from your model square?', {
                name: unclaimTarget.public_name,
              })
        }
        destructive
        isLoading={unclaimMutation.isPending}
        handleConfirm={() => {
          if (unclaimTarget !== null) unclaimMutation.mutate(unclaimTarget)
        }}
        onOpenChange={(open) => {
          if (!open) setUnclaimTarget(null)
        }}
      />
    </div>
  )
}
