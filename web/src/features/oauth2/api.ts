import type { ApiResponse } from '@/features/users/types'
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

// ============================================================================
// OAuth 2.0 Call Packages (次数包)
// ============================================================================

export interface OAuthCallPlan {
  id: number
  name: string
  call_count: number
  price_amount: number
  validity_days: number
  enabled: boolean
  sort_order: number
}

export interface OAuthCallGrant {
  id: number
  user_id: number
  plan_id: number
  plan_name: string
  calls_total: number
  calls_used: number
  expires_at: number
  status: string
  source: string
  created_at: number
}

export interface OAuthCallPlansResponse {
  plans: OAuthCallPlan[]
  balance: number
  grants: OAuthCallGrant[]
}

export async function getOAuthCallPlans(): Promise<
  ApiResponse<OAuthCallPlansResponse>
> {
  const res = await api.get('/api/user/oauth2/call-plans')
  return res.data
}

export async function purchaseOAuthCallPlan(
  planId: number
): Promise<ApiResponse<OAuthCallGrant>> {
  const res = await api.post('/api/user/oauth2/call-plans/purchase', {
    plan_id: planId,
  })
  return res.data
}

// ============================================================================
// Admin: call plan management
// ============================================================================

export async function adminListOAuthCallPlans(): Promise<
  ApiResponse<OAuthCallPlan[]>
> {
  const res = await api.get('/api/oauth2/call-plan/')
  return res.data
}

export async function adminCreateOAuthCallPlan(
  plan: Partial<OAuthCallPlan>
): Promise<ApiResponse<OAuthCallPlan>> {
  const res = await api.post('/api/oauth2/call-plan/', { plan })
  return res.data
}

export async function adminUpdateOAuthCallPlan(
  id: number,
  plan: Partial<OAuthCallPlan>
): Promise<ApiResponse> {
  const res = await api.put(`/api/oauth2/call-plan/${id}`, { plan })
  return res.data
}

export async function adminDeleteOAuthCallPlan(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/oauth2/call-plan/${id}`)
  return res.data
}
