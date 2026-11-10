import { afterEach, expect, it } from 'vitest'
import { api } from './api'

afterEach(() => { delete window.go })

it('does not replace or simulate the native backend', () => {
  const native = { Status: async () => ({}) }
  window.go = { main: { App: native as never } }
  expect(api.getApp()).toBe(native)
  delete window.go
  expect(() => api.getApp()).toThrow('backend is unavailable')
})
