import { useQuery } from '@tanstack/react-query'
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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

import { getAllTickets } from './api'
import { ClaimBadge, StatusBadge, TicketDetail } from './index'
import type { Ticket } from './types'

function formatTime(ts: number): string {
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleString()
}

const STATUS_FILTERS: Array<{
  value: '' | Ticket['status']
  labelKey: string
}> = [
  { value: '', labelKey: 'All' },
  { value: 'open', labelKey: 'Open' },
  { value: 'closed', labelKey: 'Closed' },
]

type AssigneeFilter = '' | 'pending' | 'mine'

const ASSIGNEE_FILTERS: Array<{
  value: AssigneeFilter
  labelKey: string
}> = [
  { value: '', labelKey: 'All claims' },
  { value: 'pending', labelKey: 'Unclaimed' },
  { value: 'mine', labelKey: 'Claimed by me' },
]

function AdminTicketList({
  isLoading,
  tickets,
  onSelect,
}: {
  isLoading: boolean
  tickets: Ticket[]
  onSelect: (id: number) => void
}) {
  const { t } = useTranslation()
  if (isLoading) {
    return <Skeleton className='h-60 w-full rounded-xl' />
  }
  if (tickets.length === 0) {
    return (
      <Card data-card-hover='false'>
        <CardContent className='text-muted-foreground p-6 text-center'>
          {t('No tickets yet')}
        </CardContent>
      </Card>
    )
  }
  return (
    <div className='divide-y rounded-lg border'>
      {tickets.map((ticket: Ticket) => (
        <button
          key={ticket.id}
          type='button'
          onClick={() => onSelect(ticket.id)}
          className='hover:bg-muted/40 flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors'
        >
          <div className='min-w-0'>
            <p className='truncate font-medium'>{ticket.subject}</p>
            <p className='text-muted-foreground text-xs'>
              {ticket.username} · {formatTime(ticket.updated_at)}
            </p>
          </div>
          <div className='flex shrink-0 items-center gap-2'>
            <ClaimBadge ticket={ticket} />
            <StatusBadge status={ticket.status} />
          </div>
        </button>
      ))}
    </div>
  )
}

export function AdminTickets() {
  const { t } = useTranslation()
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [statusFilter, setStatusFilter] = useState<'' | Ticket['status']>('')
  const [assigneeFilter, setAssigneeFilter] = useState<AssigneeFilter>('')

  const { data, isLoading } = useQuery({
    queryKey: ['admin-tickets', statusFilter, assigneeFilter],
    queryFn: () =>
      getAllTickets(
        1,
        100,
        statusFilter || undefined,
        assigneeFilter || undefined
      ),
  })

  const tickets = data?.data?.items ?? []

  if (selectedId != null) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Ticket Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <TicketDetail
            ticketId={selectedId}
            adminView
            onBack={() => setSelectedId(null)}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Ticket Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='flex flex-wrap items-center gap-2'>
            {STATUS_FILTERS.map((filter) => (
              <Button
                key={filter.value || 'all'}
                variant={statusFilter === filter.value ? 'default' : 'outline'}
                size='sm'
                onClick={() => setStatusFilter(filter.value)}
              >
                {t(filter.labelKey)}
              </Button>
            ))}
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            {ASSIGNEE_FILTERS.map((filter) => (
              <Button
                key={filter.value || 'all-claims'}
                variant={
                  assigneeFilter === filter.value ? 'default' : 'outline'
                }
                size='sm'
                onClick={() => setAssigneeFilter(filter.value)}
              >
                {t(filter.labelKey)}
              </Button>
            ))}
          </div>
          <AdminTicketList
            isLoading={isLoading}
            tickets={tickets}
            onSelect={setSelectedId}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
