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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable, StaticRowActions } from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { requireServerSuccess } from '@/lib/server-error-message'

import { updateSystemOption } from '../api'
import { PaymentChannelDialog } from './payment-channel-dialog'
import { usePaymentChannels, type PaymentChannel } from './payment-channels'

export function PaymentChannelsEditor() {
  const { t } = useTranslation()
  const { query, channels } = usePaymentChannels()
  const client = useQueryClient()
  const [draft, setDraft] = useState<PaymentChannel | null>(null)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState<PaymentChannel | null>(null)
  const save = useMutation({
    mutationFn: async (next: PaymentChannel[]) =>
      requireServerSuccess(
        await updateSystemOption({
          key: 'PaymentChannels',
          value: JSON.stringify(next),
        })
      ),
    onSuccess: async () => {
      setDraft(null)
      setDeleting(null)
      toast.success(t('Saved'))
      await client.invalidateQueries({ queryKey: ['system-options'] })
    },
  })
  if (query.isLoading) return <LoadingState />
  if (query.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void query.refetch()
        }}
      />
    )
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h3 className='text-lg font-medium'>{t('Payment channels')}</h3>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Create channels here, then bind each payment method in the General tab.'
            )}
          </p>
        </div>
        <Button
          type='button'
          disabled={save.isPending}
          onClick={() => {
            setEditing(false)
            setDraft({
              id: '',
              name: '',
              address: '',
              merchant_id: '',
              key: '',
              key_configured: false,
              enabled: true,
            })
          }}
        >
          <Plus aria-hidden='true' />
          {t('Add payment channel')}
        </Button>
      </div>
      {channels.length === 0 ? (
        <EmptyState
          title={t('No payment channels')}
          description={t(
            'Methods without a channel use the existing gateway settings.'
          )}
        />
      ) : (
        <StaticDataTable
          data={channels}
          getRowKey={(channel) => channel.id}
          columns={[
            {
              id: 'name',
              header: t('Name'),
              cell: (channel) => (
                <span className='break-all'>{channel.name}</span>
              ),
            },
            {
              id: 'id',
              header: t('Channel ID'),
              cell: (channel) => channel.id,
            },
            {
              id: 'address',
              header: t('Gateway address'),
              cell: (channel) => (
                <span className='break-all'>{channel.address}</span>
              ),
            },
            {
              id: 'status',
              header: t('Status'),
              cell: (channel) =>
                channel.enabled ? t('Enabled') : t('Disabled'),
            },
            {
              id: 'actions',
              header: t('Actions'),
              cell: (channel) => (
                <StaticRowActions
                  editLabel={t('Edit')}
                  deleteLabel={t('Delete')}
                  menuLabel={t('Open menu')}
                  onEdit={() => {
                    setEditing(true)
                    setDraft(channel)
                  }}
                  onDelete={() => setDeleting(channel)}
                />
              ),
            },
          ]}
        />
      )}
      {draft && (
        <PaymentChannelDialog
          key={draft.id}
          channel={draft}
          editing={editing}
          pending={save.isPending}
          onClose={() => setDraft(null)}
          onSave={(channel) => {
            if (!editing && channels.some((item) => item.id === channel.id)) {
              toast.error(t('Channel ID already exists'))
              return
            }
            const next = editing
              ? channels.map((item) =>
                  item.id === channel.id ? channel : item
                )
              : [...channels, channel]
            save.mutate(next)
          }}
        />
      )}
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete payment channel')}
        desc={t(
          'Unbind its payment methods first. Existing orders keep their original payment channel.'
        )}
        destructive
        isLoading={save.isPending}
        confirmText={t('Delete')}
        handleConfirm={() => {
          if (deleting) {
            save.mutate(
              channels.filter((channel) => channel.id !== deleting.id)
            )
          }
        }}
      />
    </div>
  )
}
