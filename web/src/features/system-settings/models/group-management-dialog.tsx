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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Tags, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  createGroupDefinition,
  deleteGroupDefinition,
  getGroupDefinitions,
  renameGroupDefinition,
  updateGroupDefinition,
  type GroupDefinition,
} from './group-management-api'
import { GroupDefinitionFields } from './group-definition-fields'
import {
  groupDefinitionSchema,
  toGroupDefinitionFormValues,
  toGroupMutationPayload,
  type GroupDefinitionFormValues,
} from './group-definition-schema'

const GROUP_DEFINITIONS_QUERY_KEY = ['group-definitions']
const FALLBACK_LANGUAGES = ['en', 'zh', 'zh-TW', 'fr', 'ru', 'ja', 'vi']
const DEFAULT_FORM_VALUES = toGroupDefinitionFormValues(null, FALLBACK_LANGUAGES)

const REGION_LABELS: Record<string, string> = {
  domestic: 'Domestic',
  overseas: 'Overseas',
}

function groupRegionLabel(region: string) {
  return REGION_LABELS[region] ?? 'Unspecified'
}

type GroupManagementDialogProps = {
  trigger: React.ReactElement
}

export function GroupManagementDialog(props: GroupManagementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingGroup, setEditingGroup] = useState<GroupDefinition | null>(null)
  const [renamingGroup, setRenamingGroup] = useState<GroupDefinition | null>(
    null
  )
  const [deletingGroup, setDeletingGroup] = useState<GroupDefinition | null>(
    null
  )
  const [nextCode, setNextCode] = useState('')

  const definitionsQuery = useQuery({
    queryKey: GROUP_DEFINITIONS_QUERY_KEY,
    queryFn: getGroupDefinitions,
  })
  const groups = definitionsQuery.data?.data?.groups ?? []
  const languages =
    definitionsQuery.data?.data?.languages?.length
      ? definitionsQuery.data.data.languages
      : FALLBACK_LANGUAGES

  const form = useForm<GroupDefinitionFormValues>({
    resolver: zodResolver(groupDefinitionSchema),
    defaultValues: DEFAULT_FORM_VALUES,
  })

  function invalidateGroups() {
    void queryClient.invalidateQueries({ queryKey: GROUP_DEFINITIONS_QUERY_KEY })
  }

  function openEditor(group: GroupDefinition | null) {
    form.reset(toGroupDefinitionFormValues(group, languages))
    setEditingGroup(group)
    setEditorOpen(true)
  }

  const saveMutation = useMutation({
    mutationFn: async (values: GroupDefinitionFormValues) => {
      const payload = {
        ratio: values.ratio,
        region: values.region,
        enabled: values.enabled,
        sort: values.sort,
        names: toGroupMutationPayload(values.names),
      }
      const response = editingGroup
        ? await updateGroupDefinition(editingGroup.id, payload)
        : await createGroupDefinition({ code: values.code.trim(), ...payload })
      return requireServerSuccess(response)
    },
    onSuccess: () => {
      toast.success(
        editingGroup ? t('Group updated') : t('Group created')
      )
      setEditorOpen(false)
      invalidateGroups()
    },
  })

  const renameMutation = useMutation({
    mutationFn: async () => {
      if (!renamingGroup) {
        throw new Error('No group selected for rename')
      }
      const response = await renameGroupDefinition(renamingGroup.id, nextCode)
      return requireServerSuccess(response)
    },
    onSuccess: (response) => {
      const result = response.data
      toast.success(
        t(
          'Group code renamed: {{channels}} channels, {{tokens}} tokens, {{users}} users, {{abilities}} abilities updated.',
          {
            channels: result?.channels ?? 0,
            tokens: result?.tokens ?? 0,
            users: result?.users ?? 0,
            abilities: result?.abilities ?? 0,
          }
        )
      )
      setRenamingGroup(null)
      invalidateGroups()
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!deletingGroup) {
        throw new Error('No group selected for deletion')
      }
      const response = await deleteGroupDefinition(deletingGroup.id)
      return requireServerSuccess(response)
    },
    onSuccess: () => {
      toast.success(t('Group deleted'))
      setDeletingGroup(null)
      invalidateGroups()
    },
  })

  const isEditingExisting = editorOpen && editingGroup !== null
  const dialogFooter = editorOpen ? (
    <div className='flex justify-end gap-2'>
      <Button variant='outline' onClick={() => setEditorOpen(false)}>
        {t('Cancel')}
      </Button>
      <Button
        disabled={saveMutation.isPending}
        onClick={form.handleSubmit((values) => saveMutation.mutate(values))}
      >
        {t('Save')}
      </Button>
    </div>
  ) : (
    <div className='flex justify-end'>
      <Button onClick={() => openEditor(null)}>
        <Plus className='mr-1 size-4' aria-hidden='true' />
        {t('New group')}
      </Button>
    </div>
  )

  return (
    <>
      <Dialog
        title={t('Group management')}
        description={t(
          'Each group has a stable code plus a display name per language. Renaming a display name never invalidates channels or keys.'
        )}
        trigger={props.trigger}
        footer={dialogFooter}
        contentClassName='sm:max-w-3xl'
      >
        {editorOpen ? (
          <form
            className='grid gap-4'
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <GroupDefinitionFields
              form={form}
              languages={languages}
              codeLocked={isEditingExisting}
            />
          </form>
        ) : (
          <StaticDataTable<GroupDefinition>
            data={groups}
            getRowKey={(row) => row.id}
            empty={groups.length === 0}
            emptyContent={t('No groups have been registered yet.')}
            columns={[
              {
                id: 'code',
                header: t('Group code'),
                cell: (row) => (
                  <span className='font-mono text-sm'>{row.code}</span>
                ),
              },
              {
                id: 'names',
                header: t('Display names'),
                cell: (row) => {
                  const primary =
                    row.names?.en ?? Object.values(row.names ?? {})[0] ?? ''
                  const count = Object.keys(row.names ?? {}).length
                  return (
                    <div className='flex items-center gap-2'>
                      <span className='text-sm'>{primary || row.code}</span>
                      <Badge variant='secondary'>{count}</Badge>
                    </div>
                  )
                },
              },
              {
                id: 'ratio',
                header: t('Ratio'),
                cell: (row) => <span className='text-sm'>{row.ratio}</span>,
              },
              {
                id: 'region',
                header: t('Region'),
                cell: (row) => (
                  <span className='text-muted-foreground text-sm'>
                    {t(groupRegionLabel(row.region))}
                  </span>
                ),
              },
              {
                id: 'enabled',
                header: t('Enabled'),
                cell: (row) => (
                  <Badge variant={row.enabled ? 'default' : 'outline'}>
                    {row.enabled ? t('Enabled') : t('Disabled')}
                  </Badge>
                ),
              },
              {
                id: 'actions',
                header: '',
                className: 'w-32',
                cell: (row) => (
                  <div className='flex justify-end gap-1'>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Edit group {{code}}', { code: row.code })}
                      title={t('Edit')}
                      onClick={() => openEditor(row)}
                    >
                      <Pencil className='size-4' aria-hidden='true' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Rename code of {{code}}', {
                        code: row.code,
                      })}
                      title={t('Rename code')}
                      onClick={() => {
                        setNextCode(row.code)
                        setRenamingGroup(row)
                      }}
                    >
                      <Tags className='size-4' aria-hidden='true' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Delete group {{code}}', { code: row.code })}
                      title={t('Delete')}
                      onClick={() => setDeletingGroup(row)}
                    >
                      <Trash2 className='size-4' aria-hidden='true' />
                    </Button>
                  </div>
                ),
              },
            ]}
          />
        )}
      </Dialog>

      <ConfirmDialog
        open={renamingGroup !== null}
        onOpenChange={(open) => {
          if (!open) setRenamingGroup(null)
        }}
        title={t('Rename group code')}
        desc={t(
          'Renaming the code rewrites every channel, token, user and ability that references it, in a single transaction.'
        )}
        confirmText={t('Rename')}
        isLoading={renameMutation.isPending}
        handleConfirm={() => renameMutation.mutate()}
      >
        <div className='grid gap-2'>
          <Label htmlFor='group-rename-code'>{t('New group code')}</Label>
          <Input
            id='group-rename-code'
            value={nextCode}
            onChange={(event) => setNextCode(event.target.value)}
          />
        </div>
      </ConfirmDialog>

      <ConfirmDialog
        open={deletingGroup !== null}
        onOpenChange={(open) => {
          if (!open) setDeletingGroup(null)
        }}
        title={t('Delete group')}
        desc={t(
          'A group that any channel, token, user or ability still references cannot be deleted.'
        )}
        destructive
        confirmText={t('Delete')}
        isLoading={deleteMutation.isPending}
        handleConfirm={() => deleteMutation.mutate()}
      />
    </>
  )
}
