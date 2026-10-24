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

import { buildSmsPayload } from '../lib/payload'

test('getPhone keeps the keyword and the explicit filters', () => {
  const payload = buildSmsPayload('getPhone', {
    keyWord: ' 毛竹 ',
    phone: '',
    province: '广东',
    cardType: '实卡',
  })

  expect(payload).toEqual({
    keyWord: '毛竹',
    province: '广东',
    cardType: '实卡',
  })
})

test('getPhone drops the blank number so the vendor answers a random one', () => {
  const payload = buildSmsPayload('getPhone', {
    keyWord: '毛竹',
    phone: '   ',
    province: '',
    cardType: '全部',
  })

  expect(payload).not.toHaveProperty('phone')
  expect(payload).toEqual({ keyWord: '毛竹', cardType: '全部' })
})

test('getMsg sends only the number and the keyword', () => {
  const payload = buildSmsPayload('getMsg', {
    keyWord: '毛竹',
    phone: '13012345678',
    province: '广东',
    cardType: '实卡',
    content: 'ignored',
  })

  expect(payload).toEqual({ phone: '13012345678', keyWord: '毛竹' })
})

test('send keeps the optional project id when it is filled in', () => {
  const payload = buildSmsPayload('send', {
    phone: '16200000000',
    toPhone: '1069000000000',
    projId: ' 42 ',
    content: ' hello ',
    keyWord: 'ignored',
  })

  expect(payload).toEqual({
    phone: '16200000000',
    toPhone: '1069000000000',
    projId: '42',
    content: 'hello',
  })
})

test('send drops a blank project id', () => {
  const payload = buildSmsPayload('send', {
    phone: '16200000000',
    toPhone: '1069000000000',
    projId: '',
    content: 'hello',
  })

  expect(payload).not.toHaveProperty('projId')
})
