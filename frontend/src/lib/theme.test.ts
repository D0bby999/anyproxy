import { expect, it } from 'vitest'
import { readTheme } from './theme'

it('keeps the previous theme until an AnyProxy preference is saved', () => {
  const values: Record<string, string> = { 'anyproxyxa.theme': 'dark' }
  const storage = { getItem: (key: string) => values[key] ?? null }
  expect(readTheme(storage)).toBe('dark')
  values['anyproxy.theme'] = 'light'
  expect(readTheme(storage)).toBe('light')
})
