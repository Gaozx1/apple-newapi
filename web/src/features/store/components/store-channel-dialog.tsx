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
import { Download } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { CHANNEL_TYPE_OPTIONS } from '@/features/channels/constants'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  createStoreGroup,
  fetchStoreChannelModels,
  listStoreGroups,
} from '../api'
import type { StoreChannelInput, StoreChannelOffer } from '../types'

export interface StoreChannelDialogProps {
  open: boolean
  saving: boolean
  offer: StoreChannelOffer | null
  onOpenChange: (open: boolean) => void
  onSubmit: (input: StoreChannelInput) => void
}

const EMPTY: StoreChannelInput = {
  name: '',
  description: '',
  channel_type: 1,
  base_url: '',
  key: '',
  models: '',
  group_name: '',
  ratio: 1,
}

/**
 * Upload or edit one channel offer. The merchant pastes the upstream address and
 * key, pulls the model list straight from the upstream, and sets the multiplier
 * buyers are billed at while their traffic runs through the channel.
 */
export function StoreChannelDialog(props: StoreChannelDialogProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<StoreChannelInput>(EMPTY)

  useEffect(() => {
    if (!props.open) return
    if (props.offer === null) {
      setForm(EMPTY)
      return
    }
    setForm({
      name: props.offer.name,
      description: props.offer.description,
      channel_type: props.offer.channel_type,
      base_url: props.offer.base_url,
      key: '',
      models: props.offer.models,
      group_name: props.offer.group_name,
      ratio: props.offer.ratio,
    })
  }, [props.open, props.offer])

  const queryClient = useQueryClient()
  const groupsQuery = useQuery({
    queryKey: ['store-groups'],
    queryFn: listStoreGroups,
    enabled: props.open,
  })

  const newGroupMutation = useMutation({
    mutationFn: (name: string) =>
      createStoreGroup({ name, ratio: 1, remark: '' }),
    onSuccess: (group) => {
      toast.success(t('Created group {{name}}', { name: group.name }))
      update({ group_name: group.name })
      void queryClient.invalidateQueries({ queryKey: ['store-groups'] })
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Create group failed'))),
  })

  const fetchModels = useMutation({
    mutationFn: () =>
      fetchStoreChannelModels({
        channel_type: form.channel_type,
        base_url: form.base_url,
        key: form.key,
      }),
    onSuccess: (models) => {
      if (models.length === 0) {
        toast.error(t('The upstream returned no models'))
        return
      }
      setForm((current) => ({ ...current, models: models.join(', ') }))
      toast.success(t('Fetched {{count}} models', { count: models.length }))
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Failed to fetch models'))),
  })

  const update = (patch: Partial<StoreChannelInput>) =>
    setForm((current) => ({ ...current, ...patch }))
  const invalid =
    form.name.trim() === '' ||
    form.base_url.trim() === '' ||
    form.models.trim() === '' ||
    (props.offer === null && form.key.trim() === '')

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.offer === null ? t('Upload a channel') : t('Edit channel')}
      description={t(
        'Claiming a channel is free. Buyers are billed at the multiplier you set here while their traffic runs through it, and you receive that usage minus the 1% platform fee.'
      )}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={props.saving || invalid}
            onClick={() => props.onSubmit(form)}
          >
            {props.saving && <Spinner className='size-3' />}
            {t('Save')}
          </Button>
        </>
      }
    >
      <div className='grid gap-3'>
        <div className='grid gap-2'>
          <Label htmlFor='store-channel-name'>{t('Channel name')}</Label>
          <Input
            id='store-channel-name'
            value={form.name}
            onChange={(event) => update({ name: event.target.value })}
          />
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-channel-description'>{t('Description')}</Label>
          <Textarea
            id='store-channel-description'
            rows={2}
            value={form.description}
            onChange={(event) => update({ description: event.target.value })}
          />
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-channel-type'>{t('Channel type')}</Label>
          <NativeSelect
            id='store-channel-type'
            value={String(form.channel_type)}
            onChange={(event) =>
              update({ channel_type: Number(event.target.value) })
            }
          >
            {CHANNEL_TYPE_OPTIONS.map((option) => (
              <NativeSelectOption
                key={option.value}
                value={String(option.value)}
              >
                {t(option.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-channel-base-url'>{t('Base URL')}</Label>
          <Input
            id='store-channel-base-url'
            placeholder='https://api.example.com'
            value={form.base_url}
            onChange={(event) => update({ base_url: event.target.value })}
          />
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-channel-key'>
            {props.offer === null
              ? t('Channel key')
              : t('Channel key (leave empty to keep)')}
          </Label>
          <Input
            id='store-channel-key'
            type='password'
            autoComplete='off'
            value={form.key}
            onChange={(event) => update({ key: event.target.value })}
          />
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='store-channel-group'>{t('Group')}</Label>
          <div className='flex gap-2'>
            <NativeSelect
              id='store-channel-group'
              value={form.group_name}
              onChange={(event) => update({ group_name: event.target.value })}
            >
              <NativeSelectOption value=''>
                {groupsQuery.data?.default_group ?? ''}
              </NativeSelectOption>
              {(groupsQuery.data?.items ?? []).map((group) => (
                <NativeSelectOption key={group.id} value={group.name}>
                  {group.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
            <Input
              placeholder={t('New group')}
              className='max-w-40'
              onKeyDown={(event) => {
                if (event.key !== 'Enter') return
                event.preventDefault()
                const name = event.currentTarget.value.trim()
                if (name !== '') newGroupMutation.mutate(name)
              }}
            />
          </div>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Only you can see this group. Press Enter in the box to create one.'
            )}
          </p>
        </div>
        <div className='grid gap-2'>
          <div className='flex items-center justify-between gap-2'>
            <Label htmlFor='store-channel-models'>
              {t('Models (comma separated)')}
            </Label>
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={
                fetchModels.isPending ||
                form.base_url.trim() === '' ||
                form.key.trim() === ''
              }
              onClick={() => fetchModels.mutate()}
            >
              {fetchModels.isPending ? (
                <Spinner className='size-3' />
              ) : (
                <Download className='size-3' aria-hidden='true' />
              )}
              {t('Fetch models')}
            </Button>
          </div>
          <Textarea
            id='store-channel-models'
            rows={2}
            placeholder='gpt-4o, claude-3-5-sonnet'
            value={form.models}
            onChange={(event) => update({ models: event.target.value })}
          />
        </div>
      </div>
    </Dialog>
  )
}
