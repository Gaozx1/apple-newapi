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
import {
  Controller,
  type FieldError as HookFormFieldError,
  type UseFormReturn,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'

import type { GroupDefinitionFormValues } from './group-definition-schema'

type GroupDefinitionFieldsProps = {
  form: UseFormReturn<GroupDefinitionFormValues>
  languages: string[]
  codeLocked: boolean
}

type FieldErrorProps = {
  message?: string
}

function FieldError(props: FieldErrorProps) {
  const { t } = useTranslation()
  if (!props.message) {
    return null
  }
  // Validation messages are English source keys so they translate like any
  // other UI copy.
  return <p className='text-destructive text-sm'>{t(props.message)}</p>
}

/**
 * Create/edit fields for one group. The code is only editable while creating:
 * channels, tokens, users and abilities reference it, so changing an existing
 * code has to go through the dedicated rename action that migrates every
 * reference.
 */
export function GroupDefinitionFields(props: GroupDefinitionFieldsProps) {
  const { t } = useTranslation()
  const errors = props.form.formState.errors
  // `names` is a record, so its refined (whole-object) error is typed as a map
  // of field errors; the refinement itself writes a single message here.
  const namesError = errors.names as unknown as HookFormFieldError | undefined

  return (
    <div className='grid gap-4'>
      <div className='grid gap-2'>
        <Label htmlFor='group-definition-code'>{t('Group code')}</Label>
        <Input
          id='group-definition-code'
          disabled={props.codeLocked}
          aria-describedby='group-definition-code-hint'
          {...props.form.register('code')}
        />
        <p
          id='group-definition-code-hint'
          className='text-muted-foreground text-xs'
        >
          {props.codeLocked
            ? t(
                'Group code is the stable identifier channels and keys use. Change it with the rename action.'
              )
            : t(
                'Any characters except a comma. This becomes the group identifier and should stay unchanged once channels use it.'
              )}
        </p>
        <FieldError message={errors.code?.message} />
      </div>

      <div className='grid gap-2'>
        <Label>{t('Display names')}</Label>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Renaming a display name never affects channels or keys. Missing languages fall back to English, then to the code.'
          )}
        </p>
        <div className='grid gap-2 sm:grid-cols-2'>
          {props.languages.map((language) => (
            <div key={language} className='grid gap-1'>
              <Label
                htmlFor={`group-definition-name-${language}`}
                className='text-xs uppercase'
              >
                {language}
              </Label>
              <Input
                id={`group-definition-name-${language}`}
                aria-label={t('Display name in {{language}}', { language })}
                {...props.form.register(`names.${language}`)}
              />
            </div>
          ))}
        </div>
        <FieldError message={namesError?.message} />
      </div>

      <div className='grid gap-4 sm:grid-cols-3'>
        <div className='grid gap-2'>
          <Label htmlFor='group-definition-ratio'>{t('Ratio')}</Label>
          <Controller
            control={props.form.control}
            name='ratio'
            render={({ field }) => (
              <Input
                id='group-definition-ratio'
                type='number'
                step='0.0001'
                min='0'
                value={field.value}
                onChange={(event) => field.onChange(Number(event.target.value))}
              />
            )}
          />
          <FieldError message={errors.ratio?.message} />
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='group-definition-region'>{t('Region')}</Label>
          <Controller
            control={props.form.control}
            name='region'
            render={({ field }) => (
              <NativeSelect
                id='group-definition-region'
                value={field.value}
                onChange={(event) => field.onChange(event.target.value)}
              >
                <NativeSelectOption value=''>
                  {t('Unspecified')}
                </NativeSelectOption>
                <NativeSelectOption value='domestic'>
                  {t('Domestic')}
                </NativeSelectOption>
                <NativeSelectOption value='overseas'>
                  {t('Overseas')}
                </NativeSelectOption>
              </NativeSelect>
            )}
          />
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='group-definition-sort'>{t('Sort order')}</Label>
          <Controller
            control={props.form.control}
            name='sort'
            render={({ field }) => (
              <Input
                id='group-definition-sort'
                type='number'
                step='1'
                value={field.value}
                onChange={(event) => field.onChange(Number(event.target.value))}
              />
            )}
          />
        </div>
      </div>

      <div className='flex items-center gap-3'>
        <Controller
          control={props.form.control}
          name='enabled'
          render={({ field }) => (
            <Switch
              id='group-definition-enabled'
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
        <Label htmlFor='group-definition-enabled'>{t('Enabled')}</Label>
      </div>
    </div>
  )
}
