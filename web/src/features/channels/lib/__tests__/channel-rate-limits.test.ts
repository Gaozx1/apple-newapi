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
import { describe, expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToUpdatePayload,
} from '../channel-form'

const MAX_ADMISSION_LIMIT = 1000000

function channelWithSettings(settings: string) {
  return channelSchema.parse({
    id: 1,
    name: 'Limited upstream',
    type: 1,
    key: 'sk-test',
    status: 1,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'gpt-4o',
    group: 'default',
    base_url: 'https://upstream.example',
    settings,
  })
}

function formInput(overrides: Record<string, unknown>) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Limited upstream',
    type: 1,
    base_url: 'https://upstream.example',
    key: 'sk-test',
    models: 'gpt-4o',
    ...overrides,
  }
}

function formValues(overrides: Record<string, unknown>) {
  return channelFormSchema.parse(formInput(overrides))
}

describe('channel admission limits', () => {
  test('reads per-key limits from channel settings and defaults to unlimited', () => {
    const unlimited = transformChannelToFormDefaults(channelWithSettings('{}'))
    expect(unlimited.concurrency_limit).toBe(0)
    expect(unlimited.rpm_limit).toBe(0)

    const limited = transformChannelToFormDefaults(
      channelWithSettings(
        JSON.stringify({ concurrency_limit: 4, rpm_limit: 600 })
      )
    )
    expect(limited.concurrency_limit).toBe(4)
    expect(limited.rpm_limit).toBe(600)
  })

  test('writes positive limits and clears them when set back to zero', () => {
    const configured = transformFormDataToUpdatePayload(
      formValues({ concurrency_limit: 4, rpm_limit: 600 }),
      1
    )
    expect(JSON.parse(configured.settings ?? '{}')).toMatchObject({
      concurrency_limit: 4,
      rpm_limit: 600,
    })

    const cleared = transformFormDataToUpdatePayload(
      formValues({
        concurrency_limit: 0,
        rpm_limit: 0,
        settings: JSON.stringify({ concurrency_limit: 4, rpm_limit: 600 }),
      }),
      1
    )
    const clearedSettings = JSON.parse(cleared.settings ?? '{}')
    expect(clearedSettings).not.toHaveProperty('concurrency_limit')
    expect(clearedSettings).not.toHaveProperty('rpm_limit')
  })

  test('rejects limits outside the backend range and accepts the bound', () => {
    expect(
      channelFormSchema.safeParse(formInput({ concurrency_limit: -1 })).success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse(
        formInput({ rpm_limit: MAX_ADMISSION_LIMIT + 1 })
      ).success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse(
        formInput({
          concurrency_limit: MAX_ADMISSION_LIMIT,
          rpm_limit: MAX_ADMISSION_LIMIT,
        })
      ).success
    ).toBe(true)
  })
})
