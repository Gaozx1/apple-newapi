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
import { Input as InputPrimitive } from '@base-ui/react/input'
import * as React from 'react'

import { cn } from '@/lib/utils'

const inputSizeClasses = {
  sm: 'h-7',
  default: 'h-8',
  lg: 'h-10',
} as const

type InputSize = keyof typeof inputSizeClasses

function Input({
  className,
  type,
  size,
  ...props
}: Omit<React.ComponentProps<'input'>, 'size'> & {
  // `number` is the native `size` attribute; accepted so existing call sites
  // that spread full input props keep type-checking, but the visual size
  // toggle only consumes the named variants.
  size?: InputSize | number
}) {
  const sizeClass =
    typeof size === 'string' ? inputSizeClasses[size] : inputSizeClasses.default

  return (
    <InputPrimitive
      type={type}
      data-slot='input'
      data-size={typeof size === 'string' ? size : undefined}
      size={typeof size === 'number' ? size : undefined}
      className={cn(
        'border-input file:text-foreground placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/60 disabled:bg-input/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:bg-input/20 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 w-full min-w-0 rounded-lg border bg-background/60 px-2.5 py-1 text-base shadow-2xs transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:ring-2 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-3 aria-invalid:ring-inset md:text-sm',
        sizeClass,
        className
      )}
      {...props}
    />
  )
}

export { Input }
