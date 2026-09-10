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
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { api } from '@/lib/api'

// ============================================================================
// 图片按分辨率计费编辑器（按模型配置 1K/2K/4K 按次价格）
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

export function ImageTierPricingEditor() {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [rows, setRows] = useState<ModelTierRow[]>([])
  const [loaded, setLoaded] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api
      .get('/api/option/')
      .then((res) => {
        const items = (res.data?.data ?? []) as Array<{
          key: string
          value: string
        }>
        const entry = items.find((item) => item.key === 'ImageTierPrice')
        setRows(parseTierConfig(entry?.value))
      })
      .catch(() => {
        toast.error(t('Failed to load configuration'))
      })
      .finally(() => setLoaded(true))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

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
      toast.success(t('Save succeeded'))
    } catch {
      toast.error(t('Save failed'))
    } finally {
      setSaving(false)
    }
  }

  if (!loaded) {
    return <Skeleton className='h-24 w-full rounded-xl' />
  }

  return (
    <div className='space-y-3'>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Per-model flat USD price per call by the ACTUAL generated image resolution: longest edge ≤ 1280px bills at 1K, ≤ 2560px at 2K, above at 4K. When a request uses "auto" or a size that cannot be measured, the model flat price applies.'
        )}
      </p>

      {rows.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('No image tier models configured')}
        </p>
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
                  placeholder={t('Model name')}
                  onChange={(e) => updateRow(index, { model: e.target.value })}
                />
                <div className='flex items-center gap-1'>
                  <span className='text-muted-foreground text-xs'>1K</span>
                  <Input
                    className='h-8 w-20'
                    type='number'
                    min={0}
                    step='0.001'
                    value={row.price_1k}
                    aria-label='1K'
                    onChange={(e) =>
                      updateRow(index, {
                        price_1k: Number.parseFloat(e.target.value) || 0,
                      })
                    }
                  />
                  <span className='text-muted-foreground text-xs'>$</span>
                </div>
                <div className='flex items-center gap-1'>
                  <span className='text-muted-foreground text-xs'>2K</span>
                  <Input
                    className='h-8 w-20'
                    type='number'
                    min={0}
                    step='0.001'
                    value={row.price_2k}
                    aria-label='2K'
                    onChange={(e) =>
                      updateRow(index, {
                        price_2k: Number.parseFloat(e.target.value) || 0,
                      })
                    }
                  />
                  <span className='text-muted-foreground text-xs'>$</span>
                </div>
                <div className='flex items-center gap-1'>
                  <span className='text-muted-foreground text-xs'>4K</span>
                  <Input
                    className='h-8 w-20'
                    type='number'
                    min={0}
                    step='0.001'
                    value={row.price_4k}
                    aria-label='4K'
                    onChange={(e) =>
                      updateRow(index, {
                        price_4k: Number.parseFloat(e.target.value) || 0,
                      })
                    }
                  />
                  <span className='text-muted-foreground text-xs'>$</span>
                </div>
                <Switch
                  checked={row.enabled}
                  onCheckedChange={(checked) =>
                    updateRow(index, { enabled: checked })
                  }
                  aria-label={t('Enabled')}
                />
                <Button
                  type='button'
                  size='icon'
                  variant='ghost'
                  className='h-8 w-8'
                  aria-label={t('Delete')}
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
          {t('Add image tier model')}
        </Button>
        <Button
          type='button'
          size='sm'
          disabled={saving || rows.length === 0}
          onClick={handleSave}
        >
          {saving ? t('Saving...') : t('Save')}
        </Button>
      </div>
    </div>
  )
}
