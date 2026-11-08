// SPDX-License-Identifier: MIT
import { describe, it, expect } from 'vitest'
import { en } from './en'
import { vi } from './vi'

describe('i18n completeness', () => {
  it('has identical keys between EN and VI', () => {
    function getKeys(obj: Record<string, unknown>, prefix = ''): string[] {
      return Object.keys(obj).flatMap((key) => {
        const val = obj[key]
        const full = prefix ? `${prefix}.${key}` : key
        if (val && typeof val === 'object') {
          return getKeys(val as Record<string, unknown>, full)
        }
        return [full]
      })
    }

    const enKeys = getKeys(en as unknown as Record<string, unknown>).sort()
    const viKeys = getKeys(vi as unknown as Record<string, unknown>).sort()
    expect(enKeys).toEqual(viKeys)
  })
})
