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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { callSmsModel } from '../../api'
import { SmsMessagePanel } from '../sms-message-panel'
import { SmsPhonePanel } from '../sms-phone-panel'
import { SmsSendPanel } from '../sms-send-panel'

vi.mock('../../api', () => ({ callSmsModel: vi.fn() }))

test('taking a number reports it back to the page', async () => {
  const user = userEvent.setup()
  vi.mocked(callSmsModel).mockResolvedValue({
    code: 'ok',
    operation: 'getPhone',
    phone: '13012345678',
    raw: '13012345678',
    task_id: 'task_1',
  })
  const onTaken = vi.fn()

  render(
    <SmsPhonePanel apiKey='sk-test' keyLoading={false} onTaken={onTaken} />
  )
  await user.type(screen.getByLabelText('SMS keyword'), '毛竹')
  await user.click(screen.getByRole('button', { name: 'Take number' }))

  await waitFor(() =>
    expect(onTaken).toHaveBeenCalledWith('13012345678', '毛竹')
  )
  expect(callSmsModel).toHaveBeenCalledWith(
    'getPhone',
    { keyWord: '毛竹', cardType: '全部' },
    'sk-test'
  )
  const card = await screen.findByTestId('sms-result')
  expect(within(card).getByText('13012345678')).toBeInTheDocument()
})

test('a vendor refusal is shown and hands nothing to the page', async () => {
  const user = userEvent.setup()
  vi.mocked(callSmsModel).mockResolvedValue({
    code: 'error',
    operation: 'getPhone',
    reason: 'ERROR:没有可用的号码',
  })
  const onTaken = vi.fn()

  render(
    <SmsPhonePanel apiKey='sk-test' keyLoading={false} onTaken={onTaken} />
  )
  await user.click(screen.getByRole('button', { name: 'Take number' }))

  expect(await screen.findByText('ERROR:没有可用的号码')).toBeInTheDocument()
  expect(onTaken).not.toHaveBeenCalled()
})

test('every panel waits for a usable API key', () => {
  render(<SmsPhonePanel apiKey='' keyLoading onTaken={vi.fn()} />)

  expect(screen.getByRole('button', { name: 'Take number' })).toBeDisabled()
})

test('reading uses the number that was taken and keeps the history', async () => {
  const user = userEvent.setup()
  vi.mocked(callSmsModel)
    .mockResolvedValueOnce({
      code: 'pending',
      operation: 'getMsg',
      phone: '13012345678',
      message: '[尚未收到] 请稍后再试',
      raw: '[尚未收到] 请稍后再试',
      pending: true,
    })
    .mockResolvedValueOnce({
      code: 'ok',
      operation: 'getMsg',
      phone: '13012345678',
      message: '【毛竹】验证码9876',
      raw: '【毛竹】验证码9876',
      pending: false,
    })

  render(
    <SmsMessagePanel
      apiKey='sk-test'
      keyLoading={false}
      takenNumber='13012345678'
      takenKeyWord='毛竹'
    />
  )
  await user.click(screen.getByRole('button', { name: 'Read once' }))

  expect(callSmsModel).toHaveBeenCalledWith(
    'getMsg',
    { phone: '13012345678', keyWord: '毛竹' },
    'sk-test'
  )
  expect(await screen.findByText('No message yet')).toBeInTheDocument()

  await user.click(screen.getByRole('button', { name: 'Read once' }))

  const history = await screen.findByRole('list')
  await waitFor(() =>
    expect(within(history).getByText('【毛竹】验证码9876')).toBeInTheDocument()
  )
  expect(within(history).getByText('[尚未收到] 请稍后再试')).toBeInTheDocument()
})

test('auto polling keeps reading while the message has not arrived', async () => {
  const user = userEvent.setup()
  vi.mocked(callSmsModel).mockResolvedValue({
    code: 'pending',
    operation: 'getMsg',
    message: '[尚未收到] 请稍后再试',
    pending: true,
  })

  render(
    <SmsMessagePanel
      apiKey='sk-test'
      keyLoading={false}
      takenNumber='13012345678'
      takenKeyWord='毛竹'
    />
  )
  const pollSwitch = screen.getByRole('switch', { name: 'Auto poll' })
  await user.click(pollSwitch)

  // An undelivered answer is not a failure: the poller stays switched on and
  // keeps reading until the message arrives or the reader stops it.
  await waitFor(() => expect(callSmsModel).toHaveBeenCalledTimes(1))
  expect(pollSwitch).toHaveAttribute('aria-checked', 'true')
})

test('reading without a number asks for one instead of calling the vendor', async () => {
  const user = userEvent.setup()

  render(
    <SmsMessagePanel
      apiKey='sk-test'
      keyLoading={false}
      takenNumber=''
      takenKeyWord=''
    />
  )
  await user.click(screen.getByRole('button', { name: 'Read once' }))

  expect(await screen.findByText('Number is required')).toBeInTheDocument()
  expect(callSmsModel).not.toHaveBeenCalled()
})

test('sending requires the message body before it reaches the vendor', async () => {
  const user = userEvent.setup()

  render(<SmsSendPanel apiKey='sk-test' keyLoading={false} takenNumber='' />)
  await user.click(screen.getByRole('button', { name: 'Send' }))

  expect(await screen.findByText('Content is required')).toBeInTheDocument()
  expect(callSmsModel).not.toHaveBeenCalled()
})

test('a sent message reports the vendor answer', async () => {
  const user = userEvent.setup()
  vi.mocked(callSmsModel).mockResolvedValue({
    code: 'ok',
    operation: 'send',
    message: '发送成功',
    raw: '发送成功',
  })

  render(
    <SmsSendPanel
      apiKey='sk-test'
      keyLoading={false}
      takenNumber='16200000000'
    />
  )
  await user.type(screen.getByLabelText('Recipient'), '1069000000000')
  await user.type(screen.getByLabelText('Content'), 'hello')
  await user.click(screen.getByRole('button', { name: 'Send' }))

  await waitFor(() =>
    expect(callSmsModel).toHaveBeenCalledWith(
      'send',
      { phone: '16200000000', toPhone: '1069000000000', content: 'hello' },
      'sk-test'
    )
  )
  expect(await screen.findByText('发送成功')).toBeInTheDocument()
})
