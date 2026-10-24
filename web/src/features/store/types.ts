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
/** One product a user published for sale. */
export interface StoreProduct {
  id: number
  user_id: number
  username: string
  name: string
  description: string
  /** Price in quota units; `price_amount` is the same price in USD. */
  price: number
  price_amount: number
  status: number
  stock: number
  cdk_count: number
  sold_count: number
  is_owner: boolean
  created_at: number
}

/** One delivered purchase, including the code the buyer paid for. */
export interface StoreOrder {
  id: number
  order_no: string
  product_id: number
  product_name: string
  seller_user_id: number
  buyer_user_id: number
  price: number
  price_amount: number
  fee: number
  fee_amount: number
  seller_income: number
  income_amount: number
  code: string
  created_at: number
}

/** Platform commission and import limits, published by the backend. */
export interface StoreMeta {
  permille: number
  rate: number
  max_codes_per_import: number
  max_price_amount: number
}

export interface StorePage<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

export interface StoreProductInput {
  name: string
  description: string
  price_amount: number
  status?: number
}

/** One user-uploaded upstream offered in the store. */
export interface StoreChannelOffer {
  id: number
  user_id: number
  username: string
  name: string
  description: string
  channel_type: number
  base_url: string
  models: string
  ratio: number
  price: number
  channel_id: number
  group_name: string
  status: number
  sold_count: number
  error_count: number
  request_count: number
  is_owner: boolean
  created_at: number
}

export interface StoreChannelInput {
  name: string
  description: string
  channel_type: number
  base_url: string
  key: string
  models: string
  group_name: string
  ratio: number
  status?: number
}

export interface StoreChannelPurchase {
  order: {
    order_no: string
    offer_id: number
    channel_id: number
    price: number
    ratio: number
    fee: number
    fee_amount: number
    seller_income: number
    created_at: number
  }
  channel: { id: number; name: string; group: string; models: string }
  group: string
}

export interface StoreChannelReport {
  id: number
  offer_id: number
  channel_id: number
  reporter_id: number
  reason: string
  status: number
  review_note: string
  created_at: number
  reviewed_at: number
}
