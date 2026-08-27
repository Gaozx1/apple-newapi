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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import {
  createOAuthClient,
  deleteOAuthClient,
  getOAuthClients,
  updateOAuthClient,
  type OAuthClient,
  type OAuthClientForm,
} from './api'

function OAuthClientFormDialog({
  client,
  onClose,
}: {
  client: OAuthClient | null
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<OAuthClientForm>({
    name: client?.name ?? '',
    redirect_uris: client?.redirect_uris ?? '',
    scopes: client?.scopes ?? '',
  })
  const [secret, setSecret] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: (data: OAuthClientForm) =>
      client
        ? updateOAuthClient(client.id, data)
        : createOAuthClient(data),
    onSuccess: (res) => {
      if (res.success) {
        if (!client && res.data?.client_secret) {
          setSecret(res.data.client_secret)
        }
        toast.success(t('Saved'))
        queryClient.invalidateQueries({ queryKey: ['oauth2-clients'] })
      } else if (res.message) {
        toast.error(res.message)
      }
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : String(err))
    },
  })

  return (
    <div className='space-y-4 rounded-lg border p-4'>
      <div className='space-y-1'>
        <label className='text-sm font-medium'>{t('Client Name')}</label>
        <Input
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          placeholder='My App'
        />
      </div>
      <div className='space-y-1'>
        <label className='text-sm font-medium'>{t('Redirect URIs')}</label>
        <textarea
          className='w-full rounded-md border p-2 text-sm'
          rows={3}
          value={form.redirect_uris}
          onChange={(e) =>
            setForm({ ...form, redirect_uris: e.target.value })
          }
          placeholder={'https://app.example.com/callback'}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'One or more redirect URIs, separated by newlines.'
          )}
        </p>
      </div>
      <div className='space-y-1'>
        <label className='text-sm font-medium'>{t('Scopes')}</label>
        <Input
          value={form.scopes}
          onChange={(e) => setForm({ ...form, scopes: e.target.value })}
          placeholder=''
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'Space-separated list of allowed scopes. Leave empty to allow any scope.'
          )}
        </p>
      </div>

      {secret ? (
        <div className='space-y-1'>
          <label className='text-sm font-medium'>{t('Client Secret')}</label>
          <code className='block break-all rounded bg-muted p-2 text-xs'>
            {secret}
          </code>
          <p className='text-destructive text-xs'>
            {t('The client secret is shown only once. Store it securely.')}
          </p>
        </div>
      ) : null}

      {client ? (
        <div className='space-y-1'>
          <label className='text-sm font-medium'>{t('Client ID')}</label>
          <code className='block break-all rounded bg-muted p-2 text-xs'>
            {client.client_id}
          </code>
        </div>
      ) : null}

      <div className='flex justify-end gap-2'>
        <Button variant='outline' onClick={onClose}>
          {t('Cancel')}
        </Button>
        <Button
          disabled={mutation.isPending || !form.name}
          onClick={() => mutation.mutate(form)}
        >
          {t('Save Client')}
        </Button>
      </div>
    </div>
  )
}

export function OAuthClientsPage() {
  const { t } = useTranslation()
  const [editing, setEditing] = useState<OAuthClient | null>(null)
  const [creating, setCreating] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['oauth2-clients'],
    queryFn: async () => {
      const res = await getOAuthClients()
      return res.data ?? []
    },
  })

  const queryClient = useQueryClient()
  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteOAuthClient(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Deleted'))
        queryClient.invalidateQueries({ queryKey: ['oauth2-clients'] })
      } else if (res.message) {
        toast.error(res.message)
      }
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : String(err))
    },
  })

  const clients = data ?? []

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div>
          <h2 className='text-lg font-semibold'>{t('OAuth 2.0 Clients')}</h2>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Register a new OAuth 2.0 client application that can obtain access tokens on behalf of users.'
            )}
          </p>
        </div>
        <Button onClick={() => { setEditing(null); setCreating(true) }}>
          {t('Add Client')}
        </Button>
      </div>

      {(creating || editing) ? (
        <OAuthClientFormDialog
          client={editing}
          onClose={() => { setCreating(false); setEditing(null) }}
        />
      ) : null}

      {isLoading ? (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      ) : (
        <div className='overflow-x-auto rounded-lg border'>
          <table className='w-full text-sm'>
            <thead className='bg-muted'>
              <tr>
                <th className='p-2 text-left'>{t('Client Name')}</th>
                <th className='p-2 text-left'>{t('Client ID')}</th>
                <th className='p-2 text-left'>{t('Scopes')}</th>
                <th className='p-2 text-left'>{t('Enabled')}</th>
                <th className='p-2 text-right'>{t('Actions')}</th>
              </tr>
            </thead>
            <tbody>
              {clients.length === 0 ? (
                <tr>
                  <td className='p-4 text-center text-muted-foreground' colSpan={5}>
                    {t('No data')}
                  </td>
                </tr>
              ) : (
                clients.map((client) => (
                  <tr key={client.id} className='border-t'>
                    <td className='p-2'>{client.name}</td>
                    <td className='p-2'>
                      <code className='text-xs'>{client.client_id}</code>
                    </td>
                    <td className='p-2'>{client.scopes || '—'}</td>
                    <td className='p-2'>
                      {client.enabled ? t('Enabled') : t('Disabled')}
                    </td>
                    <td className='p-2 text-right'>
                      <Button
                        variant='outline'
                        size='sm'
                        className='mr-2'
                        onClick={() => { setCreating(false); setEditing(client) }}
                      >
                        {t('Edit')}
                      </Button>
                      <Button
                        variant='destructive'
                        size='sm'
                        disabled={deleteMutation.isPending}
                        onClick={() => {
                          if (window.confirm(t('Delete Client') + '?')) {
                            deleteMutation.mutate(client.id)
                          }
                        }}
                      >
                        {t('Delete')}
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
