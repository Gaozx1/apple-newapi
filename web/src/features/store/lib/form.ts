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
import { z } from 'zod'

/** Product form: the price is typed in USD and converted by the backend. */
export const storeProductFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Product name is required')
    .max(80, 'Name is too long'),
  description: z.string().trim().max(1000, 'Description is too long'),
  price: z
    .string()
    .trim()
    .min(1, 'Price is required')
    .refine((value) => Number.isFinite(Number(value)), 'Price must be a number')
    .refine((value) => Number(value) > 0, 'Price must be greater than 0'),
})

export type StoreProductFormValues = z.infer<typeof storeProductFormSchema>
