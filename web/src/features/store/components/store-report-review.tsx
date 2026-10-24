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
import dayjs from 'dayjs'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

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

import type { StoreChannelReport } from '../types'

export interface StoreReportReviewProps {
  reports: StoreChannelReport[]
  onReview: (report: StoreChannelReport, accept: boolean, note: string) => void
}

/** Administrator queue: accept a report to take the channel down. */
export function StoreReportReview(props: StoreReportReviewProps) {
  const { t } = useTranslation()
  const [notes, setNotes] = useState<Record<number, string>>({})

  return (
    <div className='grid gap-3'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Offer')}</TableHead>
            <TableHead>{t('Reason')}</TableHead>
            <TableHead>{t('Time')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Review note')}</TableHead>
            <TableHead className='text-end'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.reports.map((report) => (
            <TableRow key={report.id}>
              <TableCell>#{report.offer_id}</TableCell>
              <TableCell className='max-w-64 text-xs break-words whitespace-pre-wrap'>
                {report.reason}
              </TableCell>
              <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                {dayjs.unix(report.created_at).format('YYYY-MM-DD HH:mm')}
              </TableCell>
              <TableCell>
                {report.status === 0 && (
                  <Badge variant='warning'>{t('Pending')}</Badge>
                )}
                {report.status === 1 && (
                  <Badge variant='outline'>{t('Rejected')}</Badge>
                )}
                {report.status === 2 && (
                  <Badge variant='destructive'>{t('Accepted')}</Badge>
                )}
              </TableCell>
              <TableCell>
                {report.status === 0 ? (
                  <Input
                    value={notes[report.id] ?? ''}
                    placeholder={t('Optional')}
                    onChange={(event) =>
                      setNotes((current) => ({
                        ...current,
                        [report.id]: event.target.value,
                      }))
                    }
                  />
                ) : (
                  <span className='text-muted-foreground text-xs'>
                    {report.review_note || '-'}
                  </span>
                )}
              </TableCell>
              <TableCell>
                <div className='flex items-center justify-end gap-1'>
                  <Button
                    size='sm'
                    variant='destructive'
                    disabled={report.status !== 0}
                    onClick={() =>
                      props.onReview(report, true, notes[report.id] ?? '')
                    }
                  >
                    {t('Take down')}
                  </Button>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={report.status !== 0}
                    onClick={() =>
                      props.onReview(report, false, notes[report.id] ?? '')
                    }
                  >
                    {t('Reject')}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {props.reports.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('Nothing to review')}
        </p>
      )}
    </div>
  )
}
