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

import { SectionPageLayout } from '@/components/layout'

import { OfflineImageCreateForm } from './components/offline-image-create-form'
import { OfflineImageTaskList } from './components/offline-image-task-list'

/**
 * Offline image generation: queue a task here, close the page, and the server
 * keeps generating. Results and failures stay listed below.
 */
export function OfflineImage() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Offline Image')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-6'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Choose a group and a model, then the task keeps running on the server even if you close the browser.'
            )}
          </p>

          <OfflineImageCreateForm />
          <OfflineImageTaskList />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
