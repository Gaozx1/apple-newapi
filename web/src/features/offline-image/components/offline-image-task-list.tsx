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
import { ImageIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { deleteOfflineImageTask, listOfflineImageTasks } from '../api'
import { isOfflineImageTaskActive } from '../lib/task'
import type { OfflineImageTask } from '../types'
import { OfflineImageTaskCard } from './offline-image-task-card'

const PAGE_SIZE = 10
const ACTIVE_REFETCH_MS = 4000

export function OfflineImageTaskList() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isAdmin = useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.ADMIN
  )
  const [page, setPage] = useState(1)
  const [pendingDelete, setPendingDelete] = useState<OfflineImageTask | null>(
    null
  )

  const tasksQuery = useQuery({
    queryKey: ['offline-image-tasks', page, isAdmin],
    queryFn: () =>
      listOfflineImageTasks({
        p: page,
        page_size: PAGE_SIZE,
        all: isAdmin,
      }),
    // Finished tasks never change, so polling stops once the queue is idle.
    refetchInterval: (query) =>
      query.state.data?.items.some(isOfflineImageTaskActive)
        ? ACTIVE_REFETCH_MS
        : false,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteOfflineImageTask,
    onSuccess: () => {
      toast.success(t('Task deleted'))
      setPendingDelete(null)
      void queryClient.invalidateQueries({ queryKey: ['offline-image-tasks'] })
    },
    onError: (error) => {
      handleServerError(error, t('Failed to delete the task'))
    },
  })

  const items = tasksQuery.data?.items ?? []
  const total = tasksQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className='flex flex-col gap-3'>
      <div className='flex items-center justify-between'>
        <h2 className='text-lg font-semibold'>{t('Tasks')}</h2>
        {total > 0 && (
          <span className='text-muted-foreground text-xs'>
            {t('{{count}} task(s)', { count: total })}
          </span>
        )}
      </div>

      {tasksQuery.isLoading && (
        <div className='flex flex-col gap-3'>
          <Skeleton className='h-28 w-full' />
          <Skeleton className='h-28 w-full' />
        </div>
      )}

      {!tasksQuery.isLoading && items.length === 0 && (
        <EmptyState
          icon={ImageIcon}
          title={t('No offline task yet')}
          description={t('Queue a task above and come back when it is done.')}
          bordered
        />
      )}

      {items.map((task) => (
        <OfflineImageTaskCard
          key={task.id}
          task={task}
          onDelete={setPendingDelete}
        />
      ))}

      {totalPages > 1 && (
        <div className='flex items-center justify-end gap-2'>
          <Button
            type='button'
            variant='outline'
            disabled={page <= 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
          >
            {t('Previous')}
          </Button>
          <span className='text-muted-foreground text-xs'>
            {page} / {totalPages}
          </span>
          <Button
            type='button'
            variant='outline'
            disabled={page >= totalPages}
            onClick={() => setPage((current) => current + 1)}
          >
            {t('Next')}
          </Button>
        </div>
      )}

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => {
          if (!open) setPendingDelete(null)
        }}
        title={t('Delete task')}
        desc={t(
          'The task and its images are removed from the server. This cannot be undone.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (pendingDelete) deleteMutation.mutate(pendingDelete.id)
        }}
      />
    </div>
  )
}
