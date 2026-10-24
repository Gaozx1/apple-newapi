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
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import {
  getServerErrorMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'

export interface PayProviderProfile {
  id: string
  type: string
  name: string
  pay_address: string
  epay_id: string
  epay_key: string
  stripe_api_secret: string
  stripe_webhook_secret: string
  stripe_price_id: string
  creem_api_key: string
  creem_webhook_secret: string
  waffo_api_key: string
  fee_percent: number
  enabled: boolean
}

const EMPTY_PROVIDER: PayProviderProfile = {
  id: '',
  type: 'epay',
  name: '',
  pay_address: '',
  epay_id: '',
  epay_key: '',
  stripe_api_secret: '',
  stripe_webhook_secret: '',
  stripe_price_id: '',
  creem_api_key: '',
  creem_webhook_secret: '',
  waffo_api_key: '',
  fee_percent: 0,
  enabled: true,
}

const PROVIDER_TYPES = [
  'epay',
  'stripe',
  'creem',
  'waffo',
  'waffo_pancake',
] as const

/**
 * Payment provider profiles. Each payment method (PayMethods) can name one
 * provider id, so different channels may be settled by different processors.
 */
export function PayProvidersEditor() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<PayProviderProfile | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<PayProviderProfile | null>(
    null
  )
  const [providers, setProviders] = useState<PayProviderProfile[]>([])

  const providersQuery = useQuery({
    queryKey: ['pay-providers'],
    queryFn: async () => {
      const response = await api.get('/api/option/')
      const data = requireServerSuccess(response.data).data as {
        PayProviders?: string
      }
      return parseProviders(data.PayProviders)
    },
  })

  useEffect(() => {
    if (providersQuery.data !== undefined) setProviders(providersQuery.data)
  }, [providersQuery.data])

  const saveMutation = useMutation({
    mutationFn: async (next: PayProviderProfile[]) => {
      const response = await api.put('/api/option/', {
        key: 'PayProviders',
        value: JSON.stringify(next),
      })
      return requireServerSuccess(response.data)
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      setDraft(null)
      void queryClient.invalidateQueries({ queryKey: ['pay-providers'] })
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Save failed'))),
  })

  const upsert = (profile: PayProviderProfile) => {
    const next = providers.filter((item) => item.id !== profile.id)
    next.push(profile)
    saveMutation.mutate(next)
  }

  const remove = (profile: PayProviderProfile) => {
    saveMutation.mutate(providers.filter((item) => item.id !== profile.id))
    setDeleteTarget(null)
  }

  return (
    <div className='grid gap-3'>
      <div className='flex items-center justify-between'>
        <div>
          <h3 className='text-sm font-medium'>{t('Payment providers')}</h3>
          <p className='text-muted-foreground text-xs'>
            {t(
              'A payment method can name a provider below, so different channels use different processors.'
            )}
          </p>
        </div>
        <Button
          type='button'
          size='sm'
          onClick={() => setDraft({ ...EMPTY_PROVIDER })}
        >
          <Plus className='size-4' aria-hidden='true' />
          {t('Add provider')}
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Provider ID')}</TableHead>
            <TableHead>{t('Type')}</TableHead>
            <TableHead>{t('Name')}</TableHead>
            <TableHead>{t('Fee %')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead className='text-end'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {providers.map((provider) => (
            <TableRow key={provider.id}>
              <TableCell className='font-mono text-xs'>{provider.id}</TableCell>
              <TableCell>{provider.type}</TableCell>
              <TableCell className='max-w-40 truncate'>
                {provider.name}
              </TableCell>
              <TableCell>{provider.fee_percent}</TableCell>
              <TableCell>
                {provider.enabled ? t('Enabled') : t('Disabled')}
              </TableCell>
              <TableCell className='text-end'>
                <div className='flex justify-end gap-1'>
                  <Button
                    type='button'
                    size='icon'
                    variant='ghost'
                    aria-label={t('Edit')}
                    onClick={() => setDraft(provider)}
                  >
                    <Pencil className='size-4' aria-hidden='true' />
                  </Button>
                  <Button
                    type='button'
                    size='icon'
                    variant='ghost'
                    aria-label={t('Delete')}
                    onClick={() => setDeleteTarget(provider)}
                  >
                    <Trash2 className='size-4' aria-hidden='true' />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {providers.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t(
            'No provider profiles yet. Without one, the built-in settings apply.'
          )}
        </p>
      )}

      {draft !== null && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setDraft(null)
          }}
          title={t('Payment provider')}
          description={t(
            'Credentials for one processor. Reference its id from a payment method to route that channel here.'
          )}
          contentClassName='sm:max-w-[600px]'
          contentHeight='auto'
          footer={
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => setDraft(null)}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                disabled={saveMutation.isPending || draft.id.trim() === ''}
                onClick={() => upsert(draft)}
              >
                {t('Save')}
              </Button>
            </>
          }
        >
          <div className='grid gap-3'>
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='grid gap-1'>
                <Label htmlFor='provider-id'>{t('Provider ID')}</Label>
                <Input
                  id='provider-id'
                  value={draft.id}
                  onChange={(event) =>
                    setDraft({ ...draft, id: event.target.value.trim() })
                  }
                />
              </div>
              <div className='grid gap-1'>
                <Label htmlFor='provider-type'>{t('Type')}</Label>
                <NativeSelect
                  id='provider-type'
                  value={draft.type}
                  onChange={(event) =>
                    setDraft({ ...draft, type: event.target.value })
                  }
                >
                  {PROVIDER_TYPES.map((type) => (
                    <option key={type} value={type}>
                      {type}
                    </option>
                  ))}
                </NativeSelect>
              </div>
            </div>

            <div className='grid gap-1'>
              <Label htmlFor='provider-name'>{t('Name')}</Label>
              <Input
                id='provider-name'
                value={draft.name}
                onChange={(event) =>
                  setDraft({ ...draft, name: event.target.value })
                }
              />
            </div>

            {draft.type === 'epay' && (
              <>
                <div className='grid gap-1'>
                  <Label htmlFor='provider-address'>{t('Epay address')}</Label>
                  <Input
                    id='provider-address'
                    value={draft.pay_address}
                    onChange={(event) =>
                      setDraft({ ...draft, pay_address: event.target.value })
                    }
                  />
                </div>
                <div className='grid gap-3 sm:grid-cols-2'>
                  <div className='grid gap-1'>
                    <Label htmlFor='provider-pid'>{t('Epay PID')}</Label>
                    <Input
                      id='provider-pid'
                      value={draft.epay_id}
                      onChange={(event) =>
                        setDraft({ ...draft, epay_id: event.target.value })
                      }
                    />
                  </div>
                  <div className='grid gap-1'>
                    <Label htmlFor='provider-key'>{t('Epay key')}</Label>
                    <Input
                      id='provider-key'
                      type='password'
                      value={draft.epay_key}
                      onChange={(event) =>
                        setDraft({ ...draft, epay_key: event.target.value })
                      }
                    />
                  </div>
                </div>
              </>
            )}

            {draft.type === 'stripe' && (
              <>
                <div className='grid gap-1'>
                  <Label htmlFor='provider-sk'>{t('Stripe secret key')}</Label>
                  <Input
                    id='provider-sk'
                    type='password'
                    value={draft.stripe_api_secret}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        stripe_api_secret: event.target.value,
                      })
                    }
                  />
                </div>
                <div className='grid gap-1'>
                  <Label htmlFor='provider-wh'>
                    {t('Stripe webhook secret')}
                  </Label>
                  <Input
                    id='provider-wh'
                    type='password'
                    value={draft.stripe_webhook_secret}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        stripe_webhook_secret: event.target.value,
                      })
                    }
                  />
                </div>
                <div className='grid gap-1'>
                  <Label htmlFor='provider-price'>{t('Stripe price ID')}</Label>
                  <Input
                    id='provider-price'
                    value={draft.stripe_price_id}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        stripe_price_id: event.target.value,
                      })
                    }
                  />
                </div>
              </>
            )}

            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='grid gap-1'>
                <Label htmlFor='provider-fee'>{t('Fee %')}</Label>
                <Input
                  id='provider-fee'
                  type='number'
                  min={0}
                  max={100}
                  step='0.01'
                  value={draft.fee_percent}
                  onChange={(event) =>
                    setDraft({
                      ...draft,
                      fee_percent: Number.parseFloat(event.target.value) || 0,
                    })
                  }
                />
              </div>
              <div className='flex items-end justify-between gap-2 pb-2'>
                <Label htmlFor='provider-enabled'>{t('Enabled')}</Label>
                <Switch
                  id='provider-enabled'
                  checked={draft.enabled}
                  onCheckedChange={(value) =>
                    setDraft({ ...draft, enabled: value })
                  }
                />
              </div>
            </div>
          </div>
        </Dialog>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete provider')}
        desc={t(
          'Payment methods still referencing it fall back to the legacy settings.'
        )}
        destructive
        confirmText={t('Delete')}
        handleConfirm={() => {
          if (deleteTarget !== null) remove(deleteTarget)
        }}
      />
    </div>
  )
}

function parseProviders(value: string | undefined): PayProviderProfile[] {
  if (!value) return []
  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (item): item is PayProviderProfile =>
        item !== null &&
        typeof item === 'object' &&
        typeof (item as PayProviderProfile).id === 'string'
    )
  } catch {
    return []
  }
}
