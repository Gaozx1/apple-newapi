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
import { INTERFACE_LANGUAGE_OPTIONS } from '@/i18n/languages'

/** Languages a channel name can be translated into (interface codes). */
export const CHANNEL_NAME_LANGUAGES = INTERFACE_LANGUAGE_OPTIONS.map(
  (language) => language.code
)

type ChannelNameTranslations = Record<string, string>

/**
 * Parse the stored per-language names. A malformed value yields no overrides so
 * the base name stays visible instead of showing raw JSON.
 */
export function parseChannelNameTranslations(
  value: string | null | undefined
): ChannelNameTranslations {
  const raw = (value ?? '').trim()
  if (raw === '' || !raw.startsWith('{')) return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (
      parsed === null ||
      typeof parsed !== 'object' ||
      Array.isArray(parsed)
    ) {
      return {}
    }
    const translations: ChannelNameTranslations = {}
    for (const [code, name] of Object.entries(
      parsed as Record<string, unknown>
    )) {
      if (typeof name === 'string' && name.trim() !== '') {
        translations[code] = name.trim()
      }
    }
    return translations
  } catch {
    return {}
  }
}

/**
 * Resolve the display name for the active interface language, falling back to
 * the channel's base name when that language has no translation.
 */
export function resolveChannelName(
  name: string,
  nameI18n: string | null | undefined,
  language: string
): string {
  const translations = parseChannelNameTranslations(nameI18n)
  return translations[language] ?? name
}

/** Serialize translations for storage, dropping blank entries. */
export function stringifyChannelNameTranslations(
  translations: ChannelNameTranslations
): string {
  const cleaned: ChannelNameTranslations = {}
  for (const [code, name] of Object.entries(translations)) {
    if (name.trim() !== '') cleaned[code] = name.trim()
  }
  return Object.keys(cleaned).length === 0 ? '' : JSON.stringify(cleaned)
}
