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

/** Vendor card types; the plugin defaults to 全部 when the field is absent. */
export const SMS_CARD_TYPES = ['全部', '实卡', '虚卡'] as const

/** Vendor requests are billed per call, so polling stays deliberately slow. */
export const SMS_POLL_INTERVAL_MS = 5000

/** Poll results kept on screen. */
export const SMS_HISTORY_LIMIT = 5

export const smsPhoneFormSchema = z.object({
  keyWord: z.string(),
  phone: z.string(),
  province: z.string(),
  cardType: z.enum(SMS_CARD_TYPES),
})

export type SmsPhoneFormValues = z.infer<typeof smsPhoneFormSchema>

export const smsMessageFormSchema = z.object({
  phone: z.string().trim().min(1, 'Number is required'),
  keyWord: z.string().trim().min(1, 'SMS keyword is required'),
})

export type SmsMessageFormValues = z.infer<typeof smsMessageFormSchema>

export const smsSendFormSchema = z.object({
  phone: z.string().trim().min(1, 'Number is required'),
  toPhone: z.string().trim().min(1, 'Recipient is required'),
  content: z.string().trim().min(1, 'Content is required'),
  projId: z.string(),
})

export type SmsSendFormValues = z.infer<typeof smsSendFormSchema>

export const smsPhoneFormDefaults: SmsPhoneFormValues = {
  keyWord: '',
  phone: '',
  province: '',
  cardType: '全部',
}
