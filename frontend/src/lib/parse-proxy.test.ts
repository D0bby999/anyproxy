import { expect, it } from 'vitest'
import { parseProxy } from './parse-proxy'

it('parses paste formats without putting credentials in the label', () => {
  expect(parseProxy('host.example:8001:user:pa:ss')).toMatchObject({ name: 'host.example:8001', type: 'http', username: 'user', password: 'pa:ss' })
  expect(parseProxy('socks5://user:p%40ss@[::1]:1080')).toMatchObject({ host: '::1', type: 'socks5', password: 'p@ss', port: 1080 })
  expect(parseProxy('http://host.example:80')).toMatchObject({ host: 'host.example', port: 80, type: 'http' })
  expect(parseProxy('https://user:p%40ss@host.example:443')).toMatchObject({ name: 'host.example:443', type: 'https', host: 'host.example', port: 443, username: 'user', password: 'p@ss' })
})

it('rejects invalid ports, schemes and credentials', () => {
  for (const line of ['host:0', 'host:65536', 'socks4://host:1080', 'http://user@host:80', 'http://u:%xx@host:80', 'http://u:%0A@host:80', `socks5://${'x'.repeat(256)}:pass@host:80`]) {
    expect(parseProxy(line)).toBeNull()
  }
})
