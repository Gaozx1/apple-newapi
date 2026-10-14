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
// Display helpers for the anti-jailbreak records: the stored content is free
// text from the user, so table cells need a single-line preview while the
// detail dialog shows it verbatim.

export type JailbreakActionVariant = 'danger' | 'warning' | 'neutral'

/** i18n key of the action the guard took. */
export function actionLabelKey(action: string): string {
  switch (action) {
    case 'banned':
      return 'Account banned'
    case 'already_banned':
      return 'Account already banned'
    case 'skipped_staff':
      return 'Administrator, not banned'
    default:
      return 'Recorded'
  }
}

/** statusBadge variant for the action the guard took. */
export function jailbreakActionVariant(action: string): JailbreakActionVariant {
  switch (action) {
    case 'banned':
      return 'danger'
    case 'already_banned':
      return 'warning'
    default:
      return 'neutral'
  }
}

/**
 * Collapses whitespace and shortens the user's message for a table cell. The
 * full text stays available in the record and in the detail dialog.
 */
export function contentPreview(content: string, limit = 160): string {
  const collapsed = content.replaceAll(/\s+/g, ' ').trim()
  if (collapsed.length <= limit) return collapsed
  return `${collapsed.slice(0, limit)}…`
}
