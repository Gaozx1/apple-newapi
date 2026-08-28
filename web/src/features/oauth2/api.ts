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

import type { ApiResponse } from '@/features/users/types'

export interface OAuthClient {
  id: number
  client_id: string
  client_secret?: string
  name: string
  redirect_uris: string
  scopes: string
  enabled: boolean
  user_id: number
  created_at: string
}

export type OAuthClientForm = {
  name: string
  redirect_uris: string
  scopes: string
}

export async function getOAuthClients(): Promise<ApiResponse<OAuthClient[]>> {
  const res = await api.get('/api/user/oauth2/clients')
  return res.data
}

export async function createOAuthClient(
  data: OAuthClientForm
): Promise<ApiResponse<OAuthClient>> {
  const res = await api.post('/api/user/oauth2/clients', data)
  return res.data
}

export async function updateOAuthClient(
  id: number,
  data: OAuthClientForm
): Promise<ApiResponse<OAuthClient>> {
  const res = await api.put(`/api/user/oauth2/clients/${id}`, data)
  return res.data
}

export async function deleteOAuthClient(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/oauth2/clients/${id}`)
  return res.data
}
