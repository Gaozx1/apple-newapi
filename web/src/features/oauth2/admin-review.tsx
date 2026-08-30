import { useQuery, useQueryClient } from '@tanstack/react-query'
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
import { Check, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import {
  adminListAllOAuthClients,
  adminReviewOAuthClient,
  type OAuthClient,
} from './api'

function ClientStatusBadge(props: { status?: string }) {
  const { t } = useTranslation()
  if (props.status === 'pending') {
    return (
      <Badge
        variant='outline'
        className='border-amber-500/40 text-amber-600 dark:text-amber-400'
      >
        {t('Pending review')}
      </Badge>
    )
  }
  if (props.status === 'rejected') {
    return (
      <Badge
        variant='outline'
        className='border-destructive/40 text-destructive'
      >
        {t('Rejected')}
      </Badge>
    )
  }
  return (
    <Badge
      variant='outline'
      className='border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
    >
      {t('Approved')}
    </Badge>
  )
}

function ClientReviewRow(props: {
  client: OAuthClient
  saving: boolean
  onReview: (client: OAuthClient, status: 'approved' | 'rejected') => void
}) {
  const { t } = useTranslation()
  const pending = props.client.status === 'pending'
  return (
    <div className='rounded-lg border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0 space-y-0.5'>
          <div className='flex items-center gap-2'>
            <span className='truncate text-sm font-medium'>
              {props.client.name}
            </span>
            <ClientStatusBadge status={props.client.status} />
            {!props.client.enabled && (
              <Badge variant='outline' className='text-muted-foreground'>
                {t('Disabled')}
              </Badge>
            )}
          </div>
          <p className='text-muted-foreground truncate font-mono text-xs'>
            {props.client.client_id} · {t('Owner ID')}: {props.client.user_id}
          </p>
          <p className='text-muted-foreground truncate text-xs'>
            {t('Redirect URIs')}: {props.client.redirect_uris || '-'} ·{' '}
            {t('Scopes')}: {props.client.scopes || '-'}
          </p>
        </div>
        <div className='flex shrink-0 items-center gap-1.5'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={props.saving || !pending}
            onClick={() => props.onReview(props.client, 'approved')}
          >
            <Check className='mr-1 h-3.5 w-3.5' />
            {t('Approve')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='destructive'
            disabled={props.saving || !pending}
            onClick={() => props.onReview(props.client, 'rejected')}
          >
            <X className='mr-1 h-3.5 w-3.5' />
            {t('Reject')}
          </Button>
        </div>
      </div>
    </div>
  )
}

export function OAuth2ClientReview() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [savingId, setSavingId] = useState<number | null>(null)

  const clientsQuery = useQuery({
    queryKey: ['oauth-admin-clients'],
    queryFn: adminListAllOAuthClients,
  })

  const handleReview = async (
    client: OAuthClient,
    status: 'approved' | 'rejected'
  ) => {
    setSavingId(client.id)
    try {
      const res = await adminReviewOAuthClient(client.id, status)
      if (!res.success) {
        toast.error(res.message || t('Update failed'))
        return
      }
      toast.success(t('Update succeeded'))
      await queryClient.invalidateQueries({
        queryKey: ['oauth-admin-clients'],
      })
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setSavingId(null)
    }
  }

  const clients = clientsQuery.data?.data || []
  const pendingCount = clients.filter((c) => c.status === 'pending').length

  let clientList
  if (clientsQuery.isLoading) {
    clientList = <Skeleton className='h-24 w-full rounded-xl' />
  } else if (clients.length === 0) {
    clientList = (
      <p className='text-muted-foreground text-sm'>{t('No apps yet')}</p>
    )
  } else {
    clientList = (
      <div className='space-y-3'>
        {clients.map((client) => (
          <ClientReviewRow
            key={client.id}
            client={client}
            saving={savingId !== null}
            onReview={handleReview}
          />
        ))}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('OAuth 2.0 Client Review')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-4xl space-y-4'>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Clients registered by users stay unusable until approved. Rejecting a client also revokes its codes and access tokens.'
            )}
            {clients.length > 0 && pendingCount > 0 && (
              <>
                {' · '}
                {t('{{count}} pending review', { count: pendingCount })}
              </>
            )}
          </p>
          {clientList}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
