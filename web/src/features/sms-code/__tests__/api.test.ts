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
import { expect, test, vi } from 'vitest'

import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'

import { callSmsModel, listSmsCodeKeys, resolveSmsCodeKey } from '../api'

const { postMock } = vi.hoisted(() => ({ postMock: vi.fn() }))

vi.mock('axios', () => ({ default: { create: () => ({ post: postMock }) } }))
vi.mock('@/features/keys/api', () => ({
  getApiKeys: vi.fn(),
  fetchTokenKey: vi.fn(),
}))

function apiKey(overrides: Partial<ApiKey>): ApiKey {
  return {
    id: 1,
    name: 'default',
    key: 'abcd',
    status: 1,
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 0,
    accessed_time: 0,
    group: 'default',
    auto_groups: null,
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    allow_user_quota_query: false,
    ...overrides,
  }
}

test('a model call carries the API key and the model name', async () => {
  postMock.mockResolvedValue({
    data: { code: 'ok', operation: 'getPhone', phone: '13012345678' },
  })

  const result = await callSmsModel('getPhone', { keyWord: '毛竹' }, 'sk-test')

  expect(postMock).toHaveBeenCalledWith(
    '/d1jiema/getPhone',
    { keyWord: '毛竹', model: 'getPhone' },
    expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer sk-test' }),
    })
  )
  expect(result.phone).toBe('13012345678')
})

test('a vendor refusal stays a successful response so the caller reads code', async () => {
  postMock.mockResolvedValue({
    data: {
      code: 'error',
      operation: 'getMsg',
      reason: 'ERROR:没有可用的号码',
    },
  })

  const result = await callSmsModel(
    'getMsg',
    { phone: '13012345678', keyWord: '毛竹' },
    'sk-test'
  )

  expect(result.code).toBe('error')
  expect(result.reason).toBe('ERROR:没有可用的号码')
})

test('a gateway failure surfaces its own message', async () => {
  postMock.mockRejectedValue(
    Object.assign(new Error('Request failed'), {
      isAxiosError: true,
      response: { status: 401, data: { message: 'Invalid token' } },
    })
  )

  await expect(
    callSmsModel('getPhone', { keyWord: '毛竹' }, 'sk-test')
  ).rejects.toThrow('Invalid token')
})

test('an envelope that reports nothing received is mapped to pending', async () => {
  postMock.mockResolvedValue({
    data: {
      code: 'error',
      operation: 'getMsg',
      raw: '尚未收到包含关键字“毛竹”的短信，请5秒后再收取。[尚未收到]',
      reason:
        '尚未收到短信（本次不计费），可继续读取：尚未收到包含关键字“毛竹”的短信，请5秒后再收取。[尚未收到]',
    },
  })

  const result = await callSmsModel(
    'getMsg',
    { phone: '13012345678', keyWord: '毛竹' },
    'sk-test'
  )

  expect(result.code).toBe('pending')
  expect(result.pending).toBe(true)
  expect(result.message).toContain('尚未收到')
})

test('a reading that has not received the message resolves as pending', async () => {
  postMock.mockRejectedValue(
    Object.assign(new Error('Request failed'), {
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'error',
          message:
            '尚未收到短信（本次不计费），可继续读取：[尚未收到] 请您稍后再试',
        },
      },
    })
  )

  const result = await callSmsModel(
    'getMsg',
    { phone: '13012345678', keyWord: '毛竹' },
    'sk-test'
  )

  expect(result.code).toBe('pending')
  expect(result.pending).toBe(true)
  expect(result.message).toContain('尚未收到')
})

test('only enabled keys are offered and the resolved key is prefixed', async () => {
  vi.mocked(getApiKeys).mockResolvedValue({
    success: true,
    data: {
      items: [
        apiKey({ id: 1, name: 'live', status: 1 }),
        apiKey({ id: 2, name: 'off', status: 2 }),
      ],
      total: 2,
      page: 1,
      page_size: 100,
    },
  })
  vi.mocked(fetchTokenKey).mockResolvedValue({
    success: true,
    data: { key: 'abcd1234' },
  })

  const keys = await listSmsCodeKeys()
  expect(keys.map((item) => item.name)).toEqual(['live'])
  await expect(resolveSmsCodeKey(1)).resolves.toBe('sk-abcd1234')
})

test('a token without a readable key is reported instead of used', async () => {
  vi.mocked(fetchTokenKey).mockResolvedValue({
    success: false,
    message: 'nope',
  })

  await expect(resolveSmsCodeKey(9)).rejects.toThrow('nope')
})
