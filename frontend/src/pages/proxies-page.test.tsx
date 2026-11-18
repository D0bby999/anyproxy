import React from 'react'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { api } from '../api'
import { t } from '../i18n'
import { providers } from '../lib/proxy-providers'
import { ProxiesPage } from './proxies-page'
import type { ProxyConfig } from '../types'

afterEach(() => { cleanup(); vi.restoreAllMocks() })

it('edits a backend proxy type and shows save errors', async () => {
  const proxy = { id: '1', name: 'Existing', type: 'HTTP', host: 'example.com', port: 8001 } as unknown as ProxyConfig
  render(<ProxiesPage proxies={[proxy]} defaultProxyId="1" onSaveProxy={vi.fn().mockRejectedValue(new Error('cannot save'))} onDeleteProxy={vi.fn()} onSetDefault={vi.fn()} onTestProxy={vi.fn()} />)
  fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
  const dialog = screen.getByRole('dialog', { name: t.actions.edit })
  expect((within(dialog).getByRole('combobox') as HTMLSelectElement).value).toBe('http')
  fireEvent.click(within(dialog).getByRole('button', { name: t.proxies.saveAndCheck }))
  await waitFor(() => expect(within(dialog).getByRole('alert').textContent).toContain('cannot save'))
})

it('imports valid lines, reports invalid line numbers and clears pasted credentials', async () => {
  const save = vi.fn().mockResolvedValue(undefined)
  render(<ProxiesPage proxies={[]} defaultProxyId="" onSaveProxy={save} onDeleteProxy={vi.fn()} onSetDefault={vi.fn()} onTestProxy={vi.fn()} />)
  const input = screen.getByRole('textbox', { name: 'Add proxies' }) as HTMLTextAreaElement
  fireEvent.change(input, { target: { value: 'example.com:8001:user:secret\nnot-a-proxy' } })
  fireEvent.click(screen.getByRole('button', { name: t.proxies.bulkAdd }))
  await waitFor(() => expect(screen.getByRole('status').textContent).toContain('2'))
  expect(input.value).toBe('')
  expect(save).toHaveBeenCalledWith(expect.objectContaining({ host: 'example.com', password: 'secret' }))
  expect(screen.getByRole('status').textContent).not.toContain('secret')
})

it('tests, selects, and deletes a proxy without relying on native window.confirm', async () => {
  const proxy = { id: '1', name: 'Existing', type: 'http', host: 'example.com', port: 8001 } as ProxyConfig
  const onTestProxy = vi.fn().mockResolvedValue({ success: true, avgMs: 42, summary: 'OK' })
  const onSetDefault = vi.fn()
  const onDeleteProxy = vi.fn().mockResolvedValue(undefined)
  render(<ProxiesPage proxies={[proxy]} defaultProxyId="" onSaveProxy={vi.fn()} onDeleteProxy={onDeleteProxy} onSetDefault={onSetDefault} onTestProxy={onTestProxy} />)
  expect(screen.getByRole('button', { name: 'Test' }).closest('td')?.className).toBe('col-latency')
  expect(screen.getByRole('button', { name: 'Set Active' }).closest('td')?.className).toBe('col-actions')
  expect(onTestProxy).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Test' }))
  await waitFor(() => expect(screen.getByText('42 ms')).toBeTruthy())
  fireEvent.click(screen.getByRole('button', { name: 'Set Active' }))
  expect(onSetDefault).toHaveBeenCalledWith('1')
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
  expect(onDeleteProxy).not.toHaveBeenCalled()
  const confirmDialog = screen.getByRole('dialog', { name: t.proxies.confirmDelete })
  expect(confirmDialog.textContent).toContain('Existing')
  fireEvent.click(within(confirmDialog).getByText(t.actions.cancel))
  expect(screen.queryByRole('dialog', { name: t.proxies.confirmDelete })).toBeNull()
  expect(onDeleteProxy).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
  fireEvent.click(within(screen.getByRole('dialog', { name: t.proxies.confirmDelete })).getByRole('button', { name: t.actions.delete }))
  expect(onDeleteProxy).toHaveBeenCalledWith('1')
})

it('adds an https proxy via modal and bulk paste', async () => {
  const onSaveProxy = vi.fn().mockResolvedValue(undefined)
  render(<ProxiesPage proxies={[]} defaultProxyId="" onSaveProxy={onSaveProxy} onDeleteProxy={vi.fn()} onSetDefault={vi.fn()} onTestProxy={vi.fn()} />)

  fireEvent.click(screen.getAllByRole('button', { name: t.proxies.newBtn })[0])
  fireEvent.change(screen.getByPlaceholderText(t.proxies.namePlaceholder), { target: { value: 'Edge TLS' } })
  const combos = screen.getAllByRole('combobox')
  fireEvent.change(combos[combos.length - 1], { target: { value: 'https' } })
  fireEvent.change(screen.getByPlaceholderText('127.0.0.1'), { target: { value: 'proxy.example.com' } })
  fireEvent.change(screen.getByDisplayValue('1080'), { target: { value: '443' } })
  fireEvent.click(screen.getByRole('button', { name: t.proxies.addSubmit }))

  await waitFor(() => expect(onSaveProxy).toHaveBeenCalledWith(expect.objectContaining({
    name: 'Edge TLS',
    type: 'https',
    host: 'proxy.example.com',
    port: 443,
  })))

  const bulkInput = screen.getByRole('textbox', { name: t.proxies.bulkTitle }) as HTMLTextAreaElement
  fireEvent.change(bulkInput, { target: { value: 'https://u:p@edge.example.com:8443' } })
  fireEvent.click(screen.getByRole('button', { name: t.proxies.bulkAdd }))
  await waitFor(() => expect(onSaveProxy).toHaveBeenCalledWith(expect.objectContaining({
    type: 'https',
    host: 'edge.example.com',
    port: 8443,
    username: 'u',
    password: 'p',
  })))
})

it('opens Buy proxies dialog with partner cards, filters by type, opens external link, and closes on Escape', () => {
  const openExternal = vi.spyOn(api, 'openExternal').mockImplementation(() => {})
  const proxy = { id: '1', name: 'Existing', type: 'http', host: 'example.com', port: 8001 } as ProxyConfig
  render(<ProxiesPage proxies={[proxy]} defaultProxyId="" onSaveProxy={vi.fn()} onDeleteProxy={vi.fn()} onSetDefault={vi.fn()} onTestProxy={vi.fn()} />)

  fireEvent.click(screen.getByRole('button', { name: t.proxies.buy }))
  const dialog = screen.getByRole('dialog', { name: t.proxies.buy })
  expect(within(dialog).getAllByRole('listitem').map((el) => el.querySelector('b')?.textContent))
    .toEqual(providers.map((p) => p.name))
  expect(within(dialog).getAllByRole('listitem')[0].textContent).toContain(t.partners.recommended)

  fireEvent.click(within(within(dialog).getByRole('group', { name: t.partners.filter })).getByRole('button', { name: t.partners.types.mobile }))
  const mobile = providers.filter((p) => p.types.includes('mobile'))
  expect(within(dialog).getAllByRole('listitem').map((el) => el.querySelector('b')?.textContent))
    .toEqual(mobile.map((p) => p.name))
  expect(within(dialog).queryByText('Massive')).toBeNull()

  fireEvent.click(within(dialog).getByRole('button', { name: t.partners.all }))
  expect(within(dialog).getAllByRole('listitem')).toHaveLength(providers.length)

  fireEvent.click(within(dialog).getByRole('button', { name: /Oxylabs/ }))
  expect(openExternal).toHaveBeenCalledWith('https://oxylabs.go2cloud.org/aff_c?offer_id=7&aff_id=2794')

  fireEvent.keyDown(window, { key: 'Escape' })
  expect(screen.queryByRole('dialog', { name: t.proxies.buy })).toBeNull()
})

it('saves and immediately checks proxy from textarea, displaying latency in summary', async () => {
  const onSaveProxy = vi.fn().mockImplementation(async (px: ProxyConfig) => ({ ...px, id: 'px-auto-checked' }))
  const onTestProxy = vi.fn().mockResolvedValue({ success: true, avgMs: 35, summary: 'OK' })

  render(
    <ProxiesPage
      proxies={[]}
      defaultProxyId=""
      onSaveProxy={onSaveProxy}
      onDeleteProxy={vi.fn()}
      onSetDefault={vi.fn()}
      onTestProxy={onTestProxy}
    />
  )

  const bulkInput = screen.getByRole('textbox', { name: t.proxies.bulkTitle }) as HTMLTextAreaElement
  fireEvent.change(bulkInput, { target: { value: '1.2.3.4:8080' } })
  fireEvent.click(screen.getByRole('button', { name: t.proxies.bulkAdd }))

  await waitFor(() => expect(onSaveProxy).toHaveBeenCalledTimes(1))
  await waitFor(() => expect(onTestProxy).toHaveBeenCalledWith(expect.objectContaining({ id: 'px-auto-checked' })))
  await waitFor(() => expect(screen.getByRole('status').textContent).toContain('35 ms'))
})

it('defaults blank name to host:port, checks before closing Add & Check modal, and uses default scheme in bulk paste', async () => {
  const onSaveProxy = vi.fn().mockImplementation(async (px: ProxyConfig) => ({ ...px, id: 'px-new' }))
  let resolveTest!: (v: { success: boolean; avgMs: number; summary: string }) => void
  const onTestProxy = vi.fn().mockImplementation(() => new Promise((res) => { resolveTest = res }))

  render(
    <ProxiesPage
      proxies={[]}
      defaultProxyId=""
      onSaveProxy={onSaveProxy}
      onDeleteProxy={vi.fn()}
      onSetDefault={vi.fn()}
      onTestProxy={onTestProxy}
    />
  )

  fireEvent.click(screen.getAllByRole('button', { name: t.proxies.newBtn })[0])
  fireEvent.change(screen.getByPlaceholderText('127.0.0.1'), { target: { value: '10.0.0.2' } })
  fireEvent.change(screen.getByDisplayValue('1080'), { target: { value: '9050' } })
  fireEvent.click(screen.getByRole('button', { name: t.proxies.addSubmit }))

  await waitFor(() => expect(onSaveProxy).toHaveBeenCalledWith(expect.objectContaining({
    name: '10.0.0.2:9050',
    host: '10.0.0.2',
    port: 9050,
  })))
  await waitFor(() => expect(onTestProxy).toHaveBeenCalledTimes(1))
  expect(screen.getByRole('button', { name: t.proxies.addSubmitting }).getAttribute('aria-busy')).toBe('true')

  resolveTest({ success: true, avgMs: 28, summary: 'OK' })
  await waitFor(() => expect(screen.queryByRole('button', { name: t.proxies.addSubmitting })).toBeNull())

  const schemeSelect = screen.getAllByRole('combobox')[0] as HTMLSelectElement
  fireEvent.change(schemeSelect, { target: { value: 'socks5' } })
  const bulkInput = screen.getByRole('textbox', { name: t.proxies.bulkTitle }) as HTMLTextAreaElement
  fireEvent.change(bulkInput, { target: { value: '5.6.7.8:1080' } })
  fireEvent.click(screen.getByRole('button', { name: t.proxies.bulkAdd }))
  await waitFor(() => expect(onSaveProxy).toHaveBeenLastCalledWith(expect.objectContaining({
    type: 'socks5',
    host: '5.6.7.8',
    port: 1080,
  })))
  resolveTest({ success: true, avgMs: 12, summary: 'OK' })
})

it('requires confirmation when deleting from the edit modal', async () => {
  const proxy = { id: '1', name: 'Office', type: 'http', host: 'example.com', port: 8001 } as ProxyConfig
  const onDeleteProxy = vi.fn().mockResolvedValue(undefined)
  render(<ProxiesPage proxies={[proxy]} defaultProxyId="" onSaveProxy={vi.fn()} onDeleteProxy={onDeleteProxy} onSetDefault={vi.fn()} onTestProxy={vi.fn()} />)

  fireEvent.click(screen.getByRole('button', { name: t.actions.edit }))
  const editDialog = screen.getByRole('dialog', { name: t.actions.edit })
  fireEvent.click(within(editDialog).getByRole('button', { name: t.actions.delete }))
  expect(onDeleteProxy).not.toHaveBeenCalled()

  const confirmDialog = screen.getByRole('dialog', { name: t.proxies.confirmDelete })
  expect(confirmDialog.textContent).toContain('Office')
  fireEvent.click(within(confirmDialog).getByRole('button', { name: t.actions.delete }))
  expect(onDeleteProxy).toHaveBeenCalledWith('1')
})
