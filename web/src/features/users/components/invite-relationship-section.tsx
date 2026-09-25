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
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SideDrawerSection } from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import { bindInviter, previewInviterRebate } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import type { InviterRebatePreview } from '../types'

type InviteRelationshipSectionProps = {
  userId: number
  username: string
  currentInviterId: number
  onBound: () => void
}

/**
 * Admin back-fill of a user's inviter. The rebate for the invitee's historical
 * recharges is previewed first, because crediting it changes the inviter's
 * rebate wallet and must be confirmed deliberately.
 */
export function InviteRelationshipSection(
  props: InviteRelationshipSectionProps
) {
  const { t } = useTranslation()
  const [inviter, setInviter] = useState('')
  const [preview, setPreview] = useState<InviterRebatePreview | null>(null)
  const [applyRebate, setApplyRebate] = useState(true)
  const [previewing, setPreviewing] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  // A preview belongs to the inviter it was computed for; typing another one
  // must not leave a stale amount confirmable.
  const handleInviterChange = (value: string) => {
    setInviter(value)
    setPreview(null)
  }

  const handlePreview = async () => {
    if (!inviter.trim()) return
    setPreviewing(true)
    try {
      const result = requireServerSuccess(
        await previewInviterRebate(props.userId, inviter.trim())
      )
      setPreview(result.data ?? null)
    } catch (error) {
      setPreview(null)
      handleServerError(error, t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setPreviewing(false)
    }
  }

  const handleBind = async () => {
    setSubmitting(true)
    try {
      const result = requireServerSuccess(
        await bindInviter(props.userId, inviter.trim(), applyRebate)
      )
      const credited = result.data?.rebate_quota ?? 0
      toast.success(
        credited > 0
          ? t(SUCCESS_MESSAGES.INVITER_BOUND_WITH_REBATE, {
              amount: formatQuota(credited),
            })
          : t(SUCCESS_MESSAGES.INVITER_BOUND)
      )
      setConfirmOpen(false)
      setInviter('')
      setPreview(null)
      props.onBound()
    } catch (error) {
      handleServerError(error, t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setSubmitting(false)
    }
  }

  const previewRows = preview
    ? [
        { key: 'topup', label: t('Recharge orders'), value: preview.topup_quota },
        ...(preview.redemption_quota > 0
          ? [
              {
                key: 'redemption',
                label: t('Redemption codes'),
                value: preview.redemption_quota,
              },
            ]
          : []),
        { key: 'base', label: t('Recharge base'), value: preview.base_quota },
        { key: 'rebate', label: t('Rebate to credit'), value: preview.rebate_quota },
      ]
    : []

  return (
    <>
      <SideDrawerSection>
        <h3 className='text-sm font-medium'>{t('Invite relationship')}</h3>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Bind this user to an inviter. Future recharges then credit the inviter the configured rebate.'
          )}
        </p>

        <div>
          <Label className='text-muted-foreground text-xs'>
            {t('Current inviter')}
          </Label>
          <Input
            value={
              props.currentInviterId > 0 ? `ID: ${props.currentInviterId}` : '-'
            }
            disabled
            className='mt-1'
          />
        </div>

        <div>
          <Label htmlFor='inviter-identifier'>{t('Inviter')}</Label>
          <div className='mt-1 flex gap-2'>
            <Input
              id='inviter-identifier'
              value={inviter}
              onChange={(event) => handleInviterChange(event.target.value)}
              placeholder={t('Username, user ID or invite code')}
            />
            <Button
              type='button'
              variant='outline'
              disabled={!inviter.trim() || previewing}
              onClick={() => void handlePreview()}
            >
              {previewing && <Loader2 className='mr-1 h-4 w-4 animate-spin' />}
              {t('Preview rebate')}
            </Button>
          </div>
        </div>

        {preview && (
          <div className='space-y-2'>
            <div className='bg-muted/40 space-y-1 rounded-md border p-3 text-sm'>
              {previewRows.map((row) => (
                <div key={row.key} className='flex justify-between gap-4'>
                  <span className='text-muted-foreground'>{row.label}</span>
                  <span className='font-mono'>{formatQuota(row.value)}</span>
                </div>
              ))}
              <div className='flex justify-between gap-4'>
                <span className='text-muted-foreground'>
                  {t('Rebate rate')}
                </span>
                <span className='font-mono'>{preview.rebate_rate}%</span>
              </div>
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('Inviter: {{inviter}}', {
                inviter: `${preview.inviter_username} (ID: ${preview.inviter_id})`,
              })}
            </p>
          </div>
        )}

        {preview && (
          <label className='flex items-start gap-3'>
            <Checkbox
              checked={applyRebate}
              onCheckedChange={(checked) => setApplyRebate(checked === true)}
            />
            <span className='text-sm'>
              {t('Also credit the historical rebate to the inviter')}
            </span>
          </label>
        )}

        <Button
          type='button'
          disabled={!preview || submitting}
          onClick={() => setConfirmOpen(true)}
        >
          {t('Bind inviter')}
        </Button>
      </SideDrawerSection>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm inviter binding')}
        desc={t(
          'Bind {{username}} to inviter {{inviter}}?',
          {
            username: props.username,
            inviter: preview
              ? `${preview.inviter_username} (ID: ${preview.inviter_id})`
              : '',
          }
        )}
        confirmText={t('Confirm')}
        handleConfirm={() => void handleBind()}
        isLoading={submitting}
      >
        {applyRebate && preview && preview.rebate_quota > 0 && (
          <p className='text-sm'>
            {t('Rebate to credit: {{amount}}', {
              amount: formatQuota(preview.rebate_quota),
            })}
          </p>
        )}
      </ConfirmDialog>
    </>
  )
}
