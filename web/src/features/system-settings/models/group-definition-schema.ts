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
import { z } from 'zod'

import type { GroupDefinition } from './group-management-api'

/** Matches the varchar(64) the group reference columns use. */
export const GROUP_CODE_MAX_LENGTH = 64

/**
 * A code is an opaque identifier, not a slug: existing deployments use names
 * such as `ccmax0.9`, `接码` or `Gemini Ultra 0.3`, and rejecting them would leave
 * those channels unroutable. Only the storage format constrains it — it travels
 * inside a comma separated list, so a comma is impossible, and surrounding
 * whitespace would be trimmed away and break the round trip.
 */
export function groupCodeMessage(code: string): string | null {
  if (code.trim() === '') {
    return 'Group code is required'
  }
  if (code !== code.trim()) {
    return 'Group code must not start or end with whitespace'
  }
  if (code.includes(',')) {
    return 'Group code must not contain a comma'
  }
  if ([...code].length > GROUP_CODE_MAX_LENGTH) {
    return 'Group code must be at most 64 characters'
  }
  return null
}

export const groupDefinitionSchema = z
  .object({
    code: z
      .string()
      .superRefine((value, context) => {
        const message = groupCodeMessage(value)
        if (message) {
          context.addIssue({ code: 'custom', message })
        }
      }),
    names: z.record(z.string(), z.string()),
    ratio: z.number().min(0, { message: 'Ratio must not be less than 0' }),
    region: z.enum(['', 'domestic', 'overseas']),
    enabled: z.boolean(),
    sort: z.number().int(),
  })
  .refine(
    (value) => Object.values(value.names).some((name) => name.trim() !== ''),
    { path: ['names'], message: 'At least one display name is required' }
  )

export type GroupDefinitionFormValues = z.infer<typeof groupDefinitionSchema>

export function toGroupDefinitionFormValues(
  group: GroupDefinition | null,
  languages: string[]
): GroupDefinitionFormValues {
  const names: Record<string, string> = {}
  for (const language of languages) {
    names[language] = group?.names?.[language] ?? ''
  }
  return {
    code: group?.code ?? '',
    names,
    ratio: group?.ratio ?? 1,
    region:
      group?.region === 'domestic' || group?.region === 'overseas'
        ? group.region
        : '',
    enabled: group?.enabled ?? true,
    sort: group?.sort ?? 0,
  }
}

/** Drops blank translations so the server never stores an empty display name. */
export function toGroupMutationPayload(names: Record<string, string>) {
  const trimmed: Record<string, string> = {}
  for (const [language, name] of Object.entries(names)) {
    const value = name.trim()
    if (value !== '') {
      trimmed[language] = value
    }
  }
  return trimmed
}
