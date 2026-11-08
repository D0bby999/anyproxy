// SPDX-License-Identifier: MIT

import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import React from 'react'
import App from './App'
import './preview/stub'

describe('AnyProxyXa Frontend App', () => {
  afterEach(() => {
    cleanup()
  })
  it('renders application chrome, navigation, and connections screen', async () => {
    render(<App />)

    expect(screen.getByText('AnyProxyXa')).toBeDefined()

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeDefined()
      expect(screen.getByText('Bandwidth In')).toBeDefined()
    })
  })

  it('navigates to Proxy Rules and lists rules', async () => {
    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeDefined()
    })

    const rulesTabBtn = screen.getByRole('button', { name: /^Proxy Rules/i })
    fireEvent.click(rulesTabBtn)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Proxy Rules' })).toBeDefined()
    })

    expect(screen.getByText('Burp Suite Security Inspection')).toBeDefined()
    expect(screen.getByText('Bypass Local Intranet & Dev')).toBeDefined()
  })

  it('navigates to Proxy Servers and triggers 5-step proxy test modal', async () => {
    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeDefined()
    })

    const proxyTabBtn = screen.getByRole('button', { name: /^Proxy Servers/i })
    fireEvent.click(proxyTabBtn)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Proxy Servers' })).toBeDefined()
    })

    expect(screen.getByText('Local Clash SOCKS5')).toBeDefined()

    const testButtons = screen.getAllByRole('button', { name: /^Test$/i })
    fireEvent.click(testButtons[0])

    await waitFor(() => {
      expect(screen.getByText('5-Step Proxy Diagnostic Check')).toBeDefined()
      expect(screen.getByText('All 5 Steps Passed')).toBeDefined()
    })
  })

  it('navigates to Activity Logs and displays connection logs', async () => {
    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeDefined()
    })

    const logsTabBtn = screen.getByRole('button', { name: /^Activity Logs/i })
    fireEvent.click(logsTabBtn)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Activity Logs' })).toBeDefined()
    })
  })

  it('navigates to Settings and displays OS driver diagnostics', async () => {
    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeDefined()
    })

    const settingsTabBtn = screen.getByRole('button', { name: /^Settings/i })
    fireEvent.click(settingsTabBtn)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeDefined()
      expect(screen.getByText('OS Driver Diagnostics')).toBeDefined()
      expect(screen.getByText('Software Version & Updates')).toBeDefined()
    })

    const checkUpdateBtn = screen.getByRole('button', { name: /^Check for Updates$/i })
    fireEvent.click(checkUpdateBtn)

    await waitFor(() => {
      expect(screen.getByText('Up to date')).toBeDefined()
    })
  })

  it('opens Log Filters modal and displays filter controls', async () => {
    render(<App />)

    const logsTabBtn = screen.getByRole('button', { name: /^Activity Logs/i })
    fireEvent.click(logsTabBtn)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Activity Logs' })).toBeDefined()
    })

    const filterBtn = screen.getByRole('button', { name: /^Log Filters/i })
    fireEvent.click(filterBtn)

    await waitFor(() => {
      expect(screen.getByText('Connection Log Filters')).toBeDefined()
      expect(screen.getByRole('button', { name: /^\+ Add$/i })).toBeDefined()
    })
  })

  it('triggers + Rule action from Connections table', async () => {
    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeDefined()
    })

    const addRuleButtons = screen.getAllByRole('button', { name: /^\+ Rule$/i })
    expect(addRuleButtons.length).toBeGreaterThan(0)
    fireEvent.click(addRuleButtons[0])

    await waitFor(() => {
      expect(screen.getByText('New Routing Rule')).toBeDefined()
    })
  })
})
