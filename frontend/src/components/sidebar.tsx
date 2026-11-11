// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { t } from '../i18n'
import { Icon, LogoMark } from './icons'
import type { CheckReport, EngineStatus, ProfileListSummary, ProxyConfig } from '../types'

export type Page = 'connections' | 'activity' | 'proxies' | 'rules' | 'settings'

interface SidebarProps {
  page: Page
  onNavigate: (p: Page) => void
  engine: EngineStatus | null
  onToggleEngine: () => void
  profiles: ProfileListSummary
  onSwitchProfile: (name: string) => void
  onCreateProfile: () => void
  onRenameProfile?: (name: string) => void
  onDeleteProfile?: (name: string) => void
  proxies: ProxyConfig[]
  defaultProxyId: string
  onSetDefaultProxy: (id: string) => void
  onTestProxy?: (px: ProxyConfig) => Promise<CheckReport | undefined>
  liveFlowCount: number
  rulesCount: number
  collapsed?: boolean
  onToggleCollapsed?: () => void
  onEnterCompact?: () => void
}

export function Sidebar({
  page,
  onNavigate,
  engine,
  onToggleEngine,
  profiles,
  onSwitchProfile,
  onCreateProfile,
  onRenameProfile,
  onDeleteProfile,
  proxies,
  defaultProxyId,
  onSetDefaultProxy,
  onTestProxy,
  liveFlowCount,
  rulesCount,
  collapsed = false,
  onToggleCollapsed,
  onEnterCompact,
}: SidebarProps) {
  const [testingActive, setTestingActive] = useState(false)
  const [activePing, setActivePing] = useState<CheckReport | null>(null)
  const [copiedEndpoint, setCopiedEndpoint] = useState(false)

  const isRunning = engine?.state === 'running'
  const isStarting = engine?.state === 'starting'
  const activePx = proxies.find((p) => p.id === defaultProxyId)

  const handleCycleProxy = () => {
    if (proxies.length === 0) return
    const ids = ['', ...proxies.map((p) => p.id)]
    const curIdx = ids.indexOf(defaultProxyId || '')
    const nextId = ids[(curIdx + 1) % ids.length]
    setActivePing(null)
    onSetDefaultProxy(nextId)
  }

  const handlePingActive = async () => {
    if (!activePx || !onTestProxy || testingActive) return
    setTestingActive(true)
    try {
      const rep = await onTestProxy(activePx)
      if (rep) setActivePing(rep)
    } finally {
      setTestingActive(false)
    }
  }

  const handleCopyLocal = async () => {
    try {
      await navigator.clipboard.writeText('127.0.0.1:10800')
      setCopiedEndpoint(true)
      setTimeout(() => setCopiedEndpoint(false), 1800)
    } catch {
      // ignore
    }
  }

  return (
    <aside className={`sidebar drag ${collapsed ? 'collapsed' : ''}`}>
      <div className="brandrow">
        <LogoMark />
        {!collapsed && <b>{t.appName}</b>}
        <div className="brand-tools nodrag">
          {onEnterCompact && (
            <button
              type="button"
              className="iconbtn sm"
              onClick={onEnterCompact}
              title={t.nav.compactMode}
              aria-label={t.nav.compactMode}
            >
              <Icon name="minimize" />
            </button>
          )}
          {onToggleCollapsed && (
            <button
              type="button"
              className="iconbtn sm"
              onClick={onToggleCollapsed}
              title={collapsed ? t.nav.expandSidebar : t.nav.collapseSidebar}
              aria-label={collapsed ? t.nav.expandSidebar : t.nav.collapseSidebar}
            >
              <Icon name="sidebar" />
            </button>
          )}
        </div>
      </div>

      <button
        className={`btn block engine-btn nodrag ${isRunning ? 'running' : 'primary'}`}
        onClick={onToggleEngine}
        title={isRunning ? t.engine.disable : t.engine.enable}
      >
        <Icon name="power" />
        {!collapsed &&
          (isStarting
            ? t.engine.starting
            : isRunning
            ? t.engine.disable
            : t.engine.enable)}
      </button>

      {!collapsed && (
        <div className="quick-proxy nodrag">
          <div className="quick-proxy-head">
            <label>{t.engine.activeProxy}</label>
            <div className="row" style={{ gap: 4 }}>
              {activePx && onTestProxy && (
                <button
                  type="button"
                  className="iconbtn xs"
                  disabled={testingActive}
                  aria-busy={testingActive}
                  onClick={() => void handlePingActive()}
                  title={t.actions.test}
                  aria-label={t.actions.test}
                >
                  <Icon name="refresh" className="icon xs" />
                </button>
              )}
              {proxies.length > 0 && (
                <button
                  type="button"
                  className="iconbtn xs"
                  onClick={handleCycleProxy}
                  title={t.engine.cycleProxy}
                  aria-label={t.engine.cycleProxy}
                >
                  <Icon name="zap" className="icon xs" />
                </button>
              )}
            </div>
          </div>
          <select
            className="select"
            value={defaultProxyId || ''}
            aria-label={t.engine.activeProxy}
            onChange={(e) => {
              setActivePing(null)
              onSetDefaultProxy(e.target.value)
            }}
          >
            <option value="">{t.engine.direct}</option>
            {proxies.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.type.toUpperCase()} {p.host}:{p.port})
              </option>
            ))}
          </select>
          {activePing && activePx && (
            <div className="quick-proxy-ping">
              <span className={`chip ${(activePing.success ?? activePing.ok) ? 'ok' : 'danger'}`}>
                <span className="dot" />
                <span className="chiptext">
                  {(activePing.success ?? activePing.ok)
                    ? `${activePing.avgMs ?? activePing.totalDurationMs ?? 0} ms`
                    : t.messages.testFailed}
                </span>
              </span>
              <span className="mono muted" style={{ fontSize: 11 }}>
                {activePx.host}:{activePx.port}
              </span>
            </div>
          )}
        </div>
      )}

      <nav className="sidenav nodrag">
        <button
          className="navitem"
          aria-current={page === 'connections' ? 'page' : undefined}
          onClick={() => onNavigate('connections')}
          title={t.nav.connections}
        >
          <Icon name="plug" />
          {!collapsed && <span>{t.nav.connections}</span>}
          {!collapsed && liveFlowCount > 0 && <span className="cnt live">{liveFlowCount}</span>}
        </button>

        <button
          className="navitem"
          aria-current={page === 'proxies' ? 'page' : undefined}
          onClick={() => onNavigate('proxies')}
          title={t.nav.proxies}
        >
          <Icon name="proxy" />
          {!collapsed && <span>{t.nav.proxies}</span>}
          {!collapsed && <span className="cnt">{proxies.length}</span>}
        </button>

        <button
          className="navitem"
          aria-current={page === 'rules' ? 'page' : undefined}
          onClick={() => onNavigate('rules')}
          title={t.nav.rules}
        >
          <Icon name="rules" />
          {!collapsed && <span>{t.nav.rules}</span>}
          {!collapsed && <span className="cnt">{rulesCount}</span>}
        </button>

        {!collapsed && (
          <>
            <div className="navsection">
              <span>{t.nav.profiles}</span>
              <button
                className="btn sm"
                onClick={onCreateProfile}
                title={t.nav.newProfile}
              >
                <Icon name="plus" className="icon xs" />
              </button>
            </div>

            {profiles.names.map((name) => {
              const isActive = name === profiles.active
              return (
                <div key={name} className="profile-row">
                  <button
                    type="button"
                    className={`profile-row-btn ${isActive ? 'active' : ''}`}
                    onClick={() => onSwitchProfile(name)}
                    title={name}
                  >
                    <span className="profile-row-name">{name}</span>
                    {isActive && <Icon name="check" className="icon xs" />}
                  </button>
                  <span className="profile-row-actions">
                    {onRenameProfile && (
                      <button
                        type="button"
                        className="profile-row-act"
                        onClick={() => onRenameProfile(name)}
                        title={`${t.settings.renameBtn}: ${name}`}
                        aria-label={`${t.settings.renameBtn} ${name}`}
                      >
                        <Icon name="pencil" className="icon xs" />
                      </button>
                    )}
                    {onDeleteProfile && profiles.names.length > 1 && (
                      <button
                        type="button"
                        className="profile-row-act danger"
                        onClick={() => onDeleteProfile(name)}
                        title={`${t.actions.delete}: ${name}`}
                        aria-label={`${t.actions.delete} ${name}`}
                      >
                        <Icon name="trash" className="icon xs" />
                      </button>
                    )}
                  </span>
                </div>
              )
            })}
          </>
        )}
      </nav>

      <div className="sidefoot nodrag">
        <button
          className="navitem"
          aria-current={page === 'settings' ? 'page' : undefined}
          onClick={() => onNavigate('settings')}
          title={t.nav.settings}
        >
          <Icon name="settings" />
          {!collapsed && <span>{t.nav.settings}</span>}
        </button>

        {!collapsed && (
          <>
            <button
              type="button"
              className="sb-copy-pill"
              onClick={() => void handleCopyLocal()}
              title={`${t.engine.copyEndpoint} — ${t.engine.localNotice}`}
            >
              <span className="enginechip">
                <span className={`state ${isRunning ? 'ok' : 'warn'}`} />
                <span>Local</span>
              </span>
              <span className="sb-copy-val mono">
                <span>{copiedEndpoint ? '10800 ✓' : '127.0.0.1:10800'}</span>
                <Icon name={copiedEndpoint ? 'check' : 'copy'} />
              </span>
            </button>
            {engine?.state === 'error' && engine.message && (
              <p className="engine-notice" role="alert">{engine.message}</p>
            )}
          </>
        )}
      </div>
    </aside>
  )
}
