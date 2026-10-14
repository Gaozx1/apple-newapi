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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { createServerError } from '@/lib/server-error-message'

import { getJailbreakRecords } from '../api'
import {
  actionLabelKey,
  contentPreview,
  jailbreakActionVariant,
} from '../lib/records'
import type { JailbreakRecord } from '../types'
import { JailbreakRecordDetailDialog } from './record-detail-dialog'

const route = getRouteApi('/_authenticated/jailbreak-records/')

function useJailbreakRecordColumns(
  onInspect: (record: JailbreakRecord) => void
) {
  const { t } = useTranslation()

  return useMemo<ColumnDef<JailbreakRecord>[]>(
    () => [
      {
        id: 'created_at',
        header: t('Detected At'),
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs tabular-nums'>
            {new Date((row.original.created_at || 0) * 1000).toLocaleString()}
          </span>
        ),
      },
      {
        id: 'user',
        header: t('User'),
        cell: ({ row }) => (
          <div className='min-w-0'>
            <div className='truncate text-sm font-medium'>
              {row.original.username || `#${row.original.user_id}`}
            </div>
            <div className='text-muted-foreground text-xs'>
              #{row.original.user_id}
            </div>
          </div>
        ),
      },
      {
        id: 'channel',
        header: t('Channel'),
        cell: ({ row }) => (
          <div className='min-w-0'>
            <div className='truncate text-sm'>
              {row.original.channel_name || `#${row.original.channel_id}`}
            </div>
            <div className='text-muted-foreground text-xs'>
              {row.original.model_name || '-'}
            </div>
          </div>
        ),
      },
      {
        id: 'reason',
        header: t('Reason'),
        cell: ({ row }) => (
          <span className='text-sm'>{row.original.reason || '-'}</span>
        ),
      },
      {
        id: 'content',
        header: t('User message'),
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs'>
            {contentPreview(row.original.content) || '-'}
          </span>
        ),
      },
      {
        id: 'action',
        header: t('Handling'),
        cell: ({ row }) => (
          <StatusBadge
            label={t(actionLabelKey(row.original.action))}
            variant={jailbreakActionVariant(row.original.action)}
            copyable={false}
          />
        ),
      },
      {
        id: 'inspect',
        header: t('Actions'),
        cell: ({ row }) => (
          <Button
            variant='ghost'
            size='sm'
            onClick={() => onInspect(row.original)}
          >
            {t('View details')}
          </Button>
        ),
      },
    ],
    [t, onInspect]
  )
}

/**
 * Anti-jailbreak verdicts recorded by the channel guard. Administrators review
 * the exact text a user sent before deciding whether a ban was justified.
 */
export function JailbreakRecordsTable() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [selected, setSelected] = useState<JailbreakRecord | null>(null)

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'jailbreak-records',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
    ],
    queryFn: async () => {
      const result = await getJailbreakRecords({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        keyword: globalFilter,
      })
      if (!result.success) {
        throw createServerError(result, t('Failed to load jailbreak records'))
      }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const records = data?.items || []
  const onInspect = useMemo(
    () => (record: JailbreakRecord) => setSelected(record),
    []
  )
  const columns = useJailbreakRecordColumns(onInspect)

  const { table } = useDataTable({
    data: records,
    columns,
    columnFilters,
    globalFilter,
    pagination,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    ensurePageInRange,
  })

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No jailbreak records')}
        emptyDescription={t(
          'Nothing has been flagged yet. Enable the anti-jailbreak guard on a channel to start judging its traffic.'
        )}
        skeletonKeyPrefix='jailbreak-records-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Filter by user, channel or content...'),
          searchDebounceMs: 500,
        }}
      />
      {selected && (
        <JailbreakRecordDetailDialog
          record={selected}
          open
          onOpenChange={(open) => {
            if (!open) setSelected(null)
          }}
        />
      )}
    </>
  )
}
