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
import { useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

export interface AdItem {
  id: number
  title: string
  image_url: string
  link_url: string
  html: string
  placement: 'reward' | 'sponsor' | string
  weight: number
  status: number
}

export interface AdConfig {
  enabled: boolean
  reward_quota: number
  reward_amount: number
  max_daily_views: number
  views_today: number
  remaining_today: number
}

export interface AdNextResponse {
  ad: AdItem | null
  reason?: string
}

export interface AdViewResult {
  reward_quota: number
  reward_amount: number
  views_today: number
}

export async function getAdConfig(): Promise<AdConfig> {
  const response = await api.get('/api/store/ads/config')
  return requireServerSuccess(response.data).data as AdConfig
}

export async function getNextAd(
  placement: 'reward' | 'sponsor'
): Promise<AdNextResponse> {
  const response = await api.get('/api/store/ads/next', {
    params: { placement },
  })
  return requireServerSuccess(response.data).data as AdNextResponse
}

export async function recordAdView(
  adId: number,
  placement: 'reward' | 'sponsor'
): Promise<AdViewResult> {
  const response = await api.post(`/api/store/ads/${adId}/view`, null, {
    params: { placement },
  })
  return requireServerSuccess(response.data).data as AdViewResult
}

/** Random sponsor impression for a content page; hidden when ads are off. */
export function useSponsorAd() {
  return useQuery({
    queryKey: ['ads', 'sponsor'],
    queryFn: async () => {
      const config = await getAdConfig()
      if (!config.enabled || config.remaining_today <= 0) return null
      const next = await getNextAd('sponsor')
      return next.ad
    },
    staleTime: 5 * 60 * 1000,
  })
}
