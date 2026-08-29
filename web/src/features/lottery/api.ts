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

import type {
  ApiResponse,
  GrantLotteryChancesRequest,
  LotteryDrawResult,
  LotteryPrize,
  LotteryStatusResponse,
} from './types'

// ============================================================================
// Lottery APIs
// ============================================================================

/**
 * Get the current user's lottery status: enabled flag, free chances, unused
 * coupons, and recent draw history.
 */
export async function getLotteryStatus(): Promise<
  ApiResponse<LotteryStatusResponse>
> {
  const res = await api.get('/api/user/lottery')
  return res.data
}

/**
 * Perform a single lottery draw.
 */
export async function drawLottery(
  turnstileToken?: string
): Promise<ApiResponse<LotteryDrawResult>> {
  const url = turnstileToken
    ? `/api/user/lottery/draw?turnstile=${encodeURIComponent(turnstileToken)}`
    : '/api/user/lottery/draw'
  const res = await api.post(url)
  return res.data
}

/**
 * Admin: grant lottery draw chances to a user.
 */
export async function grantLotteryChances(
  data: GrantLotteryChancesRequest
): Promise<ApiResponse> {
  const res = await api.post('/api/user/lottery/grant', data)
  return res.data
}

// ============================================================================
// Admin Lottery Prize APIs
// ============================================================================

/**
 * Admin: list all lottery prizes.
 */
export async function adminListLotteryPrizes(): Promise<
  ApiResponse<LotteryPrize[]>
> {
  const res = await api.get('/api/user/lottery/prizes')
  return res.data
}

/**
 * Admin: create a lottery prize.
 */
export async function adminCreateLotteryPrize(
  prize: Partial<LotteryPrize>
): Promise<ApiResponse<LotteryPrize>> {
  const res = await api.post('/api/user/lottery/prizes', { prize })
  return res.data
}

/**
 * Admin: update a lottery prize.
 */
export async function adminUpdateLotteryPrize(
  id: number,
  prize: Partial<LotteryPrize>
): Promise<ApiResponse> {
  const res = await api.put(`/api/user/lottery/prizes/${id}`, { prize })
  return res.data
}

/**
 * Admin: delete a lottery prize.
 */
export async function adminDeleteLotteryPrize(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/lottery/prizes/${id}`)
  return res.data
}
