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
import { expect, test } from 'vitest'

import { countImportCodes } from '../lib/import-codes'

test('counts every unique code on its own line', () => {
  expect(countImportCodes('CARD-1\nCARD-2\nCARD-3')).toBe(3)
})

test('ignores blank lines, padding and repeats', () => {
  expect(countImportCodes('\n  CARD-1  \n\nCARD-1\r\n\tCARD-2\n')).toBe(2)
})

test('reports zero for an empty import', () => {
  expect(countImportCodes('')).toBe(0)
  expect(countImportCodes('   \n\t\n')).toBe(0)
})
