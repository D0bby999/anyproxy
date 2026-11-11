// SPDX-License-Identifier: AGPL-3.0-or-later
import React from 'react'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { RulesPage } from './rules-page'
import type { ProxyConfig } from '../types'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

it('creates a rule with quick-pick app chips, segmented protocol/action controls, live preview, and error handling', async () => {
  const proxy: ProxyConfig = {
    id: 'p1',
    name: 'US Static',
    type: 'socks5',
    host: '1.2.3.4',
    port: 1080,
  }
  const onSaveRule = vi
    .fn()
    .mockRejectedValueOnce(new Error('duplicate rule'))
    .mockResolvedValueOnce(undefined)

  render(
    <RulesPage
      rules={[]}
      proxies={[proxy]}
      onSaveRule={onSaveRule}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  fireEvent.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })

  fireEvent.click(within(dialog).getByRole('button', { name: 'Google Chrome' }))
  expect((within(dialog).getByLabelText(t.rules.applications) as HTMLInputElement).value).toBe('Google Chrome')
  expect((within(dialog).getByLabelText(t.rules.name) as HTMLInputElement).value).toBe('Proxy Google Chrome')

  fireEvent.click(within(dialog).getByRole('button', { name: t.rules.tcp }))
  fireEvent.click(within(dialog).getByRole('button', { name: t.rules.actionBlock }))
  expect(within(dialog).queryByLabelText(t.rules.selectProxy)).toBeNull()

  fireEvent.click(within(dialog).getByRole('button', { name: t.rules.actionProxy }))
  const proxySelect = within(dialog).getByLabelText(t.rules.selectProxy) as HTMLSelectElement
  fireEvent.change(proxySelect, { target: { value: 'p1' } })

  expect(dialog.textContent).toContain('Google Chrome · *:* · TCP')
  expect(dialog.textContent).toContain(`${t.rules.actionProxy}: US Static`)

  fireEvent.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain('duplicate rule'))

  fireEvent.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenLastCalledWith(
    expect.objectContaining({
      name: 'Proxy Google Chrome',
      processNames: 'Google Chrome',
      excludeProcess: false,
      protocol: 'tcp',
      action: 'proxy',
      proxyConfigId: 'p1',
    })
  )
})

it('creates an exclude ("All except") rule, validates empty apps, and lets users clear Active Proxy fallback', async () => {
  const proxy: ProxyConfig = {
    id: 'p1',
    name: 'SG Fast',
    type: 'socks5',
    host: '1.2.3.4',
    port: 1080,
  }
  const onSaveRule = vi.fn().mockResolvedValue(undefined)
  const onSetDefaultProxy = vi.fn().mockResolvedValue(undefined)

  render(
    <RulesPage
      rules={[
        {
          id: 'r-existing',
          name: 'System Except Chat',
          enabled: true,
          processNames: 'Telegram; Safari',
          excludeProcess: true,
          targetHosts: '*',
          targetPorts: '*',
          protocol: 'both',
          action: 'proxy',
          proxyConfigId: 'p1',
        },
      ]}
      proxies={[proxy]}
      defaultProxyId="p1"
      onSaveRule={onSaveRule}
      onSetDefaultProxy={onSetDefaultProxy}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  expect(screen.getByText(`${t.rules.exceptPrefix}: Telegram; Safari`)).toBeTruthy()

  fireEvent.click(screen.getByRole('button', { name: t.rules.newBtn }))
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })

  fireEvent.click(within(dialog).getByRole('button', { name: t.rules.matchExclude }))
  fireEvent.change(within(dialog).getByLabelText(t.rules.name), { target: { value: 'All except Telegram' } })

  // Submitting without any excluded app shows validation error
  fireEvent.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain(t.rules.excludeRequired))
  expect(onSaveRule).not.toHaveBeenCalled()

  fireEvent.click(within(dialog).getByRole('button', { name: 'Telegram' }))
  fireEvent.click(within(dialog).getByRole('button', { name: 'Safari' }))
  expect(dialog.textContent).toContain(`${t.rules.exceptPrefix}: Telegram, Safari`)

  // Switch active proxy fallback to Direct in one click
  fireEvent.click(within(dialog).getByRole('button', { name: t.rules.switchFallbackDirect }))
  await waitFor(() => expect(onSetDefaultProxy).toHaveBeenCalledWith(''))

  fireEvent.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenCalledWith(
    expect.objectContaining({
      name: 'All except Telegram',
      processNames: 'Telegram; Safari',
      excludeProcess: true,
      action: 'proxy',
      proxyConfigId: 'p1',
    })
  )
})

