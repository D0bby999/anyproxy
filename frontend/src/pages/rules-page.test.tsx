// SPDX-License-Identifier: AGPL-3.0-or-later
import React from 'react'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { RulesPage } from './rules-page'
import type { ProxyConfig } from '../types'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

it('picks/removes multiple apps as chips, keeps Advanced collapsed by default, and saves rule', async () => {
  const user = userEvent.setup()
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

  await user.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })

  // Advanced fields stay hidden behind disclosure toggle on New Rule.
  expect(within(dialog).queryByLabelText(t.rules.targets)).toBeNull()
  expect(within(dialog).queryByLabelText(t.rules.ports)).toBeNull()

  // Selecting multiple apps adds removable chips; no apps show before selection.
  const picker = within(dialog).getByLabelText(t.rules.quickApps) as HTMLSelectElement
  await user.selectOptions(picker, 'Google Chrome')
  await user.selectOptions(picker, 'Safari')
  await user.type(within(dialog).getByLabelText(t.rules.applications), 'curl{enter}')

  const chipsList = within(dialog).getByRole('list', { name: t.rules.selectedApps })
  expect(within(chipsList).getByText('Google Chrome')).toBeTruthy()
  expect(within(chipsList).getByText('Safari')).toBeTruthy()
  expect(within(chipsList).getByText('curl')).toBeTruthy()

  // Removing one chip via its close button leaves the remaining chips intact.
  await user.click(within(chipsList).getByRole('button', { name: `${t.rules.removeApp} Safari` }))
  expect(within(chipsList).queryByText('Safari')).toBeNull()
  expect(within(chipsList).getByText('Google Chrome')).toBeTruthy()
  expect(within(chipsList).getByText('curl')).toBeTruthy()

  // Expanding Advanced reveals protocol/targets/ports.
  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  await user.click(within(dialog).getByRole('button', { name: t.rules.tcp }))
  await user.click(within(dialog).getByRole('button', { name: t.rules.actionBlock }))
  expect(within(dialog).queryByLabelText(t.rules.selectProxy)).toBeNull()

  await user.click(within(dialog).getByRole('button', { name: t.rules.actionProxy }))
  const proxySelect = within(dialog).getByLabelText(t.rules.selectProxy) as HTMLSelectElement
  await user.selectOptions(proxySelect, 'p1')

  expect(dialog.textContent).toContain('Google Chrome, curl · *:* · TCP')
  expect(dialog.textContent).toContain(`${t.rules.actionProxy}: US Static`)

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain('duplicate rule'))

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenLastCalledWith(
    expect.objectContaining({
      name: `${t.rules.actionProxy} Google Chrome`,
      processNames: 'Google Chrome; curl',
      excludeProcess: false,
      protocol: 'tcp',
      action: 'proxy',
      proxyConfigId: 'p1',
    })
  )
})

it('creates an exclude ("All except") rule directly, validates empty apps, and clears Active Proxy fallback via Advanced', async () => {
  const user = userEvent.setup()
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

  await user.click(screen.getByRole('button', { name: t.rules.newBtn }))
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })

  await user.click(within(dialog).getByRole('button', { name: t.rules.matchExclude }))
  await user.type(within(dialog).getByLabelText(t.rules.name), 'All except Telegram')

  // Submitting without any excluded app shows validation error
  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain(t.rules.excludeRequired))
  expect(onSaveRule).not.toHaveBeenCalled()

  const picker = within(dialog).getByLabelText(t.rules.quickApps) as HTMLSelectElement
  await user.selectOptions(picker, 'Telegram')
  await user.selectOptions(picker, 'Safari')
  expect(dialog.textContent).toContain(`${t.rules.exceptPrefix}: Telegram, Safari`)

  // Switch active proxy fallback to Direct in one click (inside Advanced)
  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  await user.click(within(dialog).getByRole('button', { name: t.rules.switchFallbackDirect }))
  await waitFor(() => expect(onSetDefaultProxy).toHaveBeenCalledWith(''))

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
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

it('keeps focus in other rule inputs while typing instead of jumping back to Rule name', async () => {
  const user = userEvent.setup()

  render(
    <RulesPage
      rules={[]}
      proxies={[]}
      onSaveRule={vi.fn()}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  await user.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })
  const nameInput = within(dialog).getByLabelText(t.rules.name) as HTMLInputElement
  const appsInput = within(dialog).getByLabelText(t.rules.applications) as HTMLInputElement

  // Initial mount still focuses Rule name.
  expect(document.activeElement).toBe(nameInput)

  await user.click(appsInput)
  await user.type(appsInput, 'curl')
  expect(appsInput.value).toBe('curl')
  expect(document.activeElement).toBe(appsInput)

  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  const targetsInput = within(dialog).getByLabelText(t.rules.targets) as HTMLInputElement
  const portsInput = within(dialog).getByLabelText(t.rules.ports) as HTMLInputElement

  await user.click(targetsInput)
  await user.type(targetsInput, '*.example.com')
  expect(targetsInput.value).toBe('*.example.com')
  expect(document.activeElement).toBe(targetsInput)

  await user.click(portsInput)
  await user.type(portsInput, '443')
  expect(portsInput.value).toBe('443')
  expect(document.activeElement).toBe(portsInput)
  expect(nameInput.value).toBe('')
})

it('lets users save app directly without clearing "*", opens Advanced when editing advanced rules, and auto-names Direct rules', async () => {
  const user = userEvent.setup()
  const onSaveRule = vi.fn().mockResolvedValue(undefined)

  render(
    <RulesPage
      rules={[
        {
          id: 'r-adv',
          name: 'Dev Package Proxy',
          enabled: true,
          processNames: 'git; go',
          excludeProcess: false,
          targetHosts: '*.github.com',
          targetPorts: '443',
          protocol: 'tcp',
          action: 'proxy',
          proxyConfigId: '',
        },
      ]}
      proxies={[]}
      onSaveRule={onSaveRule}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  // Editing a rule with targets/ports/protocol automatically expands Advanced & shows app chips.
  await user.click(screen.getByRole('button', { name: t.actions.edit }))
  const editDialog = screen.getByRole('dialog', { name: t.rules.editTitle })
  expect((within(editDialog).getByLabelText(t.rules.targets) as HTMLInputElement).value).toBe('*.github.com')
  expect(within(editDialog).getByText('git')).toBeTruthy()
  expect(within(editDialog).getByText('go')).toBeTruthy()
  await user.click(within(editDialog).getByRole('button', { name: t.actions.cancel }))

  await user.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })
  const appsInput = within(dialog).getByLabelText(t.rules.applications) as HTMLInputElement

  await user.click(appsInput)
  await user.type(appsInput, 'curl')
  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  await user.click(within(dialog).getByRole('button', { name: t.rules.actionDirect }))
  expect(within(dialog).queryByLabelText(t.rules.selectProxy)).toBeNull()
  expect(dialog.textContent).toContain(t.rules.directOutcome)

  // Leaving Rule Name blank auto-generates a clean rule name on Save.
  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenCalledWith(
    expect.objectContaining({
      name: `${t.rules.actionDirect} curl`,
      processNames: 'curl',
      targetHosts: '*',
      targetPorts: '*',
      action: 'direct',
    })
  )
})

