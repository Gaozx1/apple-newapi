import { describe, expect, it } from 'vitest'

import {
  groupCodeMessage,
  groupDefinitionSchema,
  toGroupMutationPayload,
} from '../group-definition-schema'

describe('groupCodeMessage', () => {
  it('accepts the opaque codes deployments already use', () => {
    for (const code of [
      'default',
      'vip',
      'ccmax0.9',
      '管key0.25',
      'plus稳定0.08',
      'Gemini Ultra 0.3',
      '[用户]测试111',
    ]) {
      expect(groupCodeMessage(code)).toBeNull()
    }
  })

  it('rejects a comma because it separates the stored group list', () => {
    expect(groupCodeMessage('a,b')).toBe('Group code must not contain a comma')
  })

  it('rejects surrounding whitespace because it would be trimmed away', () => {
    expect(groupCodeMessage(' lead')).toBe(
      'Group code must not start or end with whitespace'
    )
    expect(groupCodeMessage('trail ')).toBe(
      'Group code must not start or end with whitespace'
    )
  })

  it('rejects an empty code and one longer than the column width', () => {
    expect(groupCodeMessage('')).toBe('Group code is required')
    expect(groupCodeMessage('x'.repeat(65))).toBe(
      'Group code must be at most 64 characters'
    )
  })
})

describe('groupDefinitionSchema', () => {
  const base = {
    code: 'vip',
    names: { en: 'VIP' },
    ratio: 1,
    region: '' as const,
    enabled: true,
    sort: 0,
  }

  it('requires at least one non-blank display name', () => {
    const result = groupDefinitionSchema.safeParse({
      ...base,
      names: { en: '   ', zh: '' },
    })
    expect(result.success).toBe(false)
  })

  it('rejects a negative ratio', () => {
    const result = groupDefinitionSchema.safeParse({ ...base, ratio: -1 })
    expect(result.success).toBe(false)
  })
})

describe('toGroupMutationPayload', () => {
  it('trims names and drops the blank translations', () => {
    expect(
      toGroupMutationPayload({ en: '  VIP  ', zh: '会员', ja: '   ' })
    ).toEqual({ en: 'VIP', zh: '会员' })
  })
})
