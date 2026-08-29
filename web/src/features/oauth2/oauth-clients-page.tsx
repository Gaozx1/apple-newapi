import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
import { Check, Copy, KeyRound, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import {
  createOAuthClient,
  deleteOAuthClient,
  getOAuthClients,
  updateOAuthClient,
  type OAuthClient,
  type OAuthClientForm,
} from './api'
import { CallPackagesCard } from './call-packages-card'
import { OAuth2Docs, PricingTiers } from './oauth2-docs'

function CopyField(props: { label: string; value: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    const ok = await copyToClipboard(props.value)
    if (ok) {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    }
  }

  return (
    <div className='space-y-1'>
      <Label className='text-muted-foreground text-xs'>{props.label}</Label>
      <div className='flex items-center gap-1.5'>
        <code className='bg-muted/50 min-w-0 flex-1 truncate rounded-md border px-2 py-1.5 font-mono text-xs'>
          {props.value}
        </code>
        <Button
          variant='ghost'
          size='icon-sm'
          onClick={handleCopy}
          aria-label={t('Copy')}
        >
          {copied ? (
            <Check className='size-3.5 text-emerald-500' />
          ) : (
            <Copy className='size-3.5' />
          )}
        </Button>
      </div>
    </div>
  )
}

function ClientFormDialog(props: {
  client: OAuthClient | null
  open: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<OAuthClientForm>({
    name: props.client?.name ?? '',
    redirect_uris: props.client?.redirect_uris ?? '',
    scopes: props.client?.scopes ?? '',
  })
  const [created, setCreated] = useState<{
    client_id: string
    client_secret: string
  } | null>(null)

  const mutation = useMutation({
    mutationFn: (data: OAuthClientForm) =>
      props.client
        ? updateOAuthClient(props.client.id, data)
        : createOAuthClient(data),
    onSuccess: (res) => {
      if (res.success) {
        if (!props.client && res.data?.client_secret && res.data?.client_id) {
          setCreated({
            client_id: res.data.client_id,
            client_secret: res.data.client_secret,
          })
        } else {
          props.onClose()
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

  const handleClose = () => {
    setCreated(null)
    props.onClose()
  }

  return (
    <Dialog open={props.open} onOpenChange={(open) => !open && handleClose()}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>
            {props.client ? t('Edit App') : t('Create App')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Register an OAuth 2.0 application to obtain access tokens on behalf of users.'
            )}
          </DialogDescription>
        </DialogHeader>

        {created ? (
          <div className='space-y-3'>
            <CopyField label={t('Client ID')} value={created.client_id} />
            <div className='space-y-1'>
              <Label className='text-muted-foreground text-xs'>
                {t('Client Secret')}
              </Label>
              <code className='bg-muted/50 block rounded-md border px-2 py-1.5 font-mono text-xs break-all'>
                {created.client_secret}
              </code>
              <p className='text-destructive text-xs'>
                {t('The client secret is shown only once. Store it securely.')}
              </p>
            </div>
            <DialogFooter>
              <Button onClick={handleClose}>{t('Done')}</Button>
            </DialogFooter>
          </div>
        ) : (
          <>
            <div className='space-y-4'>
              <div className='space-y-1.5'>
                <Label>{t('App Name')}</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder={t('My App')}
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Redirect URIs')}</Label>
                <textarea
                  className='border-input focus-visible:border-ring focus-visible:ring-ring/50 flex min-h-16 w-full rounded-md border bg-transparent p-2 text-sm shadow-xs transition-colors focus-visible:ring-[3px] focus-visible:outline-1 disabled:cursor-not-allowed disabled:opacity-50'
                  rows={3}
                  value={form.redirect_uris}
                  onChange={(e) =>
                    setForm({ ...form, redirect_uris: e.target.value })
                  }
                  placeholder='https://app.example.com/callback'
                />
                <p className='text-muted-foreground text-xs'>
                  {t('One or more redirect URIs, separated by newlines.')}
                </p>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Scopes')}</Label>
                <Input
                  value={form.scopes}
                  onChange={(e) => setForm({ ...form, scopes: e.target.value })}
                  placeholder='profile'
                />
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Space-separated list of allowed scopes. Leave empty to allow any scope.'
                  )}
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button variant='outline' onClick={handleClose}>
                {t('Cancel')}
              </Button>
              <Button
                disabled={mutation.isPending || !form.name.trim()}
                onClick={() => mutation.mutate(form)}
              >
                {t('Save Client')}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

// The create response returns client_id/client_secret once; the dialog shows
// both via the `created` state until the user closes it.

function ClientCard(props: {
  client: OAuthClient
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const handleCopyId = async () => {
    const ok = await copyToClipboard(props.client.client_id)
    if (ok) {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    }
  }

  return (
    <div className='group hover:border-border hover:bg-muted/20 flex flex-col gap-3 rounded-xl border p-4 transition-colors'>
      <div className='flex items-start justify-between gap-2'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <div className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-lg'>
            <KeyRound className='size-4' />
          </div>
          <div className='min-w-0'>
            <p className='truncate text-sm font-medium'>{props.client.name}</p>
            <p className='text-muted-foreground text-xs'>
              {new Date(props.client.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
        <Badge
          variant='outline'
          className={
            props.client.enabled
              ? 'border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
              : 'text-muted-foreground'
          }
        >
          {props.client.enabled ? t('Enabled') : t('Disabled')}
        </Badge>
      </div>

      <button
        type='button'
        onClick={handleCopyId}
        className='bg-muted/40 hover:bg-muted flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-left transition-colors'
        aria-label={t('Copy')}
      >
        <span className='text-muted-foreground shrink-0 text-[11px]'>
          {t('Client ID')}
        </span>
        <code className='min-w-0 flex-1 truncate font-mono text-xs'>
          {props.client.client_id}
        </code>
        {copied ? (
          <Check className='size-3.5 shrink-0 text-emerald-500' />
        ) : (
          <Copy className='text-muted-foreground size-3.5 shrink-0' />
        )}
      </button>

      {props.client.scopes ? (
        <div className='flex flex-wrap gap-1'>
          {props.client.scopes
            .split(/\s+/)
            .filter(Boolean)
            .map((scope) => (
              <Badge key={scope} variant='secondary' className='text-[11px]'>
                {scope}
              </Badge>
            ))}
        </div>
      ) : null}

      <div className='mt-auto flex items-center justify-end gap-1.5 border-t pt-3'>
        <Button variant='ghost' size='sm' onClick={props.onEdit}>
          <Pencil className='size-3.5' />
          {t('Edit')}
        </Button>
        <Button variant='ghost' size='sm' onClick={props.onDelete}>
          <Trash2 className='size-3.5' />
          {t('Delete')}
        </Button>
      </div>
    </div>
  )
}

export function OAuthClientsPage() {
  const { t } = useTranslation()
  const [editing, setEditing] = useState<OAuthClient | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<OAuthClient | null>(null)

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
      setDeleteTarget(null)
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : String(err))
    },
  })

  const clients = data ?? []

  const openCreate = () => {
    setEditing(null)
    setDialogOpen(true)
  }
  const openEdit = (client: OAuthClient) => {
    setEditing(client)
    setDialogOpen(true)
  }

  let appsContent: ReactNode
  if (isLoading) {
    appsContent = (
      <div className='text-muted-foreground py-12 text-center text-sm'>
        {t('Loading...')}
      </div>
    )
  } else if (clients.length === 0) {
    appsContent = (
      <div className='flex flex-col items-center gap-3 rounded-xl border border-dashed py-16'>
        <div className='bg-muted/50 flex size-12 items-center justify-center rounded-xl'>
          <KeyRound className='text-muted-foreground size-5' />
        </div>
        <div className='text-center'>
          <p className='text-sm font-medium'>{t('No apps yet')}</p>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Create your first OAuth 2.0 app to start integrating.')}
          </p>
        </div>
        <Button variant='outline' size='sm' onClick={openCreate}>
          <Plus className='size-3.5' />
          {t('Create App')}
        </Button>
      </div>
    )
  } else {
    appsContent = (
      <div className='grid gap-3 sm:grid-cols-2'>
        {clients.map((client) => (
          <ClientCard
            key={client.id}
            client={client}
            onEdit={() => openEdit(client)}
            onDelete={() => setDeleteTarget(client)}
          />
        ))}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('OAuth 2.0 Apps')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button size='sm' onClick={openCreate}>
          <Plus className='size-3.5' />
          {t('Create App')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-5xl space-y-4'>
          {/* Pricing — always visible at the top of the page */}
          <div className='space-y-2 rounded-xl border p-4'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <p className='text-sm font-medium'>{t('Billing')}</p>
              <p className='text-muted-foreground text-xs'>
                {t('Per user, resets daily')}
              </p>
            </div>
            <PricingTiers />
          </div>

          <CallPackagesCard />

          <Tabs defaultValue='apps'>
            <TabsList>
              <TabsTrigger value='apps'>{t('My Apps')}</TabsTrigger>
              <TabsTrigger value='docs'>{t('API Docs')}</TabsTrigger>
            </TabsList>

            <TabsContent value='apps' className='mt-4 space-y-4'>
              {appsContent}
            </TabsContent>

            <TabsContent value='docs' className='mt-4'>
              <div className='rounded-xl border p-5'>
                <OAuth2Docs />
              </div>
            </TabsContent>
          </Tabs>

          <ClientFormDialog
            key={editing?.id ?? 'new'}
            client={editing}
            open={dialogOpen}
            onClose={() => {
              setDialogOpen(false)
              setEditing(null)
            }}
          />

          {/* Delete confirmation */}
          <Dialog
            open={deleteTarget !== null}
            onOpenChange={(open) => !open && setDeleteTarget(null)}
          >
            <DialogContent className='sm:max-w-sm'>
              <DialogHeader>
                <DialogTitle>{t('Delete App')}</DialogTitle>
                <DialogDescription>
                  {t(
                    'This will permanently delete the app. Applications using its credentials will stop working.'
                  )}
                </DialogDescription>
              </DialogHeader>
              {deleteTarget ? (
                <p className='text-sm'>
                  {t('App')}:{' '}
                  <span className='font-medium'>{deleteTarget.name}</span>
                </p>
              ) : null}
              <DialogFooter>
                <Button variant='outline' onClick={() => setDeleteTarget(null)}>
                  {t('Cancel')}
                </Button>
                <Button
                  variant='destructive'
                  disabled={deleteMutation.isPending}
                  onClick={() =>
                    deleteTarget && deleteMutation.mutate(deleteTarget.id)
                  }
                >
                  {t('Delete')}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
