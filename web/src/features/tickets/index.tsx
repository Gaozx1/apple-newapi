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
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'

import {
  createTicket,
  getMyTickets,
  getTicket,
  replyTicket,
} from './api'
import type { Ticket, TicketReply } from './types'

function formatTime(ts: number): string {
  if (!ts) return ''
  return new Date(ts * 1000).toLocaleString()
}

function StatusBadge({ status }: { status: Ticket['status'] }) {
  const { t } = useTranslation()
  const isOpen = status === 'open'
  return (
    <Badge variant={isOpen ? 'default' : 'secondary'}>
      {t(isOpen ? 'Open' : 'Closed')}
    </Badge>
  )
}

function TicketReplyItem({ reply }: { reply: TicketReply }) {
  const { t } = useTranslation()
  return (
    <div
      className={
        reply.is_admin
          ? 'bg-primary/5 rounded-lg border border-primary/20 p-3'
          : 'bg-muted/40 rounded-lg p-3'
      }
    >
      <div className='text-muted-foreground mb-1 flex items-center justify-between text-xs'>
        <span className='font-medium'>
          {reply.is_admin ? t('Support') : t('You')}
        </span>
        <span>{formatTime(reply.created_at)}</span>
      </div>
      <p className='whitespace-pre-wrap break-words text-sm'>{reply.content}</p>
    </div>
  )
}

function TicketDetail({
  ticketId,
  onBack,
}: {
  ticketId: number
  onBack: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [replyContent, setReplyContent] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['ticket-detail', ticketId],
    queryFn: () => getTicket(ticketId),
  })

  const replyMutation = useMutation({
    mutationFn: (content: string) => replyTicket(ticketId, { content }),
    onSuccess: (res) => {
      if (res.success) {
        setReplyContent('')
        queryClient.invalidateQueries({ queryKey: ['ticket-detail', ticketId] })
        queryClient.invalidateQueries({ queryKey: ['my-tickets'] })
        toast.success(t('Reply sent'))
      } else {
        toast.error(res.message || t('Failed to send reply'))
      }
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : t('Failed to send reply')
      toast.error(msg)
    },
  })

  if (isLoading) {
    return (
      <div className='space-y-3'>
        <Skeleton className='h-8 w-48' />
        <Skeleton className='h-40 w-full rounded-xl' />
        <Skeleton className='h-24 w-full rounded-xl' />
      </div>
    )
  }

  const detail = data?.data
  if (!detail) {
    return (
      <div className='space-y-3'>
        <Button variant='ghost' onClick={onBack}>
          {t('Back')}
        </Button>
        <Card data-card-hover='false'>
          <CardContent className='p-6 text-center text-muted-foreground'>
            {t('Ticket not found')}
          </CardContent>
        </Card>
      </div>
    )
  }

  const { ticket, replies } = detail
  const isClosed = ticket.status === 'closed'

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <Button variant='ghost' onClick={onBack}>
          {t('Back')}
        </Button>
        <StatusBadge status={ticket.status} />
      </div>

      <Card data-card-hover='false'>
        <CardHeader>
          <CardTitle className='text-base'>{ticket.subject}</CardTitle>
          <p className='text-muted-foreground text-xs'>
            {t('Created at')}: {formatTime(ticket.created_at)}
          </p>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='bg-muted/40 rounded-lg p-3'>
            <p className='text-muted-foreground mb-1 text-xs'>
              {t('Original message')}
            </p>
            <p className='whitespace-pre-wrap break-words text-sm'>
              {ticket.content}
            </p>
          </div>

          <div className='space-y-2'>
            {replies
              .slice(1)
              .map((reply) => (
                <TicketReplyItem key={reply.id} reply={reply} />
              ))}
          </div>

          {isClosed ? (
            <p className='text-muted-foreground text-sm'>
              {t('This ticket is closed')}
            </p>
          ) : (
            <div className='space-y-2'>
              <Textarea
                placeholder={t('Type your reply...')}
                value={replyContent}
                onChange={(e) => setReplyContent(e.target.value)}
                disabled={replyMutation.isPending}
              />
              <Button
                onClick={() => {
                  if (replyContent.trim()) replyMutation.mutate(replyContent.trim())
                }}
                disabled={replyMutation.isPending || !replyContent.trim()}
                className='w-full sm:w-auto'
              >
                {replyMutation.isPending ? t('Sending...') : t('Send reply')}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function NewTicketForm({ onCreated }: { onCreated: (id: number) => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [subject, setSubject] = useState('')
  const [content, setContent] = useState('')

  const createMutation = useMutation({
    mutationFn: () => createTicket({ subject, content }),
    onSuccess: (res) => {
      if (res.success && res.data) {
        queryClient.invalidateQueries({ queryKey: ['my-tickets'] })
        toast.success(t('Ticket submitted'))
        onCreated(res.data.id)
      } else {
        toast.error(res.message || t('Failed to submit ticket'))
      }
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : t('Failed to submit ticket')
      toast.error(msg)
    },
  })

  return (
    <Card data-card-hover='false'>
      <CardHeader>
        <CardTitle>{t('New Ticket')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        <div className='space-y-1'>
          <label className='text-sm font-medium' htmlFor='ticket-subject'>
            {t('Subject')}
          </label>
          <Input
            id='ticket-subject'
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            placeholder={t('Brief summary of your issue')}
            maxLength={255}
          />
        </div>
        <div className='space-y-1'>
          <label className='text-sm font-medium' htmlFor='ticket-content'>
            {t('Description')}
          </label>
          <Textarea
            id='ticket-content'
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder={t('Describe your issue in detail')}
            maxLength={5000}
          />
        </div>
        <Button
          onClick={() => createMutation.mutate()}
          disabled={createMutation.isPending || !subject.trim() || !content.trim()}
          className='w-full sm:w-auto'
        >
          {createMutation.isPending ? t('Submitting...') : t('Submit ticket')}
        </Button>
      </CardContent>
    </Card>
  )
}

function TicketList({
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
        <CardContent className='p-6 text-center text-muted-foreground'>
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
              {formatTime(ticket.updated_at)}
            </p>
          </div>
          <StatusBadge status={ticket.status} />
        </button>
      ))}
    </div>
  )
}

export function Tickets() {
  const { t } = useTranslation()
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [showNew, setShowNew] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['my-tickets'],
    queryFn: () => getMyTickets(1, 50),
  })

  const tickets = data?.data?.items ?? []

  if (selectedId != null) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Support Tickets')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <TicketDetail
            ticketId={selectedId}
            onBack={() => setSelectedId(null)}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Support Tickets')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {showNew ? (
            <NewTicketForm
              onCreated={(id) => {
                setShowNew(false)
                setSelectedId(id)
              }}
            />
          ) : (
            <Button
              onClick={() => setShowNew(true)}
              className='w-full sm:w-auto'
            >
              {t('New Ticket')}
            </Button>
          )}

          <TicketList
            isLoading={isLoading}
            tickets={tickets}
            onSelect={setSelectedId}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
