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
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import {
  getServerErrorMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'

interface AdminAd {
  id: number
  title: string
  image_url: string
  link_url: string
  html: string
  placement: string
  weight: number
  status: number
  views: number
  rewarded: number
}

interface AdminAdPayload {
  items: AdminAd[]
  enabled: boolean
  reward_quota: number
  reward_amount: number
  max_daily_views: number
}

interface AdDraft {
  id: number | null
  title: string
  image_url: string
  link_url: string
  html: string
  placement: string
  weight: number
  status: number
}

const EMPTY_DRAFT: AdDraft = {
  id: null,
  title: '',
  image_url: '',
  link_url: '',
  html: '',
  placement: 'reward',
  weight: 1,
  status: 1,
}

/** Admin console for ad placements, reward amount and daily impression cap. */
export function AdAdminPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<AdDraft | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AdminAd | null>(null)
  const [reward, setReward] = useState('')
  const [dailyCap, setDailyCap] = useState('')
  const [enabled, setEnabled] = useState(false)

  const adsQuery = useQuery({
    queryKey: ['admin-ads'],
    queryFn: async () =>
      requireServerSuccess(await api.get('/api/store/admin/ads'))
        .data as AdminAdPayload,
  })

  useEffect(() => {
    if (adsQuery.data === undefined) return
    setEnabled(adsQuery.data.enabled)
    setReward(String(adsQuery.data.reward_quota))
    setDailyCap(String(adsQuery.data.max_daily_views))
  }, [adsQuery.data])

  const fail = (fallback: string) => (error: unknown) =>
    toast.error(getServerErrorMessage(error, t(fallback)))

  const saveMutation = useMutation({
    mutationFn: async (payload: AdDraft) => {
      const url =
        payload.id === null
          ? '/api/store/admin/ads'
          : `/api/store/admin/ads/${payload.id}`
      const response =
        payload.id === null
          ? await api.post(url, payload)
          : await api.put(url, payload)
      return requireServerSuccess(response)
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      setDraft(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-ads'] })
    },
    onError: fail('Save failed'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ad: AdminAd) =>
      requireServerSuccess(await api.delete(`/api/store/admin/ads/${ad.id}`)),
    onSuccess: () => {
      toast.success(t('Deleted'))
      setDeleteTarget(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-ads'] })
    },
    onError: fail('Delete failed'),
  })

  const configMutation = useMutation({
    mutationFn: async () =>
      requireServerSuccess(
        await api.post('/api/store/admin/ads/config', {
          enabled,
          reward_quota: Number.parseInt(reward, 10) || 0,
          max_daily_views: Number.parseInt(dailyCap, 10) || 0,
        })
      ),
    onSuccess: () => {
      toast.success(t('Saved'))
      void queryClient.invalidateQueries({ queryKey: ['admin-ads'] })
    },
    onError: fail('Save failed'),
  })

  const items = adsQuery.data?.items ?? []

  return (
    <div className='grid gap-4'>
      <div className='grid gap-3 rounded-lg border p-4 sm:grid-cols-3'>
        <div className='flex items-center justify-between gap-2 sm:col-span-3'>
          <div>
            <Label htmlFor='ads-enabled'>{t('Enable ads')}</Label>
            <p className='text-muted-foreground text-xs'>
              {t('Turns on the ads page and the random sponsor slot.')}
            </p>
          </div>
          <Switch
            id='ads-enabled'
            checked={enabled}
            onCheckedChange={setEnabled}
          />
        </div>
        <div className='grid gap-1'>
          <Label htmlFor='ads-reward'>{t('Reward quota per view')}</Label>
          <Input
            id='ads-reward'
            type='number'
            min={0}
            value={reward}
            onChange={(event) => setReward(event.target.value)}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Quota units. 500000 = $1 by default.')}
          </p>
        </div>
        <div className='grid gap-1'>
          <Label htmlFor='ads-cap'>
            {t('Max sponsor views per user per day')}
          </Label>
          <Input
            id='ads-cap'
            type='number'
            min={0}
            value={dailyCap}
            onChange={(event) => setDailyCap(event.target.value)}
          />
        </div>
        <div className='flex items-end'>
          <Button
            type='button'
            disabled={configMutation.isPending}
            onClick={() => configMutation.mutate()}
          >
            {t('Save settings')}
          </Button>
        </div>
      </div>

      <div className='flex items-center justify-between'>
        <h3 className='text-sm font-medium'>{t('Advertisements')}</h3>
        <Button
          type='button'
          size='sm'
          onClick={() => setDraft({ ...EMPTY_DRAFT })}
        >
          <Plus className='size-4' aria-hidden='true' />
          {t('Add ad')}
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Title')}</TableHead>
            <TableHead>{t('Placement')}</TableHead>
            <TableHead>{t('Weight')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Views')}</TableHead>
            <TableHead>{t('Rewarded')}</TableHead>
            <TableHead className='text-end'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((ad) => (
            <TableRow key={ad.id}>
              <TableCell className='max-w-48 truncate'>{ad.title}</TableCell>
              <TableCell>{ad.placement}</TableCell>
              <TableCell>{ad.weight}</TableCell>
              <TableCell>
                {ad.status === 1 ? t('Enabled') : t('Disabled')}
              </TableCell>
              <TableCell>{ad.views}</TableCell>
              <TableCell>{formatCurrencyFromUSD(ad.rewarded)}</TableCell>
              <TableCell className='text-end'>
                <div className='flex justify-end gap-1'>
                  <Button
                    type='button'
                    size='icon'
                    variant='ghost'
                    aria-label={t('Edit')}
                    onClick={() =>
                      setDraft({
                        id: ad.id,
                        title: ad.title,
                        image_url: ad.image_url,
                        link_url: ad.link_url,
                        html: ad.html,
                        placement: ad.placement,
                        weight: ad.weight,
                        status: ad.status,
                      })
                    }
                  >
                    <Pencil className='size-4' aria-hidden='true' />
                  </Button>
                  <Button
                    type='button'
                    size='icon'
                    variant='ghost'
                    aria-label={t('Delete')}
                    onClick={() => setDeleteTarget(ad)}
                  >
                    <Trash2 className='size-4' aria-hidden='true' />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {items.length === 0 && (
        <p className='text-muted-foreground text-sm'>{t('No ads yet')}</p>
      )}

      {draft !== null && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setDraft(null)
          }}
          title={draft.id === null ? t('Add ad') : t('Edit ad')}
          description={t(
            'Paste the ad provider snippet (HTML/JS) or use an image with a link.'
          )}
          contentClassName='sm:max-w-[640px]'
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
                disabled={saveMutation.isPending}
                onClick={() => saveMutation.mutate(draft)}
              >
                {t('Save')}
              </Button>
            </>
          }
        >
          <div className='grid gap-3'>
            <div className='grid gap-1'>
              <Label htmlFor='ad-title'>{t('Title')}</Label>
              <Input
                id='ad-title'
                value={draft.title}
                onChange={(event) =>
                  setDraft({ ...draft, title: event.target.value })
                }
              />
            </div>
            <div className='grid gap-1'>
              <Label htmlFor='ad-placement'>{t('Placement')}</Label>
              <NativeSelect
                id='ad-placement'
                value={draft.placement}
                onChange={(event) =>
                  setDraft({ ...draft, placement: event.target.value })
                }
              >
                <option value='reward'>{t('Rewarded ads page')}</option>
                <option value='sponsor'>{t('Random site slot')}</option>
              </NativeSelect>
            </div>
            <div className='grid gap-1'>
              <Label htmlFor='ad-html'>{t('Ad HTML/JS code')}</Label>
              <Textarea
                id='ad-html'
                rows={5}
                value={draft.html}
                onChange={(event) =>
                  setDraft({ ...draft, html: event.target.value })
                }
              />
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='grid gap-1'>
                <Label htmlFor='ad-image'>{t('Image URL')}</Label>
                <Input
                  id='ad-image'
                  value={draft.image_url}
                  onChange={(event) =>
                    setDraft({ ...draft, image_url: event.target.value })
                  }
                />
              </div>
              <div className='grid gap-1'>
                <Label htmlFor='ad-link'>{t('Link URL')}</Label>
                <Input
                  id='ad-link'
                  value={draft.link_url}
                  onChange={(event) =>
                    setDraft({ ...draft, link_url: event.target.value })
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
        title={t('Delete ad')}
        desc={t('Delete this ad and its view history?')}
        destructive
        confirmText={t('Delete')}
        handleConfirm={() => {
          if (deleteTarget !== null) deleteMutation.mutate(deleteTarget)
        }}
      />
    </div>
  )
}
