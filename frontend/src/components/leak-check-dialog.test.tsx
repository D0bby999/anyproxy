// SPDX-License-Identifier: AGPL-3.0-or-later
import React from 'react'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { LeakCheckDialog } from './leak-check-dialog'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

it('runs leak check on open, renders verified exit IP and items, and re-checks on demand', async () => {
  const user = userEvent.setup()
  const onCheckLeak = vi.fn().mockResolvedValue({
    overall: 'protected',
    exitIp: '103.21.244.10',
    country: 'SG',
    proxyName: 'SG Fast',
    proxyType: 'SOCKS5',
    checkedAt: '2026-11-21T10:00:00Z',
    items: [
      { id: 'engine', name: 'Engine & Fail-Closed Guard', status: 'ok', detail: 'Active' },
      { id: 'udp', name: 'UDP & QUIC Protection', status: 'ok', detail: 'UDP relay OK' },
    ],
  })
  const onClose = vi.fn()

  render(<LeakCheckDialog onCheckLeak={onCheckLeak} onClose={onClose} />)

  await waitFor(() => expect(screen.getByText(t.leak.protected)).toBeTruthy())
  expect(screen.getByText(/103\.21\.244\.10/)).toBeTruthy()
  expect(screen.getByText('Engine & Fail-Closed Guard')).toBeTruthy()

  await user.click(screen.getByRole('button', { name: t.leak.recheck }))
  await waitFor(() => expect(onCheckLeak).toHaveBeenCalledTimes(2))
})
