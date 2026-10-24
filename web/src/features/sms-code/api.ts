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
import axios from 'axios'
import { t } from 'i18next'

import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import type { ApiKey } from '@/features/keys/types'
import {
  createServerError,
  getServerErrorMessage,
} from '@/lib/server-error-message'

import type { SmsAction, SmsCallResult } from './types'

/**
 * The plugin's native routes authenticate with an API key, while the shared
 * client always rewrites Authorization with the browser session token. These
 * calls therefore use their own interceptor-free instance.
 */
const relayClient = axios.create({ withCredentials: false })

/** Enabled API keys of the signed-in user, newest page first. */
export async function listSmsCodeKeys(): Promise<ApiKey[]> {
  const result = await getApiKeys({ p: 1, size: 100 })
  if (!result.success) {
    throw createServerError(result, t('Failed to load API keys'))
  }
  const items = result.data?.items ?? []
  return items.filter((item) => item.status === API_KEY_STATUS.ENABLED)
}

/** Resolve the unmasked `sk-` key for one token. */
export async function resolveSmsCodeKey(id: number): Promise<string> {
  const result = await fetchTokenKey(id)
  if (!result.success || !result.data?.key) {
    throw createServerError(result, t('Failed to load API keys'))
  }
  return `sk-${result.data.key}`
}

/**
 * A reading that has not received the message yet is answered as an ordinary
 * failure carrying the vendor's own wording; the gateway refunds it, so the
 * poller keeps waiting without surfacing an error.
 */
const SMS_NOT_RECEIVED_MARKERS = ['尚未收到', '[未收到]']

function isNotReceivedAnswer(message: string): boolean {
  return SMS_NOT_RECEIVED_MARKERS.some((marker) => message.includes(marker))
}

/**
 * Call one d1jiema model through the gateway. A vendor refusal is a successful
 * HTTP response carrying `code: "error"`, so the caller checks `code`.
 */
export async function callSmsModel(
  action: SmsAction,
  payload: Record<string, string>,
  apiKey: string
): Promise<SmsCallResult> {
  try {
    const response = await relayClient.post(
      `/d1jiema/${action}`,
      { ...payload, model: action },
      {
        headers: {
          Authorization: `Bearer ${apiKey}`,
          'Content-Type': 'application/json',
        },
      }
    )
    const result = response.data as SmsCallResult
    if (
      result.code === 'error' &&
      isNotReceivedAnswer(`${result.reason ?? ''} ${result.message ?? ''}`)
    ) {
      return {
        ...result,
        code: 'pending',
        pending: true,
        message: result.reason ?? result.message,
      }
    }
    return result
  } catch (error) {
    const message = getServerErrorMessage(error, t('Request failed'))
    if (isNotReceivedAnswer(message)) {
      return {
        code: 'pending',
        operation: action,
        message,
        raw: message,
        pending: true,
      }
    }
    throw new Error(message, { cause: error })
  }
}
