import React from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { CompactView } from './compact-view'
import type { EngineStatus, Profile } from '../types'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

it('switches active proxy in 1 click and expands back to full window', () => {
  const engine: EngineStatus = {
    state: 'running',
    enabled: true,
    backend: 'macos-sysproxy',
    extensionInstalled: true,
    dnsProxyActive: true,
    activeProxies: 1,
    activeRules: 0,
    activeFlows: 0,
  }
  const profile: Profile = {
    id: 'p1',
    name: 'Default',
    proxies: [
      { id: 'px-1', name: 'US Proxy', type: 'socks5', host: '1.2.3.4', port: 1080 },
    ],
    rules: [],
    defaultProxyId: '',
    trafficLoggingEnabled: true,
    dnsViaProxy: true,
    logEveryConnection: true,
    autoClearConnectionLogs: false,
  }
  const onSetDefaultProxy = vi.fn().mockResolvedValue(undefined)
  const onExpand = vi.fn()

  render(
    <CompactView
      engine={engine}
      profile={profile}
      profiles={['Default']}
      flows={[]}
      alwaysOnTop={true}
      onToggleEngine={vi.fn()}
      onSetDefaultProxy={onSetDefaultProxy}
      onTestProxy={vi.fn()}
      onSwitchProfile={vi.fn()}
      onToggleAlwaysOnTop={vi.fn()}
      onExpand={onExpand}
    />
  )

  fireEvent.click(screen.getByText('US Proxy'))
  expect(onSetDefaultProxy).toHaveBeenCalledWith('px-1')

  fireEvent.click(screen.getByRole('button', { name: /Full Window|Mở rộng đầy đủ/i }))
  expect(onExpand).toHaveBeenCalled()
})

