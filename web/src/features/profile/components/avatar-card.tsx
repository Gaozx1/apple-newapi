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
import { ImageIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { TitledCard } from '@/components/ui/titled-card'

import { updateUserProfile } from '../api'
import type { UserProfile } from '../types'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'

// ============================================================================
// Avatar Card Component
// ============================================================================

interface AvatarCardProps {
  profile: UserProfile | null
  onProfileUpdate: () => void
}

export function AvatarCard({ profile, onProfileUpdate }: AvatarCardProps) {
  const { t } = useTranslation()
  const [avatarUrl, setAvatarUrl] = useState(profile?.avatar ?? '')
  const [saving, setSaving] = useState(false)

  const displayName = profile?.display_name || profile?.username || t('User')
  const fallback = getUserAvatarFallback(displayName)
  const fallbackStyle = getUserAvatarStyle(displayName)
  const currentAvatar = profile?.avatar || ''

  const handleSave = async () => {
    setSaving(true)
    try {
      const res = await updateUserProfile({ avatar: avatarUrl.trim() })
      if (res.success) {
        toast.success(t('Avatar updated'))
        onProfileUpdate()
      } else {
        toast.error(res.message || t('Failed to update avatar'))
      }
    } catch {
      toast.error(t('Failed to update avatar'))
    } finally {
      setSaving(false)
    }
  }

  const handleReset = () => {
    setAvatarUrl(currentAvatar)
  }

  const hasChanges = avatarUrl.trim() !== currentAvatar

  return (
    <TitledCard
      title={t('Avatar')}
      description={t('Set your profile picture via URL')}
      icon={<ImageIcon className='h-4 w-4' />}
      iconTone='info'
      disableHoverEffect
    >
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center'>
        <Avatar className='h-16 w-16 shrink-0 rounded-2xl'>
          {currentAvatar ? (
            <AvatarImage src={currentAvatar} alt={displayName} />
          ) : null}
          <AvatarFallback
            className='rounded-2xl font-semibold text-white'
            style={fallbackStyle}
          >
            {fallback}
          </AvatarFallback>
        </Avatar>

        <div className='flex-1 space-y-2'>
          <Input
            type='url'
            placeholder={t('Avatar URL (optional)')}
            value={avatarUrl}
            onChange={(e) => setAvatarUrl(e.target.value)}
            disabled={saving}
          />
          <p className='text-muted-foreground text-xs'>
            {t('GitHub-bound users get their avatar automatically')}
          </p>
        </div>
      </div>

      <div className='mt-4 flex justify-end gap-2'>
        <Button
          variant='ghost'
          size='sm'
          onClick={handleReset}
          disabled={saving || !hasChanges}
        >
          {t('Cancel')}
        </Button>
        <Button
          size='sm'
          onClick={handleSave}
          disabled={saving || !hasChanges}
        >
          {saving ? t('Saving...') : t('Save Avatar')}
        </Button>
      </div>
    </TitledCard>
  )
}
