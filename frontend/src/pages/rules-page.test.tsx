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

  // Advanced fields (name, action, targets, ports) stay hidden behind disclosure toggle on New Rule.
  expect(within(dialog).queryByLabelText(t.rules.name)).toBeNull()
  expect(within(dialog).queryByLabelText(t.rules.targets)).toBeNull()
  expect(within(dialog).queryByLabelText(t.rules.ports)).toBeNull()

  // New Rule starts directly in "Only these apps" mode with proxy selector visible.
  const proxySelect = within(dialog).getByLabelText(t.rules.selectProxy) as HTMLSelectElement
  await user.selectOptions(proxySelect, 'p1')

  const picker = within(dialog).getByLabelText(t.rules.quickApps) as HTMLSelectElement
  await user.selectOptions(picker, '*/Google Chrome.app/*; Google Chrome*; *com.google.Chrome*')
  await user.selectOptions(picker, '*/Cursor.app/*; Cursor*')
  await user.type(within(dialog).getByLabelText(t.rules.applications), 'curl{enter}')

  // Grouped app presets render as clean single chips (not raw glob tokens).
  const chipsList = within(dialog).getByRole('list', { name: t.rules.selectedApps })
  expect(within(chipsList).getByText('Google Chrome')).toBeTruthy()
  expect(within(chipsList).getByText('Cursor')).toBeTruthy()
  expect(within(chipsList).getByText('curl')).toBeTruthy()

  // Removing one chip via its close button removes all its underlying tokens.
  await user.click(within(chipsList).getByRole('button', { name: `${t.rules.removeApp} Cursor` }))
  expect(within(chipsList).queryByText('Cursor')).toBeNull()
  expect(within(chipsList).getByText('Google Chrome')).toBeTruthy()
  expect(within(chipsList).getByText('curl')).toBeTruthy()

  // Expanding Advanced reveals action, name, protocol, targets, and ports.
  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  await user.click(within(dialog).getByRole('button', { name: t.rules.actionBlock }))
  expect(within(dialog).queryByLabelText(t.rules.selectProxy)).toBeNull()
  await user.click(within(dialog).getByRole('button', { name: t.rules.actionProxy }))
  await user.click(within(dialog).getByRole('button', { name: t.rules.tcp }))

  expect(dialog.textContent).toContain('Google Chrome, curl · *:* · TCP')
  expect(dialog.textContent).toContain(`${t.rules.actionProxy}: US Static`)

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain('duplicate rule'))

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenLastCalledWith(
    expect.objectContaining({
      name: `${t.rules.actionProxy} Google Chrome`,
      processNames: '*/Google Chrome.app/*; Google Chrome*; *com.google.Chrome*; curl',
      excludeProcess: false,
      protocol: 'tcp',
      action: 'proxy',
      proxyConfigId: 'p1',
    })
  )
})

it('creates an exclude ("All except") rule directly, validates empty apps, and does not show fallback warning or modify Default Route', async () => {
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

  expect(screen.getByText(`${t.rules.exceptPrefix}: Telegram, Safari`)).toBeTruthy()

  // Clicking on a rule row opens the Edit Rule modal directly without fallback warning
  await user.click(screen.getByText('System Except Chat'))
  const editDialog = screen.getByRole('dialog', { name: t.rules.editTitle })
  expect(editDialog).toBeTruthy()
  expect(within(editDialog).queryByText(t.rules.fallbackProxyWarn)).toBeNull()
  expect(within(editDialog).queryByRole('button', { name: t.rules.switchFallbackDirect })).toBeNull()
  await user.click(within(editDialog).getByRole('button', { name: t.actions.close }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.editTitle })).toBeNull())

  await user.click(screen.getByRole('button', { name: t.rules.newBtn }))
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })

  await user.click(within(dialog).getByRole('button', { name: t.rules.matchExclude }))

  // No fallback warning inside New Rule modal either
  expect(within(dialog).queryByText(t.rules.fallbackProxyWarn)).toBeNull()
  expect(within(dialog).queryByRole('button', { name: t.rules.switchFallbackDirect })).toBeNull()

  // Submitting without any excluded app shows validation error
  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain(t.rules.excludeRequired))
  expect(onSaveRule).not.toHaveBeenCalled()

  const picker = within(dialog).getByLabelText(t.rules.quickApps) as HTMLSelectElement
  await user.selectOptions(picker, '*/Telegram.app/*; Telegram*; *ru.keepcoder.Telegram*')
  await user.selectOptions(picker, '*/Cursor.app/*; Cursor*')
  expect(dialog.textContent).toContain(`${t.rules.exceptPrefix}: Telegram, Cursor`)

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSetDefaultProxy).not.toHaveBeenCalled()
  expect(onSaveRule).toHaveBeenCalledWith(
    expect.objectContaining({
      name: `${t.rules.actionProxy} ${t.rules.exceptPrefix}: Telegram`,
      processNames: '*/Telegram.app/*; Telegram*; *ru.keepcoder.Telegram*; */Cursor.app/*; Cursor*',
      excludeProcess: true,
      action: 'proxy',
      proxyConfigId: '',
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
  const appsInput = within(dialog).getByLabelText(t.rules.applications) as HTMLInputElement

  // Initial mount focuses the app input directly.
  expect(document.activeElement).toBe(appsInput)

  await user.type(appsInput, 'curl')
  expect(appsInput.value).toBe('curl')
  expect(document.activeElement).toBe(appsInput)

  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  const nameInput = within(dialog).getByLabelText(t.rules.name) as HTMLInputElement
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

it('filters rules by action and multi-word search, locks reordering while filtered, and resets filters', async () => {
  const user = userEvent.setup()
  const onMoveRule = vi.fn().mockResolvedValue(undefined)
  const proxies: ProxyConfig[] = [
    { id: 'p1', name: 'Tokyo SOCKS', type: 'socks5', host: '10.0.0.1', port: 1080 },
  ]
  const rules = [
    {
      id: 'r1',
      name: 'Chrome via Tokyo',
      enabled: true,
      processNames: 'Google Chrome*',
      targetHosts: '*',
      targetPorts: '*',
      protocol: 'both' as const,
      action: 'proxy' as const,
      proxyConfigId: 'p1',
    },
    {
      id: 'r2',
      name: 'Bypass LAN',
      enabled: true,
      processNames: '*',
      targetHosts: '192.168.0.0/16',
      targetPorts: '*',
      protocol: 'both' as const,
      action: 'direct' as const,
      proxyConfigId: '',
    },
    {
      id: 'r3',
      name: 'Block Telemetry',
      enabled: false,
      processNames: '*',
      targetHosts: '*.ads.example',
      targetPorts: '443',
      protocol: 'tcp' as const,
      action: 'block' as const,
      proxyConfigId: '',
    },
  ]

  render(
    <RulesPage
      rules={rules}
      proxies={proxies}
      defaultProxyId="p1"
      onSaveRule={vi.fn()}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={onMoveRule}
      onCloneRule={vi.fn()}
    />
  )

  // Unfiltered: second row can move up
  const moveUpBtns = screen.getAllByRole('button', { name: t.actions.moveUp })
  expect((moveUpBtns[1] as HTMLButtonElement).disabled).toBe(false)

  // Filter by Block action -> only r3 shown, reorder locked
  const filterGroup = screen.getByRole('group', { name: t.rules.action })
  await user.click(within(filterGroup).getByRole('button', { name: new RegExp(t.rules.actionBlock) }))
  expect(screen.getByText('Block Telemetry')).toBeTruthy()
  expect(screen.queryByText('Chrome via Tokyo')).toBeNull()
  expect((screen.getByRole('button', { name: t.actions.moveUp }) as HTMLButtonElement).disabled).toBe(true)

  // Switch back to All and search multi-word ("chrome tokyo")
  await user.click(within(filterGroup).getByRole('button', { name: new RegExp(t.rules.filterAll) }))
  const searchInput = screen.getByLabelText(t.rules.searchPlaceholder)
  await user.type(searchInput, 'chrome tokyo')
  expect(screen.getByText('Chrome via Tokyo')).toBeTruthy()
  expect(screen.queryByText('Bypass LAN')).toBeNull()

  // Non-matching query shows filtered empty state with Clear Filters
  await user.clear(searchInput)
  await user.type(searchInput, 'nonexistent-host-xyz')
  expect(screen.getByText(t.rules.noFilterMatches)).toBeTruthy()
  await user.click(screen.getByRole('button', { name: t.rules.clearFilters }))
  expect(screen.getByText('Chrome via Tokyo')).toBeTruthy()
  expect(screen.getByText('Bypass LAN')).toBeTruthy()
  expect(screen.getByText('Block Telemetry')).toBeTruthy()
})

it('confirms before deleting a rule and switches Default Route from the footer row', async () => {
  const user = userEvent.setup()
  const onDeleteRule = vi.fn().mockResolvedValue(undefined)
  const onSetDefaultProxy = vi.fn().mockResolvedValue(undefined)
  const proxies: ProxyConfig[] = [
    { id: 'p1', name: 'SG Fast', type: 'socks5', host: '1.1.1.1', port: 1080 },
  ]

  render(
    <RulesPage
      rules={[
        {
          id: 'r1',
          name: 'Dev Rule',
          enabled: true,
          processNames: 'git',
          targetHosts: '*',
          targetPorts: '*',
          protocol: 'tcp',
          action: 'proxy',
          proxyConfigId: '',
        },
      ]}
      proxies={proxies}
      defaultProxyId=""
      onSaveRule={vi.fn()}
      onDeleteRule={onDeleteRule}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
      onSetDefaultProxy={onSetDefaultProxy}
    />
  )

  // Default Route footer row switches default proxy directly
  const defaultSelect = screen.getByLabelText(t.rules.defaultRouteLabel) as HTMLSelectElement
  expect(defaultSelect.value).toBe('')
  await user.selectOptions(defaultSelect, 'p1')
  expect(onSetDefaultProxy).toHaveBeenCalledWith('p1')

  // Clicking delete opens confirmation modal instead of deleting immediately
  await user.click(screen.getByRole('button', { name: t.actions.delete }))
  const confirmDialog = screen.getByRole('dialog', { name: t.rules.confirmDelete })
  expect(confirmDialog.textContent).toContain('Dev Rule')
  expect(onDeleteRule).not.toHaveBeenCalled()

  // Cancel keeps the rule
  await user.click(within(confirmDialog).getByRole('button', { name: t.actions.cancel }))
  expect(onDeleteRule).not.toHaveBeenCalled()

  // Confirm deletes the rule
  await user.click(screen.getByRole('button', { name: t.actions.delete }))
  const confirmDialog2 = screen.getByRole('dialog', { name: t.rules.confirmDelete })
  await user.click(within(confirmDialog2).getByRole('button', { name: t.actions.delete }))
  await waitFor(() => expect(onDeleteRule).toHaveBeenCalledWith('r1'))
})

it('starts a rule from a starter template and saves localhostViaProxy from Advanced', async () => {
  const user = userEvent.setup()
  const onSaveRule = vi.fn().mockResolvedValue(undefined)

  render(
    <RulesPage
      rules={[]}
      proxies={[{ id: 'p1', name: 'Burp HTTP', type: 'http', host: '127.0.0.1', port: 8080 }]}
      onSaveRule={onSaveRule}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  // Click "Proxy Browser" starter template in empty state
  await user.click(screen.getByRole('button', { name: t.rules.templateProxyBrowser }))
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })
  const chipsList = within(dialog).getByRole('list', { name: t.rules.selectedApps })
  expect(within(chipsList).getByText('Google Chrome')).toBeTruthy()

  // Expand Advanced and toggle localhostViaProxy
  await user.click(within(dialog).getByRole('button', { name: new RegExp(t.rules.advancedSection) }))
  const localhostSwitch = within(dialog).getByRole('switch', { name: t.rules.localhostViaProxy })
  expect(localhostSwitch.getAttribute('aria-checked')).toBe('false')
  await user.click(localhostSwitch)
  expect(localhostSwitch.getAttribute('aria-checked')).toBe('true')

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenCalledWith(
    expect.objectContaining({
      action: 'proxy',
      localhostViaProxy: true,
      processNames: '*/Google Chrome.app/*; Google Chrome*; *com.google.Chrome*',
    })
  )
})

it('simulates a rule inline and highlights the matched row', async () => {
  const user = userEvent.setup()
  const onSimulateRule = vi.fn().mockResolvedValue({
    matched: true,
    ruleId: 'r1',
    ruleName: 'Burp Inspection',
    ruleIndex: 1,
    action: 'proxy',
    proxyId: 'p1',
    proxyName: 'Burp HTTP',
    viaDefault: false,
    failClosed: false,
    reason: 'Routed via Burp HTTP',
  })

  render(
    <RulesPage
      rules={[
        {
          id: 'r1',
          name: 'Burp Inspection',
          enabled: true,
          processNames: 'Google Chrome*',
          targetHosts: '*.staging.app',
          targetPorts: '443',
          protocol: 'tcp',
          action: 'proxy',
          proxyConfigId: 'p1',
        },
      ]}
      proxies={[{ id: 'p1', name: 'Burp HTTP', type: 'http', host: '127.0.0.1', port: 8080 }]}
      onSaveRule={vi.fn()}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
      onSimulateRule={onSimulateRule}
    />
  )

  await user.click(screen.getByRole('button', { name: t.rules.testBtn }))
  const hostInput = screen.getByLabelText(t.rules.simHost)
  await user.clear(hostInput)
  await user.type(hostInput, 'api.staging.app')
  await user.click(screen.getByRole('button', { name: t.rules.simRun }))

  await waitFor(() => expect(onSimulateRule).toHaveBeenCalledWith('Google Chrome', 'api.staging.app', 443, 'tcp'))
  expect(screen.getByText(t.rules.simMatched(1, 'Burp Inspection'))).toBeTruthy()
  const matchedRow = screen.getByText('Burp Inspection').closest('tr')
  expect(matchedRow?.className).toContain('is-sim-match')
})

it('switches Default Route to Direct when saving an include-only Proxy rule for the default proxy, but preserves it for a different proxy', async () => {
  const user = userEvent.setup()
  const proxies: ProxyConfig[] = [
    { id: 'p1', name: 'SG Fast', type: 'socks5', host: '1.1.1.1', port: 1080 },
    { id: 'p2', name: 'US Static', type: 'http', host: '2.2.2.2', port: 8080 },
  ]
  const onSaveRule = vi.fn().mockResolvedValue(undefined)
  const onSetDefaultProxy = vi.fn().mockResolvedValue(undefined)

  render(
    <RulesPage
      rules={[]}
      proxies={proxies}
      defaultProxyId="p1"
      onSaveRule={onSaveRule}
      onSetDefaultProxy={onSetDefaultProxy}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  // Case 1: "Only these apps" with no app selected shows validation error instead of saving as "*"
  await user.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  let dialog = screen.getByRole('dialog', { name: t.rules.newBtn })
  await user.click(within(dialog).getByRole('button', { name: t.rules.matchInclude }))
  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain(t.rules.includeRequired))
  expect(onSaveRule).not.toHaveBeenCalled()

  // Case 2: "Only these apps" using a DIFFERENT proxy (p2) preserves Default Route (p1)
  await user.type(within(dialog).getByLabelText(t.rules.applications), 'curl{enter}')
  await user.selectOptions(within(dialog).getByLabelText(t.rules.selectProxy), 'p2')
  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSetDefaultProxy).not.toHaveBeenCalled()
  expect(onSaveRule).toHaveBeenLastCalledWith(
    expect.objectContaining({ processNames: 'curl', action: 'proxy', proxyConfigId: 'p2' })
  )

  // Case 2: "Only these apps" using Default Proxy (p1) pins the rule to p1 and switches Default Route to Direct
  await user.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  dialog = screen.getByRole('dialog', { name: t.rules.newBtn })
  await user.click(within(dialog).getByRole('button', { name: t.rules.matchInclude }))
  await user.type(within(dialog).getByLabelText(t.rules.applications), 'Telegram{enter}')
  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenLastCalledWith(
    expect.objectContaining({ processNames: 'Telegram', action: 'proxy', proxyConfigId: 'p1' })
  )
  expect(onSetDefaultProxy).toHaveBeenCalledWith('')
})

it('expands a browsed macOS .app bundle to match all helpers while displaying a single clean chip', async () => {
  const user = userEvent.setup()
  const onSaveRule = vi.fn().mockResolvedValue(undefined)
  const onPickExecutable = vi.fn().mockResolvedValue('/Applications/Discord.app')

  render(
    <RulesPage
      rules={[]}
      proxies={[{ id: 'p1', name: 'SG Fast', type: 'socks5', host: '1.1.1.1', port: 1080 }]}
      onSaveRule={onSaveRule}
      onPickExecutable={onPickExecutable}
      onDeleteRule={vi.fn()}
      onToggleRule={vi.fn()}
      onMoveRule={vi.fn()}
      onCloneRule={vi.fn()}
    />
  )

  await user.click(screen.getAllByRole('button', { name: t.rules.newBtn })[0])
  const dialog = screen.getByRole('dialog', { name: t.rules.newBtn })
  await user.click(within(dialog).getByRole('button', { name: t.rules.browseApp }))

  const chipsList = within(dialog).getByRole('list', { name: t.rules.selectedApps })
  await waitFor(() => expect(within(chipsList).getByText('Discord')).toBeTruthy())

  await user.click(within(dialog).getByRole('button', { name: t.actions.save }))
  await waitFor(() => expect(screen.queryByRole('dialog', { name: t.rules.newBtn })).toBeNull())
  expect(onSaveRule).toHaveBeenCalledWith(
    expect.objectContaining({
      name: `${t.rules.actionProxy} Discord`,
      processNames: '*/Discord.app/*; Discord*',
      action: 'proxy',
    })
  )
})

