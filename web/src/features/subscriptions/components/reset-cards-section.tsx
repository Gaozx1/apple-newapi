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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import type {
  ColumnDef,
  ColumnFiltersState,
  OnChangeFn,
  PaginationState,
} from '@tanstack/react-table'
import type { TFunction } from 'i18next'
import { Plus, PowerOff, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import {
  DataTablePage,
  DataTableRowActionMenu,
  useDataTable,
} from '@/components/data-table'
import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { searchUsers } from '@/features/users/api'
import { useDebounce } from '@/hooks'
import { formatTimestamp, formatTimestampToDate } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { createServerError } from '@/lib/server-error-message'

import {
  adminDeleteResetCard,
  adminDisableResetCard,
  adminGrantResetCards,
  getAdminPlans,
  getAdminResetCards,
  getUserSubscriptions,
} from '../api'
import type { ResetCard } from '../types'
import { useSubscriptions } from './subscriptions-provider'

// The list endpoint returns the owner username alongside the ResetCard fields.
type ResetCardRow = ResetCard & { username?: string }

type ResetCardStatus = 'unused' | 'used' | 'expired' | 'disabled'

const RESET_CARD_STATUS_CONFIG: Record<
  ResetCardStatus,
  { labelKey: string; variant: StatusVariant }
> = {
  unused: { labelKey: 'Unused', variant: 'success' },
  used: { labelKey: 'Used', variant: 'neutral' },
  expired: { labelKey: 'Expired', variant: 'neutral' },
  disabled: { labelKey: 'Disabled', variant: 'neutral' },
}

const RESET_CARD_SOURCE_LABELS: Record<string, string> = {
  admin: 'Admin',
  order: 'Order',
  balance: 'Balance',
  redemption: 'Redemption',
  lottery: 'Lottery',
}

const GRANT_RESET_CARDS_FORM_ID = 'grant-reset-cards-form'

function createGrantResetCardsSchema(t: TFunction) {
  return z
    .object({
      user_id: z.number().int().min(0),
      count: z
        .number()
        .int()
        .min(1, t('Count must be between 1 and 100'))
        .max(100, t('Count must be between 1 and 100')),
      never_expires: z.boolean(),
      expires_at: z.date().optional(),
      target_subscription_id: z.number().int().min(0),
      note: z
        .string()
        .max(255, t('Note must be 255 characters or fewer'))
        .optional(),
    })
    .superRefine((values, ctx) => {
      if (values.never_expires) return
      if (!values.expires_at) {
        ctx.addIssue({
          code: 'custom',
          path: ['expires_at'],
          message: t('Please select an expiry date'),
        })
        return
      }
      if (values.expires_at.getTime() <= Date.now()) {
        ctx.addIssue({
          code: 'custom',
          path: ['expires_at'],
          message: t('Expiry must be in the future'),
        })
      }
    })
}

type GrantResetCardsFormValues = z.infer<
  ReturnType<typeof createGrantResetCardsSchema>
>

const GRANT_RESET_CARDS_DEFAULTS: GrantResetCardsFormValues = {
  user_id: 0,
  count: 1,
  never_expires: true,
  expires_at: undefined,
  target_subscription_id: 0,
  note: '',
}

type ResetCardRowActionsProps = {
  card: ResetCardRow
  complianceConfirmed: boolean
  onDisable: (card: ResetCardRow) => void
  onDelete: (card: ResetCardRow) => void
}

function ResetCardRowActions(props: ResetCardRowActionsProps) {
  const { t } = useTranslation()
  const canDisable = props.card.status === 'unused'

  if (props.card.status === 'used') {
    return <span className='text-muted-foreground text-sm'>-</span>
  }

  return (
    <div className='-ml-1.5 flex items-center justify-end'>
      <DataTableRowActionMenu ariaLabel={t('Open menu')} modal={false}>
        {canDisable && (
          <DropdownMenuItem
            disabled={!props.complianceConfirmed}
            onClick={() => props.onDisable(props.card)}
          >
            {t('Disable')}
            <DropdownMenuShortcut>
              <PowerOff size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        {canDisable && <DropdownMenuSeparator />}
        <DropdownMenuItem
          variant='destructive'
          disabled={!props.complianceConfirmed}
          onClick={() => props.onDelete(props.card)}
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DataTableRowActionMenu>
    </div>
  )
}

function useResetCardsColumns(
  complianceConfirmed: boolean,
  onDisable: (card: ResetCardRow) => void,
  onDelete: (card: ResetCardRow) => void
): ColumnDef<ResetCardRow>[] {
  const { t } = useTranslation()

  return useMemo(
    (): ColumnDef<ResetCardRow>[] => [
      {
        accessorKey: 'id',
        header: t('ID'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <TableId value={row.original.id} className='w-[60px]' />
        ),
        size: 80,
      },
      {
        accessorKey: 'card_key',
        header: t('Card Key'),
        meta: { mobileTitle: true },
        cell: ({ row }) => (
          <div className='flex max-w-[280px] min-w-0 items-center gap-1'>
            <span
              className='truncate font-mono text-xs'
              title={row.original.card_key}
            >
              {row.original.card_key}
            </span>
            <CopyButton
              value={row.original.card_key}
              className='size-7'
              iconClassName='size-3.5'
              tooltip={t('Copy card key')}
              aria-label={t('Copy card key')}
            />
          </div>
        ),
        enableSorting: false,
        size: 280,
      },
      {
        accessorKey: 'user_id',
        header: t('Owner'),
        cell: ({ row }) => {
          const card = row.original
          if (card.user_id <= 0) {
            return <span className='text-muted-foreground text-sm'>-</span>
          }
          return (
            <div className='min-w-0'>
              <div className='truncate text-sm font-medium'>
                {card.username || `#${card.user_id}`}
              </div>
              <div className='text-muted-foreground text-xs'>
                #{card.user_id}
              </div>
            </div>
          )
        },
        size: 160,
      },
      {
        accessorKey: 'target_subscription_id',
        header: t('Resettable Subscription'),
        cell: ({ row }) => {
          const targetSubscriptionId = row.original.target_subscription_id ?? 0
          if (targetSubscriptionId <= 0) {
            return (
              <span className='text-muted-foreground text-sm'>
                {t('Any active subscription')}
              </span>
            )
          }
          return (
            <div className='min-w-0'>
              <div className='text-sm font-medium tabular-nums'>
                #{targetSubscriptionId}
              </div>
              {row.original.target_subscription_plan_title ? (
                <div className='text-muted-foreground truncate text-xs'>
                  {row.original.target_subscription_plan_title}
                </div>
              ) : null}
            </div>
          )
        },
        size: 170,
      },
      {
        accessorKey: 'status',
        header: t('Status'),
        meta: { mobileBadge: true },
        cell: ({ row }) => {
          const statusConfig =
            RESET_CARD_STATUS_CONFIG[row.original.status as ResetCardStatus]
          if (!statusConfig) {
            return (
              <StatusBadge
                label={row.original.status}
                variant='neutral'
                copyable={false}
                className='-ml-1.5'
              />
            )
          }
          return (
            <StatusBadge
              label={t(statusConfig.labelKey)}
              variant={statusConfig.variant}
              copyable={false}
              className='-ml-1.5'
            />
          )
        },
        size: 110,
      },
      {
        accessorKey: 'source',
        header: t('Source'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const source = row.original.source
          if (!source) {
            return <span className='text-muted-foreground text-sm'>-</span>
          }
          const labelKey = RESET_CARD_SOURCE_LABELS[source]
          return (
            <span className='text-sm'>{labelKey ? t(labelKey) : source}</span>
          )
        },
        size: 110,
      },
      {
        accessorKey: 'note',
        header: t('Note'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const note = row.original.note
          if (!note) {
            return <span className='text-muted-foreground text-sm'>-</span>
          }
          return (
            <span
              className='text-muted-foreground block max-w-[200px] truncate text-sm'
              title={note}
            >
              {note}
            </span>
          )
        },
        enableSorting: false,
        size: 200,
      },
      {
        accessorKey: 'expires_at',
        header: t('Expires At'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const expiresAt = row.original.expires_at
          if (!expiresAt) {
            return (
              <StatusBadge
                label={t('Never')}
                variant='neutral'
                copyable={false}
                className='-ml-1.5'
              />
            )
          }
          return (
            <div className='min-w-[160px] font-mono text-sm'>
              {formatTimestampToDate(expiresAt)}
            </div>
          )
        },
        size: 170,
      },
      {
        accessorKey: 'created_at',
        header: t('Created At'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <div className='min-w-[160px] font-mono text-sm'>
            {formatTimestampToDate(row.original.created_at)}
          </div>
        ),
        size: 170,
      },
      {
        id: 'actions',
        header: () => t('Actions'),
        cell: ({ row }) => (
          <ResetCardRowActions
            card={row.original}
            complianceConfirmed={complianceConfirmed}
            onDisable={onDisable}
            onDelete={onDelete}
          />
        ),
        meta: { pinned: 'right' as const },
      },
    ],
    [t, complianceConfirmed, onDisable, onDelete]
  )
}

type GrantResetCardsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onGranted: () => void
}

function GrantResetCardsDialog(props: GrantResetCardsDialogProps) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const submittingRef = useRef(false)
  const [pendingAllUsersGrant, setPendingAllUsersGrant] =
    useState<GrantResetCardsFormValues | null>(null)
  const schema = useMemo(() => createGrantResetCardsSchema(t), [t])
  const form = useForm<GrantResetCardsFormValues>({
    resolver: zodResolver(schema),
    defaultValues: GRANT_RESET_CARDS_DEFAULTS,
  })
  const neverExpires = form.watch('never_expires')
  const userId = form.watch('user_id')
  const targetSubscriptionId = form.watch('target_subscription_id')
  const isAllUsers = userId === 0
  const previousUserIdRef = useRef(0)
  const [userSearch, setUserSearch] = useState('')
  const [selectedUserLabel, setSelectedUserLabel] = useState('')
  const debouncedUserSearch = useDebounce(userSearch, 300)

  const { data: userSearchResults = [] } = useQuery({
    queryKey: ['admin-user-search', debouncedUserSearch],
    queryFn: async () => {
      const result = await searchUsers({
        keyword: debouncedUserSearch,
        page_size: 20,
      })
      return result.data?.items ?? []
    },
    enabled: props.open,
    staleTime: 30 * 1000,
  })

  const userPickerOptions = useMemo(() => {
    const options = userSearchResults.map((user) => ({
      value: String(user.id),
      label: `${user.username} (#${user.id})`,
      ...(user.display_name && user.display_name !== user.username
        ? { description: user.display_name }
        : {}),
    }))
    if (
      userId > 0 &&
      !options.some((option) => option.value === String(userId))
    ) {
      options.unshift({
        value: String(userId),
        label: selectedUserLabel || `#${userId}`,
      })
    }
    return options
  }, [userSearchResults, userId, selectedUserLabel])

  const handleUserValueChange = (value: string | null) => {
    const next = (value ?? '').trim()
    if (next === '') {
      setSelectedUserLabel('')
      setUserSearch('')
      form.setValue('user_id', 0, { shouldValidate: true })
      return
    }
    const matched = userPickerOptions.find((option) => option.value === next)
    if (matched) {
      setSelectedUserLabel(matched.label)
      setUserSearch('')
      form.setValue('user_id', Number(matched.value), { shouldValidate: true })
      return
    }
    if (/^\d+$/.test(next)) {
      setSelectedUserLabel(`#${next}`)
      setUserSearch('')
      form.setValue('user_id', Number(next), { shouldValidate: true })
      return
    }
    // Free-text search: drop any previous selection so a search term is never
    // submitted as a stale user id; the admin must pick an option or type an id.
    if (userId !== 0) {
      setSelectedUserLabel('')
      form.setValue('user_id', 0, { shouldValidate: true })
    }
    setUserSearch(next)
  }

  const { data: userSubscriptions = [], isFetching: subscriptionsFetching } =
    useQuery({
      queryKey: ['admin-user-subscriptions', userId],
      queryFn: async () => {
        const result = await getUserSubscriptions(userId)
        return result.data ?? []
      },
      enabled: userId > 0,
    })
  const { data: plans = [] } = useQuery({
    queryKey: ['admin-subscription-plans'],
    queryFn: async () => {
      const result = await getAdminPlans()
      return result.data ?? []
    },
    enabled: props.open,
    staleTime: 5 * 60 * 1000,
  })

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const record of plans) {
      if (record.plan.id > 0) {
        map.set(record.plan.id, record.plan.title || `#${record.plan.id}`)
      }
    }
    return map
  }, [plans])

  const subscriptionOptions = useMemo(() => {
    const options = [
      { value: '0', label: t('Any active subscription') },
      ...userSubscriptions.map((record) => {
        const subscription = record.subscription
        const planTitle =
          planTitleMap.get(subscription.plan_id) || `#${subscription.plan_id}`
        return {
          value: String(subscription.id),
          label: `#${subscription.id} · ${planTitle} · ${formatTimestamp(subscription.start_time)} → ${formatTimestamp(subscription.end_time)}`,
        }
      }),
    ]
    const selectedId = targetSubscriptionId
    if (
      selectedId > 0 &&
      !options.some((option) => option.value === String(selectedId))
    ) {
      options.push({ value: String(selectedId), label: `#${selectedId}` })
    }
    return options
  }, [userSubscriptions, planTitleMap, targetSubscriptionId, t])

  useEffect(() => {
    if (previousUserIdRef.current === userId) return
    previousUserIdRef.current = userId
    form.setValue('target_subscription_id', 0)
  }, [userId, form])

  useEffect(() => {
    if (props.open) {
      previousUserIdRef.current = 0
      submittingRef.current = false
      setPendingAllUsersGrant(null)
      setUserSearch('')
      setSelectedUserLabel('')
      form.reset(GRANT_RESET_CARDS_DEFAULTS)
    }
  }, [props.open, form])

  const grantResetCards = async (
    values: GrantResetCardsFormValues
  ): Promise<boolean> => {
    // The ref blocks a second call fired before React re-renders the disabled
    // button, so one confirmation can never create two grants.
    if (submittingRef.current) return false
    submittingRef.current = true
    setSubmitting(true)
    try {
      const allUsers = values.user_id === 0
      const result = await adminGrantResetCards({
        user_id: values.user_id,
        all_users: allUsers,
        count: values.count,
        expires_at: values.never_expires
          ? 0
          : Math.floor((values.expires_at?.getTime() ?? 0) / 1000),
        target_subscription_id: values.target_subscription_id,
        note: values.note?.trim() || undefined,
      })
      if (result.success) {
        if (allUsers) {
          toast.success(
            t('Granted {{count}} reset cards to {{users}} users', {
              count: result.data?.total_cards ?? 0,
              users: result.data?.granted_users ?? 0,
            })
          )
        } else {
          const grantedCount = result.data?.card_keys?.length ?? values.count
          toast.success(
            t('Granted {{count}} reset cards', { count: grantedCount })
          )
        }
        props.onGranted()
        props.onOpenChange(false)
        return true
      }
      handleServerError(result, t('Operation failed'))
      return false
    } catch (error) {
      handleServerError(error, t('Operation failed'))
      return false
    } finally {
      submittingRef.current = false
      setSubmitting(false)
    }
  }

  const handleSubmit = async (values: GrantResetCardsFormValues) => {
    // Granting to every active user requires explicit confirmation because it
    // cannot be undone. Keep the validated values until the admin confirms.
    if (values.user_id === 0) {
      setPendingAllUsersGrant(values)
      return
    }
    await grantResetCards(values)
  }

  const handleConfirmAllUsers = async () => {
    if (!pendingAllUsersGrant || submitting) return
    const granted = await grantResetCards(pendingAllUsersGrant)
    // Keep the confirmation open on failure so the admin can retry or cancel.
    if (granted) setPendingAllUsersGrant(null)
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t('Grant Reset Cards')}
        description={t(
          'Grant reset cards to a user. Each card resets the used quota of one active subscription.'
        )}
        contentClassName='sm:max-w-lg'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              disabled={submitting}
              onClick={() => props.onOpenChange(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              form={GRANT_RESET_CARDS_FORM_ID}
              disabled={submitting}
            >
              {submitting ? t('Processing...') : t('Grant')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={GRANT_RESET_CARDS_FORM_ID}
            className='space-y-4'
            onSubmit={form.handleSubmit(handleSubmit)}
          >
            <FormField
              control={form.control}
              name='user_id'
              render={() => (
                <FormItem>
                  <FormLabel>{t('User ID')}</FormLabel>
                  <FormControl>
                    <Combobox
                      options={userPickerOptions}
                      value={userId > 0 ? String(userId) : ''}
                      onValueChange={handleUserValueChange}
                      allowCustomValue
                      placeholder={t('Search users by name or ID')}
                      searchPlaceholder={t('Search users by name or ID')}
                      emptyText={t('No results found')}
                      className='w-full'
                      disabled={submitting}
                      aria-label={t('User ID')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave blank to grant to all users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='target_subscription_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Resettable Subscription')}</FormLabel>
                  <FormControl>
                    <Combobox
                      options={subscriptionOptions}
                      value={String(field.value ?? 0)}
                      onValueChange={(value) => {
                        if (value === null || value.trim() === '') {
                          field.onChange(0)
                          return
                        }
                        const numericValue = Number(value)
                        if (
                          Number.isInteger(numericValue) &&
                          numericValue >= 0
                        ) {
                          field.onChange(numericValue)
                        }
                      }}
                      allowCustomValue
                      placeholder={
                        subscriptionsFetching
                          ? t('Loading...')
                          : t('Any active subscription')
                      }
                      emptyText={t('No active subscriptions available.')}
                      searchPlaceholder={t('Search or type a subscription ID')}
                      className='w-full'
                      disabled={submitting}
                    />
                  </FormControl>
                  {isAllUsers && targetSubscriptionId > 0 ? (
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Only the owner of subscription #{{id}} can use these cards.',
                        { id: targetSubscriptionId }
                      )}
                    </p>
                  ) : null}
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='count'
              render={({ field }) => (
                <FormItem>
                  <FormLabel required>
                    {isAllUsers ? t('Cards per user') : t('Count')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='number'
                      min='1'
                      max='100'
                      step='1'
                      value={field.value || ''}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='never_expires'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between gap-3 rounded-md border px-3 py-2'>
                  <FormLabel className='text-sm font-normal'>
                    {t('Never expires')}
                  </FormLabel>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={(checked) =>
                        field.onChange(Boolean(checked))
                      }
                      aria-label={t('Never expires')}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            {!neverExpires && (
              <FormField
                control={form.control}
                name='expires_at'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiry')}</FormLabel>
                    <FormControl>
                      <DateTimePicker
                        value={field.value}
                        onChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <FormField
              control={form.control}
              name='note'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Note (optional)')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      value={field.value ?? ''}
                      maxLength={255}
                      placeholder={t('Optional')}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
      </Dialog>

      <ConfirmDialog
        open={pendingAllUsersGrant !== null}
        onOpenChange={(open) => {
          if (!open) setPendingAllUsersGrant(null)
        }}
        title={t('Grant Reset Cards')}
        desc={t(
          'Grant {{count}} reset cards to all active users? This cannot be undone.',
          { count: pendingAllUsersGrant?.count ?? 0 }
        )}
        confirmText={t('Grant to all users')}
        handleConfirm={handleConfirmAllUsers}
        isLoading={submitting}
      />
    </>
  )
}

type ResetCardConfirmAction = {
  type: 'disable' | 'delete'
  card: ResetCardRow
}

export function ResetCardsSection() {
  const { t } = useTranslation()
  const { complianceConfirmed } = useSubscriptions()
  const queryClient = useQueryClient()
  const [globalFilter, setGlobalFilter] = useState('')
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const [grantOpen, setGrantOpen] = useState(false)
  const [confirmAction, setConfirmAction] =
    useState<ResetCardConfirmAction | null>(null)
  const [confirmLoading, setConfirmLoading] = useState(false)

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const statusFilterValue = statusFilter[0] ?? ''

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'admin-reset-cards',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      statusFilterValue,
    ],
    queryFn: async () => {
      const result = await getAdminResetCards({
        keyword: globalFilter.trim() || undefined,
        status: statusFilterValue || undefined,
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
      if (!result.success) {
        throw createServerError(result, t('Failed to load reset cards'))
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const cards = useMemo(() => data?.items ?? [], [data])

  const refreshList = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['admin-reset-cards'] })
  }, [queryClient])

  const handleDisable = useCallback((card: ResetCardRow) => {
    setConfirmAction({ type: 'disable', card })
  }, [])
  const handleDelete = useCallback((card: ResetCardRow) => {
    setConfirmAction({ type: 'delete', card })
  }, [])
  const columns = useResetCardsColumns(
    complianceConfirmed,
    handleDisable,
    handleDelete
  )

  const handleGlobalFilterChange: OnChangeFn<string> = useCallback(
    (updater) => {
      setGlobalFilter((previous) =>
        typeof updater === 'function' ? updater(previous) : updater
      )
      setPagination((previous) =>
        previous.pageIndex === 0 ? previous : { ...previous, pageIndex: 0 }
      )
    },
    []
  )
  const handleColumnFiltersChange: OnChangeFn<ColumnFiltersState> = useCallback(
    (updater) => {
      setColumnFilters((previous) =>
        typeof updater === 'function' ? updater(previous) : updater
      )
      setPagination((previous) =>
        previous.pageIndex === 0 ? previous : { ...previous, pageIndex: 0 }
      )
    },
    []
  )
  const ensurePageInRange = useCallback((pageCount: number) => {
    setPagination((previous) => {
      if (pageCount > 0 && previous.pageIndex >= pageCount) {
        return { ...previous, pageIndex: pageCount - 1 }
      }
      return previous
    })
  }, [])

  const { table } = useDataTable({
    data: cards,
    columns,
    columnFilters,
    globalFilter,
    pagination,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    onPaginationChange: setPagination,
    onGlobalFilterChange: handleGlobalFilterChange,
    onColumnFiltersChange: handleColumnFiltersChange,
    ensurePageInRange,
  })

  const statusOptions = useMemo(
    () => [
      { label: t('Unused'), value: 'unused' },
      { label: t('Used'), value: 'used' },
      { label: t('Expired'), value: 'expired' },
      { label: t('Disabled'), value: 'disabled' },
    ],
    [t]
  )

  const handleConfirm = async () => {
    if (!confirmAction) return
    setConfirmLoading(true)
    try {
      const result =
        confirmAction.type === 'disable'
          ? await adminDisableResetCard(confirmAction.card.id)
          : await adminDeleteResetCard(confirmAction.card.id)
      if (result.success) {
        toast.success(
          confirmAction.type === 'disable'
            ? t('Reset card disabled')
            : t('Reset card deleted')
        )
        refreshList()
        setConfirmAction(null)
      } else {
        handleServerError(result, t('Operation failed'))
      }
    } catch (error) {
      handleServerError(error, t('Operation failed'))
    } finally {
      setConfirmLoading(false)
    }
  }

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No reset cards found')}
        emptyDescription={t(
          'No reset cards available. Grant a reset card to a user to get started.'
        )}
        skeletonKeyPrefix='reset-cards-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by card key or note...'),
          searchDebounceMs: 500,
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: statusOptions,
              singleSelect: true,
            },
          ],
          preActions: (
            <Button
              size='sm'
              onClick={() => setGrantOpen(true)}
              disabled={!complianceConfirmed}
            >
              <Plus className='h-4 w-4' />
              {t('Grant Reset Cards')}
            </Button>
          ),
        }}
      />

      <GrantResetCardsDialog
        open={grantOpen}
        onOpenChange={setGrantOpen}
        onGranted={refreshList}
      />

      {confirmAction && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open) setConfirmAction(null)
          }}
          title={
            confirmAction.type === 'disable'
              ? t('Confirm disable reset card')
              : t('Confirm delete reset card')
          }
          desc={
            confirmAction.type === 'disable'
              ? t(
                  'Disable this reset card? It will no longer be usable. This action cannot be undone.'
                )
              : t(
                  'Delete this reset card permanently? This action cannot be undone.'
                )
          }
          confirmText={
            confirmAction.type === 'disable' ? t('Disable') : t('Delete')
          }
          destructive={confirmAction.type === 'delete'}
          handleConfirm={handleConfirm}
          isLoading={confirmLoading}
        />
      )}
    </>
  )
}
