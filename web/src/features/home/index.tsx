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
import { useCallback, useEffect, useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { useTheme } from '@/context/theme-provider'
import { isLikelyHtml } from '@/lib/content-format'
import { useAuthStore } from '@/stores/auth-store'

import { Hero } from './components/sections/hero'
import { HowItWorks } from './components/sections/how-it-works'
import { useHomePageContent } from './hooks'

export function Home() {
  const { i18n, t } = useTranslation()
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const { resolvedTheme } = useTheme()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()

  const syncIframePreferences = useCallback(() => {
    try {
      iframeRef.current?.contentWindow?.postMessage(
        { themeMode: resolvedTheme },
        '*'
      )
      iframeRef.current?.contentWindow?.postMessage(
        { lang: i18n.language },
        '*'
      )
    } catch {
      // Cross-origin frames may reject access while navigating.
    }
  }, [i18n.language, resolvedTheme])

  useEffect(() => {
    if (isUrl) {
      syncIframePreferences()
    }
  }, [isUrl, syncIframePreferences])

  const contentIsHtml = !!content && !isUrl && isLikelyHtml(content)
  const isMarkdown = !!content && !isUrl && !contentIsHtml

  let main: ReactNode
  if (!isLoaded) {
    main = (
      <div className='flex min-h-screen items-center justify-center'>
        <div className='text-muted-foreground'>{t('Loading...')}</div>
      </div>
    )
  } else if (isUrl) {
    // User activation allows links from the configured page to open in the
    // top-level window without granting the page same-origin access.
    main = (
      <iframe
        ref={iframeRef}
        src={content}
        className='h-screen w-full border-none'
        title={t('Custom Home Page')}
        sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation-by-user-activation'
        onLoad={syncIframePreferences}
      />
    )
  } else if (contentIsHtml) {
    main = (
      <RichContent
        mode='html'
        htmlVariant='isolated'
        content={content}
        className='custom-home-content'
      />
    )
  } else if (isMarkdown) {
    main = (
      <div className='mx-auto max-w-6xl px-4 py-8'>
        <RichContent
          mode='markdown'
          content={content}
          className='custom-home-content'
        />
      </div>
    )
  } else {
    main = (
      <>
        <Hero isAuthenticated={isAuthenticated} />
        <HowItWorks />
      </>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      showFooter={isLoaded && (!content || isMarkdown)}
    >
      {main}
    </PublicLayout>
  )
}
