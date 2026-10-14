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
  GetJailbreakRecordsParams,
  GetJailbreakRecordsResponse,
} from './types'

// Get paginated anti-jailbreak verdicts (admin only).
export async function getJailbreakRecords(
  params: GetJailbreakRecordsParams = {}
): Promise<GetJailbreakRecordsResponse> {
  const { p = 1, page_size = 20, keyword = '', user_id, channel_id } = params
  const query = new URLSearchParams()
  query.set('p', String(p))
  query.set('page_size', String(page_size))
  if (keyword.trim()) query.set('keyword', keyword.trim())
  if (user_id) query.set('user_id', String(user_id))
  if (channel_id) query.set('channel_id', String(channel_id))
  const res = await api.get(`/api/jailbreak/records?${query.toString()}`)
  return res.data
}
