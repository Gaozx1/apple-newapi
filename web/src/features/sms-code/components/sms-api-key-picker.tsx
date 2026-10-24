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
import { Link } from '@tanstack/react-router'
import { KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { SmsApiKeyState } from '../hooks/use-sms-code-api-key'

/**
 * Plugin routes are token authenticated, so the page calls them with one of the
 * user's own API keys. The choice is remembered between visits.
 */
export function SmsApiKeyPicker(props: { state: SmsApiKeyState }) {
  const { t } = useTranslation()
  const state = props.state

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <KeyRound className='size-4' aria-hidden='true' />
          {t('API Key')}
        </CardTitle>
        <CardDescription>
          {t(
            'Calls on this page go through this key, so the gateway bills them to your account.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-2'>
        {state.keys.length === 0 && !state.keysLoading ? (
          <p className='text-muted-foreground text-sm'>
            {t('No enabled API key found.')}{' '}
            <Link
              to='/keys'
              className='text-primary underline-offset-4 hover:underline'
            >
              {t('Create one first')}
            </Link>
          </p>
        ) : (
          <>
            <Label htmlFor='sms-code-api-key'>{t('API Key')}</Label>
            <Select
              value={
                state.selectedId === null ? null : String(state.selectedId)
              }
              onValueChange={(value) => {
                if (value === null) return
                state.selectKey(Number(value))
              }}
            >
              <SelectTrigger
                id='sms-code-api-key'
                aria-label={t('API Key')}
                className='w-full sm:w-80'
              >
                <SelectValue placeholder={t('Select an API key')} />
              </SelectTrigger>
              <SelectContent>
                {state.keys.map((item) => (
                  <SelectItem key={item.id} value={String(item.id)}>
                    {item.name === '' ? `#${item.id}` : item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </>
        )}
        {state.error !== undefined && state.error !== null && (
          <p className='text-destructive text-xs'>
            {t('Failed to load API keys')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
