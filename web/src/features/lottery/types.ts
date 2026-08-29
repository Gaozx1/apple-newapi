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
// ============================================================================
// Lottery Type Definitions
// ============================================================================

/**
 * Generic API response envelope used across the app.
 */
export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

/**
 * Lottery prize type returned by the backend.
 */
export type LotteryPrizeType = 'balance' | 'coupon' | 'subscription' | 'thanks'

/**
 * A single prize tier as presented to the user.
 */
export interface LotteryTier {
  /** Prize type */
  type: LotteryPrizeType
  /** Human-readable prize label (i18n key on frontend) */
  label: string
  /** Probability weight (percentage) */
  weight: number
  /** Balance prize amount in quota (0 for non-balance) */
  quota?: number
  /** Coupon rebate percentage (0 for non-coupon) */
  rebate_rate?: number
  /** Subscription plan id for subscription prizes */
  plan_id?: number
}

/**
 * Admin-editable lottery prize row.
 */
export interface LotteryPrize {
  id: number
  type: LotteryPrizeType
  label: string
  weight: number
  quota: number
  rebate_rate: number
  plan_id: number
  enabled: boolean
  sort_order: number
}

/**
 * A lottery draw history record.
 */
export interface LotteryRecord {
  id: number
  user_id: number
  prize_type: LotteryPrizeType
  prize_label: string
  prize_value: number
  rebate_rate: number
  prize_plan_id?: number
  cost_quota: number
  created_at: number
}

/**
 * Lottery status response.
 */
export interface LotteryStatusResponse {
  /** Whether the lottery feature is enabled */
  enabled: boolean
  /** Quota cost per draw without a free chance */
  cost_quota: number
  /** Number of free draw chances */
  chances: number
  /** Number of unused recharge coupons */
  coupon_count: number
  /** Recent draw history */
  records: LotteryRecord[]
  /** Active prize table */
  prizes: LotteryTier[]
}

/**
 * Lottery draw result.
 */
export interface LotteryDrawResult {
  prize_type: LotteryPrizeType
  prize_label: string
  prize_value: number
  rebate_rate: number
  prize_plan_id?: number
  cost_quota: number
  free_draw: boolean
}

/**
 * Admin grant lottery chances request.
 */
export interface GrantLotteryChancesRequest {
  user_id: number
  count: number
}
