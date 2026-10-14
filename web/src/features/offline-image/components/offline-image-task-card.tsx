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
import { Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { formatTimestampToDate } from '@/lib/format'

import {
  isOfflineImageTaskActive,
  offlineImageStatusLabel,
  offlineImageStatusVariant,
} from '../lib/task'
import type { OfflineImageTask } from '../types'
import { OfflineImageThumbnail } from './offline-image-thumbnail'

export function OfflineImageTaskCard(props: {
  task: OfflineImageTask
  onDelete: (task: OfflineImageTask) => void
}) {
  const { t } = useTranslation()
  const task = props.task

  return (
    <Card>
      <CardContent className='flex flex-col gap-3'>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant={offlineImageStatusVariant(task.status)}>
            {t(offlineImageStatusLabel(task.status))}
          </Badge>
          <span className='truncate font-medium'>{task.model}</span>
          <Badge variant='outline'>{task.group}</Badge>
          {task.username && (
            <span className='text-muted-foreground text-xs'>
              {task.username}
            </span>
          )}
          <span className='text-muted-foreground ml-auto text-xs'>
            {formatTimestampToDate(task.created_at)}
          </span>
          <Button
            type='button'
            variant='ghost'
            aria-label={t('Delete task')}
            disabled={task.status === 'RUNNING'}
            onClick={() => props.onDelete(task)}
          >
            <Trash2 data-icon='inline-start' aria-hidden='true' />
            {t('Delete')}
          </Button>
        </div>

        <p className='text-sm break-words whitespace-pre-wrap'>{task.prompt}</p>

        {task.fail_reason && (
          <p className='text-destructive text-xs'>{task.fail_reason}</p>
        )}

        {isOfflineImageTaskActive(task) && (
          <p className='text-muted-foreground text-xs'>
            {t('Running on the server. You can close this page.')}
          </p>
        )}

        {task.image_count > 0 && (
          <div className='flex flex-wrap gap-2'>
            {Array.from({ length: task.image_count }, (_, index) => (
              <OfflineImageThumbnail
                key={index}
                taskId={task.id}
                index={index}
              />
            ))}
          </div>
        )}

        {task.has_reference && (
          <div className='flex flex-wrap gap-2'>
            <OfflineImageThumbnail taskId={task.id} />
          </div>
        )}
      </CardContent>
    </Card>
  )
}
