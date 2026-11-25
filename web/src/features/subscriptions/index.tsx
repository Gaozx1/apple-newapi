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
import { Info } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { ResetCardsSection } from './components/reset-cards-section'
import { SubscriptionsDialogs } from './components/subscriptions-dialogs'
import { SubscriptionsPrimaryButtons } from './components/subscriptions-primary-buttons'
import {
  SubscriptionsProvider,
  useSubscriptions,
} from './components/subscriptions-provider'
import { SubscriptionsTable } from './components/subscriptions-table'

const SUBSCRIPTION_TAB_PLANS = 'plans'
const SUBSCRIPTION_TAB_RESET_CARDS = 'reset-cards'

function SubscriptionsContent() {
  const { t } = useTranslation()
  const { complianceConfirmed } = useSubscriptions()
  const [tab, setTab] = useState(SUBSCRIPTION_TAB_PLANS)

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Subscription Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {tab === SUBSCRIPTION_TAB_PLANS ? (
            <div className='flex items-center gap-2'>
              <Alert variant='default' className='hidden px-3 py-2 sm:flex'>
                <Info className='h-4 w-4' />
                <AlertDescription className='text-xs'>
                  {t(
                    'Stripe/Creem requires creating products on the third-party platform and entering the ID'
                  )}
                </AlertDescription>
              </Alert>
              <SubscriptionsPrimaryButtons />
            </div>
          ) : null}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {!complianceConfirmed ? (
              <Alert variant='destructive' className='shrink-0'>
                <AlertDescription>
                  {t(
                    'Subscription plan creation and changes are locked until the administrator confirms compliance terms in Payment Gateway settings.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            <Tabs
              value={tab}
              onValueChange={setTab}
              className='flex h-full min-h-0 flex-col gap-3'
            >
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                <TabsTrigger value={SUBSCRIPTION_TAB_PLANS}>
                  {t('Subscription Plans')}
                </TabsTrigger>
                <TabsTrigger value={SUBSCRIPTION_TAB_RESET_CARDS}>
                  {t('Reset Cards')}
                </TabsTrigger>
              </TabsList>
              <TabsContent
                value={SUBSCRIPTION_TAB_PLANS}
                className='min-h-0 flex-1'
              >
                <div className='h-full min-h-0'>
                  <SubscriptionsTable />
                </div>
              </TabsContent>
              <TabsContent
                value={SUBSCRIPTION_TAB_RESET_CARDS}
                className='min-h-0 flex-1'
              >
                <div className='h-full min-h-0'>
                  <ResetCardsSection />
                </div>
              </TabsContent>
            </Tabs>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SubscriptionsDialogs />
    </>
  )
}

export function Subscriptions() {
  return (
    <SubscriptionsProvider>
      <SubscriptionsContent />
    </SubscriptionsProvider>
  )
}
