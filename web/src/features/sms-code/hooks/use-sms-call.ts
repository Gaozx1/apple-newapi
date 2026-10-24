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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getServerErrorMessage } from '@/lib/server-error-message'

import { callSmsModel } from '../api'
import type { SmsAction, SmsCallResult } from '../types'

export interface SmsCallState {
  busy: boolean
  result: SmsCallResult | null
  error: string | null
  run: (payload: Record<string, string>) => Promise<SmsCallResult | null>
  reset: () => void
}

/**
 * Run one d1jiema model. A vendor refusal resolves with `code: "error"` so the
 * panel can show the vendor's own wording, while a transport or gateway failure
 * rejects into `error`. `run` resolves to null only for that second case, which
 * is what a poller needs to know before asking again.
 */
export function useSmsCall(action: SmsAction, apiKey: string): SmsCallState {
  const { t } = useTranslation()
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<SmsCallResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const run = useCallback(
    async (payload: Record<string, string>): Promise<SmsCallResult | null> => {
      setBusy(true)
      setError(null)
      try {
        const next = await callSmsModel(action, payload, apiKey)
        setResult(next)
        return next
      } catch (caught) {
        setError(getServerErrorMessage(caught, t('Request failed')))
        return null
      } finally {
        setBusy(false)
      }
    },
    [action, apiKey, t]
  )

  const reset = useCallback(() => {
    setResult(null)
    setError(null)
  }, [])

  return { busy, result, error, run, reset }
}
