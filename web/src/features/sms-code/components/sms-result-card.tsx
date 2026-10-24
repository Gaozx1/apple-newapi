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

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'

import type { SmsCallResult } from '../types'

/**
 * One result of a getPhone/getMsg/send call. A vendor refusal arrives as
 * `code: "error"` with `reason`, which is rendered next to the value fields so
 * the operator sees the vendor's own wording. A reading that has not received
 * the message yet arrives as `code: "pending"` and is never billed.
 */
export function SmsResultCard(props: { result: SmsCallResult | null }) {
  const { t } = useTranslation()
  const result = props.result
  if (result === null) return null

  const ok = result.code === 'ok'
  const pending = result.code === 'pending'
  let badgeVariant: 'secondary' | 'warning' | 'destructive' = 'destructive'
  let badgeLabel = t('Failed')
  if (ok) {
    badgeVariant = 'secondary'
    badgeLabel = t('Success')
  } else if (pending) {
    badgeVariant = 'warning'
    badgeLabel = t('No message yet')
  }

  return (
    <div
      data-testid='sms-result'
      className='bg-muted/40 grid gap-2 rounded-lg border p-3'
    >
      <div className='flex flex-wrap items-center gap-2'>
        <Badge variant={badgeVariant}>{badgeLabel}</Badge>
        {result.phone !== undefined && result.phone !== '' && (
          <span className='flex items-center gap-1'>
            <span className='font-mono text-sm'>{result.phone}</span>
            <CopyButton
              value={result.phone}
              aria-label={t('Copy number')}
              className='size-6'
            />
          </span>
        )}
        {!pending && result.pending === true && (
          <Badge variant='warning'>{t('No message yet')}</Badge>
        )}
      </div>

      {result.message !== undefined && result.message !== '' && (
        <p className='text-sm break-all whitespace-pre-wrap'>
          {result.message}
        </p>
      )}

      {!ok && result.reason !== undefined && result.reason !== '' && (
        <p className='text-destructive text-sm break-all'>{result.reason}</p>
      )}

      {result.task_id !== undefined && result.task_id !== '' && (
        <p className='text-muted-foreground text-xs'>
          {t('Task ID')}: {result.task_id}
        </p>
      )}

      {result.raw !== undefined &&
        result.raw !== '' &&
        result.raw !== result.message &&
        result.raw !== result.phone && (
          <details>
            <summary className='text-muted-foreground cursor-pointer text-xs'>
              {t('Raw response')}
            </summary>
            <pre className='text-muted-foreground mt-1 text-xs break-all whitespace-pre-wrap'>
              {result.raw}
            </pre>
          </details>
        )}
    </div>
  )
}
