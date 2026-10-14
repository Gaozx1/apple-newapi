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
import { ImagePlus, Sparkles, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { ApiKeyGroupCombobox } from '@/features/keys/components/api-key-group-combobox'
import { getUserGroups } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'

import { createOfflineImageTask, getOfflineImageOptions } from '../api'
import {
  createOfflineImageFormSchema,
  offlineImageFormDefaults,
  type OfflineImageFormValues,
} from '../lib/form'
import type { OfflineImageMode } from '../types'

export function OfflineImageCreateForm(props: { onCreated?: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [reference, setReference] = useState<File | null>(null)
  const [group, setGroup] = useState('')
  const [mode, setMode] = useState<OfflineImageMode>('generate')

  const groupsQuery = useQuery({
    queryKey: ['offline-image-groups'],
    queryFn: getUserGroups,
  })

  const optionsQuery = useQuery({
    queryKey: ['offline-image-options', group],
    queryFn: () => getOfflineImageOptions(group),
    enabled: group !== '',
  })

  const maxCount = optionsQuery.data?.max_n ?? 1
  const resolver = useMemo(
    () => zodResolver(createOfflineImageFormSchema(maxCount)),
    [maxCount]
  )
  const form = useForm<OfflineImageFormValues>({
    resolver,
    defaultValues: offlineImageFormDefaults,
  })

  const groupOptions = useMemo(() => {
    const groups = groupsQuery.data?.data ?? {}
    return Object.entries(groups).map(([value, meta]) => ({
      value,
      label: value,
      desc: meta.desc,
      ratio: meta.ratio,
      region: meta.region,
    }))
  }, [groupsQuery.data])

  const modelOptions = useMemo(
    () =>
      (optionsQuery.data?.models ?? []).map((model) => ({
        value: model,
        label: model,
      })),
    [optionsQuery.data]
  )

  const mutation = useMutation({
    mutationFn: createOfflineImageTask,
    onSuccess: () => {
      toast.success(t('Task created. You can close this page.'))
      setReference(null)
      form.reset({ ...offlineImageFormDefaults, group, model: '' })
      void queryClient.invalidateQueries({ queryKey: ['offline-image-tasks'] })
      props.onCreated?.()
    },
    onError: (error) => {
      handleServerError(error, t('Failed to create the task'))
    },
  })

  const errors = form.formState.errors

  const onSubmit = form.handleSubmit((values) => {
    mutation.mutate({
      group: values.group,
      model: values.model,
      mode: values.mode,
      prompt: values.prompt,
      size: values.size,
      quality: values.quality,
      n: values.mode === 'edit' ? 1 : values.count,
      reference,
    })
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Sparkles className='size-4' aria-hidden='true' />
          {t('New offline task')}
        </CardTitle>
        <CardDescription>
          {t(
            'The server runs the task on its own, so you can close this page and collect the images later.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className='grid gap-4'>
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='grid gap-2'>
              <Label htmlFor='offline-image-group'>{t('Group')}</Label>
              <ApiKeyGroupCombobox
                options={groupOptions}
                value={group}
                onValueChange={(value) => {
                  setGroup(value)
                  form.setValue('group', value, { shouldValidate: true })
                  form.setValue('model', '', { shouldValidate: false })
                }}
                placeholder={t('Select a group')}
              />
              {errors.group && (
                <p className='text-destructive text-xs'>
                  {t(errors.group.message ?? '')}
                </p>
              )}
            </div>

            <div className='grid gap-2'>
              <Label htmlFor='offline-image-model'>{t('Model')}</Label>
              <Combobox
                id='offline-image-model'
                aria-label={t('Model')}
                options={modelOptions}
                value={form.watch('model')}
                onValueChange={(value) =>
                  form.setValue('model', value ?? '', { shouldValidate: true })
                }
                placeholder={t('Select a model')}
                searchPlaceholder={t('Search models')}
                emptyText={t('No model available in this group')}
                disabled={group === '' || optionsQuery.isLoading}
              />
              {errors.model && (
                <p className='text-destructive text-xs'>
                  {t(errors.model.message ?? '')}
                </p>
              )}
            </div>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='offline-image-mode'>{t('Task type')}</Label>
            <Select
              value={mode}
              onValueChange={(value: string | null) => {
                const next: OfflineImageMode =
                  value === 'edit' ? 'edit' : 'generate'
                setMode(next)
                form.setValue('mode', next, { shouldValidate: true })
              }}
            >
              <SelectTrigger id='offline-image-mode' className='w-full sm:w-64'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='generate'>{t('Text to image')}</SelectItem>
                <SelectItem value='edit'>{t('Image to image')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='offline-image-prompt'>{t('Prompt')}</Label>
            <Textarea
              id='offline-image-prompt'
              rows={3}
              placeholder={t('Describe the image you want')}
              aria-invalid={Boolean(errors.prompt)}
              {...form.register('prompt')}
            />
            {errors.prompt && (
              <p className='text-destructive text-xs'>
                {t(errors.prompt.message ?? '')}
              </p>
            )}
          </div>

          {mode === 'edit' && (
            <div className='grid gap-2'>
              <Label htmlFor='offline-image-reference'>
                {t('Reference image')}
              </Label>
              <div className='flex flex-wrap items-center gap-2'>
                <Input
                  id='offline-image-reference'
                  type='file'
                  accept='image/png,image/jpeg,image/webp'
                  className='max-w-sm'
                  onChange={(event) =>
                    setReference(event.target.files?.[0] ?? null)
                  }
                />
                {reference && (
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    onClick={() => setReference(null)}
                  >
                    <Trash2 data-icon='inline-start' />
                    {t('Remove')}
                  </Button>
                )}
              </div>
              <p className='text-muted-foreground text-xs'>
                <ImagePlus className='mr-1 inline size-3' aria-hidden='true' />
                {t('PNG, JPEG or WebP up to {{size}} MB', {
                  size: optionsQuery.data?.reference_max_mb ?? 10,
                })}
              </p>
            </div>
          )}

          {mode === 'generate' && (
            <div className='grid gap-4 sm:grid-cols-3'>
              <div className='grid gap-2'>
                <Label htmlFor='offline-image-size'>{t('Size')}</Label>
                <Input
                  id='offline-image-size'
                  placeholder='1024x1024'
                  {...form.register('size')}
                />
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='offline-image-quality'>{t('Quality')}</Label>
                <Input
                  id='offline-image-quality'
                  placeholder='high'
                  {...form.register('quality')}
                />
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='offline-image-count'>{t('Images')}</Label>
                <Input
                  id='offline-image-count'
                  type='number'
                  min={1}
                  max={maxCount}
                  {...form.register('count', { valueAsNumber: true })}
                />
                {errors.count && (
                  <p className='text-destructive text-xs'>
                    {t(errors.count.message ?? '')}
                  </p>
                )}
              </div>
            </div>
          )}

          <div className='flex flex-wrap items-center justify-between gap-3'>
            <p className='text-muted-foreground text-xs'>
              {t('Queueing limit')}:{' '}
              <Badge variant='secondary'>
                {optionsQuery.data?.max_queued ?? 10}
              </Badge>
            </p>
            <Button type='submit' disabled={mutation.isPending}>
              {mutation.isPending ? t('Creating...') : t('Create task')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
