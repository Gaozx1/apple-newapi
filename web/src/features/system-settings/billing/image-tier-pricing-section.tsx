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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// ============================================================================
// 图片按分辨率计费（按模型配置 1K/2K/4K 按次价格）
// ============================================================================

type ModelTierRow = {
  model: string
  enabled: boolean
  price_1k: number
  price_2k: number
  price_4k: number
}

function parseTierConfig(raw: string | undefined): ModelTierRow[] {
  if (!raw || !raw.trim()) return []
  try {
    const parsed = JSON.parse(raw) as Record<
      string,
      Partial<
        Pick<ModelTierRow, 'enabled' | 'price_1k' | 'price_2k' | 'price_4k'>
      >
    >
    return Object.entries(parsed)
      .filter(([model]) => Boolean(model))
      .map(([model, cfg]) => ({
        model,
        enabled: Boolean(cfg?.enabled),
        price_1k: Number(cfg?.price_1k ?? 0),
        price_2k: Number(cfg?.price_2k ?? 0),
        price_4k: Number(cfg?.price_4k ?? 0),
      }))
  } catch {
    return []
  }
}

export function ImageTierPricingSection() {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [rows, setRows] = useState<ModelTierRow[]>([])
  const [saving, setSaving] = useState(false)

  // Lazy-load the current table from system options on first render.
  useEffect(() => {
    api
      .get('/api/option/')
      .then((res) => {
        const items = (res.data?.data ?? []) as Array<{
          key: string
          value: string
        }>
        const entry = items.find((item) => item.key === 'ImageTierPrice')
        const raw = entry?.value ?? '{}'
        setRows(parseTierConfig(raw))
      })
      .catch(() => {
        setRows([])
      })
  }, [])

  const handleSave = async () => {
    const table: Record<
      string,
      Pick<ModelTierRow, 'enabled' | 'price_1k' | 'price_2k' | 'price_4k'>
    > = {}
    for (const row of rows) {
      const model = row.model.trim()
      if (!model) continue
      table[model] = {
        enabled: row.enabled,
        price_1k: Number(row.price_1k) || 0,
        price_2k: Number(row.price_2k) || 0,
        price_4k: Number(row.price_4k) || 0,
      }
    }
    setSaving(true)
    try {
      await updateOption.mutateAsync({
        key: 'ImageTierPrice',
        value: JSON.stringify(table),
      })
      toast.success(t('保存成功'))
    } catch {
      toast.error(t('保存失败'))
    } finally {
      setSaving(false)
    }
  }

  const updateRow = (index: number, patch: Partial<ModelTierRow>) => {
    setRows((current) =>
      current.map((row, i) => (i === index ? { ...row, ...patch } : row))
    )
  }

  const removeRow = (index: number) => {
    setRows((current) => current.filter((_, i) => i !== index))
  }

  const addRow = () => {
    setRows((current) => [
      ...current,
      { model: '', enabled: true, price_1k: 0, price_2k: 0, price_4k: 0 },
    ])
  }

  return (
    <SettingsSection title={t('图片按分辨率计费')}>
      <div className='space-y-3'>
        <p className='text-muted-foreground text-xs'>
          {t(
            '为生图模型按实际出图分辨率配置每次调用的价格：最长边 ≤1280 为 1K，≤2560 为 2K，更高为 4K。启用后按实际出图分档扣费；未匹配到分档价格时用模型固定价格。'
          )}
        </p>

        {rows.length === 0 ? (
          <p className='text-muted-foreground text-sm'>{t('暂无配置')}</p>
        ) : (
          <div className='space-y-3'>
            {rows.map((row, index) => (
              <div
                key={row.model || `row-${index}`}
                className='rounded-lg border p-3'
              >
                <div className='flex flex-wrap items-center gap-2'>
                  <Input
                    className='h-8 w-44 min-w-[140px] flex-1'
                    value={row.model}
                    placeholder={t('模型名称')}
                    onChange={(e) =>
                      updateRow(index, { model: e.target.value })
                    }
                  />
                  {(['1K', '2K', '4K'] as const).map((tier) => (
                    <div key={tier} className='flex items-center gap-1'>
                      <span className='text-muted-foreground text-xs'>
                        {tier}
                      </span>
                      <Input
                        className='h-8 w-20'
                        type='number'
                        min={0}
                        step='0.001'
                        value={
                          row[
                            `price_${tier.toLowerCase()}` as
                              | 'price_1k'
                              | 'price_2k'
                              | 'price_4k'
                          ]
                        }
                        aria-label={tier}
                        onChange={(e) =>
                          updateRow(index, {
                            [`price_${tier.toLowerCase()}`]:
                              Number.parseFloat(e.target.value) || 0,
                          } as Partial<ModelTierRow>)
                        }
                      />
                      <span className='text-muted-foreground text-xs'>$</span>
                    </div>
                  ))}
                  <Switch
                    checked={row.enabled}
                    onCheckedChange={(checked) =>
                      updateRow(index, { enabled: checked })
                    }
                    aria-label={t('启用')}
                  />
                  <Button
                    type='button'
                    size='icon'
                    variant='ghost'
                    className='h-8 w-8'
                    aria-label={t('删除')}
                    disabled={saving}
                    onClick={() => removeRow(index)}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={saving}
            onClick={addRow}
          >
            <Plus className='mr-1 h-3.5 w-3.5' />
            {t('添加模型')}
          </Button>
          <Button
            type='button'
            size='sm'
            disabled={saving || rows.length === 0}
            onClick={handleSave}
          >
            {saving ? t('保存中...') : t('保存全部')}
          </Button>
        </div>
      </div>
    </SettingsSection>
  )
}
