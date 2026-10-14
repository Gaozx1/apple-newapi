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
import { Code2, Palette, Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toIntlLocale } from '@/i18n/languages'
import { formatCurrencyFromUSD } from '@/lib/currency'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// ============================================================================
// User level ladder (user_level_setting.enabled / user_level_setting.levels)
//
// A level is unlocked by the cumulative amount a user paid (wallet top-ups,
// subscription purchases and redemption codes). Each level lists the groups it
// grants a lower ratio and/or a higher request rate limit for. Grants are
// explicit per level and never inherited from a lower level.
// ============================================================================

type GroupRateLimit = [number, number]

// id is a client-only identity: it keeps a level's inputs mounted while its
// name or threshold is edited and is stripped before the ladder is saved.
type UserLevelDraft = {
  id: string
  name: string
  threshold: number
  group_ratios: Record<string, number>
  group_rate_limits: Record<string, GroupRateLimit>
}

type UserLevelSectionValues = {
  enabled: boolean
  levels: string
  groupRatio: string
}

type UserLevelSettingsSectionProps = {
  defaultValues: UserLevelSectionValues
}

const LEVELS_OPTION_KEY = 'user_level_setting.levels'
const ENABLED_OPTION_KEY = 'user_level_setting.enabled'

let levelDraftSequence = 0

function nextLevelDraftId(): string {
  levelDraftSequence += 1
  return `level-draft-${levelDraftSequence}`
}

function parseGroupNames(groupRatio: string): string[] {
  if (!groupRatio || groupRatio.trim() === '') return []
  try {
    const parsed: unknown = JSON.parse(groupRatio)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return Object.keys(parsed as Record<string, unknown>).sort((a, b) =>
        a.localeCompare(b)
      )
    }
  } catch {
    /* A malformed group ratio option leaves the selector empty. */
  }
  return []
}

function parseLevels(value: string): UserLevelDraft[] {
  if (!value || value.trim() === '' || value.trim() === 'null') return []
  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed.map((entry) => {
      const record = (entry ?? {}) as Record<string, unknown>
      const ratios: Record<string, number> = {}
      for (const [group, ratio] of Object.entries(
        (record.group_ratios ?? {}) as Record<string, unknown>
      )) {
        ratios[group] = Number(ratio) || 0
      }
      const limits: Record<string, GroupRateLimit> = {}
      for (const [group, pair] of Object.entries(
        (record.group_rate_limits ?? {}) as Record<string, unknown>
      )) {
        if (Array.isArray(pair) && pair.length === 2) {
          limits[group] = [Number(pair[0]) || 0, Number(pair[1]) || 0]
        }
      }
      return {
        id: nextLevelDraftId(),
        name: String(record.name ?? ''),
        threshold: Number(record.threshold) || 0,
        group_ratios: ratios,
        group_rate_limits: limits,
      }
    })
  } catch {
    return []
  }
}

function serializeLevels(levels: UserLevelDraft[]): string {
  // The backend numbers levels by ascending threshold, so persist them sorted
  // and drop the client-only draft identity.
  const sorted = [...levels].sort((a, b) => a.threshold - b.threshold)
  return JSON.stringify(
    sorted.map((level) => ({
      name: level.name.trim(),
      threshold: level.threshold,
      group_ratios: level.group_ratios,
      group_rate_limits: level.group_rate_limits,
    })),
    null,
    2
  )
}

// levelNumbers ranks the drafted levels by threshold, mirroring how the backend
// numbers them (the lowest threshold is level 1).
function levelNumbers(levels: UserLevelDraft[]): number[] {
  const order = levels
    .map((level, index) => ({ index, threshold: level.threshold }))
    .sort((a, b) => a.threshold - b.threshold)
  const ranks = Array.from({ length: levels.length }, () => 0)
  order.forEach((entry, position) => {
    ranks[entry.index] = position + 1
  })
  return ranks
}

function validationErrorKey(levels: UserLevelDraft[]): string | null {
  const seen = new Set<number>()
  for (const level of levels) {
    if (level.name.trim() === '') return 'Every level needs a name'
    const cents = Math.round(level.threshold * 100)
    if (seen.has(cents)) {
      return 'Every level needs a unique cumulative recharge threshold'
    }
    seen.add(cents)
  }
  return null
}

function emptyLevel(groups: string[], used: Set<number>): UserLevelDraft {
  const next = [0, 100, 500, 1000, 2000, 5000].find(
    (candidate) => !used.has(Math.round(candidate * 100))
  )
  return {
    id: nextLevelDraftId(),
    name: '',
    threshold: next ?? 0,
    group_ratios: groups.length > 0 ? { [groups[0]]: 1 } : {},
    group_rate_limits: {},
  }
}

type GroupRatioRowsProps = {
  groups: string[]
  labels: { group: string; value: string; add: string; remove: string }
  ratios: Record<string, number>
  onChange: (ratios: Record<string, number>) => void
}

function GroupRatioRows(props: GroupRatioRowsProps) {
  const { t } = useTranslation()
  const rows = Object.entries(props.ratios)

  const update = (patch: Record<string, number>) => props.onChange(patch)

  const rename = (from: string, to: string) => {
    if (from === to) return
    const next: Record<string, number> = {}
    for (const [group, ratio] of rows) {
      next[group === from ? to : group] = ratio
    }
    update(next)
  }

  const remove = (group: string) => {
    const next: Record<string, number> = {}
    for (const [name, ratio] of rows) {
      if (name !== group) next[name] = ratio
    }
    update(next)
  }

  const add = () => {
    const available =
      props.groups.find((group) => !(group in props.ratios)) ?? ''
    update({ ...props.ratios, [available]: 1 })
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between'>
        <Label className='text-xs'>{props.labels.group}</Label>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='h-7'
          onClick={add}
          disabled={props.groups.length === 0}
        >
          <Plus className='mr-1 h-3.5 w-3.5' />
          {t(props.labels.add)}
        </Button>
      </div>
      {rows.length === 0 && (
        <p className='text-muted-foreground text-xs'>{t('No entries')}</p>
      )}
      {rows.map(([group, ratio]) => (
        <div key={group} className='flex items-center gap-2'>
          <Select
            items={props.groups.map((name) => ({ value: name, label: name }))}
            value={group}
            onValueChange={(value) => rename(group, value ?? '')}
          >
            <SelectTrigger className='h-8 flex-1'>
              <SelectValue placeholder={t('Select a group')} />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {props.groups.map((name) => (
                  <SelectItem key={name} value={name}>
                    {name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Input
            className='h-8 w-28'
            type='number'
            min={0}
            step='0.01'
            value={ratio}
            aria-label={`${props.labels.value} ${group}`}
            onChange={(event) =>
              update({
                ...props.ratios,
                [group]: Number.parseFloat(event.target.value) || 0,
              })
            }
          />
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='h-8 w-8'
            aria-label={`${t(props.labels.remove)} ${group}`}
            onClick={() => remove(group)}
          >
            <Trash2 className='h-4 w-4' />
          </Button>
        </div>
      ))}
    </div>
  )
}

type GroupRateLimitRowsProps = {
  groups: string[]
  limits: Record<string, GroupRateLimit>
  onChange: (limits: Record<string, GroupRateLimit>) => void
}

function GroupRateLimitRows(props: GroupRateLimitRowsProps) {
  const { t } = useTranslation()
  const rows = Object.entries(props.limits)

  const patch = (group: string, pair: GroupRateLimit) =>
    props.onChange({ ...props.limits, [group]: pair })

  const rename = (from: string, to: string) => {
    if (from === to) return
    const next: Record<string, GroupRateLimit> = {}
    for (const [group, pair] of rows) {
      next[group === from ? to : group] = pair
    }
    props.onChange(next)
  }

  const remove = (group: string) => {
    const next: Record<string, GroupRateLimit> = {}
    for (const [name, pair] of rows) {
      if (name !== group) next[name] = pair
    }
    props.onChange(next)
  }

  const add = () => {
    const available =
      props.groups.find((group) => !(group in props.limits)) ?? ''
    props.onChange({ ...props.limits, [available]: [0, 1000] })
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between'>
        <Label className='text-xs'>{t('Group rate limits')}</Label>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='h-7'
          onClick={add}
          disabled={props.groups.length === 0}
        >
          <Plus className='mr-1 h-3.5 w-3.5' />
          {t('Add group rate limit')}
        </Button>
      </div>
      {rows.length === 0 && (
        <p className='text-muted-foreground text-xs'>{t('No entries')}</p>
      )}
      {rows.map(([group, pair]) => (
        <div key={group} className='flex items-center gap-2'>
          <Select
            items={props.groups.map((name) => ({ value: name, label: name }))}
            value={group}
            onValueChange={(value) => rename(group, value ?? '')}
          >
            <SelectTrigger className='h-8 flex-1'>
              <SelectValue placeholder={t('Select a group')} />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {props.groups.map((name) => (
                  <SelectItem key={name} value={name}>
                    {name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Input
            className='h-8 w-24'
            type='number'
            min={0}
            value={pair[0]}
            aria-label={`${t('Max Requests (incl. failures)')} ${group}`}
            onChange={(event) =>
              patch(group, [
                Number.parseInt(event.target.value, 10) || 0,
                pair[1],
              ])
            }
          />
          <Input
            className='h-8 w-24'
            type='number'
            min={0}
            value={pair[1]}
            aria-label={`${t('Max Success')} ${group}`}
            onChange={(event) =>
              patch(group, [
                pair[0],
                Number.parseInt(event.target.value, 10) || 0,
              ])
            }
          />
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='h-8 w-8'
            aria-label={`${t('Delete')} ${group}`}
            onClick={() => remove(group)}
          >
            <Trash2 className='h-4 w-4' />
          </Button>
        </div>
      ))}
      <p className='text-muted-foreground text-xs'>
        {t('0 means unlimited; a level only ever loosens the group limit')}
      </p>
    </div>
  )
}

type LevelCardProps = {
  level: UserLevelDraft
  number: number
  groups: string[]
  onChange: (level: UserLevelDraft) => void
  onRemove: () => void
}

function LevelCard(props: LevelCardProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='bg-muted rounded px-2 py-0.5 font-mono text-xs'>
          {t('Level')} {props.number}
        </span>
        <Input
          className='h-8 w-40 min-w-[140px] flex-1'
          value={props.level.name}
          placeholder={t('Level name')}
          aria-label={t('Level name')}
          onChange={(event) =>
            props.onChange({ ...props.level, name: event.target.value })
          }
        />
        <Input
          className='h-8 w-32'
          type='number'
          min={0}
          step='0.01'
          value={props.level.threshold}
          aria-label={t('Cumulative recharge threshold')}
          onChange={(event) =>
            props.onChange({
              ...props.level,
              threshold: Number.parseFloat(event.target.value) || 0,
            })
          }
        />
        <span className='text-muted-foreground text-xs'>
          {formatCurrencyFromUSD(props.level.threshold, { locale })}
        </span>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='ml-auto h-8 w-8'
          aria-label={t('Remove level')}
          onClick={props.onRemove}
        >
          <Trash2 className='h-4 w-4' />
        </Button>
      </div>

      <div className='grid gap-3 md:grid-cols-2'>
        <GroupRatioRows
          groups={props.groups}
          labels={{
            group: 'Group ratios',
            value: 'Ratio',
            add: 'Add group ratio',
            remove: 'Remove group ratio',
          }}
          ratios={props.level.group_ratios}
          onChange={(ratios) => props.onChange({ ...props.level, group_ratios: ratios })}
        />
        <GroupRateLimitRows
          groups={props.groups}
          limits={props.level.group_rate_limits}
          onChange={(limits) =>
            props.onChange({ ...props.level, group_rate_limits: limits })
          }
        />
      </div>
    </div>
  )
}

export function UserLevelSettingsSection(
  props: UserLevelSettingsSectionProps
) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [useVisualEditor, setUseVisualEditor] = useState(true)
  const [enabled, setEnabled] = useState(props.defaultValues.enabled)
  const [levels, setLevels] = useState<UserLevelDraft[]>(() =>
    parseLevels(props.defaultValues.levels)
  )
  const [levelsJson, setLevelsJson] = useState(props.defaultValues.levels)

  useEffect(() => {
    setEnabled(props.defaultValues.enabled)
    setLevels(parseLevels(props.defaultValues.levels))
    setLevelsJson(props.defaultValues.levels)
  }, [props.defaultValues.enabled, props.defaultValues.levels])

  const groups = useMemo(
    () => parseGroupNames(props.defaultValues.groupRatio),
    [props.defaultValues.groupRatio]
  )
  const numbers = useMemo(() => levelNumbers(levels), [levels])
  const errorKey = validationErrorKey(levels)

  const handleSave = async () => {
    if (useVisualEditor && errorKey) return
    await updateOption.mutateAsync({
      key: LEVELS_OPTION_KEY,
      value: useVisualEditor ? serializeLevels(levels) : levelsJson,
    })
    await updateOption.mutateAsync({ key: ENABLED_OPTION_KEY, value: enabled })
  }

  const handleReset = () => {
    setEnabled(props.defaultValues.enabled)
    setLevels(parseLevels(props.defaultValues.levels))
    setLevelsJson(props.defaultValues.levels)
  }

  const toggleEditor = () => {
    // Keep the two representations in sync when switching modes.
    if (useVisualEditor) {
      setLevelsJson(serializeLevels(levels))
    } else {
      setLevels(parseLevels(levelsJson))
    }
    setUseVisualEditor(!useVisualEditor)
  }

  const updateLevel = (index: number, level: UserLevelDraft) => {
    const next = [...levels]
    next[index] = level
    setLevels(next)
  }

  const removeLevel = (index: number) => {
    setLevels(levels.filter((_, position) => position !== index))
  }

  const addLevel = () => {
    const used = new Set(levels.map((level) => Math.round(level.threshold * 100)))
    setLevels([...levels, emptyLevel(groups, used)])
  }

  return (
    <SettingsSection title={t('User Levels')}>
      <div className='space-y-4'>
        <SettingsPageFormActions
          onSave={handleSave}
          onReset={handleReset}
          isSaving={updateOption.isPending}
          isSaveDisabled={useVisualEditor && Boolean(errorKey)}
          saveLabel='Save user levels'
        />

        <p className='text-muted-foreground text-sm'>
          {t(
            'Levels are unlocked by the cumulative amount a user has paid (wallet top-ups, subscription purchases and redemption codes). Each level lists the groups it grants a lower ratio and a higher request rate limit for.'
          )}
        </p>
        <p className='text-muted-foreground text-sm'>
          {t(
            'A level never makes a group more expensive or stricter: the more favorable of the level grant and the group configuration is used. Levels do not inherit grants from lower levels.'
          )}
        </p>

        <SettingsSwitchField
          controlId='user-levels-enabled'
          checked={enabled}
          onCheckedChange={setEnabled}
          label={t('Enable user levels')}
          description={t(
            'When disabled, group ratios and rate limits keep their configured values for every user.'
          )}
        />

        <div className='flex items-center justify-between'>
          <Label>{t('Level configuration')}</Label>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={toggleEditor}
          >
            {useVisualEditor ? (
              <>
                <Code2 className='mr-2 h-4 w-4' />
                {t('JSON Mode')}
              </>
            ) : (
              <>
                <Palette className='mr-2 h-4 w-4' />
                {t('Visual Mode')}
              </>
            )}
          </Button>
        </div>

        {useVisualEditor && errorKey && (
          <p className='text-destructive text-sm'>{t(errorKey)}</p>
        )}

        {useVisualEditor ? (
          <div className='space-y-3'>
            {levels.length === 0 && (
              <p className='text-muted-foreground text-sm'>
                {t('No levels configured. Click "Add level" to get started.')}
              </p>
            )}
            {levels.map((level, index) => (
              <LevelCard
                key={level.id}
                level={level}
                number={numbers[index]}
                groups={groups}
                onChange={(next) => updateLevel(index, next)}
                onRemove={() => removeLevel(index)}
              />
            ))}
            <Button type='button' variant='outline' size='sm' onClick={addLevel}>
              <Plus className='mr-1 h-3.5 w-3.5' />
              {t('Add level')}
            </Button>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Thresholds are amounts in the currency the dashboard displays; levels are ranked by ascending threshold.'
              )}
            </p>
          </div>
        ) : (
          <JsonCodeEditor
            value={levelsJson}
            onChange={setLevelsJson}
            name={LEVELS_OPTION_KEY}
            placeholder={`[\n  {\n    "name": "Bronze",\n    "threshold": 0,\n    "group_ratios": { "default": 1 },\n    "group_rate_limits": { "default": [200, 100] }\n  }\n]`}
          />
        )}
      </div>
    </SettingsSection>
  )
}
