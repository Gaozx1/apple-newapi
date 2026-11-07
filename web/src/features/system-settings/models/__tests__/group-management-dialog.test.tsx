import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as groupApi from '../group-management-api'
import type { GroupDefinition } from '../group-management-api'
import { GroupManagementDialog } from '../group-management-dialog'

vi.mock('../group-management-api', () => ({
  getGroupDefinitions: vi.fn(),
  createGroupDefinition: vi.fn(),
  updateGroupDefinition: vi.fn(),
  renameGroupDefinition: vi.fn(),
  deleteGroupDefinition: vi.fn(),
}))

const VIP_GROUP: GroupDefinition = {
  id: 1,
  code: 'vip',
  names: { en: 'VIP', zh: '会员' },
  ratio: 0.9,
  region: 'overseas',
  enabled: true,
  sort: 0,
}

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <GroupManagementDialog trigger={<button type='button'>open</button>} />
    </QueryClientProvider>
  )
}

async function openDialog() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'open' }))
  await screen.findByText('vip')
  return user
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(groupApi.getGroupDefinitions).mockResolvedValue({
    success: true,
    data: { groups: [VIP_GROUP], languages: ['en', 'zh'] },
  })
})

describe('GroupManagementDialog', () => {
  it('shows the stable code together with its display name', async () => {
    renderDialog()
    await openDialog()

    expect(screen.getByText('vip')).toBeTruthy()
    expect(screen.getByText('VIP')).toBeTruthy()
  })

  it('sends only the filled display names when creating a group', async () => {
    vi.mocked(groupApi.createGroupDefinition).mockResolvedValue({
      success: true,
      data: { ...VIP_GROUP, id: 2, code: 'premium' },
    })
    renderDialog()
    const user = await openDialog()

    const newGroupButton = screen.getByRole('button', {
      name: /new group|新建分组/i,
    })
    await user.click(newGroupButton)

    const codeInput = document.querySelector<HTMLInputElement>(
      '#group-definition-code'
    )
    expect(codeInput).toBeTruthy()
    await user.type(codeInput as HTMLInputElement, 'premium')

    const zhInput = document.querySelector<HTMLInputElement>(
      '#group-definition-name-zh'
    )
    expect(zhInput?.disabled).toBe(false)
    await user.type(zhInput as HTMLInputElement, '  高级会员  ')

    await user.click(screen.getByRole('button', { name: /save|保存/i }))

    await waitFor(() => {
      expect(groupApi.createGroupDefinition).toHaveBeenCalledTimes(1)
    })
    const payload = vi.mocked(groupApi.createGroupDefinition).mock.calls[0][0]
    expect(payload.code).toBe('premium')
    expect(payload.names).toEqual({ zh: '高级会员' })
  })

  it('keeps the code read-only when editing an existing group', async () => {
    renderDialog()
    const user = await openDialog()

    const vipCell = screen.getByText('vip')
    const row = vipCell.closest('tr')
    expect(row).toBeTruthy()
    const editButton = within(row as HTMLElement).getAllByRole('button')[0]
    await user.click(editButton)

    const codeInput = document.querySelector<HTMLInputElement>(
      '#group-definition-code'
    )
    expect(codeInput?.disabled).toBe(true)
    const zhInput = document.querySelector<HTMLInputElement>(
      '#group-definition-name-zh'
    )
    expect(zhInput?.value).toBe('会员')
  })

  it('renames a code through the migration endpoint and reports the counts', async () => {
    vi.mocked(groupApi.renameGroupDefinition).mockResolvedValue({
      success: true,
      data: { channels: 3, tokens: 7, users: 2, abilities: 9 },
    })
    renderDialog()
    const user = await openDialog()

    const row = screen.getByText('vip').closest('tr') as HTMLElement
    const renameButton = within(row).getAllByRole('button')[1]
    await user.click(renameButton)

    const codeInput = document.querySelector<HTMLInputElement>(
      '#group-rename-code'
    )
    expect(codeInput?.value).toBe('vip')
    await user.clear(codeInput as HTMLInputElement)
    await user.type(codeInput as HTMLInputElement, 'premium')

    await user.click(screen.getByRole('button', { name: /rename|重命名/i }))

    await waitFor(() => {
      expect(groupApi.renameGroupDefinition).toHaveBeenCalledWith(1, 'premium')
    })
    await waitFor(() => {
      expect(document.querySelector('#group-rename-code')).toBeNull()
    })
  })
})
