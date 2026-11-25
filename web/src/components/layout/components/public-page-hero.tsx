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
import { cn } from '@/lib/utils'

export interface PublicPageHeroProps {
  eyebrow?: React.ReactNode
  title: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  breadcrumb?: React.ReactNode
  children?: React.ReactNode
  align?: 'center' | 'left'
  size?: 'default' | 'lg'
  className?: string
}

export function PublicPageHero(props: PublicPageHeroProps) {
  return (
    <section className={cn('studio-page-heading', props.className)}>
      {props.breadcrumb && (
        <div className='studio-page-breadcrumb'>{props.breadcrumb}</div>
      )}
      <div className='studio-page-intro'>
        {props.eyebrow && <p className='studio-page-kicker'>{props.eyebrow}</p>}
        <h1 className={cn(props.size === 'lg' && 'studio-title-large')}>
          {props.title}
        </h1>
        {props.description && (
          <p className='studio-page-description'>{props.description}</p>
        )}
        {props.actions && (
          <div className='mt-4 flex flex-wrap items-center gap-2'>
            {props.actions}
          </div>
        )}
      </div>
      {props.children && (
        <div className='studio-page-tools'>{props.children}</div>
      )}
    </section>
  )
}
