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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'

import { useTrainingConsent } from '../hooks/use-training-consent'

const visitorChoiceKey = 'new-api:training-consent:v1'

export function TrainingConsentGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { consent, choose, signedIn } = useTrainingConsent()
  const [visitorDecided, setVisitorDecided] = useState(() => {
    try {
      const choice = localStorage.getItem(visitorChoiceKey)
      return choice === 'accepted' || choice === 'declined'
    } catch {
      return false
    }
  })

  const decided = signedIn
    ? consent.data?.decision === 'accepted' ||
      consent.data?.decision === 'declined'
    : visitorDecided
  if (decided) return children

  function save(decision: 'accepted' | 'declined') {
    if (signedIn) {
      choose.mutate(decision)
      return
    }
    try {
      localStorage.setItem(visitorChoiceKey, decision)
    } catch {
      // The choice still applies for this visit if storage is unavailable.
    }
    setVisitorDecided(true)
  }

  const loading = signedIn && !consent.data && !consent.isError
  const loadFailed = signedIn && !consent.data && consent.isError

  return (
    <AlertDialog open>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Training data consent')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'If you agree, text from new Playground chats will be stored for 24 hours for model training. You can change your choice in your profile.'
            )}
          </AlertDialogDescription>
          <p className='text-muted-foreground text-sm'>
            {t('You can continue using the site with either choice.')}
          </p>
        </AlertDialogHeader>
        {loading && <p role='status'>{t('Loading...')}</p>}
        {loadFailed && (
          <p role='alert'>{t('Could not load your training data choice.')}</p>
        )}
        {choose.isError && <p role='alert'>{t('Failed to update settings')}</p>}
        <AlertDialogFooter>
          {loadFailed ? (
            <Button
              onClick={() => void consent.refetch()}
              disabled={consent.isFetching}
            >
              {t('Retry')}
            </Button>
          ) : (
            <>
              <Button
                variant='outline'
                disabled={loading || choose.isPending}
                onClick={() => save('declined')}
              >
                {t('Decline training use')}
              </Button>
              <Button
                variant='outline'
                disabled={loading || choose.isPending}
                onClick={() => save('accepted')}
              >
                {t('Allow training use')}
              </Button>
            </>
          )}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export function TrainingConsentCard() {
  const { t } = useTranslation()
  const { consent, choose } = useTrainingConsent()
  const accepted = consent.data?.decision === 'accepted'

  return (
    <TitledCard
      title={t('Training data consent')}
      description={t(
        'If you agree, text from new Playground chats will be stored for 24 hours for model training. You can change your choice in your profile.'
      )}
      disableHoverEffect
    >
      {consent.data && (
        <Button
          variant='outline'
          disabled={choose.isPending}
          onClick={() => choose.mutate(accepted ? 'declined' : 'accepted')}
        >
          {accepted ? t('Withdraw training consent') : t('Allow training use')}
        </Button>
      )}
      {choose.isError && <p role='alert'>{t('Failed to update settings')}</p>}
    </TitledCard>
  )
}
