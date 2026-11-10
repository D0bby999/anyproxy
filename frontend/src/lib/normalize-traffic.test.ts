import { expect, it } from 'vitest'
import { normalizeConnectionLog, normalizeFlow } from './normalize-traffic'

it('maps native traffic fields for the UI without breaking preview fields', () => {
  expect(normalizeFlow({ destIp: '1.2.3.4', port: 443, host: 'example.com', isUdp: false, proxyLabel: 'Proxy', closed: true, sent: 3, received: 5 })).toMatchObject({ destIP: '1.2.3.4', destPort: 443, resolvedDomain: 'example.com', action: 'proxy', state: 'closed', bytesOut: 3, bytesIn: 5 })
  expect(normalizeConnectionLog({ destIp: '1.2.3.4', host: '', port: 80, action: 'BLOCK', ruleId: 'r' })).toMatchObject({ destIP: '1.2.3.4', destPort: 80, action: 'block', ruleName: 'r' })
  expect(normalizeFlow({ destIP: 'x', protocol: 'TCP', state: 'open', action: 'direct' })).toMatchObject({ destIP: 'x', state: 'open' })
})
