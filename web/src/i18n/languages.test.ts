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

import { convertDetectedLanguage } from './languages'

describe('convertDetectedLanguage', () => {
  test.each(['zhCN', 'zhTW', 'zh-CN', 'zh-Hant-TW', 'zh-HK', 'zh'])(
    '%s falls back to English',
    (code) => {
      expect(convertDetectedLanguage(code)).toBe('en')
    }
  )

  test('keeps supported stored languages and normalizes regional tags', () => {
    expect(convertDetectedLanguage('en')).toBe('en')
    expect(convertDetectedLanguage('fr-FR')).toBe('fr')
    expect(convertDetectedLanguage('ja')).toBe('ja')
  })
})

test('loads only the selected translation and keeps English available when switching back', async () => {
  const { default: i18n } = await import('./config')
  await i18n.changeLanguage('en')
  expect(i18n.t('footer.columns.docs.links.quickStart')).toBe('Quick Start')
  expect(i18n.hasResourceBundle('fr', 'translation')).toBe(false)
  await i18n.changeLanguage('fr')
  expect(i18n.t('Home')).toBe('Accueil')
  expect(i18n.hasResourceBundle('ja', 'translation')).toBe(false)
  await i18n.changeLanguage('en')
  expect(i18n.t('Home')).toBe('Home')
  expect(i18n.hasResourceBundle('fr', 'translation')).toBe(true)
})
