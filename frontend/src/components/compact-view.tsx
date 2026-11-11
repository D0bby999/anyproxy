// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { Icon, LogoMark } from './icons'
import { t } from '../i18n'
import type { CheckReport, EngineStatus, Flow, Profile, ProxyConfig } from '../types'
import type { Page } from './sidebar'

export function CompactView({
  engine,
  profile,
  profiles,
  flows,
  alwaysOnTop,
  onToggleEngine,
  onSetDefaultProxy,
  onTestProxy,
  onSwitchProfile,
  onToggleAlwaysOnTop,
  onExpand,
}: {
  engine: EngineStatus | null
  profile: Profile | null
  profiles: string[]
  flows: Flow[]
  alwaysOnTop: boolean
  onToggleEngine: () => void
  onSetDefaultProxy: (id: string) => Promise<void>
  onTestProxy: (px: ProxyConfig) => Promise<CheckReport | undefined>
  onSwitchProfile: (name: string) => void
  onToggleAlwaysOnTop: () => void
  onExpand: (targetPage?: Page) => void
}) {
  const [copied, setCopied] = useState(false)
  const [checkingId, setCheckingId] = useState('')
  const [pingingAll, setPingingAll] = useState(false)
  const [results, setResults] = useState<Record<string, CheckReport>>({})

  const isRunning = engine?.state === 'running'
  const proxies = profile?.proxies || []
  const defaultProxyId = profile?.defaultProxyId || ''
  const openFlows = flows.filter((f) => f.state === 'open')
  const recentFlows = (openFlows.length > 0 ? openFlows : flows).slice(0, 3)

  const handleCopyEndpoint = async () => {
    try {
      await navigator.clipboard.writeText('127.0.0.1:10800')
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      // ignore
    }
  }

  const handlePingOne = async (e: React.MouseEvent, px: ProxyConfig) => {
    e.stopPropagation()
    if (checkingId || pingingAll) return
    setCheckingId(px.id)
    try {
      const rep = await onTestProxy(px)
      if (rep) setResults((prev) => ({ ...prev, [px.id]: rep }))
    } finally {
      setCheckingId('')
    }
  }

  const handlePingAll = async () => {
    if (pingingAll || proxies.length === 0) return
    setPingingAll(true)
    try {
      for (const px of proxies) {
        setCheckingId(px.id)
        const rep = await onTestProxy(px)
        if (rep) setResults((prev) => ({ ...prev, [px.id]: rep }))
      }
    } finally {
      setCheckingId('')
      setPingingAll(false)
    }
  }

  return (
    <div className="compact-shell">
      <header className="compact-top drag">
        <div className="compact-brand">
          <LogoMark />
          <span className="compact-title">{t.appName}</span>
          <span className={`chip ${isRunning ? 'ok' : ''}`}>
            <span className="dot" />
            <span className="chiptext">{isRunning ? t.engine.running : t.engine.stopped}</span>
          </span>
        </div>
        <div className="compact-actions nodrag">
          <button
            type="button"
            className={`iconbtn sm ${alwaysOnTop ? 'active-pin' : ''}`}
            onClick={onToggleAlwaysOnTop}
            title={t.nav.alwaysOnTop}
            aria-label={t.nav.alwaysOnTop}
            aria-pressed={alwaysOnTop}
          >
            <Icon name="pin" />
          </button>
          <button
            type="button"
            className="iconbtn sm"
            onClick={() => onExpand()}
            title={t.nav.expandMode}
            aria-label={t.nav.expandMode}
          >
            <Icon name="maximize" />
          </button>
        </div>
      </header>

      <div className="compact-body">
        <section className="compact-engine-card">
          <div className="compact-engine-row">
            <button
              type="button"
              className={`btn ${isRunning ? 'running-btn' : 'primary'} compact-power`}
              onClick={onToggleEngine}
            >
              <Icon name="power" />
              <span>{isRunning ? t.engine.disable : t.engine.enable}</span>
            </button>
            <button
              type="button"
              className="btn sm mono compact-copy"
              onClick={() => void handleCopyEndpoint()}
              title={t.engine.copyEndpoint}
            >
              <Icon name={copied ? 'check' : 'copy'} />
              <span>127.0.0.1:10800</span>
            </button>
          </div>
          <div className="compact-metrics">
            <span>
              {t.status.profile}:{' '}
              <select
                className="compact-profile-select"
                value={profile?.name || 'Default'}
                onChange={(e) => onSwitchProfile(e.target.value)}
                aria-label={t.status.profile}
              >
                {profiles.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </span>
            <span className="compact-metrics-stats mono">
              {openFlows.length} active · {profile?.rules?.filter((r) => r.enabled).length || 0} rules
            </span>
          </div>
        </section>

        <section className="compact-section">
          <div className="compact-sec-head">
            <span className="compact-sec-title">{t.engine.quickSwitch}</span>
            <div className="row" style={{ gap: 4 }}>
              {proxies.length > 0 && (
                <button
                  type="button"
                  className="btn sm"
                  disabled={pingingAll || !!checkingId}
                  onClick={() => void handlePingAll()}
                >
                  <Icon name="refresh" />
                  <span>{t.engine.pingAll}</span>
                </button>
              )}
              <button
                type="button"
                className="iconbtn sm"
                onClick={() => onExpand('proxies')}
                title={t.proxies.newBtn}
                aria-label={t.proxies.newBtn}
              >
                <Icon name="plus" />
              </button>
            </div>
          </div>

          <div className="compact-proxy-list" role="listbox" aria-label={t.engine.activeProxy}>
            <div
              role="option"
              tabIndex={0}
              aria-selected={!defaultProxyId}
              className={`compact-proxy-item ${!defaultProxyId ? 'active' : ''}`}
              onClick={() => void onSetDefaultProxy('')}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  void onSetDefaultProxy('')
                }
              }}
            >
              <div className="compact-proxy-main">
                <span className="compact-radio" />
                <div className="compact-proxy-text">
                  <b>{t.engine.direct}</b>
                  <span className="muted">{t.engine.directSub}</span>
                </div>
              </div>
              <span className="chip">
                <span className="chiptext">DIRECT</span>
              </span>
            </div>

            {proxies.map((px) => {
              const selected = px.id === defaultProxyId
              const rep = results[px.id]
              const ok = rep ? (rep.success ?? rep.ok) : undefined
              const ms = rep ? (rep.avgMs ?? rep.totalDurationMs ?? 0) : undefined
              return (
                <div
                  key={px.id}
                  role="option"
                  tabIndex={0}
                  aria-selected={selected}
                  className={`compact-proxy-item ${selected ? 'active' : ''}`}
                  onClick={() => void onSetDefaultProxy(px.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      void onSetDefaultProxy(px.id)
                    }
                  }}
                >
                  <div className="compact-proxy-main">
                    <span className="compact-radio" />
                    <div className="compact-proxy-text">
                      <b title={px.name}>{px.name}</b>
                      <span className="mono">
                        {px.type.toUpperCase()} · {px.host}:{px.port}
                      </span>
                    </div>
                  </div>
                  <div className="compact-proxy-right">
                    {rep && (
                      <span className={`chip ${ok ? 'ok' : 'danger'}`} title={rep.summary}>
                        <span className="dot" />
                        <span className="chiptext">{ok ? `${ms} ms` : 'ERR'}</span>
                      </span>
                    )}
                    <button
                      type="button"
                      className="iconbtn sm"
                      disabled={!!checkingId || pingingAll}
                      aria-busy={checkingId === px.id}
                      onClick={(e) => void handlePingOne(e, px)}
                      title={t.actions.test}
                      aria-label={t.actions.test}
                    >
                      <Icon name="refresh" />
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        </section>

        <section className="compact-flows">
          <div className="compact-sec-head">
            <span className="compact-sec-title">{t.engine.liveFlowsShort}</span>
            <div className="row" style={{ gap: 6, flexShrink: 0 }}>
              <button
                type="button"
                className="btn sm"
                onClick={() => onExpand('rules')}
                title={t.nav.rules}
                aria-label={t.nav.rules}
              >
                <Icon name="rules" />
                <span>{t.status.rules}</span>
              </button>
              <button
                type="button"
                className="btn sm"
                onClick={() => onExpand('connections')}
                title={t.nav.connections}
                aria-label={t.nav.connections}
              >
                <Icon name="plug" />
                <span>{t.nav.connections}</span>
              </button>
            </div>
          </div>
          {recentFlows.length === 0 ? (
            <div className="compact-flow-empty">{t.connections.emptyLiveText}</div>
          ) : (
            <div className="compact-flow-list">
              {recentFlows.map((f) => (
                <div key={f.id} className="compact-flow-row">
                  <span className="compact-flow-proc" title={f.processName}>
                    {f.processName || 'System'}
                  </span>
                  <span className="compact-flow-dest mono" title={f.resolvedDomain || f.destIP}>
                    {f.resolvedDomain || f.destIP}:{f.destPort}
                  </span>
                  <span
                    className={`chip ${
                      f.action === 'proxy' ? 'accent' : f.action === 'block' ? 'danger' : ''
                    }`}
                  >
                    <span className="chiptext">
                      {f.action === 'proxy' ? f.proxyName || 'PROXY' : f.action.toUpperCase()}
                    </span>
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
