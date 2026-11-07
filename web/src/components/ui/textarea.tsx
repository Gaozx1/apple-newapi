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
import * as React from 'react'

import { cn } from '@/lib/utils'

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot='textarea'
      className={cn(
        'bg-input/70 placeholder:text-muted-foreground focus-visible:bg-background focus-visible:ring-ring/70 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:bg-input/40 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 flex field-sizing-content max-h-96 min-h-16 w-full overflow-y-auto rounded-lg border border-transparent px-2.5 py-2 text-base shadow-[inset_0_1px_2px_rgba(0,0,0,0.04)] transition-[background-color,box-shadow,color] duration-200 outline-none focus-visible:border-transparent focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-3 md:text-sm',
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
