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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { SmsApiKeyPicker } from './components/sms-api-key-picker'
import { SmsMessagePanel } from './components/sms-message-panel'
import { SmsPhonePanel } from './components/sms-phone-panel'
import { SmsSendPanel } from './components/sms-send-panel'
import { useSmsCodeApiKey } from './hooks/use-sms-code-api-key'

/**
 * Receive SMS verification codes with the d1jiema models this gateway serves:
 * take a number, read the message it receives, and send one back. The number a
 * taking call returns is shared with the other two panels.
 */
export function SmsCode() {
  const { t } = useTranslation()
  const keyState = useSmsCodeApiKey()
  const [takenNumber, setTakenNumber] = useState('')
  const [takenKeyWord, setTakenKeyWord] = useState('')

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('SMS Codes')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-6'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Take a number, read its verification code and send a message through the d1jiema models of this gateway.'
            )}
          </p>

          <SmsApiKeyPicker state={keyState} />

          <div className='grid gap-6 lg:grid-cols-2'>
            <SmsPhonePanel
              apiKey={keyState.apiKey}
              keyLoading={keyState.keyLoading}
              onTaken={(phone, keyWord) => {
                setTakenNumber(phone)
                setTakenKeyWord(keyWord)
              }}
            />
            <SmsMessagePanel
              apiKey={keyState.apiKey}
              keyLoading={keyState.keyLoading}
              takenNumber={takenNumber}
              takenKeyWord={takenKeyWord}
            />
          </div>

          <SmsSendPanel
            apiKey={keyState.apiKey}
            keyLoading={keyState.keyLoading}
            takenNumber={takenNumber}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
