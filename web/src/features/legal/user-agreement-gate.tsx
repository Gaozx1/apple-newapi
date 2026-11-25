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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SignOutDialog } from '@/components/sign-out-dialog'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'
import {
  AuthOperationError,
  authRequestOptions,
  authResult,
} from '@/lib/secure-verification'
import { useAuthStore } from '@/stores/auth-store'

type AgreementStatus = { revision: string; required: boolean }
const endpoint = '/api/user/self/user-agreement'

export function UserAgreementGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const sessionId = useAuthStore((state) => state.auth.session?.sid)
  const queryClient = useQueryClient()
  const [signOutOpen, setSignOutOpen] = useState(false)
  const queryKey = ['user-agreement-status', sessionId] as const
  const status = useQuery({
    queryKey,
    enabled: !!sessionId,
    refetchInterval: 60_000,
    queryFn: ({ signal }) =>
      authResult<AgreementStatus>(
        api.get(endpoint, {
          ...authRequestOptions,
          disableDuplicate: true,
          signal,
        })
      ),
  })
  const accept = useMutation({
    mutationFn: (revision: string) =>
      authResult<AgreementStatus>(
        api.put(endpoint, { revision }, authRequestOptions)
      ),
    onMutate: () => queryClient.cancelQueries({ queryKey }),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKey, data)
      void queryClient.invalidateQueries({ queryKey })
    },
    onError: () => void queryClient.invalidateQueries({ queryKey }),
  })

  if (!sessionId || status.data?.required === false) return children
  const revision = status.data?.revision

  return (
    <>
      <AlertDialog open={!signOutOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('User Agreement')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Read and accept the current User Agreement to continue.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {revision ? (
            <Button
              variant='link'
              render={
                <a
                  href='/user-agreement'
                  target='_blank'
                  rel='noopener noreferrer'
                />
              }
            >
              {t('View document')}
            </Button>
          ) : status.isError ? (
            <p role='alert'>{t('Could not load the User Agreement.')}</p>
          ) : (
            <p role='status'>{t('Loading...')}</p>
          )}
          {accept.isError && (
            <p role='alert'>
              {accept.error instanceof AuthOperationError &&
              accept.error.code === 'USER_AGREEMENT_UPDATED'
                ? t('The user agreement changed. Review the latest version.')
                : t('Failed to update settings')}
            </p>
          )}
          <AlertDialogFooter>
            <Button variant='outline' onClick={() => setSignOutOpen(true)}>
              {t('Sign out')}
            </Button>
            {revision ? (
              <Button
                disabled={accept.isPending}
                onClick={() => accept.mutate(revision)}
              >
                {t('Accept and continue')}
              </Button>
            ) : status.isError ? (
              <Button
                disabled={status.isFetching}
                onClick={() => void status.refetch()}
              >
                {t('Retry')}
              </Button>
            ) : null}
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <SignOutDialog open={signOutOpen} onOpenChange={setSignOutOpen} />
    </>
  )
}
