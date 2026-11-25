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
import { Compass } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import type { TopNavLink } from '../types'

export function TopNav(props: { links: TopNavLink[] }) {
  const { t } = useTranslation()

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        aria-label={t('Header navigation')}
        render={<Button size='icon' variant='outline' className='size-8' />}
      >
        <Compass aria-hidden />
      </DropdownMenuTrigger>
      <DropdownMenuContent side='bottom' align='end'>
        {props.links.map((link) => (
          <DropdownMenuItem
            key={`${link.title}-${link.href}`}
            disabled={link.disabled}
            render={
              link.external ? (
                <a href={link.href} target='_blank' rel='noopener noreferrer' />
              ) : (
                <Link to={link.href} />
              )
            }
          >
            {t(link.title)}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
