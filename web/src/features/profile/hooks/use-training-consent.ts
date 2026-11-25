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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { authRequestOptions, authResult } from '@/lib/secure-verification'
import { useAuthStore } from '@/stores/auth-store'

type Choice = 'accepted' | 'declined'
type Consent = { decision: Choice | 'undecided' }
const endpoint = '/api/user/self/training-consent'

export function useTrainingConsent() {
  const sessionId = useAuthStore((state) => state.auth.session?.sid)
  const queryClient = useQueryClient()
  const queryKey = ['training-consent', sessionId] as const
  const consent = useQuery({
    queryKey,
    enabled: !!sessionId,
    queryFn: ({ signal }) =>
      authResult<Consent>(
        api.get(endpoint, {
          ...authRequestOptions,
          disableDuplicate: true,
          signal,
        })
      ),
  })
  const choose = useMutation({
    mutationFn: (decision: Choice) =>
      authResult<Consent>(api.put(endpoint, { decision }, authRequestOptions)),
    onMutate: () => queryClient.cancelQueries({ queryKey }),
    onSuccess: (value) => queryClient.setQueryData(queryKey, value),
  })
  return { consent, choose, signedIn: !!sessionId }
}
