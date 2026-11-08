// SPDX-License-Identifier: MIT
import React from 'react'
import { t } from '../i18n'
import { Icon, LogoMark } from './icons'
import type { EngineStatus, ProfileListSummary, ProxyConfig } from '../types'

export type Page = 'connections' | 'activity' | 'proxies' | 'rules' | 'settings'

interface SidebarProps {
  page: Page
  onNavigate: (p: Page) => void
  engine: EngineStatus | null
  onToggleEngine: () => void
  profiles: ProfileListSummary
  onSwitchProfile: (name: string) => void
  onCreateProfile: () => void
  proxies: ProxyConfig[]
  defaultProxyId: string
  onSetDefaultProxy: (id: string) => void
  liveFlowCount: number
  rulesCount: number
}

export function Sidebar({
  page,
  onNavigate,
  engine,
  onToggleEngine,
  profiles,
  onSwitchProfile,
  onCreateProfile,
  proxies,
  defaultProxyId,
  onSetDefaultProxy,
  liveFlowCount,
  rulesCount,
}: SidebarProps) {
  const isRunning = engine?.state === 'running'
  const isStarting = engine?.state === 'starting'

  return (
    <aside className="sidebar drag">
      <div className="brandrow">
        <LogoMark />
        <b>{t.appName}</b>
      </div>

      <button
        className={`btn block engine-btn nodrag ${isRunning ? 'running' : 'primary'}`}
        onClick={onToggleEngine}
      >
        <Icon name="power" />
        {isStarting
          ? t.engine.starting
          : isRunning
          ? t.engine.disable
          : t.engine.enable}
      </button>

      <div className="quick-proxy nodrag">
        <label>{t.engine.activeProxy}</label>
        <select
          className="select"
          value={defaultProxyId || ''}
          onChange={(e) => onSetDefaultProxy(e.target.value)}
        >
          <option value="">{t.engine.direct}</option>
          {proxies.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      <nav className="sidenav nodrag">
        <button
          className="navitem"
          aria-current={page === 'connections' ? 'page' : undefined}
          onClick={() => onNavigate('connections')}
        >
          <Icon name="plug" />
          <span>{t.nav.connections}</span>
          {liveFlowCount > 0 && <span className="cnt live">{liveFlowCount}</span>}
        </button>

        <button
          className="navitem"
          aria-current={page === 'activity' ? 'page' : undefined}
          onClick={() => onNavigate('activity')}
        >
          <Icon name="list" />
          <span>{t.nav.activity}</span>
        </button>

        <button
          className="navitem"
          aria-current={page === 'proxies' ? 'page' : undefined}
          onClick={() => onNavigate('proxies')}
        >
          <Icon name="proxy" />
          <span>{t.nav.proxies}</span>
          <span className="cnt">{proxies.length}</span>
        </button>

        <button
          className="navitem"
          aria-current={page === 'rules' ? 'page' : undefined}
          onClick={() => onNavigate('rules')}
        >
          <Icon name="rules" />
          <span>{t.nav.rules}</span>
          <span className="cnt">{rulesCount}</span>
        </button>

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

        {profiles.names.map((name) => (
          <button
            key={name}
            className="navitem"
            style={{
              fontWeight: name === profiles.active ? 600 : 400,
              color: name === profiles.active ? 'var(--accent-ink)' : undefined,
            }}
            onClick={() => onSwitchProfile(name)}
          >
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {name}
            </span>
            {name === profiles.active && (
              <Icon name="check" className="icon xs muted" />
            )}
          </button>
        ))}
      </nav>

      <div className="sidefoot nodrag">
        <button
          className="navitem"
          aria-current={page === 'settings' ? 'page' : undefined}
          onClick={() => onNavigate('settings')}
        >
          <Icon name="settings" />
          <span>{t.nav.settings}</span>
        </button>

        <div className="enginechip">
          <span className={`state ${isRunning ? 'ok' : 'warn'}`} />
          <span>{engine?.backend || 'System Driver'}</span>
        </div>
      </div>
    </aside>
  )
}
