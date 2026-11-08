// SPDX-License-Identifier: MIT

import React, { useEffect, useState, useMemo } from 'react'
import { api } from './api'
import type {
  Profile,
  ProfileListSummary,
  EngineStatus,
  Flow,
  ConnectionLogEntry,
  ActivityLogEntry,
  CheckReport,
  ProxyConfig,
  ProxyRule,
  LogFilter,
} from './types'

type ActiveTab = 'connections' | 'rules' | 'proxies' | 'logs' | 'settings'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

export function App() {
  const [tab, setTab] = useState<ActiveTab>('connections')
  const [profileSummary, setProfileSummary] = useState<ProfileListSummary>({ names: [], active: '' })
  const [profile, setProfile] = useState<Profile | null>(null)
  const [engineStatus, setEngineStatus] = useState<EngineStatus | null>(null)
  const [flows, setFlows] = useState<Flow[]>([])
  const [connLogs, setConnLogs] = useState<ConnectionLogEntry[]>([])
  const [activityLogs, setActivityLogs] = useState<ActivityLogEntry[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [logSubTab, setLogSubTab] = useState<'connections' | 'system'>('connections')

  // Modals state
  const [editingRule, setEditingRule] = useState<ProxyRule | null>(null)
  const [isRuleModalOpen, setIsRuleModalOpen] = useState(false)

  const [editingProxy, setEditingProxy] = useState<ProxyConfig | null>(null)
  const [isProxyModalOpen, setIsProxyModalOpen] = useState(false)

  const [testingProxyId, setTestingProxyId] = useState<string | null>(null)
  const [checkReport, setCheckReport] = useState<CheckReport | null>(null)
  const [isCheckingProxy, setIsCheckingProxy] = useState(false)

  const [isFilterModalOpen, setIsFilterModalOpen] = useState(false)
  const [newProfileName, setNewProfileName] = useState('')
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false)

  const app = api.getApp()

  const refreshAll = async () => {
    try {
      const [sum, prof, status, fls, cLogs, aLogs] = await Promise.all([
        app.ListProfiles(),
        app.ActiveProfile(),
        app.Status(),
        app.GetFlows(100),
        app.GetConnectionLogs(100),
        app.GetActivityLogs(100),
      ])
      setProfileSummary(sum)
      setProfile(prof)
      setEngineStatus(status)
      setFlows(fls)
      setConnLogs(cLogs)
      setActivityLogs(aLogs)
    } catch (err) {
      console.error('Refresh error:', err)
    }
  }

  useEffect(() => {
    refreshAll()
    const timer = setInterval(() => {
      app.GetFlows(100).then(setFlows).catch(() => {})
      app.Status().then(setEngineStatus).catch(() => {})
    }, 2000)
    return () => clearInterval(timer)
  }, [])

  // Event listeners from backend
  useEffect(() => {
    const unsubProfile = api.subscribeEvent('profile:changed', (data) => {
      if (data) setProfile(data as Profile)
    })
    const unsubEngine = api.subscribeEvent('engine:status', (data) => {
      if (data) setEngineStatus(data as EngineStatus)
    })
    return () => {
      unsubProfile()
      unsubEngine()
    }
  }, [])

  // Filtered flows
  const filteredFlows = useMemo(() => {
    if (!searchQuery.trim()) return flows
    const q = searchQuery.toLowerCase()
    return flows.filter(
      (f) =>
        f.processName.toLowerCase().includes(q) ||
        f.destIP.toLowerCase().includes(q) ||
        f.resolvedDomain.toLowerCase().includes(q) ||
        f.proxyName.toLowerCase().includes(q)
    )
  }, [flows, searchQuery])

  // Filtered rules
  const filteredRules = useMemo(() => {
    if (!profile) return []
    if (!searchQuery.trim()) return profile.rules
    const q = searchQuery.toLowerCase()
    return profile.rules.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        r.processNames.toLowerCase().includes(q) ||
        r.targetHosts.toLowerCase().includes(q) ||
        r.targetPorts.toLowerCase().includes(q)
    )
  }, [profile, searchQuery])

  // KPI Calculations
  const kpiData = useMemo(() => {
    const activeCount = flows.filter((f) => f.state === 'open').length
    const totalIn = flows.reduce((acc, f) => acc + f.bytesIn, 0)
    const totalOut = flows.reduce((acc, f) => acc + f.bytesOut, 0)
    const blockedCount = connLogs.filter((l) => l.action === 'block').length
    return {
      activeCount,
      totalIn: formatBytes(totalIn),
      totalOut: formatBytes(totalOut),
      blockedCount,
    }
  }, [flows, connLogs])

  const handleToggleEngine = async () => {
    if (!engineStatus) return
    try {
      if (engineStatus.state === 'running') {
        const next = await app.SetEngineEnabled(false)
        setEngineStatus(next)
      } else {
        const next = await app.SetEngineEnabled(true)
        setEngineStatus(next)
      }
    } catch (err) {
      console.error(err)
    }
  }

  const handleSwitchProfile = async (name: string) => {
    try {
      const updated = await app.SwitchProfile(name)
      setProfile(updated)
      setProfileSummary((prev) => ({ ...prev, active: name }))
      refreshAll()
    } catch (err) {
      alert(String(err))
    }
  }

  const handleRunProxyTest = async (pxId: string) => {
    setTestingProxyId(pxId)
    setIsCheckingProxy(true)
    setCheckReport(null)
    try {
      const rep = await app.TestProxy(pxId, 'www.google.com', 80)
      setCheckReport(rep)
    } catch (err) {
      alert(String(err))
    } finally {
      setIsCheckingProxy(false)
    }
  }

  const handleSaveRule = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingRule) return
    try {
      const updated = await app.SaveRule(editingRule)
      setProfile(updated)
      setIsRuleModalOpen(false)
      setEditingRule(null)
    } catch (err) {
      alert(String(err))
    }
  }

  const handleSaveProxy = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingProxy) return
    try {
      const updated = await app.SaveProxy(editingProxy)
      setProfile(updated)
      setIsProxyModalOpen(false)
      setEditingProxy(null)
    } catch (err) {
      alert(String(err))
    }
  }

  const handlePickProcess = async () => {
    try {
      const path = await app.PickProcessExecutable()
      if (path && editingRule) {
        const parts = path.split('/')
        const name = parts[parts.length - 1].replace('.app', '')
        const current = editingRule.processNames.trim()
        const next = current === '*' || current === '' ? name : `${current};${name}`
        setEditingRule({ ...editingRule, processNames: next })
      }
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <div className="app-frame">
      {/* Sidebar */}
      <aside className="app-sidebar">
        <div className="brand-row">
          <div className="brand-mark">XP</div>
          <div className="brand-title">AnyProxyXa</div>
          <span className="brand-badge">v1.0</span>
        </div>

        <nav className="nav-group">
          <button
            className={`nav-item ${tab === 'connections' ? 'active' : ''}`}
            onClick={() => setTab('connections')}
          >
            <span>Connections</span>
            <span className="nav-item-count">{flows.filter((f) => f.state === 'open').length}</span>
          </button>
          <button
            className={`nav-item ${tab === 'rules' ? 'active' : ''}`}
            onClick={() => setTab('rules')}
          >
            <span>Proxy Rules</span>
            <span className="nav-item-count">{profile?.rules.length || 0}</span>
          </button>
          <button
            className={`nav-item ${tab === 'proxies' ? 'active' : ''}`}
            onClick={() => setTab('proxies')}
          >
            <span>Proxy Servers</span>
            <span className="nav-item-count">{profile?.proxies.length || 0}</span>
          </button>
          <button
            className={`nav-item ${tab === 'logs' ? 'active' : ''}`}
            onClick={() => setTab('logs')}
          >
            <span>Activity Logs</span>
            <span className="nav-item-count">{activityLogs.length + connLogs.length}</span>
          </button>
          <button
            className={`nav-item ${tab === 'settings' ? 'active' : ''}`}
            onClick={() => setTab('settings')}
          >
            <span>Settings</span>
          </button>
        </nav>

        <div className="sidebar-status-card">
          <div className="sidebar-status-title">OS INTERCEPTION DRIVER</div>
          <div className={`sidebar-status-pill ${
            engineStatus?.state === 'running' ? 'pill-pos' : engineStatus?.state === 'paused' ? 'pill-warn' : 'pill-neg'
          }`}>
            <span className="pulse-dot" />
            <span>{engineStatus?.backend || 'macos-ne'} • {engineStatus?.state || 'stopped'}</span>
          </div>
          <div style={{ fontSize: '11px', color: 'var(--muted)' }}>
            DNS Interception: {engineStatus?.dnsProxyActive ? 'Active' : 'Disabled'}
          </div>
        </div>
      </aside>

      {/* Main Area */}
      <main className="app-main">
        {/* Topbar */}
        <header className="app-topbar">
          <div className="topbar-left">
            <div className="profile-select-wrap">
              <span className="profile-select-label">Active Profile:</span>
              <select
                className="profile-select"
                value={profileSummary.active}
                onChange={(e) => handleSwitchProfile(e.target.value)}
              >
                {profileSummary.names.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </div>
            <button
              className="btn btn-sm"
              onClick={() => {
                setNewProfileName('')
                setIsProfileModalOpen(true)
              }}
            >
              + Profile
            </button>
          </div>

          <div className="topbar-actions">
            <button
              className={`master-engine-btn ${engineStatus?.state || 'stopped'}`}
              onClick={handleToggleEngine}
              title="Click to pause or resume transparent interception"
            >
              <span className="pulse-dot" />
              <span>
                Engine: {engineStatus?.state === 'running' ? 'Running' : engineStatus?.state === 'paused' ? 'Paused' : 'Stopped'}
              </span>
            </button>
          </div>
        </header>

        {/* White Inset Sheet */}
        <div className="app-sheet">
          {tab === 'connections' && (
            <>
              <div className="sheet-header">
                <div className="sheet-title-group">
                  <h1 className="sheet-title">Connections</h1>
                  <span className="sheet-subtitle">Transparent flow interception & live bandwidth</span>
                </div>
                <div className="sheet-actions">
                  <input
                    type="text"
                    className="input-text search-input"
                    placeholder="Search flows (Cmd+F)..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                  <button
                    className="btn btn-sm"
                    onClick={async () => {
                      await app.ClearClosedFlows()
                      const fls = await app.GetFlows(100)
                      setFlows(fls)
                    }}
                  >
                    Clear Closed
                  </button>
                  <button
                    className="btn btn-sm btn-primary"
                    onClick={async () => {
                      await app.SimulateFlow(4090, 'curl', '10.200.1.99', 'api.target.internal', 443, false)
                      const fls = await app.GetFlows(100)
                      setFlows(fls)
                    }}
                  >
                    Simulate Flow
                  </button>
                </div>
              </div>

              {/* KPI Deck */}
              <div className="kpi-deck">
                <div className="kpi-card">
                  <span className="kpi-label">Active Flows</span>
                  <span className="kpi-value">{kpiData.activeCount}</span>
                </div>
                <div className="kpi-card">
                  <span className="kpi-label">Bandwidth In</span>
                  <span className="kpi-value">{kpiData.totalIn}</span>
                </div>
                <div className="kpi-card">
                  <span className="kpi-label">Bandwidth Out</span>
                  <span className="kpi-value">{kpiData.totalOut}</span>
                </div>
                <div className="kpi-card">
                  <span className="kpi-label">Blocked Requests</span>
                  <span className="kpi-value">{kpiData.blockedCount}</span>
                </div>
              </div>

              <div className="sheet-content">
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Process</th>
                        <th>Target Destination</th>
                        <th>Protocol</th>
                        <th>Action</th>
                        <th>Routed Proxy</th>
                        <th className="num-cell">In / Out</th>
                        <th className="num-cell">Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredFlows.length === 0 ? (
                        <tr>
                          <td colSpan={7} className="empty-state">
                            <span className="empty-state-title">No matching flows</span>
                            <span>Traffic matching active rules will stream here in real time.</span>
                          </td>
                        </tr>
                      ) : (
                        filteredFlows.map((f) => (
                          <tr key={f.id}>
                            <td>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <strong>{f.processName}</strong>
                                <span className="pill pill-neu mono-cell">PID {f.pid}</span>
                              </div>
                            </td>
                            <td>
                              <div className="mono-cell">
                                {f.resolvedDomain ? (
                                  <span>
                                    <strong>{f.resolvedDomain}</strong> ({f.destIP}:{f.destPort})
                                  </span>
                                ) : (
                                  <span>
                                    {f.destIP}:{f.destPort}
                                  </span>
                                )}
                              </div>
                            </td>
                            <td>
                              <span className="pill pill-neu">{f.protocol}</span>
                            </td>
                            <td>
                              <span className={`pill ${
                                f.action === 'proxy' ? 'pill-pos' : f.action === 'direct' ? 'pill-warn' : 'pill-neg'
                              }`}>
                                {f.action.toUpperCase()}
                              </span>
                            </td>
                            <td>{f.proxyName || '—'}</td>
                            <td className="num-cell">
                              {formatBytes(f.bytesIn)} / {formatBytes(f.bytesOut)}
                            </td>
                            <td className="num-cell">
                              <span className={`pill ${f.state === 'open' ? 'pill-pos' : 'pill-neu'}`}>
                                {f.state}
                              </span>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </>
          )}

          {tab === 'rules' && (
            <>
              <div className="sheet-header">
                <div className="sheet-title-group">
                  <h1 className="sheet-title">Proxy Rules</h1>
                  <span className="sheet-subtitle">Priority-ordered pattern matching & traffic dispatch</span>
                </div>
                <div className="sheet-actions">
                  <input
                    type="text"
                    className="input-text search-input"
                    placeholder="Search rules..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                  <button
                    className="btn btn-sm btn-primary"
                    onClick={() => {
                      setEditingRule({
                        id: '',
                        name: 'New Routing Rule',
                        enabled: true,
                        processNames: '*',
                        targetHosts: '*',
                        targetPorts: '*',
                        protocol: 'both',
                        action: 'proxy',
                        proxyConfigId: profile?.defaultProxyId,
                      })
                      setIsRuleModalOpen(true)
                    }}
                  >
                    + Add Rule
                  </button>
                  <button
                    className="btn btn-sm"
                    onClick={async () => {
                      const json = await app.ExportRulesJSON([])
                      navigator.clipboard.writeText(json)
                      alert('Rules JSON copied to clipboard!')
                    }}
                  >
                    Export
                  </button>
                </div>
              </div>

              <div className="sheet-content">
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th style={{ width: '40px' }}>On</th>
                        <th style={{ width: '50px' }} className="num-cell">#</th>
                        <th>Rule Name</th>
                        <th>Process Names</th>
                        <th>Target Hosts</th>
                        <th>Ports</th>
                        <th>Action</th>
                        <th>Target Proxy</th>
                        <th style={{ width: '180px' }} className="num-cell">Manage</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredRules.map((r, idx) => (
                        <tr key={r.id}>
                          <td>
                            <input
                              type="checkbox"
                              checked={r.enabled}
                              onChange={async (e) => {
                                const updated = await app.ToggleRule(r.id, e.target.checked)
                                setProfile(updated)
                              }}
                            />
                          </td>
                          <td className="num-cell">{idx + 1}</td>
                          <td><strong>{r.name}</strong></td>
                          <td className="mono-cell">{r.processNames}</td>
                          <td className="mono-cell">{r.targetHosts}</td>
                          <td className="mono-cell">{r.targetPorts}</td>
                          <td>
                            <span className={`pill ${
                              r.action === 'proxy' ? 'pill-pos' : r.action === 'direct' ? 'pill-warn' : 'pill-neg'
                            }`}>
                              {r.action.toUpperCase()}
                            </span>
                          </td>
                          <td>
                            {r.action === 'proxy'
                              ? profile?.proxies.find((p) => p.id === r.proxyConfigId)?.name || 'Default Proxy'
                              : '—'}
                          </td>
                          <td className="num-cell">
                            <button
                              className="btn btn-sm"
                              disabled={idx === 0}
                              onClick={async () => {
                                const u = await app.MoveRule(r.id, -1)
                                setProfile(u)
                              }}
                            >
                              ↑
                            </button>{' '}
                            <button
                              className="btn btn-sm"
                              disabled={idx === (profile?.rules.length || 0) - 1}
                              onClick={async () => {
                                const u = await app.MoveRule(r.id, 1)
                                setProfile(u)
                              }}
                            >
                              ↓
                            </button>{' '}
                            <button
                              className="btn btn-sm"
                              onClick={() => {
                                setEditingRule(r)
                                setIsRuleModalOpen(true)
                              }}
                            >
                              Edit
                            </button>{' '}
                            <button
                              className="btn btn-sm btn-danger"
                              onClick={async () => {
                                if (confirm(`Delete rule "${r.name}"?`)) {
                                  const u = await app.DeleteRule(r.id)
                                  setProfile(u)
                                }
                              }}
                            >
                              ×
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </>
          )}

          {tab === 'proxies' && (
            <>
              <div className="sheet-header">
                <div className="sheet-title-group">
                  <h1 className="sheet-title">Proxy Servers</h1>
                  <span className="sheet-subtitle">Upstream SOCKS5, SOCKS4, HTTP and HTTPS proxies</span>
                </div>
                <div className="sheet-actions">
                  <button
                    className="btn btn-sm btn-primary"
                    onClick={() => {
                      setEditingProxy({
                        id: '',
                        name: 'New Proxy Server',
                        type: 'socks5',
                        host: '127.0.0.1',
                        port: 1080,
                        sendDomainToProxy: true,
                      })
                      setIsProxyModalOpen(true)
                    }}
                  >
                    + Add Proxy
                  </button>
                </div>
              </div>

              <div className="sheet-content">
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th style={{ width: '40px' }}>Default</th>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Endpoint</th>
                        <th>Auth</th>
                        <th className="num-cell">Latency</th>
                        <th style={{ width: '220px' }} className="num-cell">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {profile?.proxies.map((px) => {
                        const isDefault = profile.defaultProxyId === px.id
                        return (
                          <tr key={px.id}>
                            <td>
                              <button
                                className={`btn btn-sm ${isDefault ? 'btn-primary' : ''}`}
                                style={{
                                  fontSize: '11px',
                                  padding: '2px 6px',
                                  height: '22px',
                                }}
                                onClick={async () => {
                                  const u = await app.SetDefaultProxy(px.id)
                                  setProfile(u)
                                }}
                                title={isDefault ? 'Default Proxy' : 'Set as Default'}
                              >
                                {isDefault ? 'Default' : 'Set'}
                              </button>
                            </td>
                            <td>
                              <strong>{px.name}</strong>
                              {px.description && (
                                <div style={{ fontSize: '11px', color: 'var(--muted)' }}>
                                  {px.description}
                                </div>
                              )}
                            </td>
                            <td>
                              <span className="pill pill-accent">{px.type.toUpperCase()}</span>
                            </td>
                            <td className="mono-cell">
                              {px.host}:{px.port}
                            </td>
                            <td>{px.username ? 'Credentials set' : 'None'}</td>
                            <td className="num-cell">
                              {px.latencyMs ? `${px.latencyMs}ms` : '—'}
                            </td>
                            <td className="num-cell">
                              <button
                                className="btn btn-sm btn-primary"
                                onClick={() => handleRunProxyTest(px.id)}
                              >
                                Test
                              </button>{' '}
                              <button
                                className="btn btn-sm"
                                onClick={() => {
                                  setEditingProxy(px)
                                  setIsProxyModalOpen(true)
                                }}
                              >
                                Edit
                              </button>{' '}
                              <button
                                className="btn btn-sm btn-danger"
                                onClick={async () => {
                                  if (confirm(`Delete proxy "${px.name}"?`)) {
                                    const u = await app.DeleteProxy(px.id)
                                    setProfile(u)
                                  }
                                }}
                              >
                                Delete
                              </button>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            </>
          )}

          {tab === 'logs' && (
            <>
              <div className="sheet-header">
                <div className="sheet-title-group">
                  <h1 className="sheet-title">Activity Logs</h1>
                  <span className="sheet-subtitle">Historical connection routing decisions and engine events</span>
                </div>
                <div className="sheet-actions">
                  <div style={{ display: 'flex', gap: '4px' }}>
                    <button
                      className={`btn btn-sm ${logSubTab === 'connections' ? 'btn-primary' : ''}`}
                      onClick={() => setLogSubTab('connections')}
                    >
                      Connections ({connLogs.length})
                    </button>
                    <button
                      className={`btn btn-sm ${logSubTab === 'system' ? 'btn-primary' : ''}`}
                      onClick={() => setLogSubTab('system')}
                    >
                      System ({activityLogs.length})
                    </button>
                  </div>
                  <button
                    className="btn btn-sm"
                    onClick={async () => {
                      if (logSubTab === 'connections') {
                        await app.ClearConnectionLogs()
                        setConnLogs([])
                      } else {
                        await app.ClearActivityLogs()
                        setActivityLogs([])
                      }
                    }}
                  >
                    Clear Logs
                  </button>
                </div>
              </div>

              <div className="sheet-content">
                <div className="table-wrap">
                  {logSubTab === 'connections' ? (
                    <table className="data-table">
                      <thead>
                        <tr>
                          <th>Timestamp</th>
                          <th>Process</th>
                          <th>Destination</th>
                          <th>Action</th>
                          <th>Proxy</th>
                          <th className="num-cell">Bytes</th>
                          <th className="num-cell">Duration</th>
                        </tr>
                      </thead>
                      <tbody>
                        {connLogs.map((l) => (
                          <tr key={l.id}>
                            <td className="mono-cell">
                              {new Date(l.timestamp).toLocaleTimeString()}
                            </td>
                            <td>
                              <strong>{l.processName}</strong> (PID {l.pid})
                            </td>
                            <td className="mono-cell">
                              {l.resolvedDomain ? `${l.resolvedDomain} (${l.destIP}:${l.destPort})` : `${l.destIP}:${l.destPort}`}
                            </td>
                            <td>
                              <span className={`pill ${
                                l.action === 'proxy' ? 'pill-pos' : l.action === 'direct' ? 'pill-warn' : 'pill-neg'
                              }`}>
                                {l.action.toUpperCase()}
                              </span>
                            </td>
                            <td>{l.proxyName}</td>
                            <td className="num-cell">
                              {formatBytes(l.bytesIn + l.bytesOut)}
                            </td>
                            <td className="num-cell">{formatDuration(l.durationMs)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  ) : (
                    <table className="data-table">
                      <thead>
                        <tr>
                          <th>Timestamp</th>
                          <th>Level</th>
                          <th>Message</th>
                        </tr>
                      </thead>
                      <tbody>
                        {activityLogs.map((a) => (
                          <tr key={a.id}>
                            <td className="mono-cell">
                              {new Date(a.timestamp).toLocaleTimeString()}
                            </td>
                            <td>
                              <span className={`pill ${
                                a.level === 'ERROR' ? 'pill-neg' : a.level === 'WARN' ? 'pill-warn' : 'pill-neu'
                              }`}>
                                {a.level}
                              </span>
                            </td>
                            <td className="mono-cell">{a.message}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              </div>
            </>
          )}

          {tab === 'settings' && (
            <>
              <div className="sheet-header">
                <div className="sheet-title-group">
                  <h1 className="sheet-title">Settings</h1>
                  <span className="sheet-subtitle">Interception options, DNS handling, and profile persistence</span>
                </div>
              </div>

              <div className="sheet-content" style={{ maxWidth: '640px' }}>
                <div className="kpi-card" style={{ gap: '12px' }}>
                  <div style={{ fontWeight: 600, fontSize: '15px' }}>Runtime Traffic Options</div>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={profile?.trafficLoggingEnabled || false}
                      onChange={async (e) => {
                        if (!profile) return
                        const u = await app.UpdateOptions(
                          e.target.checked,
                          profile.dnsViaProxy,
                          profile.logEveryConnection,
                          profile.autoClearConnectionLogs
                        )
                        setProfile(u)
                      }}
                    />
                    <span>Enable Live Traffic Logging & Metrics</span>
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={profile?.dnsViaProxy || false}
                      onChange={async (e) => {
                        if (!profile) return
                        const u = await app.UpdateOptions(
                          profile.trafficLoggingEnabled,
                          e.target.checked,
                          profile.logEveryConnection,
                          profile.autoClearConnectionLogs
                        )
                        setProfile(u)
                      }}
                    />
                    <span>Resolve DNS queries through upstream proxy (Remote DNS)</span>
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={profile?.logEveryConnection || false}
                      onChange={async (e) => {
                        if (!profile) return
                        const u = await app.UpdateOptions(
                          profile.trafficLoggingEnabled,
                          profile.dnsViaProxy,
                          e.target.checked,
                          profile.autoClearConnectionLogs
                        )
                        setProfile(u)
                      }}
                    />
                    <span>Record every individual connection into Activity Logs</span>
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={profile?.autoClearConnectionLogs || false}
                      onChange={async (e) => {
                        if (!profile) return
                        const u = await app.UpdateOptions(
                          profile.trafficLoggingEnabled,
                          profile.dnsViaProxy,
                          profile.logEveryConnection,
                          e.target.checked
                        )
                        setProfile(u)
                      }}
                    />
                    <span>Auto-clear connection logs periodically</span>
                  </label>
                </div>

                <div className="kpi-card" style={{ gap: '12px' }}>
                  <div style={{ fontWeight: 600, fontSize: '15px' }}>OS Driver Diagnostics</div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px' }}>
                    <span style={{ color: 'var(--muted)' }}>Backend Engine:</span>
                    <strong>{engineStatus?.backend || 'macos-ne'}</strong>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px' }}>
                    <span style={{ color: 'var(--muted)' }}>Extension Installed:</span>
                    <strong>{engineStatus?.extensionInstalled ? 'Yes' : 'No'}</strong>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px' }}>
                    <span style={{ color: 'var(--muted)' }}>DNS Proxy Service:</span>
                    <strong>{engineStatus?.dnsProxyActive ? 'Enabled (Port 53)' : 'Disabled'}</strong>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px' }}>
                    <span style={{ color: 'var(--muted)' }}>Active Flow Count:</span>
                    <strong className="num-cell">{engineStatus?.activeFlows || 0}</strong>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      </main>

      {/* Rule Editor Modal */}
      {isRuleModalOpen && editingRule && (
        <div className="modal-overlay" onClick={() => setIsRuleModalOpen(false)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <form onSubmit={handleSaveRule}>
              <div className="modal-header">
                <div className="modal-title">{editingRule.id ? 'Edit Routing Rule' : 'New Routing Rule'}</div>
                <button type="button" className="btn btn-sm" onClick={() => setIsRuleModalOpen(false)}>×</button>
              </div>
              <div className="modal-body">
                <div className="form-group">
                  <label className="form-label">Rule Name</label>
                  <input
                    type="text"
                    required
                    className="input-text"
                    value={editingRule.name}
                    onChange={(e) => setEditingRule({ ...editingRule, name: e.target.value })}
                  />
                </div>

                <div className="form-group">
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <label className="form-label">Process Names (semicolon-separated, e.g. curl;Chrome;*)</label>
                    <button type="button" className="btn btn-sm" onClick={handlePickProcess}>
                      Browse Executable...
                    </button>
                  </div>
                  <input
                    type="text"
                    required
                    className="input-text mono-cell"
                    value={editingRule.processNames}
                    onChange={(e) => setEditingRule({ ...editingRule, processNames: e.target.value })}
                  />
                </div>

                <div className="form-row">
                  <div className="form-group">
                    <label className="form-label">Target Hosts (IP, CIDR, or *.domain)</label>
                    <input
                      type="text"
                      required
                      className="input-text mono-cell"
                      value={editingRule.targetHosts}
                      onChange={(e) => setEditingRule({ ...editingRule, targetHosts: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label">Ports (e.g. 80,443 or 8000-8080)</label>
                    <input
                      type="text"
                      required
                      className="input-text mono-cell"
                      value={editingRule.targetPorts}
                      onChange={(e) => setEditingRule({ ...editingRule, targetPorts: e.target.value })}
                    />
                  </div>
                </div>

                <div className="form-row">
                  <div className="form-group">
                    <label className="form-label">Protocol</label>
                    <select
                      className="input-text"
                      value={editingRule.protocol}
                      onChange={(e) => setEditingRule({ ...editingRule, protocol: e.target.value as any })}
                    >
                      <option value="both">TCP & UDP</option>
                      <option value="tcp">TCP Only</option>
                      <option value="udp">UDP Only</option>
                    </select>
                  </div>
                  <div className="form-group">
                    <label className="form-label">Action</label>
                    <select
                      className="input-text"
                      value={editingRule.action}
                      onChange={(e) => setEditingRule({ ...editingRule, action: e.target.value as any })}
                    >
                      <option value="proxy">Proxy via Upstream</option>
                      <option value="direct">Direct (Bypass Proxy)</option>
                      <option value="block">Block Connection</option>
                    </select>
                  </div>
                </div>

                {editingRule.action === 'proxy' && (
                  <div className="form-group">
                    <label className="form-label">Route via Proxy Server</label>
                    <select
                      className="input-text"
                      value={editingRule.proxyConfigId || ''}
                      onChange={(e) => setEditingRule({ ...editingRule, proxyConfigId: e.target.value })}
                    >
                      <option value="">Default Profile Proxy</option>
                      {profile?.proxies.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name} ({p.type.toUpperCase()} {p.host}:{p.port})
                        </option>
                      ))}
                    </select>
                  </div>
                )}
              </div>
              <div className="modal-footer">
                <button type="button" className="btn" onClick={() => setIsRuleModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Save Rule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Proxy Editor Modal */}
      {isProxyModalOpen && editingProxy && (
        <div className="modal-overlay" onClick={() => setIsProxyModalOpen(false)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <form onSubmit={handleSaveProxy}>
              <div className="modal-header">
                <div className="modal-title">{editingProxy.id ? 'Edit Proxy Server' : 'Add Proxy Server'}</div>
                <button type="button" className="btn btn-sm" onClick={() => setIsProxyModalOpen(false)}>×</button>
              </div>
              <div className="modal-body">
                <div className="form-group">
                  <label className="form-label">Proxy Name</label>
                  <input
                    type="text"
                    required
                    className="input-text"
                    value={editingProxy.name}
                    onChange={(e) => setEditingProxy({ ...editingProxy, name: e.target.value })}
                  />
                </div>

                <div className="form-row">
                  <div className="form-group">
                    <label className="form-label">Proxy Type</label>
                    <select
                      className="input-text"
                      value={editingProxy.type}
                      onChange={(e) => setEditingProxy({ ...editingProxy, type: e.target.value as any })}
                    >
                      <option value="socks5">SOCKS5</option>
                      <option value="socks4">SOCKS4</option>
                      <option value="http">HTTP</option>
                      <option value="https">HTTPS</option>
                    </select>
                  </div>
                  <div className="form-group">
                    <label className="form-label">Host or IP</label>
                    <input
                      type="text"
                      required
                      className="input-text mono-cell"
                      value={editingProxy.host}
                      onChange={(e) => setEditingProxy({ ...editingProxy, host: e.target.value })}
                    />
                  </div>
                  <div className="form-group" style={{ maxWidth: '100px' }}>
                    <label className="form-label">Port</label>
                    <input
                      type="number"
                      required
                      className="input-text mono-cell"
                      value={editingProxy.port}
                      onChange={(e) => setEditingProxy({ ...editingProxy, port: parseInt(e.target.value) || 0 })}
                    />
                  </div>
                </div>

                <div className="form-row">
                  <div className="form-group">
                    <label className="form-label">Username (optional)</label>
                    <input
                      type="text"
                      className="input-text"
                      value={editingProxy.username || ''}
                      onChange={(e) => setEditingProxy({ ...editingProxy, username: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label">Password (optional)</label>
                    <input
                      type="password"
                      className="input-text"
                      placeholder="••••••••"
                      value={editingProxy.password || ''}
                      onChange={(e) => setEditingProxy({ ...editingProxy, password: e.target.value })}
                    />
                  </div>
                </div>

                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px' }}>
                  <input
                    type="checkbox"
                    checked={editingProxy.sendDomainToProxy || false}
                    onChange={(e) => setEditingProxy({ ...editingProxy, sendDomainToProxy: e.target.checked })}
                  />
                  <span>Send domain name directly to proxy (Remote DNS)</span>
                </label>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn" onClick={() => setIsProxyModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Save Proxy
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 5-Step Proxy Checker Modal */}
      {testingProxyId && (
        <div className="modal-overlay" onClick={() => !isCheckingProxy && setTestingProxyId(null)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <div className="modal-title">5-Step Proxy Diagnostic Check</div>
              <button
                type="button"
                className="btn btn-sm"
                disabled={isCheckingProxy}
                onClick={() => setTestingProxyId(null)}
              >
                ×
              </button>
            </div>
            <div className="modal-body">
              {isCheckingProxy && (
                <div style={{ textAlign: 'center', padding: '16px' }}>
                  <span className="pulse-dot" style={{ background: 'var(--accent)' }} /> Running 5-step diagnostic check...
                </div>
              )}
              {checkReport && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
                  <div className={`sidebar-status-pill ${checkReport.ok ? 'pill-pos' : 'pill-neg'}`}>
                    {checkReport.ok ? 'All 5 Steps Passed' : 'Diagnostic Check Failed'}
                  </div>
                  <div className="check-steps-list">
                    {checkReport.steps.map((st, i) => (
                      <div key={i} className="check-step-item">
                        <div className="check-step-left">
                          <span className={`pill ${st.status === 'ok' ? 'pill-pos' : st.status === 'failed' ? 'pill-neg' : 'pill-neu'}`}>
                            {st.status.toUpperCase()}
                          </span>
                          <div>
                            <div className="check-step-title">{st.name}</div>
                            <div className="check-step-detail">{st.description}</div>
                          </div>
                        </div>
                        {st.durationMs !== undefined && (
                          <span className="num-cell" style={{ fontSize: '12px' }}>
                            {st.durationMs}ms
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                  <div style={{ fontSize: '13px', color: 'var(--muted)', background: 'var(--raised)', padding: '10px', borderRadius: '6px' }}>
                    <strong>Summary:</strong> {checkReport.summary}
                  </div>
                </div>
              )}
            </div>
            <div className="modal-footer">
              <button
                className="btn"
                disabled={isCheckingProxy}
                onClick={() => setTestingProxyId(null)}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* New Profile Modal */}
      {isProfileModalOpen && (
        <div className="modal-overlay" onClick={() => setIsProfileModalOpen(false)}>
          <div className="modal-card" style={{ width: '380px' }} onClick={(e) => e.stopPropagation()}>
            <form
              onSubmit={async (e) => {
                e.preventDefault()
                if (!newProfileName.trim()) return
                try {
                  await app.CreateProfile(newProfileName.trim())
                  setIsProfileModalOpen(false)
                  refreshAll()
                } catch (err) {
                  alert(String(err))
                }
              }}
            >
              <div className="modal-header">
                <div className="modal-title">Create New Profile</div>
                <button type="button" className="btn btn-sm" onClick={() => setIsProfileModalOpen(false)}>×</button>
              </div>
              <div className="modal-body">
                <div className="form-group">
                  <label className="form-label">Profile Name</label>
                  <input
                    type="text"
                    required
                    autoFocus
                    className="input-text"
                    placeholder="e.g. Work-VPN, Pentest-Staging"
                    value={newProfileName}
                    onChange={(e) => setNewProfileName(e.target.value)}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn" onClick={() => setIsProfileModalOpen(false)}>Cancel</button>
                <button type="submit" className="btn btn-primary">Create Profile</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
export default App
