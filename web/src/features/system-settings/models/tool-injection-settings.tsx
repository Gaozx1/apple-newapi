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
import { toast } from 'sonner'
import { Check, Globe } from 'lucide-react'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Label } from '@/components/ui/label'

import {
  SettingsControlGroup,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const ALL_TOOLS = [
  { id: 'web_search', labelKey: 'Web Search' },
  { id: 'web_search_preview', labelKey: 'Web Search Preview' },
  { id: 'google_search', labelKey: 'Google Search (Gemini)' },
  { id: 'file_search', labelKey: 'File Search' },
] as const

type ToolInjectionSettingsProps = {
  defaultEnabled: boolean
  defaultTools: string[]
}

export function ToolInjectionSettings({
  defaultEnabled,
  defaultTools,
}: ToolInjectionSettingsProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [enabled, setEnabled] = useState(defaultEnabled)
  const [selectedTools, setSelectedTools] = useState<string[]>(defaultTools)

  const toggleTool = (toolId: string) => {
    setSelectedTools((prev) =>
      prev.includes(toolId)
        ? prev.filter((id) => id !== toolId)
        : [...prev, toolId]
    )
  }

  const isDirty =
    enabled !== defaultEnabled ||
    selectedTools.length !== defaultTools.length ||
    !selectedTools.every((id) => defaultTools.includes(id))

  const handleSave = async () => {
    if (!enabled && selectedTools.length > 0) {
      toast.error(t('Enable tool injection before selecting tools'))
      return
    }

    const toolsValue = JSON.stringify(selectedTools)
    await updateOption.mutateAsync({
      key: 'tool_injection.enabled',
      value: enabled,
    })
    await updateOption.mutateAsync({
      key: 'tool_injection.tools',
      value: toolsValue,
    })
  }

  const handleReset = () => {
    setEnabled(defaultEnabled)
    setSelectedTools(defaultTools)
  }

  return (
    <SettingsSection title={t('Tool Injection')}>
      <Alert>
        <AlertDescription className='text-sm'>
          {t(
            'Force built-in tools onto every outbound request so upstream models can discover and use them even when the client does not send a tools array. Injected tools execute at the upstream provider; the gateway only forwards the definition and bills the resulting calls.'
          )}
        </AlertDescription>
      </Alert>

      <SettingsSwitchField
        checked={enabled}
        onCheckedChange={setEnabled}
        label={t('Enable Tool Injection')}
        description={t(
          'When enabled, the selected built-in tools are automatically added to requests routed to compatible channels.'
        )}
      />

      <SettingsControlGroup className={enabled ? undefined : 'opacity-60'}>
        <Label className='text-sm font-medium'>{t('Built-in Tools')}</Label>
        <p className='text-muted-foreground text-xs'>
          {t('Select which built-in tools to make available to models.')}
        </p>
        <div className='flex flex-col gap-2 pt-1'>
          {ALL_TOOLS.map((tool) => {
            const checked = selectedTools.includes(tool.id)
            return (
              <button
                type='button'
                key={tool.id}
                disabled={!enabled}
                onClick={() => toggleTool(tool.id)}
                className='flex items-center justify-between rounded-lg border px-3 py-2 text-left text-sm transition-colors hover:bg-muted/40 disabled:cursor-not-allowed'
                aria-pressed={checked}
              >
                <span className='flex items-center gap-2'>
                  <Globe
                    className='text-muted-foreground h-4 w-4'
                    aria-hidden='true'
                  />
                  {t(tool.labelKey)}
                  <code className='bg-muted rounded px-1.5 py-0.5 text-xs'>
                    {tool.id}
                  </code>
                </span>
                <span
                  className={`flex h-5 w-5 items-center justify-center rounded-full border ${
                    checked
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-muted-foreground/40'
                  }`}
                  aria-hidden='true'
                >
                  {checked ? <Check className='h-3 w-3' /> : null}
                </span>
              </button>
            )
          })}
        </div>
      </SettingsControlGroup>

      <SettingsPageFormActions
        onSave={handleSave}
        onReset={handleReset}
        isSaving={updateOption.isPending}
        isSaveDisabled={!isDirty}
        isResetDisabled={!isDirty || updateOption.isPending}
      />
    </SettingsSection>
  )
}
