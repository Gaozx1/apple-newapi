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
import { api } from '@/lib/api'

/** Language tag to display name, e.g. `{ en: 'VIP', zh: '会员' }`. */
export type GroupDisplayNames = Record<string, string>

/**
 * A group registered on the server. `code` is the stable identifier channels and
 * keys reference; `names` holds one display name per language. Renaming a
 * display name never invalidates a reference, which is why editing a group must
 * go through the update endpoint instead of rewriting option JSON by hand.
 */
export type GroupDefinition = {
  id: number
  code: string
  names: GroupDisplayNames
  ratio: number
  region: string
  enabled: boolean
  sort: number
}

export type GroupDefinitionsPayload = {
  groups: GroupDefinition[]
  languages: string[]
}

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}

export type GroupMutationPayload = {
  code?: string
  names?: GroupDisplayNames
  ratio?: number
  region?: string
  enabled?: boolean
  sort?: number
}

export type GroupRenameResult = {
  channels: number
  tokens: number
  users: number
  abilities: number
}

export async function getGroupDefinitions() {
  const res = await api.get<ApiEnvelope<GroupDefinitionsPayload>>(
    '/api/group/definitions'
  )
  return res.data
}

export async function createGroupDefinition(payload: GroupMutationPayload) {
  const res = await api.post<ApiEnvelope<GroupDefinition>>(
    '/api/group/definitions',
    payload
  )
  return res.data
}

export async function updateGroupDefinition(
  id: number,
  payload: GroupMutationPayload
) {
  const res = await api.put<ApiEnvelope<GroupDefinition>>(
    `/api/group/definitions/${id}`,
    payload
  )
  return res.data
}

export async function renameGroupDefinition(id: number, code: string) {
  const res = await api.post<ApiEnvelope<GroupRenameResult>>(
    `/api/group/definitions/${id}/rename`,
    { code }
  )
  return res.data
}

export async function deleteGroupDefinition(id: number) {
  const res = await api.delete<ApiEnvelope<null>>(
    `/api/group/definitions/${id}`
  )
  return res.data
}
