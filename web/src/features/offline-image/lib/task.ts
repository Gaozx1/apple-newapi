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
import type { OfflineImageTask } from '../types'

export type OfflineImageStatusVariant =
  | 'default'
  | 'secondary'
  | 'warning'
  | 'destructive'

/**
 * Statuses whose outcome the server has not decided yet. The list keeps
 * refreshing while any task is in one of them.
 */
const activeStatuses = new Set(['PENDING', 'QUEUED', 'RUNNING'])

// Non-terminal tasks are the ones the page must keep polling for.
export function isOfflineImageTaskActive(
  task: Pick<OfflineImageTask, 'status'>
): boolean {
  return activeStatuses.has(task.status)
}

// English source strings double as the i18n keys the badge renders through t().
export function offlineImageStatusLabel(status: string): string {
  switch (status) {
    case 'SUCCESS':
      return 'Completed'
    case 'FAILURE':
      return 'Failed'
    case 'RUNNING':
      return 'Generating'
    default:
      return 'Queued'
  }
}

export function offlineImageStatusVariant(
  status: string
): OfflineImageStatusVariant {
  switch (status) {
    case 'SUCCESS':
      return 'default'
    case 'FAILURE':
      return 'destructive'
    case 'RUNNING':
      return 'warning'
    default:
      return 'secondary'
  }
}
