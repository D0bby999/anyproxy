// SPDX-License-Identifier: AGPL-3.0-or-later
import { expect, it } from 'vitest'
import { en } from '../i18n/en'
import { vi as viCatalog } from '../i18n/vi'
import { providerLogo, providers, providerTypes } from './proxy-providers'

it('has unique ids, https links, bundled logos, and descriptions in both languages', () => {
  expect(providers.length).toBe(22)
  expect(new Set(providers.map((p) => p.id)).size).toBe(providers.length)
  expect(new Set(providers.map((p) => p.name)).size).toBe(providers.length)
  for (const p of providers) {
    expect(new URL(p.url).protocol, p.id).toBe('https:')
    expect(p.types.length, p.id).toBeGreaterThan(0)
    for (const k of p.types) expect(providerTypes, p.id).toContain(k)
    expect(providerLogo(p), p.id).toBeTruthy()
    expect(en.partners.desc[p.id], p.id).toBeTruthy()
    expect(viCatalog.partners.desc[p.id], p.id).toBeTruthy()
  }
  for (const k of providerTypes) {
    expect([en.partners.types[k], viCatalog.partners.types[k]].every(Boolean), k).toBe(true)
  }
  expect(providers[0].id).toBe('oxylabs')
  expect(providers[0].recommended).toBe(true)
})
