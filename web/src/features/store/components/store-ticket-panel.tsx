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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  addStoreTicketMessage,
  complainStoreTicket,
  getStoreTicket,
  listAdminComplaints,
  listMyStoreTickets,
  listStoreSalesTickets,
  refundStoreTicket,
  resolveStoreTicket,
} from '../api'

const STATUS_LABEL: Record<number, string> = {
  0: 'Open',
  1: 'Refunded',
  2: 'Complained',
  3: 'Resolved',
  4: 'Closed',
}

function statusVariant(status: number) {
  if (status === 2) return 'destructive' as const
  if (status === 1 || status === 3) return 'secondary' as const
  return 'outline' as const
}

export interface StoreTicketPanelProps {
  /** buyer = my purchases, seller = tickets raised against me, admin = escalations */
  mode: 'buyer' | 'seller' | 'admin'
}

/** After-sales: a private thread with the merchant, refund, and escalation. */
export function StoreTicketPanel(props: StoreTicketPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [openTicketId, setOpenTicketId] = useState<number | null>(null)
  const [reply, setReply] = useState('')
  const [complaint, setComplaint] = useState('')

  const listQuery = useQuery({
    queryKey: ['store-tickets', props.mode],
    queryFn: async () => {
      if (props.mode === 'buyer') return listMyStoreTickets()
      if (props.mode === 'seller') return listStoreSalesTickets()
      return listAdminComplaints()
    },
  })

  const detailQuery = useQuery({
    queryKey: ['store-ticket', openTicketId],
    queryFn: () => getStoreTicket(openTicketId ?? 0),
    enabled: openTicketId !== null,
  })

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['store-tickets'] })
    void queryClient.invalidateQueries({
      queryKey: ['store-ticket', openTicketId],
    })
  }

  const fail = (fallback: string) => (error: unknown) =>
    toast.error(getServerErrorMessage(error, t(fallback)))

  const replyMutation = useMutation({
    mutationFn: () => addStoreTicketMessage(openTicketId ?? 0, reply),
    onSuccess: () => {
      setReply('')
      refresh()
    },
    onError: fail('Send failed'),
  })

  const refundMutation = useMutation({
    mutationFn: () => refundStoreTicket(openTicketId ?? 0),
    onSuccess: () => {
      toast.success(t('Refunded'))
      refresh()
    },
    onError: fail('Refund failed'),
  })

  const complainMutation = useMutation({
    mutationFn: () => complainStoreTicket(openTicketId ?? 0, complaint),
    onSuccess: () => {
      toast.success(t('Complaint submitted'))
      setComplaint('')
      refresh()
    },
    onError: fail('Complaint failed'),
  })

  const resolveMutation = useMutation({
    mutationFn: () => resolveStoreTicket(openTicketId ?? 0, reply),
    onSuccess: () => {
      toast.success(t('Resolved'))
      setReply('')
      refresh()
    },
    onError: fail('Save failed'),
  })

  const tickets = listQuery.data ?? []
  const detail = detailQuery.data

  return (
    <div className='grid gap-3'>
      {tickets.length === 0 && !listQuery.isLoading && (
        <p className='text-muted-foreground text-sm'>
          {t('No after-sales cases yet')}
        </p>
      )}

      {tickets.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Time')}</TableHead>
              <TableHead>{t('Purchase')}</TableHead>
              <TableHead>{t('Buyer')}</TableHead>
              <TableHead>{t('Seller')}</TableHead>
              <TableHead>{t('Reason')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-end'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tickets.map((ticket) => (
              <TableRow key={ticket.id}>
                <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                  {dayjs.unix(ticket.created_at).format('YYYY-MM-DD HH:mm')}
                </TableCell>
                <TableCell className='max-w-40 truncate'>
                  {ticket.product_name || `#${ticket.order_id}`}
                </TableCell>
                <TableCell className='max-w-28 truncate'>
                  {ticket.buyer_name}
                </TableCell>
                <TableCell className='max-w-28 truncate'>
                  {ticket.seller_name}
                </TableCell>
                <TableCell className='max-w-48 truncate text-xs'>
                  {ticket.reason}
                </TableCell>
                <TableCell>
                  <Badge variant={statusVariant(ticket.status)}>
                    {t(STATUS_LABEL[ticket.status] ?? 'Open')}
                  </Badge>
                </TableCell>
                <TableCell className='text-end'>
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => setOpenTicketId(ticket.id)}
                  >
                    {t('Open')}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {openTicketId !== null && detail && (
        <div className='grid gap-3 rounded-lg border p-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='text-sm'>
              <span className='font-medium'>{detail.ticket.product_name}</span>
              <span className='text-muted-foreground ml-2 text-xs'>
                {t('Buyer')}: {detail.ticket.buyer_name} · {t('Seller')}:{' '}
                {detail.ticket.seller_name}
              </span>
            </div>
            <Badge variant={statusVariant(detail.ticket.status)}>
              {t(STATUS_LABEL[detail.ticket.status] ?? 'Open')}
            </Badge>
          </div>

          <p className='text-muted-foreground text-xs whitespace-pre-wrap'>
            {detail.ticket.reason}
          </p>
          {detail.ticket.complaint_reason !== '' && (
            <p className='text-destructive text-xs whitespace-pre-wrap'>
              {t('Complaint')}: {detail.ticket.complaint_reason}
            </p>
          )}

          <div className='grid max-h-64 gap-2 overflow-auto rounded-md border p-2'>
            {detail.messages.map((message) => (
              <div key={message.id} className='text-sm'>
                <span className='text-muted-foreground mr-2 text-xs'>
                  {message.sender_name} ·{' '}
                  {dayjs.unix(message.created_at).format('MM-DD HH:mm')}
                </span>
                <span className='break-words whitespace-pre-wrap'>
                  {message.body}
                </span>
              </div>
            ))}
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Input
              value={reply}
              placeholder={t('Reply')}
              onChange={(event) => setReply(event.target.value)}
            />
            <Button
              size='sm'
              disabled={replyMutation.isPending || reply.trim() === ''}
              onClick={() => replyMutation.mutate()}
            >
              {t('Send')}
            </Button>
            {props.mode !== 'buyer' && (
              <Button
                size='sm'
                variant='destructive'
                disabled={refundMutation.isPending}
                onClick={() => refundMutation.mutate()}
              >
                {t('Refund')}
              </Button>
            )}
            {props.mode === 'admin' && (
              <Button
                size='sm'
                variant='outline'
                disabled={resolveMutation.isPending}
                onClick={() => resolveMutation.mutate()}
              >
                {t('Resolve')}
              </Button>
            )}
            {props.mode === 'buyer' && (
              <>
                <Input
                  className='max-w-56'
                  value={complaint}
                  placeholder={t('Complaint reason')}
                  onChange={(event) => setComplaint(event.target.value)}
                />
                <Button
                  size='sm'
                  variant='outline'
                  disabled={
                    complainMutation.isPending || complaint.trim() === ''
                  }
                  onClick={() => complainMutation.mutate()}
                >
                  {t('Complain')}
                </Button>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
