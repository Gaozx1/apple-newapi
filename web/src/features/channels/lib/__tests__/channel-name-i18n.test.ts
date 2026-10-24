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
import { describe, expect, it } from 'vitest'

import {
  parseChannelNameTranslations,
  resolveChannelName,
  stringifyChannelNameTranslations,
} from '../channel-name-i18n'

describe('channel name translations', () => {
  it('falls back to the base name when the language has no translation', () => {
    const stored = JSON.stringify({ zhCN: '中文名', en: 'English name' })

    expect(resolveChannelName('base', stored, 'fr')).toBe('base')
    expect(resolveChannelName('base', stored, 'zhCN')).toBe('中文名')
  })

  it('drops blank and non-string entries instead of showing them', () => {
    const stored = JSON.stringify({ zhCN: '  ', en: 42, ja: '日本語' })

    expect(parseChannelNameTranslations(stored)).toEqual({ ja: '日本語' })
  })

  it('treats malformed stored JSON as no translations', () => {
    expect(parseChannelNameTranslations('{not json')).toEqual({})
    expect(parseChannelNameTranslations('')).toEqual({})
    expect(resolveChannelName('base', '{not json', 'en')).toBe('base')
  })

  it('serializes only filled languages and clears when all are empty', () => {
    expect(stringifyChannelNameTranslations({ zhCN: '中文', en: '  ' })).toBe(
      '{"zhCN":"中文"}'
    )
    expect(stringifyChannelNameTranslations({ zhCN: '   ' })).toBe('')
  })
})
