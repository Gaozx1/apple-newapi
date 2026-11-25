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
import { lazy, Suspense } from 'react'

import type { HtmlContentVariant } from '@/components/html-content'
import { LoadingState } from '@/components/loading-state'

const HtmlContent = lazy(() =>
  import('@/components/html-content').then((module) => ({
    default: module.HtmlContent,
  }))
)
const Markdown = lazy(() =>
  import('@/components/ui/markdown').then((module) => ({
    default: module.Markdown,
  }))
)

type RichContentMode = 'markdown' | 'html'

interface RichContentProps {
  content: string
  mode?: RichContentMode
  breaks?: boolean
  className?: string
  htmlVariant?: HtmlContentVariant
}

export function RichContent(props: RichContentProps) {
  if (!props.content) return null

  return (
    <Suspense fallback={<LoadingState inline size='sm' />}>
      {props.mode === 'html' ? (
        <HtmlContent
          content={props.content}
          className={props.className}
          variant={props.htmlVariant}
        />
      ) : (
        <Markdown breaks={props.breaks} className={props.className}>
          {props.content}
        </Markdown>
      )}
    </Suspense>
  )
}
