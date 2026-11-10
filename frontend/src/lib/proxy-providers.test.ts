import { expect, it } from 'vitest'
import { providers } from './proxy-providers'

it('contains distinct HTTPS provider links with at least one category', () => {
  expect(providers.length).toBe(22)
  expect(new Set(providers.map((provider) => provider.name)).size).toBe(providers.length)
  for (const provider of providers) {
    expect(new URL(provider.url).protocol).toBe('https:')
    expect(provider.types.length).toBeGreaterThan(0)
  }
})
