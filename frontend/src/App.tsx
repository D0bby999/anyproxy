// SPDX-License-Identifier: MIT
import React, { useEffect, useState, useRef } from 'react'
import { api } from './api'
import { t } from './i18n'
import { normalizeConnectionLog, normalizeFlow } from './lib/normalize-traffic'
import { Sidebar, type Page } from './components/sidebar'
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
      setToasts((prev) => prev.filter((t) => t.id !== id))
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
    } catch (e: any) {
      const message = e.message || String(e)
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

    const unsubProfile = api.subscribeEvent('profile:changed', (data: any) => {
      if (data) setProfile(data)
    })
    const unsubEngine = api.subscribeEvent('engine:status', (data: any) => {
      if (data) setEngine(data)
    })

    return () => {
      cancelled = true
      clearTimeout(startupTimer)
      clearInterval(timer)
      unsubProfile()
      unsubEngine()
    }
  }, [])

  const handleToggleEngine = async () => {
    try {
      const isRunning = engine?.state === 'running'
      const next = isRunning ? await app.StopEngine() : await app.StartEngine()
      setEngine(next)
      toast(isRunning ? t.messages.engineStopped : t.messages.engineStarted, 'success')
    } catch (e: any) {
      toast(e.message || String(e), 'error')
    }
  }

  const handleSetDefaultProxy = async (id: string) => {
    try {
      const updated = await app.SetDefaultProxy(id)
      setProfile(updated)
      toast(t.messages.defaultUpdated, 'success')
    } catch (e: any) {
      toast(e.message || String(e), 'error')
    }
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
        onCreateProfile={async () => {
          const name = prompt(t.messages.profilePrompt)
          if (name) {
            const p = await app.CreateProfile(name)
            setProfile(p)
            reloadData()
          }
        }}
        proxies={profile?.proxies || []}
        defaultProxyId={profile?.defaultProxyId || ''}
        onSetDefaultProxy={handleSetDefaultProxy}
        liveFlowCount={flows.filter((f) => f.state === 'open').length}
        rulesCount={profile?.rules?.length || 0}
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
            onTestProxy={async (px) => {
              toast(`${t.messages.testing} ${px.name}...`, 'info')
              try {
                const rep = await app.TestProxy(px.id, 'httpbin.org', 80)
                const success = rep.success ?? rep.ok
                toast(
                  success ? `${t.messages.testSuccess}: ${rep.avgMs ?? rep.totalDurationMs} ms` : `${t.messages.testFailed}: ${rep.summary}`,
                  success ? 'success' : 'error'
                )
                reloadData()
                return rep
              } catch (e: any) {
                toast(`${t.messages.testError}: ${e.message || String(e)}`, 'error')
              }
            }}
          />
        )}

        {page === 'rules' && (
          <RulesPage
            rules={profile?.rules || []}
            proxies={profile?.proxies || []}
            onSaveRule={async (r) => {
              const updated = await app.SaveRule(r)
              setProfile(updated)
              toast(t.messages.ruleSaved, 'success')
            }}
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
          />
        )}

        {page === 'settings' && (
          <SettingsPage
            profile={profile}
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
          />
        )}

        <StatusBar
          engine={engine}
          profile={profile}
          connRate={flows.filter((f) => f.state === 'open').length}
        />
      </main>

      <div className="toasts">
        {toasts.map((t) => (
          <div key={t.id} className={`toast ${t.type}`}>
            <span>{t.msg}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
