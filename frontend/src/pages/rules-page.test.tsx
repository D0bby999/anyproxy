// SPDX-License-Identifier: MIT
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
      protocol: 'tcp',
      action: 'proxy',
      proxyConfigId: 'p1',
    })
  )
})

