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

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'

import { actionLabelKey, jailbreakActionVariant } from '../lib/records'
import type { JailbreakRecord } from '../types'

interface Props {
  record: JailbreakRecord
  open: boolean
  onOpenChange: (open: boolean) => void
}

function Field(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='grid grid-cols-[7rem_1fr] gap-2 text-sm'>
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='min-w-0 break-words'>{props.children}</span>
    </div>
  )
}

/**
 * Full evidence of one verdict, including the exact text the user sent and the
 * classifier's raw reply — everything an administrator needs to review a ban.
 */
export function JailbreakRecordDetailDialog(props: Props) {
  const { t } = useTranslation()
  const record = props.record

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Jailbreak Record')}
      description={t('What the user sent and why the classifier flagged it.')}
      contentHeight='70vh'
    >
      <div className='space-y-2'>
        <Field label={t('User')}>
          {record.username || `#${record.user_id}`}
          <span className='text-muted-foreground'> (#{record.user_id})</span>
        </Field>
        <Field label={t('Channel')}>
          {record.channel_name || `#${record.channel_id}`}
          <span className='text-muted-foreground'> (#{record.channel_id})</span>
        </Field>
        <Field label={t('Model')}>{record.model_name || '-'}</Field>
        <Field label={t('API Key')}>{record.token_name || '-'}</Field>
        <Field label={t('Judge Model')}>{record.judge_model || '-'}</Field>
        <Field label={t('Handling')}>
          <StatusBadge
            label={t(actionLabelKey(record.action))}
            variant={jailbreakActionVariant(record.action)}
            copyable={false}
          />
        </Field>
        <Field label={t('Reason')}>{record.reason || '-'}</Field>
        <Field label={t('Detected At')}>
          {new Date((record.created_at || 0) * 1000).toLocaleString()}
        </Field>
        <Field label={t('Request ID')}>{record.request_id || '-'}</Field>
      </div>

      <div className='mt-4 space-y-1'>
        <div className='text-sm font-medium'>{t('User message')}</div>
        <pre className='bg-muted max-h-64 overflow-auto rounded-md p-3 text-xs whitespace-pre-wrap'>
          {record.content || '-'}
        </pre>
      </div>

      <div className='mt-3 space-y-1'>
        <div className='text-sm font-medium'>{t('Classifier Reply')}</div>
        <pre className='bg-muted max-h-40 overflow-auto rounded-md p-3 text-xs whitespace-pre-wrap'>
          {record.raw_verdict || '-'}
        </pre>
      </div>
    </Dialog>
  )
}
