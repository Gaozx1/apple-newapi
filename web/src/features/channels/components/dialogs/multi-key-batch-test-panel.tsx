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
import { Loader2, PlayCircle } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { handleServerError } from '@/lib/handle-server-error'

import { batchTestMultiKeys } from '../../api'
import type { BatchKeyTestSummary, Channel } from '../../types'

interface Props {
  channel: Channel
  onFinished: () => void | Promise<void>
}

/**
 * Manual "test every key" runner for a multi-key channel. Keys whose upstream
 * error proves them unusable are auto-disabled by the backend; recovered keys
 * are restored. Manually disabled keys are skipped.
 */
export function MultiKeyBatchTestPanel(props: Props) {
  const { t } = useTranslation()
  const [testModel, setTestModel] = useState(
    props.channel.test_model?.trim() || ''
  )
  const [running, setRunning] = useState(false)
  const [summary, setSummary] = useState<BatchKeyTestSummary | null>(null)

  const modelPlaceholder = useMemo(
    () => props.channel.models?.split(',')[0]?.trim() || 'gpt-4o-mini',
    [props.channel.models]
  )

  const handleRun = async () => {
    setRunning(true)
    try {
      const response = await batchTestMultiKeys({
        channel_id: props.channel.id,
        test_model: testModel.trim() || undefined,
      })
      if (!response.success) {
        handleServerError(response, t('Failed to test keys'))
        return
      }
      const data = response.data as BatchKeyTestSummary | undefined
      setSummary(data ?? null)
      if (data) {
        toast.success(
          t(
            'Tested {{tested}} keys: {{succeeded}} succeeded, {{failed}} failed, {{disabled}} auto-disabled',
            {
              tested: data.tested,
              succeeded: data.succeeded,
              failed: data.failed,
              disabled: data.auto_disabled,
            }
          )
        )
      }
      // Key statuses may have changed, so refresh the table from the server.
      await props.onFinished()
    } catch (error: unknown) {
      handleServerError(error, t('Failed to test keys'))
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className='space-y-3 rounded-md border p-3'>
      <div className='space-y-1'>
        <div className='text-sm font-medium'>{t('Batch Test Keys')}</div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Tests every key of this channel concurrently. Keys whose error proves them unusable are auto-disabled; recovered keys are restored. Manually disabled keys are skipped.'
          )}
        </p>
      </div>

      <div className='flex flex-wrap items-end gap-2'>
        <div className='min-w-[200px] flex-1 space-y-1.5'>
          <Label htmlFor='multi-key-test-model' className='text-xs'>
            {t('Test Model')}
          </Label>
          <Input
            id='multi-key-test-model'
            value={testModel}
            onChange={(event) => setTestModel(event.target.value)}
            placeholder={modelPlaceholder}
            disabled={running}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !running) handleRun()
            }}
          />
        </div>
        <Button onClick={handleRun} disabled={running}>
          {running ? (
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
          ) : (
            <PlayCircle className='mr-2 h-4 w-4' />
          )}
          {running ? t('Testing...') : t('Test All Keys')}
        </Button>
      </div>

      {summary && summary.results.length > 0 && (
        <StaticDataTable
          className='rounded-none border-0'
          tableClassName='min-w-[560px]'
          data={summary.results}
          getRowKey={(row) => row.index}
          columns={[
            {
              id: 'index',
              header: t('Index'),
              className: 'w-16',
              cellClassName: 'font-mono text-sm',
              cell: (row) => `#${row.index + 1}`,
            },
            {
              id: 'result',
              header: t('Result'),
              className: 'w-32',
              cell: (row) =>
                row.success ? (
                  <StatusBadge
                    label={t('Success')}
                    variant='success'
                    copyable={false}
                  />
                ) : (
                  <StatusBadge
                    label={t('Failed')}
                    variant='danger'
                    copyable={false}
                  />
                ),
            },
            {
              id: 'action',
              header: t('Action'),
              className: 'w-32',
              cell: (row) => {
                if (row.auto_disabled) {
                  return (
                    <StatusBadge
                      label={t('Auto Disabled')}
                      variant='danger'
                      copyable={false}
                    />
                  )
                }
                if (row.re_enabled) {
                  return (
                    <StatusBadge
                      label={t('Re-enabled')}
                      variant='success'
                      copyable={false}
                    />
                  )
                }
                return <span className='text-muted-foreground'>-</span>
              },
            },
            {
              id: 'message',
              header: t('Message'),
              className: 'min-w-[220px]',
              cellClassName: 'max-w-md truncate text-xs',
              cell: (row) => row.message || '-',
            },
          ]}
        />
      )}
    </div>
  )
}
