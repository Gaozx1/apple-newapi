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
import { Code2, Globe, Palette } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// ============================================================================
// Group regions (group_region_setting.groups)
//
// Groups can be classified as serving overseas models or domestic models.
// Picking a group that serves overseas models requires the user to declare
// themselves a foreign user first; the relay rejects such requests until the
// declaration is recorded (see model.ConfirmOverseasDeclaration).
// ============================================================================

export const GROUP_REGIONS_OPTION_KEY = 'group_region_setting.groups'

type GroupRegionValues = {
  groupRatio: string
  regions: string
}

type GroupRegionSettingsSectionProps = {
  defaultValues: GroupRegionValues
}

export const REGION_OVERSEAS = 'overseas'
export const REGION_DOMESTIC = 'domestic'

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
    /* A malformed group ratio option leaves the list empty. */
  }
  return []
}

function parseRegions(value: string): Record<string, string> {
  if (!value || value.trim() === '' || value.trim() === 'null') return {}
  try {
    const parsed: unknown = JSON.parse(value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }
    const regions: Record<string, string> = {}
    for (const [group, region] of Object.entries(
      parsed as Record<string, unknown>
    )) {
      if (region === REGION_OVERSEAS || region === REGION_DOMESTIC) {
        regions[group] = region
      }
    }
    return regions
  } catch {
    return {}
  }
}

function serializeRegions(
  groups: string[],
  regions: Record<string, string>
): string {
  // Persist only the classified groups, keeping the configured group order.
  const configured: Record<string, string> = {}
  for (const group of groups) {
    const region = regions[group]
    if (region === REGION_OVERSEAS || region === REGION_DOMESTIC) {
      configured[group] = region
    }
  }
  return JSON.stringify(configured, null, 2)
}

export function GroupRegionSettingsSection(
  props: GroupRegionSettingsSectionProps
) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [useVisualEditor, setUseVisualEditor] = useState(true)
  const [regionsJson, setRegionsJson] = useState(props.defaultValues.regions)
  const [regions, setRegions] = useState<Record<string, string>>(() =>
    parseRegions(props.defaultValues.regions)
  )

  useEffect(() => {
    setRegionsJson(props.defaultValues.regions)
    setRegions(parseRegions(props.defaultValues.regions))
  }, [props.defaultValues.regions])

  const groups = useMemo(
    () => parseGroupNames(props.defaultValues.groupRatio),
    [props.defaultValues.groupRatio]
  )
  const overseasCount = useMemo(
    () =>
      Object.values(regions).filter((region) => region === REGION_OVERSEAS)
        .length,
    [regions]
  )

  const handleSave = async () => {
    await updateOption.mutateAsync({
      key: GROUP_REGIONS_OPTION_KEY,
      value: useVisualEditor ? serializeRegions(groups, regions) : regionsJson,
    })
  }

  const handleReset = () => {
    setRegionsJson(props.defaultValues.regions)
    setRegions(parseRegions(props.defaultValues.regions))
  }

  const setRegion = (group: string, region: string) => {
    setRegions((current) => ({ ...current, [group]: region }))
  }

  const toggleEditor = () => {
    if (useVisualEditor) {
      setRegionsJson(serializeRegions(groups, regions))
    } else {
      setRegions(parseRegions(regionsJson))
    }
    setUseVisualEditor(!useVisualEditor)
  }

  return (
    <SettingsSection title={t('Group Regions')}>
      <div className='space-y-4'>
        <SettingsPageFormActions
          onSave={handleSave}
          onReset={handleReset}
          isSaving={updateOption.isPending}
          saveLabel='Save group regions'
        />

        <p className='text-muted-foreground text-sm'>
          {t(
            'Groups that serve overseas models are only usable after the user declares that they are a foreign user. Domestic groups need no declaration.'
          )}
        </p>

        <div className='flex items-center justify-between'>
          <Label>
            <Globe className='mr-1 inline h-3.5 w-3.5' />
            {t('Group regions')}{' '}
            <span className='text-muted-foreground font-normal'>
              ({t('{{count}} overseas', { count: overseasCount })})
            </span>
          </Label>
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

        {useVisualEditor ? (
          <div className='space-y-2'>
            {groups.map((group) => (
              <div
                key={group}
                className='flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2'
              >
                <span className='truncate font-medium'>{group}</span>
                <Select
                  items={[
                    { value: REGION_DOMESTIC, label: t('Domestic models') },
                    { value: REGION_OVERSEAS, label: t('Overseas models') },
                  ]}
                  value={regions[group] ?? REGION_DOMESTIC}
                  onValueChange={(value) =>
                    setRegion(group, value ?? REGION_DOMESTIC)
                  }
                >
                  <SelectTrigger className='h-8 w-40'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value={REGION_DOMESTIC}>
                        {t('Domestic models')}
                      </SelectItem>
                      <SelectItem value={REGION_OVERSEAS}>
                        {t('Overseas models')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            ))}
            {groups.length === 0 && (
              <p className='text-muted-foreground text-sm'>
                {t('No groups configured. Add a group ratio first.')}
              </p>
            )}
          </div>
        ) : (
          <JsonCodeEditor
            value={regionsJson}
            onChange={setRegionsJson}
            name={GROUP_REGIONS_OPTION_KEY}
            placeholder={`{\n  "default": "domestic",\n  "overseas-group": "overseas"\n}`}
          />
        )}
      </div>
    </SettingsSection>
  )
}
