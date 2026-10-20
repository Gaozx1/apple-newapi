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
import { t } from 'i18next'

import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  StoreChannelInput,
  StoreChannelOffer,
  StoreChannelReport,
  StoreMeta,
  StoreOrder,
  StorePage,
  StoreProduct,
  StoreProductInput,
} from './types'

/** Read the `data` payload of one management response. */
function storePayload<T>(payload: unknown): T {
  const body = requireServerSuccess(payload) as { data?: T } | undefined
  if (body?.data === undefined) {
    throw new Error(t('Something went wrong!'))
  }
  return body.data
}

export async function getStoreMeta(): Promise<StoreMeta> {
  const response = await api.get('/api/store/meta')
  return storePayload<StoreMeta>(response.data)
}

export async function listStoreProducts(
  keyword: string,
  page: number,
  pageSize: number
): Promise<StorePage<StoreProduct>> {
  const response = await api.get('/api/store/products', {
    params: { keyword, p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreProduct>>(response.data)
}

export async function createStoreProduct(
  input: StoreProductInput
): Promise<StoreProduct> {
  const response = await api.post('/api/store/products', input)
  return storePayload<StoreProduct>(response.data)
}

export async function updateStoreProduct(
  id: number,
  input: StoreProductInput
): Promise<StoreProduct> {
  const response = await api.put(`/api/store/products/${id}`, input)
  return storePayload<StoreProduct>(response.data)
}

export async function deleteStoreProduct(id: number): Promise<void> {
  const response = await api.delete(`/api/store/products/${id}`)
  requireServerSuccess(response.data)
}

export async function importStoreCodes(
  id: number,
  codes: string
): Promise<{ imported: number; duplicated: number; product: StoreProduct }> {
  const response = await api.post(`/api/store/products/${id}/cdks`, { codes })
  return storePayload(response.data)
}

export async function purchaseStoreProduct(id: number): Promise<StoreOrder> {
  const response = await api.post(`/api/store/products/${id}/buy`, {})
  return storePayload<{ order: StoreOrder }>(response.data).order
}

export async function listMyStoreProducts(
  page: number,
  pageSize: number
): Promise<StorePage<StoreProduct>> {
  const response = await api.get('/api/store/mine/products', {
    params: { p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreProduct>>(response.data)
}

export async function createAfterSalesTicket(
  orderId: number,
  reason: string
): Promise<StoreTicket> {
  return createStoreTicket({ order_id: orderId, reason })
}

export async function createGroupAfterSalesTicket(
  groupId: number,
  reason: string
): Promise<StoreTicket> {
  return createStoreTicket({ group_id: groupId, reason })
}

export async function listMyStoreOrders(
  page: number,
  pageSize: number
): Promise<StorePage<StoreOrder>> {
  const response = await api.get('/api/store/mine/orders', {
    params: { p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreOrder>>(response.data)
}

export async function listMyStoreSales(
  page: number,
  pageSize: number
): Promise<StorePage<StoreOrder>> {
  const response = await api.get('/api/store/mine/sales', {
    params: { p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreOrder>>(response.data)
}

export interface StoreEarning {
  id: number
  created_at: number
  buyer_user_id: number
  buyer_name: string
  group_name: string
  channel_name: string
  model_name: string
  quota: number
  quota_amount: number
  fee: number
  fee_amount: number
  income: number
  income_amount: number
  ratio: number
  request_id: string
}

export interface StoreEarningSummary {
  orders: number
  consumed: number
  consumed_amount: number
  fee: number
  fee_amount: number
  income: number
  income_amount: number
  unique_buyers: number
}

export async function listMyStoreEarnings(
  page: number,
  pageSize: number
): Promise<{
  page: StorePage<StoreEarning>
  summary: StoreEarningSummary
}> {
  const response = await api.get('/api/store/mine/earnings', {
    params: { p: page, page_size: pageSize },
  })
  return storePayload<{
    page: StorePage<StoreEarning>
    summary: StoreEarningSummary
  }>(response.data)
}

export async function listStoreChannels(
  keyword: string,
  page: number,
  pageSize: number
): Promise<StorePage<StoreChannelOffer>> {
  const response = await api.get('/api/store/channels', {
    params: { keyword, p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreChannelOffer>>(response.data)
}

export async function listMyStoreChannels(
  page: number,
  pageSize: number
): Promise<StorePage<StoreChannelOffer>> {
  const response = await api.get('/api/store/my-channels', {
    params: { p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreChannelOffer>>(response.data)
}

export interface StoreTicket {
  id: number
  order_id: number
  group_id: number
  product_name: string
  buyer_user_id: number
  buyer_name: string
  seller_user_id: number
  seller_name: string
  reason: string
  complaint_reason: string
  status: number
  refunded_quota: number
  refunded_amount: number
  resolved_note: string
  created_at: number
  updated_at: number
}

export interface StoreTicketMessage {
  id: number
  sender_id: number
  sender_name: string
  sender_role: number
  body: string
  created_at: number
}

export async function createStoreTicket(input: {
  order_id?: number
  group_id?: number
  reason: string
}): Promise<StoreTicket> {
  const response = await api.post('/api/store/tickets', input)
  return storePayload<StoreTicket>(response.data)
}

export async function listMyStoreTickets(): Promise<StoreTicket[]> {
  const response = await api.get('/api/store/tickets/mine')
  return storePayload<{ items: StoreTicket[] }>(response.data)?.items ?? []
}

export async function listStoreSalesTickets(): Promise<StoreTicket[]> {
  const response = await api.get('/api/store/tickets/sales')
  return storePayload<{ items: StoreTicket[] }>(response.data)?.items ?? []
}

export async function listAdminComplaints(): Promise<StoreTicket[]> {
  const response = await api.get('/api/store/admin/tickets')
  return storePayload<{ items: StoreTicket[] }>(response.data)?.items ?? []
}

export async function getStoreTicket(id: number): Promise<{
  ticket: StoreTicket
  messages: StoreTicketMessage[]
}> {
  const response = await api.get(`/api/store/tickets/${id}`)
  return storePayload<{ ticket: StoreTicket; messages: StoreTicketMessage[] }>(
    response.data
  )
}

export async function addStoreTicketMessage(
  id: number,
  body: string
): Promise<void> {
  const response = await api.post(`/api/store/tickets/${id}/messages`, { body })
  requireServerSuccess(response.data)
}

export async function refundStoreTicket(id: number): Promise<void> {
  const response = await api.post(`/api/store/tickets/${id}/refund`, {})
  requireServerSuccess(response.data)
}

export async function complainStoreTicket(
  id: number,
  reason: string
): Promise<void> {
  const response = await api.post(`/api/store/tickets/${id}/complain`, {
    reason,
  })
  requireServerSuccess(response.data)
}

export async function resolveStoreTicket(
  id: number,
  note: string
): Promise<void> {
  const response = await api.post(`/api/store/admin/tickets/${id}/resolve`, {
    note,
  })
  requireServerSuccess(response.data)
}

export interface MarketGroup {
  id: number
  user_id: number
  username: string
  name: string
  public_name: string
  ratio: number
  remark: string
  listed: boolean
  created_at: number
  is_owner: boolean
  claimed: boolean
}

export async function listMarketGroups(): Promise<MarketGroup[]> {
  const response = await api.get('/api/store/market')
  return storePayload<{ items: MarketGroup[] }>(response.data)?.items ?? []
}

export async function claimStoreGroup(groupId: number): Promise<void> {
  const response = await api.post('/api/store/claims', { group_id: groupId })
  requireServerSuccess(response.data)
}

export async function unclaimStoreGroup(groupId: number): Promise<void> {
  const response = await api.delete('/api/store/claims', {
    data: { group_id: groupId },
  })
  requireServerSuccess(response.data)
}

export async function reportStoreGroup(
  groupId: number,
  reason: string
): Promise<void> {
  const response = await api.post('/api/store/group-reports', {
    group_id: groupId,
    reason,
  })
  requireServerSuccess(response.data)
}

export async function fetchStoreChannelModels(input: {
  channel_type: number
  base_url: string
  key: string
}): Promise<string[]> {
  const response = await api.post('/api/store/fetch-models', input)
  return storePayload<{ models: string[] }>(response.data)?.models ?? []
}

export interface StoreGroup {
  id: number
  user_id: number
  name: string
  ratio: number
  remark: string
  listed: boolean
  created_at: number
}

export async function listStoreGroups(): Promise<{
  items: StoreGroup[]
  default_group: string
}> {
  const response = await api.get('/api/store/groups')
  return storePayload<{ items: StoreGroup[]; default_group: string }>(
    response.data
  )
}

export async function createStoreGroup(input: {
  name: string
  ratio: number
  remark: string
}): Promise<StoreGroup> {
  const response = await api.post('/api/store/groups', input)
  return storePayload<StoreGroup>(response.data)
}

export async function updateStoreGroup(
  id: number,
  input: { ratio: number; remark: string }
): Promise<void> {
  const response = await api.put(`/api/store/groups/${id}`, input)
  requireServerSuccess(response.data)
}

export async function setStoreGroupListing(
  id: number,
  listed: boolean
): Promise<void> {
  const response = await api.post(`/api/store/groups/${id}/listing`, { listed })
  requireServerSuccess(response.data)
}

export async function deleteStoreGroup(id: number): Promise<void> {
  const response = await api.delete(`/api/store/groups/${id}`)
  requireServerSuccess(response.data)
}

export async function createStoreChannel(
  input: StoreChannelInput
): Promise<StoreChannelOffer> {
  const response = await api.post('/api/store/channels', input)
  return storePayload<StoreChannelOffer>(response.data)
}

export async function updateStoreChannel(
  id: number,
  input: StoreChannelInput
): Promise<StoreChannelOffer> {
  const response = await api.put(`/api/store/channels/${id}`, input)
  return storePayload<StoreChannelOffer>(response.data)
}

export async function deleteStoreChannel(id: number): Promise<void> {
  const response = await api.delete(`/api/store/channels/${id}`)
  requireServerSuccess(response.data)
}

export async function setStoreChannelListing(
  id: number,
  listed: boolean
): Promise<StoreChannelOffer> {
  const response = await api.post(`/api/store/channels/${id}/listing`, {
    listed,
  })
  return storePayload<StoreChannelOffer>(response.data)
}

export async function reportStoreChannel(
  offerId: number,
  channelId: number,
  reason: string
): Promise<void> {
  const response = await api.post('/api/store/reports', {
    offer_id: offerId,
    channel_id: channelId,
    reason,
  })
  requireServerSuccess(response.data)
}

export async function listStoreReports(
  status: number,
  page: number,
  pageSize: number
): Promise<StorePage<StoreChannelReport>> {
  const response = await api.get('/api/store/admin/reports', {
    params: { status, p: page, page_size: pageSize },
  })
  return storePayload<StorePage<StoreChannelReport>>(response.data)
}

export async function reviewStoreReport(
  id: number,
  accept: boolean,
  note: string
): Promise<void> {
  const response = await api.post(`/api/store/admin/reports/${id}/review`, {
    accept,
    note,
  })
  requireServerSuccess(response.data)
}
