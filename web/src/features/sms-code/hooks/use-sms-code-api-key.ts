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
import { useCallback, useEffect, useMemo, useState } from 'react'

import type { ApiKey } from '@/features/keys/types'

import { listSmsCodeKeys, resolveSmsCodeKey } from '../api'

const SELECTED_KEY_STORAGE = 'sms-code-api-key-id'

export interface SmsApiKeyState {
  keys: ApiKey[]
  keysLoading: boolean
  selectedId: number | null
  apiKey: string
  keyLoading: boolean
  error: unknown
  selectKey: (id: number) => void
}

function readStoredKeyId(): number | null {
  if (typeof window === 'undefined') return null
  const stored = window.localStorage.getItem(SELECTED_KEY_STORAGE)
  if (stored === null) return null
  const parsed = Number(stored)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

/**
 * Resolve which API key the SMS tool calls with. Plugin routes are token
 * authenticated, so the page needs a real `sk-` key: prefer the remembered
 * choice, otherwise the first enabled key, and keep the page usable while the
 * key is still loading.
 */
export function useSmsCodeApiKey(): SmsApiKeyState {
  const keysQuery = useQuery({
    queryKey: ['sms-code-api-keys'],
    queryFn: listSmsCodeKeys,
    staleTime: 60 * 1000,
  })
  const keys = useMemo(() => keysQuery.data ?? [], [keysQuery.data])
  const [selectedId, setSelectedId] = useState<number | null>(readStoredKeyId)

  useEffect(() => {
    if (keys.length === 0) return
    if (selectedId !== null && keys.some((item) => item.id === selectedId)) {
      return
    }
    const fallback = keys[0]
    setSelectedId(fallback.id)
    window.localStorage.setItem(SELECTED_KEY_STORAGE, String(fallback.id))
  }, [keys, selectedId])

  const selectKey = useCallback((id: number) => {
    setSelectedId(id)
    window.localStorage.setItem(SELECTED_KEY_STORAGE, String(id))
  }, [])

  const keyQuery = useQuery({
    queryKey: ['sms-code-api-key', selectedId],
    queryFn: () => resolveSmsCodeKey(selectedId ?? 0),
    enabled: selectedId !== null,
    staleTime: 5 * 60 * 1000,
  })

  return {
    keys,
    keysLoading: keysQuery.isLoading,
    selectedId,
    apiKey: keyQuery.data ?? '',
    keyLoading: selectedId !== null && keyQuery.isLoading,
    error: keysQuery.error ?? keyQuery.error,
    selectKey,
  }
}
