// SPDX-License-Identifier: MIT
import React, { useEffect, useState } from 'react'
import { api } from './api'
import { t } from './i18n'
import { normalizeConnectionLog, normalizeFlow } from './lib/normalize-traffic'
import { Sidebar, type Page } from './components/sidebar'
import { CompactView } from './components/compact-view'
import { StatusBar } from './components/status-bar'
import { ProxiesPage } from './pages/proxies-page'
import { RulesPage } from './pages/rules-page'
import { ConnectionsPage } from './pages/connections-page'
import { ActivityPage } from './pages/activity-page'
import { SettingsPage } from './pages/settings-page'
import type {
  Profile,
  ProfileListSummary,
  EngineStatus,
  Flow,
  ConnectionLogEntry,
  ActivityLogEntry,
  ProxyConfig,
  ProxyRule,
} from './types'

export default function App() {
  const [page, setPage] = useState<Page>('connections')
  const [compact, setCompact] = useState(false)
  const [alwaysOnTop, setAlwaysOnTop] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [profile, setProfile] = useState<Profile | null>(null)
  const [profiles, setProfiles] = useState<ProfileListSummary>({ names: [], active: '' })
  const [engine, setEngine] = useState<EngineStatus | null>(null)
  const [flows, setFlows] = useState<Flow[]>([])
  const [connLogs, setConnLogs] = useState<ConnectionLogEntry[]>([])
  const [activityLogs, setActivityLogs] = useState<ActivityLogEntry[]>([])
  const [toasts, setToasts] = useState<{ id: string; type: string; msg: string }[]>([])

  const toast = (msg: string, type: 'info' | 'success' | 'error' = 'info') => {
    const id = Math.random().toString(36).slice(2)
    setToasts((prev) => [...prev, { id, type, msg }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((item) => item.id !== id))
    }, 4000)
  }

  const app = api.getApp()

  const reloadData = async (silent = false): Promise<boolean> => {
    try {
      const p = await app.ActiveProfile()
      setProfile(p)
      const list = await app.ListProfiles()
      setProfiles(list)
      const st = await app.Status()
      setEngine(st)
      return true
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : String(e)
      if (silent && message.includes('service is not initialized')) return false
      toast(message, 'error')
      return true
    }
  }

  useEffect(() => {
    let cancelled = false
    let startupTimer: ReturnType<typeof setTimeout>
    const waitForStartup = async () => {
      if (!(await reloadData(true)) && !cancelled) {
        startupTimer = setTimeout(waitForStartup, 1000)
      }
    }
    void waitForStartup()
    const timer = setInterval(async () => {
      try {
        const [fl, cl, al] = await Promise.all([
          app.GetFlows(100),
          app.GetConnectionLogs(100),
          app.GetActivityLogs(100),
        ])
        setFlows((fl ?? []).map(normalizeFlow))
        setConnLogs((cl ?? []).map(normalizeConnectionLog))
        setActivityLogs(al ?? [])
      } catch {}
    }, 1200)

    const unsubProfile = api.subscribeEvent('profile:changed', (data: unknown) => {
      if (data) setProfile(data as Profile)
    })
    const unsubEngine = api.subscribeEvent('engine:status', (data: unknown) => {
      if (data) setEngine(data as EngineStatus)
    })

    return () => {
      cancelled = true
      clearTimeout(startupTimer)
      clearInterval(timer)
      unsubProfile()
      unsubEngine()
    }
  }, [])

  const handleEnterCompact = async () => {
    setCompact(true)
    try {
      await app.SetCompactMode(true, alwaysOnTop)
    } catch {}
  }

  const handleExpandFromCompact = async (targetPage?: Page) => {
    if (targetPage) setPage(targetPage)
    setCompact(false)
    try {
      await app.SetCompactMode(false, false)
    } catch {}
  }

  const handleToggleAlwaysOnTop = async () => {
    const next = !alwaysOnTop
    setAlwaysOnTop(next)
    try {
      await app.SetCompactMode(true, next)
    } catch {}
  }

  const handleToggleEngine = async () => {
    try {
      const isRunning = engine?.state === 'running'
      const next = isRunning ? await app.StopEngine() : await app.StartEngine()
      setEngine(next)
      toast(isRunning ? t.messages.engineStopped : t.messages.engineStarted, 'success')
    } catch (e: unknown) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const handleSetDefaultProxy = async (id: string) => {
    try {
      const updated = await app.SetDefaultProxy(id)
      setProfile(updated)
      toast(t.messages.defaultUpdated, 'success')
    } catch (e: unknown) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const handleTestProxy = async (px: ProxyConfig) => {
    toast(`${t.messages.testing} ${px.name}...`, 'info')
    try {
      const rep = await app.TestProxy(px.id, 'httpbin.org', 80)
      const success = rep.success ?? rep.ok
      toast(
        success
          ? `${t.messages.testSuccess}: ${rep.avgMs ?? rep.totalDurationMs} ms`
          : `${t.messages.testFailed}: ${rep.summary}`,
        success ? 'success' : 'error'
      )
      void reloadData()
      return rep
    } catch (e: unknown) {
      toast(`${t.messages.testError}: ${e instanceof Error ? e.message : String(e)}`, 'error')
    }
  }

  const handleSaveRule = async (r: ProxyRule) => {
    const updated = await app.SaveRule(r)
    setProfile(updated)
    toast(t.messages.ruleSaved, 'success')
  }

  const handleCreateProfile = async () => {
    const name = prompt(t.messages.profilePrompt)
    if (name?.trim()) {
      const p = await app.CreateProfile(name.trim())
      setProfile(p)
      void reloadData()
      toast(t.messages.profileCreated, 'success')
    }
  }

  const knownApps = Array.from(
    new Set([
      ...flows.map((f) => f.processName),
      ...connLogs.map((l) => l.processName),
    ])
  ).filter(Boolean)

  if (compact) {
    return (
      <>
        <CompactView
          engine={engine}
          profile={profile}
          profiles={profiles.names.length > 0 ? profiles.names : [profile?.name || 'Default']}
          flows={flows}
          alwaysOnTop={alwaysOnTop}
          onToggleEngine={handleToggleEngine}
          onSetDefaultProxy={handleSetDefaultProxy}
          onTestProxy={handleTestProxy}
          onSwitchProfile={async (name) => {
            const p = await app.SwitchProfile(name)
            setProfile(p)
            setProfiles((prev) => ({ ...prev, active: name }))
          }}
          onToggleAlwaysOnTop={() => void handleToggleAlwaysOnTop()}
          onExpand={(targetPage) => void handleExpandFromCompact(targetPage)}
        />
        <div className="toasts">
          {toasts.map((item) => (
            <div key={item.id} className={`toast ${item.type}`}>
              <span>{item.msg}</span>
            </div>
          ))}
        </div>
      </>
    )
  }

  return (
    <div className="app">
      <Sidebar
        page={page}
        onNavigate={setPage}
        engine={engine}
        onToggleEngine={handleToggleEngine}
        profiles={profiles}
        onSwitchProfile={async (name) => {
          const p = await app.SwitchProfile(name)
          setProfile(p)
          setProfiles((prev) => ({ ...prev, active: name }))
        }}
        onCreateProfile={handleCreateProfile}
        proxies={profile?.proxies || []}
        defaultProxyId={profile?.defaultProxyId || ''}
        onSetDefaultProxy={handleSetDefaultProxy}
        onTestProxy={handleTestProxy}
        liveFlowCount={flows.filter((f) => f.state === 'open').length}
        rulesCount={profile?.rules?.length || 0}
        collapsed={sidebarCollapsed}
        onToggleCollapsed={() => setSidebarCollapsed((prev) => !prev)}
        onEnterCompact={() => void handleEnterCompact()}
      />

      <main className="main">
        {page === 'connections' && (
          <ConnectionsPage
            flows={flows}
            logs={connLogs}
            onClearClosed={async () => {
              await app.ClearClosedFlows()
              setFlows([])
            }}
            onClearLogs={async () => {
              await app.ClearConnectionLogs()
              setConnLogs([])
            }}
            onQuickCreateRule={handleSaveRule}
          />
        )}

        {page === 'activity' && (
          <ActivityPage
            logs={activityLogs}
            onClearLogs={async () => {
              await app.ClearActivityLogs()
              setActivityLogs([])
            }}
          />
        )}

        {page === 'proxies' && (
          <ProxiesPage
            proxies={profile?.proxies || []}
            defaultProxyId={profile?.defaultProxyId || ''}
            onSaveProxy={async (px) => {
              const updated = await app.SaveProxy(px)
              setProfile(updated)
              toast(t.messages.proxySaved, 'success')
            }}
            onDeleteProxy={async (id) => {
              const updated = await app.DeleteProxy(id)
              setProfile(updated)
              toast(t.messages.proxyDeleted, 'info')
            }}
            onSetDefault={handleSetDefaultProxy}
            onTestProxy={handleTestProxy}
          />
        )}

        {page === 'rules' && (
          <RulesPage
            rules={profile?.rules || []}
            proxies={profile?.proxies || []}
            knownApps={knownApps}
            onSaveRule={handleSaveRule}
            onDeleteRule={async (id) => {
              const updated = await app.DeleteRule(id)
              setProfile(updated)
              toast(t.messages.ruleDeleted, 'info')
            }}
            onToggleRule={async (id, en) => {
              const updated = await app.ToggleRule(id, en)
              setProfile(updated)
            }}
            onMoveRule={async (id, delta) => {
              const updated = await app.MoveRule(id, delta)
              setProfile(updated)
            }}
            onCloneRule={async (id) => {
              const updated = await app.CloneRule(id)
              setProfile(updated)
              toast(t.messages.ruleCloned, 'success')
            }}
            onPickExecutable={async () => app.PickProcessExecutable()}
            onImportRules={async (raw) => {
              const updated = await app.ImportRulesJSON(raw)
              setProfile(updated)
              toast(t.messages.rulesImported, 'success')
            }}
            onExportRules={async () => {
              const raw = await app.ExportRulesJSON([])
              await navigator.clipboard.writeText(raw)
              toast(t.messages.rulesExported, 'success')
            }}
          />
        )}

        {page === 'settings' && (
          <SettingsPage
            profile={profile}
            profiles={profiles.names}
            onUpdateOptions={async (tl, dns, lgc, clr) => {
              const updated = await app.UpdateOptions(tl, dns, lgc, clr)
              setProfile(updated)
              toast(t.messages.settingsUpdated, 'success')
            }}
            onCheckUpdates={async () => {
              toast(t.messages.checkingUpdates, 'info')
              const res = await app.CheckForUpdates()
              toast(
                res.isUpdateAvailable
                  ? `${t.messages.updateAvailable}: v${res.latestVersion}`
                  : t.messages.upToDate,
                'info'
              )
            }}
            onSwitchProfile={async (name) => {
              const p = await app.SwitchProfile(name)
              setProfile(p)
              setProfiles((prev) => ({ ...prev, active: name }))
            }}
            onCreateProfile={handleCreateProfile}
            onDuplicateProfile={async (srcName) => {
              const newName = prompt(t.settings.duplicatePrompt, `${srcName} Copy`)
              if (!newName?.trim()) return
              const p = await app.DuplicateProfile(srcName, newName.trim())
              setProfile(p)
              void reloadData()
              toast(t.messages.profileDuplicated, 'success')
            }}
            onRenameProfile={async (oldName) => {
              const newName = prompt(t.settings.renamePrompt, oldName)
              if (!newName?.trim() || newName.trim() === oldName) return
              const p = await app.RenameProfile(oldName, newName.trim())
              setProfile(p)
              void reloadData()
              toast(t.messages.profileRenamed, 'success')
            }}
            onDeleteProfile={async (name) => {
              if (!confirm(`${t.settings.confirmDeleteProfile}\n${name}`)) return
              const p = await app.DeleteProfile(name)
              setProfile(p)
              void reloadData()
              toast(t.messages.profileDeleted, 'info')
            }}
            onImportProfile={async (raw) => {
              const p = await app.ImportProfileJSON(raw, true)
              setProfile(p)
              void reloadData()
              toast(t.messages.profileImported, 'success')
            }}
            onExportProfile={async (includePasswords) => {
              if (!profile) return
              const raw = await app.ExportProfileJSON(profile.name, includePasswords)
              await navigator.clipboard.writeText(raw)
              toast(t.messages.profileExported, 'success')
            }}
          />
        )}

        <StatusBar
          engine={engine}
          profile={profile}
          connRate={flows.filter((f) => f.state === 'open').length}
        />
      </main>

      <div className="toasts">
        {toasts.map((item) => (
          <div key={item.id} className={`toast ${item.type}`}>
            <span>{item.msg}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
