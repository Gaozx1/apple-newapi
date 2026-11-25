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
import type { Row } from '@tanstack/react-table'
import {
  Pencil,
  Trash2,
  Power,
  PowerOff,
  ArrowUp,
  ArrowDown,
  KeyRound,
  ShieldAlert,
  Link2,
  CreditCard,
  Gift,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { AdminUserManageAction } from '@/features/auth/secure-verification'
import { UserSubscriptionsDialog } from '@/features/subscriptions/components/dialogs/user-subscriptions-dialog'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'

import { manageUser, resetUserPasskey, resetUserTwoFA } from '../api'
import {
  USER_STATUS,
  USER_ROLE,
  ERROR_MESSAGES,
  isUserDeleted,
} from '../constants'
import { getUserActionMessage } from '../lib'
import type { User } from '../types'
import { GrantLotteryChancesDialog } from './dialogs/grant-lottery-chances-dialog'
import { UserBindingDialog } from './dialogs/user-binding-dialog'
import { useUsers } from './users-provider'

const MANAGE_ACTION_TITLES: Record<AdminUserManageAction, string> = {
  disable: 'Verify to disable user',
  enable: 'Verify to enable user',
  promote: 'Verify to promote user',
  demote: 'Verify to demote user',
}

interface DataTableRowActionsProps {
  row: Row<User>
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation()
  const user = row.original
  const {
    setOpen,
    setCurrentRow,
    triggerRefresh,
    requestVerification,
    verificationActive,
  } = useUsers()
  const [disableOpen, setDisableOpen] = useState(false)
  const [banReason, setBanReason] = useState('')
  const [managePending, setManagePending] = useState(false)
  const [resetPasskeyOpen, setResetPasskeyOpen] = useState(false)
  const [resetTwoFAOpen, setResetTwoFAOpen] = useState(false)
  const [bindingDialogOpen, setBindingDialogOpen] = useState(false)
  const [subscriptionsDialogOpen, setSubscriptionsDialogOpen] = useState(false)
  const [lotteryGrantOpen, setLotteryGrantOpen] = useState(false)

  const handleEdit = () => {
    setCurrentRow(user)
    setOpen('update')
  }

  const handleDelete = () => {
    setCurrentRow(user)
    setOpen('delete')
  }

  const trimmedBanReason = banReason.trim()
  const validBanReason =
    /[^\p{White_Space}\p{Cc}\p{Cf}]/u.test(trimmedBanReason) &&
    !trimmedBanReason.includes('\0') &&
    [...trimmedBanReason].length <= 200

  const handleManage = async (action: AdminUserManageAction) => {
    if (managePending || (action === 'disable' && !validBanReason)) return
    setManagePending(true)
    try {
      const proof = await requestVerification({
        scope: 'admin.user.manage',
        context: { user_id: user.id, action },
        title: t(MANAGE_ACTION_TITLES[action]),
        description: t(
          'Confirm your identity before changing the account {{username}}.',
          { username: user.username }
        ),
      })
      if (!proof) return
      const result = await manageUser(
        user.id,
        action,
        proof.proof_token,
        action === 'disable' ? trimmedBanReason : undefined
      )
      if (result.success) {
        setDisableOpen(false)
        setBanReason('')
        toast.success(t(getUserActionMessage(action)))
        triggerRefresh()
      } else {
        handleServerError(result, t('Failed to {{action}} user', { action }))
      }
    } catch (error) {
      handleServerError(
        AuthOperationError.from(error),
        t(ERROR_MESSAGES.UNEXPECTED)
      )
    } finally {
      setManagePending(false)
    }
  }

  const handleResetPasskey = async () => {
    try {
      const proof = await requestVerification({
        scope: 'admin.user.passkey.reset',
        context: { user_id: user.id },
        title: t('Verify to reset Passkey'),
        description: t(
          'Confirm your identity before changing the account {{username}}.',
          { username: user.username }
        ),
      })
      if (!proof) return
      const result = await resetUserPasskey(user.id, proof.proof_token)
      if (result.success) {
        toast.success(t('Passkey reset successfully'))
        triggerRefresh()
      } else {
        handleServerError(result, t('Failed to reset Passkey'))
      }
    } catch (error) {
      handleServerError(
        AuthOperationError.from(error),
        t(ERROR_MESSAGES.UNEXPECTED)
      )
    } finally {
      setResetPasskeyOpen(false)
    }
  }

  const handleResetTwoFA = async () => {
    try {
      const proof = await requestVerification({
        scope: 'admin.user.2fa.disable',
        context: { user_id: user.id },
        title: t('Verify to reset 2FA'),
        description: t(
          'Confirm your identity before changing the account {{username}}.',
          { username: user.username }
        ),
      })
      if (!proof) return
      const result = await resetUserTwoFA(user.id, proof.proof_token)
      if (result.success) {
        toast.success(t('Two-factor authentication reset'))
        triggerRefresh()
      } else {
        handleServerError(result, t('Failed to reset 2FA'))
      }
    } catch (error) {
      handleServerError(
        AuthOperationError.from(error),
        t(ERROR_MESSAGES.UNEXPECTED)
      )
    } finally {
      setResetTwoFAOpen(false)
    }
  }

  const isDisabled = user.status === USER_STATUS.DISABLED
  const isAdmin = user.role >= USER_ROLE.ADMIN
  const isRoot = user.role === USER_ROLE.ROOT

  if (isUserDeleted(user)) {
    return null
  }

  return (
    <div className='-ml-1.5 flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={handleEdit}
              aria-label={t('Edit')}
            />
          }
        >
          <Pencil />
        </TooltipTrigger>
        <TooltipContent>{t('Edit')}</TooltipContent>
      </Tooltip>

      <DataTableRowActionMenu
        ariaLabel={t('Open menu')}
        contentClassName='w-48'
      >
        {isDisabled ? (
          <DropdownMenuItem onClick={() => handleManage('enable')}>
            {t('Enable')}
            <DropdownMenuShortcut>
              <Power size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault()
              setDisableOpen(true)
            }}
            disabled={isRoot}
          >
            {t('Disable')}
            <DropdownMenuShortcut>
              <PowerOff size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        {isAdmin && !isRoot && (
          <DropdownMenuItem onClick={() => handleManage('demote')}>
            {t('Demote')}
            <DropdownMenuShortcut>
              <ArrowDown size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        {!isAdmin && (
          <DropdownMenuItem onClick={() => handleManage('promote')}>
            {t('Promote')}
            <DropdownMenuShortcut>
              <ArrowUp size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setBindingDialogOpen(true)
          }}
        >
          {t('Manage Bindings')}
          <DropdownMenuShortcut>
            <Link2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setSubscriptionsDialogOpen(true)
          }}
        >
          {t('Manage Subscriptions')}
          <DropdownMenuShortcut>
            <CreditCard size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setLotteryGrantOpen(true)
          }}
        >
          {t('Grant Lottery Chances')}
          <DropdownMenuShortcut>
            <Gift size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setResetPasskeyOpen(true)
          }}
          disabled={isRoot}
        >
          {t('Reset Passkey')}
          <DropdownMenuShortcut>
            <KeyRound size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            setResetTwoFAOpen(true)
          }}
          disabled={isRoot}
        >
          {t('Reset 2FA')}
          <DropdownMenuShortcut>
            <ShieldAlert size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          onClick={handleDelete}
          className='text-destructive focus:text-destructive'
          disabled={isRoot}
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DataTableRowActionMenu>

      <ConfirmDialog
        open={disableOpen && !verificationActive}
        onOpenChange={(open) => {
          setDisableOpen(open)
          if (!open) setBanReason('')
        }}
        title={t('Disable')}
        desc={t(
          'This reason will be shown on the public ban board. Do not include personal information or secrets.'
        )}
        confirmText={t('Continue')}
        handleConfirm={() => handleManage('disable')}
        disabled={!validBanReason}
        isLoading={managePending}
        destructive
      >
        <div className='grid gap-2'>
          <Label htmlFor={`ban-reason-${user.id}`}>
            {t('Public ban reason')}
          </Label>
          <Textarea
            id={`ban-reason-${user.id}`}
            value={banReason}
            onChange={(event) => setBanReason(event.target.value)}
            aria-describedby={`ban-reason-help-${user.id}`}
            required
          />
          <p
            id={`ban-reason-help-${user.id}`}
            className='text-muted-foreground text-xs'
          >
            {t('Enter a public ban reason of up to 200 characters')}
          </p>
        </div>
      </ConfirmDialog>

      <ConfirmDialog
        open={resetPasskeyOpen && !verificationActive}
        onOpenChange={setResetPasskeyOpen}
        title={t('Reset Passkey')}
        desc={t(
          'Reset Passkey for {{username}}? The user will need to register a new Passkey before using passwordless login.',
          { username: user.username }
        )}
        confirmText={t('Reset Passkey')}
        handleConfirm={handleResetPasskey}
      />

      <ConfirmDialog
        open={resetTwoFAOpen && !verificationActive}
        onOpenChange={setResetTwoFAOpen}
        title={t('Reset Two-Factor Authentication')}
        desc={t(
          'Reset 2FA for {{username}}? The user must set up 2FA again to continue using it.',
          { username: user.username }
        )}
        confirmText={t('Reset 2FA')}
        handleConfirm={handleResetTwoFA}
      />

      <UserBindingDialog
        open={bindingDialogOpen}
        onOpenChange={setBindingDialogOpen}
        userId={user.id}
        onUnbindSuccess={triggerRefresh}
      />

      <UserSubscriptionsDialog
        open={subscriptionsDialogOpen}
        onOpenChange={setSubscriptionsDialogOpen}
        user={{ id: user.id, username: user.username }}
        onSuccess={triggerRefresh}
      />

      <GrantLotteryChancesDialog
        open={lotteryGrantOpen}
        onOpenChange={setLotteryGrantOpen}
        userId={user.id}
        username={user.username}
        onSuccess={triggerRefresh}
      />
    </div>
  )
}
