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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { RiskAcknowledgementDialog } from '@/components/risk-acknowledgement-dialog'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

// ============================================================================
// Overseas model declaration
//
// Groups classified as serving overseas models (settings → user levels and
// group regions) are only usable after the user declares themselves a foreign
// user. The declaration is recorded server-side (POST /api/user/overseas-confirm)
// and the gate that enforces it lives in the relay middleware, so this dialog is
// the account-facing half of the flow.
// ============================================================================

type OverseasDeclarationResponse = {
  success?: boolean
  message?: string
  data?: {
    overseas_confirmed?: boolean
    overseas_confirmed_at?: number
  }
}

type OverseasDeclarationDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called after the declaration was recorded. */
  onConfirmed: () => void
  /** Group that triggered the prompt, shown in the description. */
  group?: string
}

export function OverseasDeclarationDialog(
  props: OverseasDeclarationDialogProps
) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const setUser = useAuthStore((state) => state.auth.setUser)
  const user = useAuthStore((state) => state.auth.user)

  const handleConfirm = async () => {
    setSubmitting(true)
    try {
      const res = await api.post('/api/user/overseas-confirm')
      const payload = res.data as OverseasDeclarationResponse
      if (!payload?.success) {
        handleServerError(payload, t('Failed to record the declaration'))
        return
      }
      if (user) {
        setUser({
          ...user,
          overseas_confirmed: true,
          overseas_confirmed_at: payload.data?.overseas_confirmed_at,
        })
      }
      toast.success(t('Declaration recorded'))
      props.onOpenChange(false)
      props.onConfirmed()
    } catch (error) {
      handleServerError(error, t('Failed to record the declaration'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <RiskAcknowledgementDialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      destructive={false}
      title={t('Overseas model declaration')}
      description={
        props.group
          ? t(
              'Group {{group}} serves overseas models. Declare that you are a foreign user to use it.',
              { group: props.group }
            )
          : t(
              'Groups that serve overseas models require you to declare that you are a foreign user.'
            )
      }
      checklist={[
        t('I confirm that I am a foreign user, not a mainland China user.'),
        t(
          'I understand that using overseas models is subject to the laws and policies of the model provider region, and I take responsibility for that use.'
        ),
      ]}
      confirmText={t('Confirm and continue')}
      isLoading={submitting}
      onConfirm={handleConfirm}
    />
  )
}
