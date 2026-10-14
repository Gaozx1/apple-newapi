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
import { Globe } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { OverseasDeclarationDialog } from '@/components/overseas-declaration-dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import { useStatus } from '@/hooks/use-status'
import { toIntlLocale } from '@/i18n/languages'
import { overseasGroupNames, readGroupRegions } from '@/lib/group-regions'

import type { UserProfile } from '../types'

// ============================================================================
// Overseas declaration card
//
// The declaration unlocks the groups that serve overseas models. The card stays
// hidden while the site has no such group, so operators that do not split their
// groups by region never see it.
// ============================================================================

interface OverseasDeclarationCardProps {
  profile: UserProfile | null
  onProfileUpdate: () => void
}

export function OverseasDeclarationCard(props: OverseasDeclarationCardProps) {
  const { t, i18n } = useTranslation()
  const { status } = useStatus()
  const [dialogOpen, setDialogOpen] = useState(false)

  const overseasGroups = overseasGroupNames(readGroupRegions(status))
  if (overseasGroups.length === 0) return null

  const confirmed = props.profile?.overseas_confirmed === true
  const listFormat = new Intl.ListFormat(
    toIntlLocale(i18n.resolvedLanguage || i18n.language),
    { style: 'short', type: 'conjunction' }
  )
  const groupList = listFormat.format(overseasGroups)

  return (
    <TitledCard
      title={t('Overseas models')}
      description={t('Declaration required for overseas model groups')}
      icon={<Globe className='h-4 w-4' />}
      iconTone='warning'
      disableHoverEffect
    >
      <div className='space-y-3'>
        <StatusBadge
          label={
            confirmed ? t('Declaration recorded') : t('Declaration required')
          }
          variant={confirmed ? 'success' : 'warning'}
          copyable={false}
        />
        <p className='text-muted-foreground text-sm'>
          {confirmed
            ? t(
                'You declared that you are a foreign user, so these groups are available: {{groups}}',
                { groups: groupList }
              )
            : t(
                'These groups serve overseas models and require you to declare that you are a foreign user: {{groups}}',
                { groups: groupList }
              )}
        </p>
        {!confirmed && (
          <Button type='button' size='sm' onClick={() => setDialogOpen(true)}>
            {t('Declare and unlock')}
          </Button>
        )}
      </div>
      <OverseasDeclarationDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onConfirmed={props.onProfileUpdate}
      />
    </TitledCard>
  )
}
