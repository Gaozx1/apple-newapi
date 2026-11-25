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
import i18n, { type BackendModule } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import { convertDetectedLanguage } from './languages'
import en from './locales/en.json'
// The default dictionary is available synchronously. i18next caches each
// additional dictionary after the first switch or a saved locale is restored.
const localeLoaders = new Map<
  string,
  () => Promise<{ default: { translation: Record<string, string> } }>
>([
  ['fr', () => import(/* webpackChunkName: "locale-fr" */ './locales/fr.json')],
  ['ru', () => import(/* webpackChunkName: "locale-ru" */ './locales/ru.json')],
  ['ja', () => import(/* webpackChunkName: "locale-ja" */ './locales/ja.json')],
  ['vi', () => import(/* webpackChunkName: "locale-vi" */ './locales/vi.json')],
])

const localeBackend: BackendModule = {
  type: 'backend',
  init() {},
  read(language, namespace) {
    const load = localeLoaders.get(language)
    if (namespace !== 'translation' || !load) return {}
    return load().then(({ default: dictionary }) => dictionary.translation)
  },
}

i18n
  .use(localeBackend)
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { en },
    partialBundledLanguages: true,
    initAsync: false,
    fallbackLng: 'en',
    supportedLngs: ['en', 'fr', 'ru', 'ja', 'vi'],
    load: 'currentOnly',
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage'],
      caches: ['localStorage'],
      convertDetectedLanguage,
    },
  })

export default i18n
