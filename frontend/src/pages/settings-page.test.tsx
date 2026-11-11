// SPDX-License-Identifier: MIT
import React from 'react'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { SettingsPage } from './settings-page'
import type { Profile } from '../types'

const baseProfile: Profile = {
  id: 'p1',
  name: 'Default',
  proxies: [],
  rules: [],
  defaultProxyId: '',
  trafficLoggingEnabled: true,
  dnsViaProxy: true,
  logEveryConnection: false,
  autoClearConnectionLogs: false,
}

afterEach(() => {
  cleanup()
  localStorage.clear()
  delete document.documentElement.dataset.theme
  vi.restoreAllMocks()
})

it('toggles traffic options including auto-clear logs, switches theme live, and pairs export credentials toggle', async () => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  })

  const onUpdateOptions = vi.fn().mockResolvedValue(undefined)
  const onExportProfile = vi.fn().mockResolvedValue(undefined)

  render(
    <SettingsPage
      profile={baseProfile}
      profiles={['Default', 'Work']}
      onUpdateOptions={onUpdateOptions}
      onCheckUpdates={vi.fn().mockResolvedValue(undefined)}
      onSwitchProfile={vi.fn()}
      onExportProfile={onExportProfile}
    />
  )

  fireEvent.click(screen.getByRole('switch', { name: t.settings.autoClearLogs }))
  expect(onUpdateOptions).toHaveBeenCalledWith(true, true, false, true)

  const darkBtn = screen.getByRole('button', { name: t.settings.dark })
  fireEvent.click(darkBtn)
  expect(document.documentElement.dataset.theme).toBe('dark')
  expect(darkBtn.getAttribute('aria-pressed')).toBe('true')

  fireEvent.click(screen.getByRole('switch', { name: t.settings.includePasswords }))
  fireEvent.click(screen.getByRole('button', { name: t.settings.exportProfileBtn }))
  expect(onExportProfile).toHaveBeenCalledWith(true)
})

it('handles import profile modal errors, Escape dismissal, and valid submission', async () => {
  const onImportProfile = vi
    .fn()
    .mockRejectedValueOnce(new Error('invalid json'))
    .mockResolvedValueOnce(undefined)

  render(
    <SettingsPage
      profile={baseProfile}
      profiles={['Default']}
      onUpdateOptions={vi.fn().mockResolvedValue(undefined)}
      onCheckUpdates={vi.fn().mockResolvedValue(undefined)}
      onSwitchProfile={vi.fn()}
      onImportProfile={onImportProfile}
    />
  )

  fireEvent.click(screen.getByRole('button', { name: t.settings.importProfileBtn }))
  const dialog = screen.getByRole('dialog', { name: t.settings.importProfileTitle })
  fireEvent.change(within(dialog).getByLabelText(t.settings.importProfileTitle), {
    target: { value: '{bad}' },
  })
  fireEvent.click(within(dialog).getByRole('button', { name: t.actions.import }))

  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain('invalid json'))

  fireEvent.keyDown(window, { key: 'Escape' })
  expect(screen.queryByRole('dialog', { name: t.settings.importProfileTitle })).toBeNull()

  fireEvent.click(screen.getByRole('button', { name: t.settings.importProfileBtn }))
  const dialog2 = screen.getByRole('dialog', { name: t.settings.importProfileTitle })
  fireEvent.change(within(dialog2).getByLabelText(t.settings.importProfileTitle), {
    target: { value: '{"name":"Imported"}' },
  })
  fireEvent.click(within(dialog2).getByRole('button', { name: t.actions.import }))

  await waitFor(() =>
    expect(screen.queryByRole('dialog', { name: t.settings.importProfileTitle })).toBeNull()
  )
  expect(onImportProfile).toHaveBeenLastCalledWith('{"name":"Imported"}')
})

