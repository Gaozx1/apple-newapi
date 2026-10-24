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
import dayjs from 'dayjs'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  createStoreChannel,
  createStoreGroup,
  deleteStoreGroup,
  listMyStoreEarnings,
  listStoreGroups,
  setStoreGroupListing,
  updateStoreGroup,
  type StoreGroup,
  deleteStoreChannel,
  listMyStoreChannels,
  updateStoreChannel,
} from '@/features/store/api'
import { StoreChannelDialog } from '@/features/store/components/store-channel-dialog'
import type {
  StoreChannelInput,
  StoreChannelOffer,
} from '@/features/store/types'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { getServerErrorMessage } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

/**
 * 我的渠道: every user keeps their own channels in their own group. Nobody else
 * sees them until the owner lists one on the market, and the owner's own traffic
 * through them is free.
 */
export function MyChannelsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id ?? 0)
  const groupName = `u${userId}`

  const [newGroup, setNewGroup] = useState({
    name: '',
    ratio: '1',
    remark: '',
  })
  const [editingGroup, setEditingGroup] = useState<StoreGroup | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<StoreChannelOffer | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<StoreChannelOffer | null>(
    null
  )

  const fail = (fallback: string) => (error: unknown) =>
    toast.error(getServerErrorMessage(error, t(fallback)))

  const groupsQuery = useQuery({
    queryKey: ['store-groups'],
    queryFn: listStoreGroups,
  })

  const groupSaveMutation = useMutation({
    mutationFn: async () => {
      const ratio = Number(newGroup.ratio)
      if (editingGroup === null) {
        return createStoreGroup({
          name: newGroup.name,
          ratio,
          remark: newGroup.remark,
        })
      }
      return updateStoreGroup(editingGroup.id, {
        ratio,
        remark: newGroup.remark,
      })
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      setNewGroup({ name: '', ratio: '1', remark: '' })
      setEditingGroup(null)
      refresh()
      void queryClient.invalidateQueries({ queryKey: ['store-groups'] })
    },
    onError: fail('Save failed'),
  })

  const groupListingMutation = useMutation({
    mutationFn: (payload: { group: StoreGroup; listed: boolean }) =>
      setStoreGroupListing(payload.group.id, payload.listed),
    onSuccess: (_data, payload) => {
      toast.success(
        payload.listed ? t('Listed on the market') : t('Taken off the market')
      )
      refresh()
      void queryClient.invalidateQueries({ queryKey: ['store-groups'] })
    },
    onError: fail('Save failed'),
  })

  const groupDeleteMutation = useMutation({
    mutationFn: (group: StoreGroup) => deleteStoreGroup(group.id),
    onSuccess: () => {
      toast.success(t('Deleted'))
      refresh()
      void queryClient.invalidateQueries({ queryKey: ['store-groups'] })
    },
    onError: fail('Delete failed'),
  })

  const channelsQuery = useQuery({
    queryKey: ['my-channels'],
    queryFn: () => listMyStoreChannels(1, 100),
  })

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['my-channels'] })
    void queryClient.invalidateQueries({ queryKey: ['store-channels'] })
  }

  const saveMutation = useMutation({
    mutationFn: (input: StoreChannelInput) => {
      if (editing === null) return createStoreChannel(input)
      return updateStoreChannel(editing.id, {
        ...input,
        status: editing.status,
      })
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      setDialogOpen(false)
      setEditing(null)
      refresh()
    },
    onError: fail('Save failed'),
  })

  const deleteMutation = useMutation({
    mutationFn: (offer: StoreChannelOffer) => deleteStoreChannel(offer.id),
    onSuccess: () => {
      toast.success(t('Deleted'))
      setDeleteTarget(null)
      refresh()
    },
    onError: fail('Delete failed'),
  })

  const earningsQuery = useQuery({
    queryKey: ['my-earnings'],
    queryFn: () => listMyStoreEarnings(1, 100),
  })

  const channels = channelsQuery.data?.items ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('My groups')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-4'>
          <Card>
            <CardHeader className='gap-2'>
              <CardTitle>{t('My groups')}</CardTitle>
              <CardDescription>
                {t(
                  'A group carries the multiplier and the note. Listing a group publishes every channel inside it; only you can use it before that, and your own traffic is always free.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='flex flex-wrap items-end gap-2'>
                <Input
                  className='max-w-48'
                  placeholder={t('Group')}
                  value={newGroup.name}
                  disabled={editingGroup !== null}
                  onChange={(event) =>
                    setNewGroup((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
                <Input
                  className='max-w-24'
                  inputMode='decimal'
                  placeholder={t('Ratio')}
                  value={newGroup.ratio}
                  onChange={(event) =>
                    setNewGroup((current) => ({
                      ...current,
                      ratio: event.target.value,
                    }))
                  }
                />
                <Input
                  className='max-w-64'
                  placeholder={t('Remark')}
                  value={newGroup.remark}
                  onChange={(event) =>
                    setNewGroup((current) => ({
                      ...current,
                      remark: event.target.value,
                    }))
                  }
                />
                <Button
                  size='sm'
                  disabled={groupSaveMutation.isPending}
                  onClick={() => groupSaveMutation.mutate()}
                >
                  {editingGroup === null ? t('Add a group') : t('Save')}
                </Button>
                {editingGroup !== null && (
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => {
                      setEditingGroup(null)
                      setNewGroup({ name: '', ratio: '1', remark: '' })
                    }}
                  >
                    {t('Cancel')}
                  </Button>
                )}
              </div>

              {(groupsQuery.data?.items ?? []).length > 0 && (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Group')}</TableHead>
                      <TableHead>{t('Ratio')}</TableHead>
                      <TableHead>{t('Remark')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead className='text-end'>{t('Actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(groupsQuery.data?.items ?? []).map((group) => (
                      <TableRow key={group.id}>
                        <TableCell className='font-mono text-xs'>
                          {group.name}
                        </TableCell>
                        <TableCell>{group.ratio}</TableCell>
                        <TableCell className='max-w-64 truncate'>
                          {group.remark}
                        </TableCell>
                        <TableCell>
                          {group.listed ? (
                            <Badge variant='secondary'>
                              {t('On the market')}
                            </Badge>
                          ) : (
                            <Badge variant='outline'>{t('Private')}</Badge>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className='flex items-center justify-end gap-1'>
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={groupListingMutation.isPending}
                              onClick={() =>
                                groupListingMutation.mutate({
                                  group,
                                  listed: !group.listed,
                                })
                              }
                            >
                              {group.listed
                                ? t('Take off the market')
                                : t('List on the market')}
                            </Button>
                            <Button
                              size='sm'
                              variant='ghost'
                              onClick={() => {
                                setEditingGroup(group)
                                setNewGroup({
                                  name: group.name,
                                  ratio: String(group.ratio),
                                  remark: group.remark,
                                })
                              }}
                            >
                              {t('Edit')}
                            </Button>
                            <Button
                              size='sm'
                              variant='ghost'
                              onClick={() => groupDeleteMutation.mutate(group)}
                            >
                              {t('Delete')}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='gap-2'>
              <CardTitle>{t('My channels')}</CardTitle>
              <CardDescription>
                {t(
                  'Your channels live in your own group {{group}}. Only you can see and use them until you list one on the market; your own traffic through them is free.',
                  { group: groupName }
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='flex justify-end'>
                <Button
                  size='sm'
                  onClick={() => {
                    setEditing(null)
                    setDialogOpen(true)
                  }}
                >
                  <Plus className='size-3' aria-hidden='true' />
                  {t('Add a channel')}
                </Button>
              </div>

              {channelsQuery.isLoading && <LoadingState />}
              {!channelsQuery.isLoading && channels.length === 0 && (
                <p className='text-muted-foreground text-sm'>
                  {t('No channels yet')}
                </p>
              )}

              {channels.length > 0 && (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Group')}</TableHead>
                      <TableHead>{t('Channel name')}</TableHead>
                      <TableHead>{t('Models')}</TableHead>
                      <TableHead>{t('Ratio')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead className='text-end'>{t('Actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {channels.map((offer) => (
                      <TableRow key={offer.id}>
                        <TableCell className='font-mono text-xs'>
                          {offer.group_name}
                        </TableCell>
                        <TableCell className='max-w-48 truncate'>
                          {offer.name}
                        </TableCell>
                        <TableCell className='max-w-64 truncate font-mono text-xs'>
                          {offer.models}
                        </TableCell>
                        <TableCell>{offer.ratio}</TableCell>
                        <TableCell>
                          {offer.status === 1 ? (
                            <Badge variant='secondary'>
                              {t('On the market')}
                            </Badge>
                          ) : (
                            <Badge variant='outline'>{t('Private')}</Badge>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className='flex items-center justify-end gap-1'>
                            <Button
                              size='sm'
                              variant='ghost'
                              onClick={() => {
                                setEditing(offer)
                                setDialogOpen(true)
                              }}
                            >
                              {t('Edit channel')}
                            </Button>
                            <Button
                              size='sm'
                              variant='ghost'
                              onClick={() => setDeleteTarget(offer)}
                            >
                              {t('Delete')}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='gap-2'>
              <CardTitle>{t('My revenue')}</CardTitle>
              <CardDescription>
                {t(
                  'Every request that ran through your channels: who used it, how much quota they consumed, and what you received after the 1% fee.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='grid gap-2 sm:grid-cols-4'>
                <div className='rounded-lg border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Requests')}
                  </p>
                  <p className='text-lg font-semibold'>
                    {earningsQuery.data?.summary.orders ?? 0}
                  </p>
                </div>
                <div className='rounded-lg border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Consumed by buyers')}
                  </p>
                  <p className='text-lg font-semibold'>
                    {formatCurrencyFromUSD(
                      earningsQuery.data?.summary.consumed_amount ?? 0
                    )}
                  </p>
                </div>
                <div className='rounded-lg border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Platform fee')}
                  </p>
                  <p className='text-lg font-semibold'>
                    {formatCurrencyFromUSD(
                      earningsQuery.data?.summary.fee_amount ?? 0
                    )}
                  </p>
                </div>
                <div className='rounded-lg border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('My income')}
                  </p>
                  <p className='text-lg font-semibold'>
                    {formatCurrencyFromUSD(
                      earningsQuery.data?.summary.income_amount ?? 0
                    )}
                  </p>
                </div>
              </div>

              {(earningsQuery.data?.page.items ?? []).length > 0 && (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Time')}</TableHead>
                      <TableHead>{t('Buyer')}</TableHead>
                      <TableHead>{t('Group')}</TableHead>
                      <TableHead>{t('Channel name')}</TableHead>
                      <TableHead>{t('Models')}</TableHead>
                      <TableHead className='text-end'>
                        {t('Consumed')}
                      </TableHead>
                      <TableHead className='text-end'>
                        {t('Platform fee')}
                      </TableHead>
                      <TableHead className='text-end'>
                        {t('My income')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(earningsQuery.data?.page.items ?? []).map((row) => (
                      <TableRow key={row.id}>
                        <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                          {dayjs
                            .unix(row.created_at)
                            .format('YYYY-MM-DD HH:mm')}
                        </TableCell>
                        <TableCell className='max-w-32 truncate'>
                          {row.buyer_name || `#${row.buyer_user_id}`}
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {row.group_name}
                        </TableCell>
                        <TableCell className='max-w-32 truncate'>
                          {row.channel_name}
                        </TableCell>
                        <TableCell className='max-w-40 truncate font-mono text-xs'>
                          {row.model_name}
                        </TableCell>
                        <TableCell className='text-end'>
                          {formatCurrencyFromUSD(row.quota_amount)}
                        </TableCell>
                        <TableCell className='text-end'>
                          {formatCurrencyFromUSD(row.fee_amount)}
                        </TableCell>
                        <TableCell className='text-end font-medium'>
                          {formatCurrencyFromUSD(row.income_amount)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}

              {!earningsQuery.isLoading &&
                (earningsQuery.data?.page.items ?? []).length === 0 && (
                  <p className='text-muted-foreground text-sm'>
                    {t('No revenue yet')}
                  </p>
                )}
            </CardContent>
          </Card>

          <StoreChannelDialog
            open={dialogOpen}
            saving={saveMutation.isPending}
            offer={editing}
            onOpenChange={(open) => {
              setDialogOpen(open)
              if (!open) setEditing(null)
            }}
            onSubmit={(input) => saveMutation.mutate(input)}
          />

          <ConfirmDialog
            open={deleteTarget !== null}
            title={t('Delete channel')}
            desc={t(
              'The channel is disabled immediately and disappears from the market.'
            )}
            destructive
            isLoading={deleteMutation.isPending}
            handleConfirm={() => {
              if (deleteTarget !== null) deleteMutation.mutate(deleteTarget)
            }}
            onOpenChange={(open) => {
              if (!open) setDeleteTarget(null)
            }}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
