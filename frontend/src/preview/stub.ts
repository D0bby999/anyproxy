// SPDX-License-Identifier: MIT

import type {
  Profile,
  ProfileListSummary,
  EngineStatus,
  Flow,
  ConnectionLogEntry,
  ActivityLogEntry,
  CheckReport,
  CheckStep,
  ProxyConfig,
  ProxyRule,
  LogFilter,
} from '../types'

let mockProxies: ProxyConfig[] = []
let mockRules: ProxyRule[] = []
let activeProfileData: Profile = {
  id: 'profile-default',
  name: 'Default',
  proxies: mockProxies,
  rules: mockRules,
  defaultProxyId: '',
  trafficLoggingEnabled: true,
  dnsViaProxy: true,
  logEveryConnection: false,
  autoClearConnectionLogs: true,
}

let allProfileNames = ['Default']

const mockFlows: Flow[] = []
const mockConnectionLogs: ConnectionLogEntry[] = []
const mockActivityLogs: ActivityLogEntry[] = []

export const stubApp = {
  StartupError: async (): Promise<string> => '',
  ListProfiles: async (): Promise<ProfileListSummary> => ({
    names: allProfileNames,
    active: activeProfileData.name,
  }),
  ActiveProfile: async (): Promise<Profile> => ({ ...activeProfileData }),
  SwitchProfile: async (name: string): Promise<Profile> => {
    activeProfileData = {
      ...activeProfileData,
      name,
      proxies: mockProxies,
      rules: mockRules,
    }
    return { ...activeProfileData }
  },
  CreateProfile: async (name: string): Promise<Profile> => {
    if (!allProfileNames.includes(name)) allProfileNames.push(name)
    activeProfileData = {
      id: `profile-${name.toLowerCase().replace(/\s+/g, '-')}`,
      name,
      proxies: [],
      rules: [],
      defaultProxyId: '',
      trafficLoggingEnabled: true,
      dnsViaProxy: true,
      logEveryConnection: false,
      autoClearConnectionLogs: true,
    }
    return { ...activeProfileData }
  },
  DuplicateProfile: async (_srcName: string, newName: string): Promise<Profile> => {
    if (!allProfileNames.includes(newName)) allProfileNames.push(newName)
    activeProfileData = {
      ...activeProfileData,
      id: `profile-${newName.toLowerCase().replace(/\s+/g, '-')}`,
      name: newName,
    }
    return { ...activeProfileData }
  },
  RenameProfile: async (oldName: string, newName: string): Promise<Profile> => {
    allProfileNames = allProfileNames.map((n) => (n === oldName ? newName : n))
    activeProfileData.name = newName
    return { ...activeProfileData }
  },
  DeleteProfile: async (name: string): Promise<Profile> => {
    allProfileNames = allProfileNames.filter((n) => n !== name)
    if (allProfileNames.length === 0) allProfileNames = ['Default']
    activeProfileData.name = allProfileNames[0]
    return { ...activeProfileData }
  },
  ImportProfileJSON: async (raw: string, makeActive: boolean): Promise<Profile> => {
    const parsed = JSON.parse(raw)
    const p: Profile = {
      id: parsed.id || 'imported-profile',
      name: parsed.name || 'Imported Profile',
      proxies: parsed.proxies || [],
      rules: parsed.rules || [],
      defaultProxyId: parsed.defaultProxyId || '',
      trafficLoggingEnabled: parsed.trafficLoggingEnabled ?? true,
      dnsViaProxy: parsed.dnsViaProxy ?? true,
      logEveryConnection: parsed.logEveryConnection ?? false,
      autoClearConnectionLogs: parsed.autoClearConnectionLogs ?? true,
    }
    if (makeActive) {
      if (!allProfileNames.includes(p.name)) allProfileNames.push(p.name)
      activeProfileData = p
    }
    return p
  },
  ExportProfileJSON: async (_name: string, _includePasswords: boolean): Promise<string> => {
    return JSON.stringify(activeProfileData, null, 2)
  },
  SaveProxy: async (px: ProxyConfig): Promise<Profile> => {
    const idx = mockProxies.findIndex((p) => p.id === px.id)
    if (idx >= 0) {
      mockProxies[idx] = { ...px }
    } else {
      mockProxies.push({
        ...px,
        id: px.id || `px-${Date.now()}`,
      })
    }
    activeProfileData.proxies = [...mockProxies]
    return { ...activeProfileData }
  },
  DeleteProxy: async (proxyId: string): Promise<Profile> => {
    mockProxies = mockProxies.filter((p) => p.id !== proxyId)
    if (activeProfileData.defaultProxyId === proxyId) {
      activeProfileData.defaultProxyId = ''
    }
    activeProfileData.proxies = [...mockProxies]
    return { ...activeProfileData }
  },
  SetDefaultProxy: async (proxyId: string): Promise<Profile> => {
    activeProfileData.defaultProxyId = proxyId
    return { ...activeProfileData }
  },
  TestProxy: async (
    proxyId: string,
    targetHost: string,
    targetPort: number
  ): Promise<CheckReport> => {
    const px = mockProxies.find((p) => p.id === proxyId)
    const steps: CheckStep[] = [
      {
        name: 'TCP Connectivity',
        description: `Connect to proxy endpoint at ${px?.host || '127.0.0.1'}:${px?.port || 1080}`,
        status: 'ok',
        durationMs: 8,
      },
      {
        name: 'Handshake & Auth',
        description: `Perform ${px?.type || 'socks5'} protocol negotiation`,
        status: 'ok',
        durationMs: 14,
      },
      {
        name: 'Target Tunnel Establishment',
        description: `Request proxy tunnel to ${targetHost}:${targetPort}`,
        status: 'ok',
        durationMs: 22,
      },
      {
        name: 'HTTP Probe Request',
        description: `Send HTTP GET probe through tunnel`,
        status: 'ok',
        durationMs: 35,
      },
      {
        name: 'Latency & Egress Measurement',
        description: 'Multi-sample RTT measurement and exit IP resolution',
        status: 'ok',
        durationMs: 18,
      },
    ]

    const report: CheckReport = {
      ok: true,
      target: `${targetHost}:${targetPort}`,
      proxyType: px?.type || 'socks5',
      steps,
      summary: 'All 5 diagnostic check stages completed successfully with responsive latency.',
      resolvedIP: '198.51.100.42',
      responseStatus: 'HTTP/1.1 200 OK',
      totalDurationMs: 97,
    }

    if (px) {
      px.latencyMs = 97
      px.lastChecked = Date.now()
    }
    return report
  },
  SaveRule: async (r: ProxyRule): Promise<Profile> => {
    const idx = mockRules.findIndex((item) => item.id === r.id)
    if (idx >= 0) {
      mockRules[idx] = { ...r }
    } else {
      mockRules.push({
        ...r,
        id: r.id || `rl-${Date.now()}`,
      })
    }
    activeProfileData.rules = [...mockRules]
    return { ...activeProfileData }
  },
  DeleteRule: async (ruleId: string): Promise<Profile> => {
    mockRules = mockRules.filter((r) => r.id !== ruleId)
    activeProfileData.rules = [...mockRules]
    return { ...activeProfileData }
  },
  ToggleRule: async (ruleId: string, enabled: boolean): Promise<Profile> => {
    const r = mockRules.find((item) => item.id === ruleId)
    if (r) r.enabled = enabled
    activeProfileData.rules = [...mockRules]
    return { ...activeProfileData }
  },
  MoveRule: async (ruleId: string, delta: number): Promise<Profile> => {
    const idx = mockRules.findIndex((item) => item.id === ruleId)
    if (idx >= 0) {
      const target = idx + delta
      if (target >= 0 && target < mockRules.length) {
        const [moved] = mockRules.splice(idx, 1)
        mockRules.splice(target, 0, moved)
      }
    }
    activeProfileData.rules = [...mockRules]
    return { ...activeProfileData }
  },
  CloneRule: async (ruleId: string): Promise<Profile> => {
    const r = mockRules.find((item) => item.id === ruleId)
    if (r) {
      mockRules.push({
        ...r,
        id: `rl-${Date.now()}`,
        name: `${r.name} (Copy)`,
      })
    }
    activeProfileData.rules = [...mockRules]
    return { ...activeProfileData }
  },
  ImportRulesJSON: async (raw: string): Promise<Profile> => {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      mockRules.push(...parsed)
      activeProfileData.rules = [...mockRules]
    }
    return { ...activeProfileData }
  },
  ExportRulesJSON: async (ruleIds: string[]): Promise<string> => {
    const selected =
      ruleIds.length === 0
        ? mockRules
        : mockRules.filter((r) => ruleIds.includes(r.id))
    return JSON.stringify(selected, null, 2)
  },
  StartEngine: async (): Promise<EngineStatus> => ({
    state: 'running',
    enabled: true,
    backend: 'NETransparentProxyProvider',
    extensionInstalled: true,
    dnsProxyActive: true,
    activeProxies: mockProxies.length,
    activeRules: mockRules.filter((r) => r.enabled).length,
    activeFlows: mockFlows.filter((f) => f.state === 'open').length,
    message: 'System extension active and routing outbound IP packets',
  }),
  StopEngine: async (): Promise<EngineStatus> => ({
    state: 'stopped',
    enabled: false,
    backend: 'NETransparentProxyProvider',
    extensionInstalled: true,
    dnsProxyActive: false,
    activeProxies: 0,
    activeRules: 0,
    activeFlows: 0,
    message: 'Transparent proxy network tunnel deactivated',
  }),
  SetEngineEnabled: async (enabled: boolean): Promise<EngineStatus> => {
    return enabled
      ? stubApp.StartEngine()
      : stubApp.StopEngine()
  },
  Status: async (): Promise<EngineStatus> => ({
    state: 'stopped',
    enabled: false,
    backend: 'NETransparentProxyProvider',
    extensionInstalled: true,
    dnsProxyActive: false,
    activeProxies: mockProxies.length,
    activeRules: mockRules.filter((r) => r.enabled).length,
    activeFlows: 0,
  }),
  UpdateOptions: async (
    trafficLogging: boolean,
    dnsViaProxy: boolean,
    logEveryConn: boolean,
    autoClearLogs: boolean
  ): Promise<Profile> => {
    activeProfileData.trafficLoggingEnabled = trafficLogging
    activeProfileData.dnsViaProxy = dnsViaProxy
    activeProfileData.logEveryConnection = logEveryConn
    activeProfileData.autoClearConnectionLogs = autoClearLogs
    return { ...activeProfileData }
  },
  SaveLogFilters: async (filters: LogFilter[]): Promise<Profile> => {
    activeProfileData.logFilters = filters
    return { ...activeProfileData }
  },
  GetFlows: async (_limit: number): Promise<Flow[]> => [...mockFlows],
  ClearClosedFlows: async (): Promise<number> => {
    const before = mockFlows.length
    const remaining = mockFlows.filter((f) => f.state === 'open')
    mockFlows.length = 0
    mockFlows.push(...remaining)
    return before - remaining.length
  },
  GetConnectionLogs: async (_limit: number): Promise<ConnectionLogEntry[]> => [
    ...mockConnectionLogs,
  ],
  ClearConnectionLogs: async (): Promise<void> => {
    mockConnectionLogs.length = 0
  },
  GetActivityLogs: async (_limit: number): Promise<ActivityLogEntry[]> => [
    ...mockActivityLogs,
  ],
  ClearActivityLogs: async (): Promise<void> => {
    mockActivityLogs.length = 0
  },
  SimulateFlow: async (): Promise<string> => 'simulated-ok',
  PickProcessExecutable: async (): Promise<string> => '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  CheckForUpdates: async () => ({
    currentVersion: '1.0.0',
    latestVersion: '1.0.0',
    isUpdateAvailable: false,
    releaseDate: '2026-11-08',
  }),
}

if (typeof window !== 'undefined') {
  ;(window as unknown as { go: { main: { App: typeof stubApp } } }).go = {
    main: { App: stubApp },
  }
}
