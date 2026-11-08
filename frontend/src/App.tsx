// SPDX-License-Identifier: MIT

import React, { useEffect, useState, useMemo, useRef } from 'react'
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
  ReleaseInfo,
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

  const [newProfileName, setNewProfileName] = useState('')
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false)

  const [isRenameProfileOpen, setIsRenameProfileOpen] = useState(false)
  const [renameProfileName, setRenameProfileName] = useState('')

  const [isDuplicateProfileOpen, setIsDuplicateProfileOpen] = useState(false)
  const [duplicateProfileName, setDuplicateProfileName] = useState('')

  const [isLogFiltersModalOpen, setIsLogFiltersModalOpen] = useState(false)
  const [newFilterMode, setNewFilterMode] = useState<'EXCLUDE' | 'INCLUDE'>('EXCLUDE')
  const [newFilterType, setNewFilterType] = useState<'host' | 'process' | 'port'>('host')
  const [newFilterPattern, setNewFilterPattern] = useState('')

  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false)
  const profileMenuRef = useRef<HTMLDivElement>(null)

  const profileFileInputRef = useRef<HTMLInputElement>(null)
  const rulesFileInputRef = useRef<HTMLInputElement>(null)

  const [updateInfo, setUpdateInfo] = useState<ReleaseInfo | null>(null)
  const [isCheckingUpdate, setIsCheckingUpdate] = useState(false)

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
    const handleClickOutside = (e: MouseEvent) => {
      if (profileMenuRef.current && !profileMenuRef.current.contains(e.target as Node)) {
        setIsProfileMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

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

  const handleCheckForUpdates = async () => {
    setIsCheckingUpdate(true)
    try {
      const info = await app.CheckForUpdates()
      setUpdateInfo(info)
    } catch (err) {
      setUpdateInfo({
        currentVersion: '1.0.0',
        latestVersion: '1.0.0',
        isUpdateAvailable: false,
        error: String(err),
      })
    } finally {
      setIsCheckingUpdate(false)
    }
  }

  const handlePickProcess = async () => {
    try {
      const path = await app.PickProcessExecutable()
      if (path && editingRule) {
        const normalized = path.replace(/\\/g, '/')
        const parts = normalized.split('/')
        const name = parts[parts.length - 1].replace('.app', '').replace('.exe', '')
        const current = editingRule.processNames.trim()
        const next = current === '*' || current === '' ? name : `${current};${name}`
        setEditingRule({ ...editingRule, processNames: next })
      }
    } catch (err) {
      console.error(err)
    }
  }

  const handleRenameProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!renameProfileName.trim()) return
    try {
      const updated = await app.RenameProfile(profileSummary.active, renameProfileName.trim())
      setProfile(updated)
      setIsRenameProfileOpen(false)
      await refreshAll()
    } catch (err) {
      alert(String(err))
    }
  }

  const handleDuplicateProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!duplicateProfileName.trim()) return
    try {
      const updated = await app.DuplicateProfile(profileSummary.active, duplicateProfileName.trim())
      setProfile(updated)
      setIsDuplicateProfileOpen(false)
      await refreshAll()
    } catch (err) {
      alert(String(err))
    }
  }

  const handleDeleteProfile = async () => {
    if (profileSummary.names.length <= 1) {
      alert('Cannot delete the only profile.')
      return
    }
    if (profileSummary.active === 'Default') {
      alert('Cannot delete the Default profile.')
      return
    }
    if (!confirm(`Are you sure you want to delete profile "${profileSummary.active}"?`)) return
    try {
      const next = await app.DeleteProfile(profileSummary.active)
      setProfile(next)
      await refreshAll()
    } catch (err) {
      alert(String(err))
    }
  }

  const handleExportProfile = async () => {
    try {
      const json = await app.ExportProfileJSON(profileSummary.active, false)
      const blob = new Blob([json], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${profileSummary.active}.pbprofile`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      alert(String(err))
    }
  }

  const handleImportProfileClick = () => {
    profileFileInputRef.current?.click()
  }

  const handleImportProfileFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const updated = await app.ImportProfileJSON(text, true)
      setProfile(updated)
      await refreshAll()
      alert(`Profile "${updated.name}" imported successfully!`)
    } catch (err) {
      alert('Failed to import profile: ' + String(err))
    } finally {
      e.target.value = ''
    }
  }

  const handleCloneRule = async (ruleId: string) => {
    try {
      const updated = await app.CloneRule(ruleId)
      setProfile(updated)
    } catch (err) {
      alert(String(err))
    }
  }

  const handleExportRules = async () => {
    try {
      const json = await app.ExportRulesJSON([])
      const blob = new Blob([json], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${profileSummary.active}-rules.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      alert(String(err))
    }
  }

  const handleImportRulesClick = () => {
    rulesFileInputRef.current?.click()
  }

  const handleImportRulesFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const updated = await app.ImportRulesJSON(text)
      setProfile(updated)
      alert('Rules imported successfully!')
    } catch (err) {
      alert('Failed to import rules: ' + String(err))
    } finally {
      e.target.value = ''
    }
  }

  const handleCreateRuleFromFlow = (f: Flow) => {
    const domain = f.resolvedDomain
    const hostPattern = domain
      ? (domain.startsWith('www.') ? `*.${domain.slice(4)}` : domain)
      : f.destIP
    const ruleName = `Route ${f.processName || domain || f.destIP}`
    setEditingRule({
      id: '',
      name: ruleName,
      enabled: true,
      processNames: f.processName || '*',
      targetHosts: hostPattern || '*',
      targetPorts: String(f.destPort || '*'),
      protocol: f.protocol.toLowerCase() === 'udp' ? 'udp' : 'tcp',
      action: profile?.defaultProxyId ? 'proxy' : 'direct',
      proxyConfigId: profile?.defaultProxyId || '',
    })
    setIsRuleModalOpen(true)
    setTab('rules')
  }

  const handleAddLogFilter = async () => {
    if (!newFilterPattern.trim() || !profile) return
    const currentFilters = profile.logFilters || []
    const newFilter: LogFilter = {
      id: 'flt_' + Date.now().toString(36),
      mode: newFilterMode,
      host: newFilterType === 'host' ? newFilterPattern.trim() : '*',
      process: newFilterType === 'process' ? newFilterPattern.trim() : '*',
      port: newFilterType === 'port' ? newFilterPattern.trim() : '*',
      protocol: 'ALL',
      action: 'ALL',
    }
    try {
      const updated = await app.SaveLogFilters([...currentFilters, newFilter])
      setProfile(updated)
      setNewFilterPattern('')
    } catch (err) {
      alert(String(err))
    }
  }

  const handleRemoveLogFilter = async (id: string) => {
    if (!profile) return
    const next = (profile.logFilters || []).filter((f) => f.id !== id)
    try {
      const updated = await app.SaveLogFilters(next)
      setProfile(updated)
    } catch (err) {
      alert(String(err))
    }
  }

  return (
    <div className="app-frame">
      {/* Sidebar */}
      <aside className="app-sidebar">
        <div className="brand-row">
          <div className="brand-mark-svg">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
              <path d="M12 2L3 7V12C3 17.52 6.84 22.74 12 24C17.16 22.74 21 17.52 21 12V7L12 2Z" fill="url(#brandGrad)" />
              <path d="M8 12L11 9M11 9L14 12M11 9V15M16 12L13 15M13 15L10 12M13 15V9" stroke="#ffffff" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
              <defs>
                <linearGradient id="brandGrad" x1="3" y1="2" x2="21" y2="24" gradientUnits="userSpaceOnUse">
                  <stop stopColor="#3b82f6"/>
                  <stop offset="1" stopColor="#1d4ed8"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
          <div className="brand-title">AnyProxyXa</div>
          <span className="brand-badge">Pro</span>
        </div>

        <nav className="nav-group">
          <button
            className={`nav-item ${tab === 'connections' ? 'active' : ''}`}
            onClick={() => setTab('connections')}
          >
            <span className="nav-item-main">
              <svg className="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10"/>
                <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
              </svg>
              <span>Connections</span>
            </span>
            <span className="nav-item-count">{flows.filter((f) => f.state === 'open').length}</span>
          </button>
          <button
            className={`nav-item ${tab === 'rules' ? 'active' : ''}`}
            onClick={() => setTab('rules')}
          >
            <span className="nav-item-main">
              <svg className="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
              </svg>
              <span>Proxy Rules</span>
            </span>
            <span className="nav-item-count">{profile?.rules.length || 0}</span>
          </button>
          <button
            className={`nav-item ${tab === 'proxies' ? 'active' : ''}`}
            onClick={() => setTab('proxies')}
          >
            <span className="nav-item-main">
              <svg className="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
                <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
                <line x1="6" y1="6" x2="6.01" y2="6"/>
                <line x1="6" y1="18" x2="6.01" y2="18"/>
              </svg>
              <span>Proxy Servers</span>
            </span>
            <span className="nav-item-count">{profile?.proxies.length || 0}</span>
          </button>
          <button
            className={`nav-item ${tab === 'logs' ? 'active' : ''}`}
            onClick={() => setTab('logs')}
          >
            <span className="nav-item-main">
              <svg className="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="8" y1="6" x2="21" y2="6"/>
                <line x1="8" y1="12" x2="21" y2="12"/>
                <line x1="8" y1="18" x2="21" y2="18"/>
                <line x1="3" y1="6" x2="3.01" y2="6"/>
                <line x1="3" y1="12" x2="3.01" y2="12"/>
                <line x1="3" y1="18" x2="3.01" y2="18"/>
              </svg>
              <span>Activity Logs</span>
            </span>
            <span className="nav-item-count">{activityLogs.length + connLogs.length}</span>
          </button>
          <button
            className={`nav-item ${tab === 'settings' ? 'active' : ''}`}
            onClick={() => setTab('settings')}
          >
            <span className="nav-item-main">
              <svg className="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="3"/>
                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
              </svg>
              <span>Settings</span>
            </span>
          </button>
        </nav>

        <div className="sidebar-status-card">
          <div className="sidebar-status-header">
            <span className="sidebar-status-title">Driver Engine</span>
            <div className={`status-dot ${engineStatus?.state === 'running' ? 'dot-pos' : engineStatus?.state === 'paused' ? 'dot-warn' : 'dot-neg'}`} />
          </div>
          <div className="sidebar-status-name">
            {engineStatus?.backend || 'macos-ne'} • <span className="status-state-text">{engineStatus?.state || 'stopped'}</span>
          </div>
          <div className="sidebar-status-sub">
            DNS Interception: <span className={engineStatus?.dnsProxyActive ? 'text-pos' : 'text-muted'}>{engineStatus?.dnsProxyActive ? 'Active' : 'Disabled'}</span>
          </div>
        </div>
      </aside>

      {/* Main Area */}
      <main className="app-main">
        {/* Topbar */}
        <header className="app-topbar">
          <input
            type="file"
            ref={profileFileInputRef}
            style={{ display: 'none' }}
            accept=".pbprofile,.json"
            onChange={handleImportProfileFile}
          />
          <input
            type="file"
            ref={rulesFileInputRef}
            style={{ display: 'none' }}
            accept=".json"
            onChange={handleImportRulesFile}
          />

          <div className="topbar-left">
            <div className="profile-switcher-pill" ref={profileMenuRef}>
              <span className="profile-pill-label">Profile</span>
              <div className="profile-select-container">
                <select
                  className="profile-native-select"
                  value={profileSummary.active}
                  onChange={(e) => handleSwitchProfile(e.target.value)}
                  aria-label="Select active profile"
                >
                  {profileSummary.names.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
                <span className="profile-select-chevron">▾</span>
              </div>

              <button
                className="btn btn-sm btn-ghost profile-action-btn"
                onClick={() => {
                  setNewProfileName('')
                  setIsProfileModalOpen(true)
                }}
                title="Create new profile"
              >
                + New
              </button>

              <button
                className="btn btn-sm btn-icon profile-action-btn"
                onClick={() => setIsProfileMenuOpen(!isProfileMenuOpen)}
                title="Profile actions"
                aria-label="Profile actions"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                  <circle cx="12" cy="5" r="2.2"/>
                  <circle cx="12" cy="12" r="2.2"/>
                  <circle cx="12" cy="19" r="2.2"/>
                </svg>
              </button>

              {isProfileMenuOpen && (
                <div className="dropdown-menu">
                  <div className="dropdown-title">Profile Actions</div>
                  <button
                    className="dropdown-item"
                    onClick={() => {
                      setIsProfileMenuOpen(false)
                      setRenameProfileName(profileSummary.active)
                      setIsRenameProfileOpen(true)
                    }}
                  >
                    Rename Profile…
                  </button>
                  <button
                    className="dropdown-item"
                    onClick={() => {
                      setIsProfileMenuOpen(false)
                      setDuplicateProfileName(`${profileSummary.active} Copy`)
                      setIsDuplicateProfileOpen(true)
                    }}
                  >
                    Duplicate Profile…
                  </button>
                  <div className="dropdown-divider" />
                  <button
                    className="dropdown-item"
                    onClick={() => {
                      setIsProfileMenuOpen(false)
                      handleExportProfile()
                    }}
                  >
                    Export Profile (.pbprofile)…
                  </button>
                  <button
                    className="dropdown-item"
                    onClick={() => {
                      setIsProfileMenuOpen(false)
                      handleImportProfileClick()
                    }}
                  >
                    Import Profile (.pbprofile)…
                  </button>
                  {profileSummary.names.length > 1 && profileSummary.active !== 'Default' && (
                    <>
                      <div className="dropdown-divider" />
                      <button
                        className="dropdown-item text-danger"
                        onClick={() => {
                          setIsProfileMenuOpen(false)
                          handleDeleteProfile()
                        }}
                      >
                        Delete Profile
                      </button>
                    </>
                  )}
                </div>
              )}
            </div>
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
                        <th className="num-cell" style={{ width: '70px' }}>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredFlows.length === 0 ? (
                        <tr>
                          <td colSpan={8} className="empty-state">
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
                            <td className="num-cell">
                              <button
                                className="btn btn-sm"
                                title="Create routing rule from this flow"
                                onClick={() => handleCreateRuleFromFlow(f)}
                              >
                                + Rule
                              </button>
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
                    title="Import rules from a JSON file"
                    onClick={handleImportRulesClick}
                  >
                    Import
                  </button>
                  <button
                    className="btn btn-sm"
                    title="Export all rules to a JSON file"
                    onClick={handleExportRules}
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
                        <th style={{ width: '220px' }} className="num-cell">Manage</th>
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
                              title="Move rule up"
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
                              title="Move rule down"
                              onClick={async () => {
                                const u = await app.MoveRule(r.id, 1)
                                setProfile(u)
                              }}
                            >
                              ↓
                            </button>{' '}
                            <button
                              className="btn btn-sm"
                              title="Duplicate rule"
                              onClick={() => handleCloneRule(r.id)}
                            >
                              Clone
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
                    title="Manage connection log inclusion and exclusion filters"
                    onClick={() => setIsLogFiltersModalOpen(true)}
                  >
                    Log Filters ({profile?.logFilters?.length || 0})
                  </button>
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
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ fontWeight: 600, fontSize: '15px' }}>Software Version & Updates</div>
                    <span className="pill pill-neu">v1.0.0</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontSize: '13px', color: 'var(--muted)' }}>
                      AnyProxyXa Core Engine & Desktop Client
                    </span>
                    <button
                      type="button"
                      className="btn btn-sm"
                      disabled={isCheckingUpdate}
                      onClick={handleCheckForUpdates}
                    >
                      {isCheckingUpdate ? 'Checking...' : 'Check for Updates'}
                    </button>
                  </div>

                  {updateInfo && (
                    <div style={{ borderTop: '1px solid var(--rule)', paddingTop: '10px', fontSize: '13px' }}>
                      {updateInfo.error ? (
                        <div style={{ color: 'var(--negative)' }}>
                          Update check failed: {updateInfo.error}
                        </div>
                      ) : updateInfo.isUpdateAvailable ? (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                            <span className="pill pill-pos">Update Available: v{updateInfo.latestVersion}</span>
                            {updateInfo.releaseDate && (
                              <span style={{ color: 'var(--muted)', fontSize: '12px' }}>
                                Released {updateInfo.releaseDate}
                              </span>
                            )}
                          </div>
                          {updateInfo.releaseNotes && (
                            <div style={{ color: 'var(--text)', fontSize: '12px', marginTop: '2px' }}>
                              {updateInfo.releaseNotes}
                            </div>
                          )}
                          {updateInfo.downloadUrl && (
                            <div style={{ marginTop: '4px' }}>
                              <a
                                href={updateInfo.downloadUrl}
                                target="_blank"
                                rel="noreferrer"
                                className="btn btn-sm btn-primary"
                                style={{ display: 'inline-block', textDecoration: 'none' }}
                              >
                                Download v{updateInfo.latestVersion}
                              </a>
                            </div>
                          )}
                        </div>
                      ) : (
                        <div style={{ color: 'var(--positive)', display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <span className="pill pill-pos">Up to date</span>
                          <span>You are running the latest version of AnyProxyXa.</span>
                        </div>
                      )}
                    </div>
                  )}
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

      {/* Rename Profile Modal */}
      {isRenameProfileOpen && (
        <div className="modal-overlay" onClick={() => setIsRenameProfileOpen(false)}>
          <div className="modal-card" style={{ width: '380px' }} onClick={(e) => e.stopPropagation()}>
            <form onSubmit={handleRenameProfile}>
              <div className="modal-header">
                <div className="modal-title">Rename Profile</div>
                <button type="button" className="btn btn-sm" onClick={() => setIsRenameProfileOpen(false)}>×</button>
              </div>
              <div className="modal-body">
                <div className="form-group">
                  <label className="form-label">New Profile Name</label>
                  <input
                    type="text"
                    required
                    autoFocus
                    className="input-text"
                    value={renameProfileName}
                    onChange={(e) => setRenameProfileName(e.target.value)}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn" onClick={() => setIsRenameProfileOpen(false)}>Cancel</button>
                <button type="submit" className="btn btn-primary">Save Name</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Duplicate Profile Modal */}
      {isDuplicateProfileOpen && (
        <div className="modal-overlay" onClick={() => setIsDuplicateProfileOpen(false)}>
          <div className="modal-card" style={{ width: '380px' }} onClick={(e) => e.stopPropagation()}>
            <form onSubmit={handleDuplicateProfile}>
              <div className="modal-header">
                <div className="modal-title">Duplicate Profile</div>
                <button type="button" className="btn btn-sm" onClick={() => setIsDuplicateProfileOpen(false)}>×</button>
              </div>
              <div className="modal-body">
                <div className="form-group">
                  <label className="form-label">New Profile Name</label>
                  <input
                    type="text"
                    required
                    autoFocus
                    className="input-text"
                    value={duplicateProfileName}
                    onChange={(e) => setDuplicateProfileName(e.target.value)}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn" onClick={() => setIsDuplicateProfileOpen(false)}>Cancel</button>
                <button type="submit" className="btn btn-primary">Duplicate</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Log Filters Modal */}
      {isLogFiltersModalOpen && (
        <div className="modal-overlay" onClick={() => setIsLogFiltersModalOpen(false)}>
          <div className="modal-card modal-card-lg" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <div>
                <div className="modal-title">Connection Log Filters</div>
                <div style={{ fontSize: '12px', color: 'var(--muted)', marginTop: '2px' }}>
                  Filter background telemetry or exclude noisy IPs and ports from connection history.
                </div>
              </div>
              <button type="button" className="btn btn-sm" onClick={() => setIsLogFiltersModalOpen(false)}>×</button>
            </div>
            <div className="modal-body">
              <div className="filter-list-wrap">
                {!profile?.logFilters || profile.logFilters.length === 0 ? (
                  <div style={{ padding: '16px', textAlign: 'center', color: 'var(--muted)', fontSize: '13px' }}>
                    No active log filters. All connections are currently logged.
                  </div>
                ) : (
                  profile.logFilters.map((f) => (
                    <div key={f.id} className="filter-item-row">
                      <div className="filter-item-desc">
                        <span className={`pill ${f.mode === 'INCLUDE' || f.mode === 'include' ? 'pill-pos' : 'pill-neg'}`}>
                          {f.mode.toUpperCase()}
                        </span>
                        <span className="mono-cell">
                          {f.host && f.host !== '*' && `Host: ${f.host} `}
                          {f.process && f.process !== '*' && `Process: ${f.process} `}
                          {f.port && f.port !== '*' && `Port: ${f.port} `}
                        </span>
                      </div>
                      <button
                        type="button"
                        className="btn btn-sm btn-danger"
                        title="Delete filter"
                        onClick={() => handleRemoveLogFilter(f.id)}
                      >
                        ×
                      </button>
                    </div>
                  ))
                )}
              </div>

              <div style={{ borderTop: '1px solid var(--rule)', paddingTop: '12px' }}>
                <div style={{ fontSize: '12px', fontWeight: 600, marginBottom: '8px' }}>Add Filter Rule</div>
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                  <select
                    className="input-text"
                    style={{ width: '120px' }}
                    value={newFilterMode}
                    onChange={(e) => setNewFilterMode(e.target.value as 'EXCLUDE' | 'INCLUDE')}
                  >
                    <option value="EXCLUDE">EXCLUDE</option>
                    <option value="INCLUDE">INCLUDE</option>
                  </select>
                  <select
                    className="input-text"
                    style={{ width: '140px' }}
                    value={newFilterType}
                    onChange={(e) => setNewFilterType(e.target.value as 'host' | 'process' | 'port')}
                  >
                    <option value="host">Host / Domain</option>
                    <option value="process">Process Name</option>
                    <option value="port">Port</option>
                  </select>
                  <input
                    type="text"
                    className="input-text mono-cell"
                    style={{ flex: 1, minWidth: '160px' }}
                    placeholder={
                      newFilterType === 'host'
                        ? 'e.g. *.telemetry.* or 127.0.0.1'
                        : newFilterType === 'process'
                        ? 'e.g. curl or chrome'
                        : 'e.g. 53, 443, or 8000-8080'
                    }
                    value={newFilterPattern}
                    onChange={(e) => setNewFilterPattern(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        handleAddLogFilter()
                      }
                    }}
                  />
                  <button type="button" className="btn btn-sm btn-primary" onClick={handleAddLogFilter}>
                    + Add
                  </button>
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <button type="button" className="btn" onClick={() => setIsLogFiltersModalOpen(false)}>
                Done
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
export default App
