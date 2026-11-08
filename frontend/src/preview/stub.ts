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

const mockProxies: ProxyConfig[] = [
  {
    id: 'px-clash',
    name: 'Local Clash SOCKS5',
    type: 'socks5',
    host: '127.0.0.1',
    port: 7890,
    sendDomainToProxy: true,
    description: 'Local mixed rule provider proxy',
    latencyMs: 2,
    lastChecked: Date.now() - 120000,
  },
  {
    id: 'px-burp',
    name: 'Burp Suite HTTP',
    type: 'http',
    host: '127.0.0.1',
    port: 8080,
    sendDomainToProxy: false,
    description: 'Burp Suite web inspection proxy',
    latencyMs: 4,
    lastChecked: Date.now() - 300000,
  },
  {
    id: 'px-wireguard',
    name: 'WireGuard Exit SOCKS5',
    type: 'socks5',
    host: '10.64.0.1',
    port: 1080,
    username: 'sec-tunnel',
    password: '••••••••',
    sendDomainToProxy: true,
    description: 'Dedicated cloud egress gateway',
    latencyMs: 38,
    lastChecked: Date.now() - 450000,
  },
]

const mockRules: ProxyRule[] = [
  {
    id: 'rl-burp',
    name: 'Burp Suite Security Inspection',
    enabled: true,
    processNames: 'curl;Postman;Google Chrome',
    targetHosts: '*.target.internal;*.staging.app',
    targetPorts: '80,443,8080,8443',
    protocol: 'tcp',
    action: 'proxy',
    proxyConfigId: 'px-burp',
  },
  {
    id: 'rl-bypass-local',
    name: 'Bypass Local Intranet & Dev',
    enabled: true,
    processNames: '*',
    targetHosts: 'localhost;127.0.0.1;10.0.0.0/8;192.168.0.0/16;*.local',
    targetPorts: '*',
    protocol: 'both',
    action: 'direct',
  },
  {
    id: 'rl-block-telemetry',
    name: 'Block Tracking & Ad Telemetry',
    enabled: true,
    processNames: '*',
    targetHosts: '*.telemetry.tracking.io;*.analytics.ad;telemetry.dropbox.com',
    targetPorts: '*',
    protocol: 'both',
    action: 'block',
  },
  {
    id: 'rl-dev-pkgs',
    name: 'Developer Tool Package Proxy',
    enabled: true,
    processNames: 'git;node;go;cargo;npm',
    targetHosts: 'github.com;*.githubusercontent.com;registry.npmjs.org;proxy.golang.org',
    targetPorts: '443',
    protocol: 'tcp',
    action: 'proxy',
    proxyConfigId: 'px-clash',
  },
  {
    id: 'rl-default-egress',
    name: 'Default Cloud Exit Route',
    enabled: true,
    processNames: '*',
    targetHosts: '*',
    targetPorts: '*',
    protocol: 'both',
    action: 'proxy',
    proxyConfigId: 'px-wireguard',
  },
]

let activeProfileData: Profile = {
  id: 'prof-pentest-us',
  name: 'Pentest-US',
  proxies: mockProxies,
  rules: mockRules,
  defaultProxyId: 'px-clash',
  trafficLoggingEnabled: true,
  dnsViaProxy: true,
  logEveryConnection: true,
  autoClearConnectionLogs: false,
  logFilters: [],
}

let allProfileNames: string[] = ['Pentest-US', 'Development-Direct', 'Security-Audit']

let engineStatusData: EngineStatus = {
  state: 'running',
  enabled: true,
  backend: 'macos-ne',
  extensionInstalled: true,
  dnsProxyActive: true,
  activeProxies: 3,
  activeRules: 5,
  activeFlows: 4,
}

let liveFlows: Flow[] = [
  {
    id: 'flow-01',
    pid: 4182,
    processName: 'Google Chrome',
    processPath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    protocol: 'TCP',
    destIP: '10.200.1.5',
    destPort: 443,
    resolvedDomain: 'admin.target.internal',
    ruleId: 'rl-burp',
    ruleName: 'Burp Suite Security Inspection',
    action: 'proxy',
    proxyId: 'px-burp',
    proxyName: 'Burp Suite HTTP',
    state: 'open',
    bytesIn: 146200,
    bytesOut: 24890,
    startedAt: Date.now() - 34000,
  },
  {
    id: 'flow-02',
    pid: 5930,
    processName: 'curl',
    processPath: '/usr/bin/curl',
    protocol: 'TCP',
    destIP: '10.200.1.18',
    destPort: 80,
    resolvedDomain: 'api.staging.app',
    ruleId: 'rl-burp',
    ruleName: 'Burp Suite Security Inspection',
    action: 'proxy',
    proxyId: 'px-burp',
    proxyName: 'Burp Suite HTTP',
    state: 'open',
    bytesIn: 8640,
    bytesOut: 1240,
    startedAt: Date.now() - 12000,
  },
  {
    id: 'flow-03',
    pid: 7421,
    processName: 'go',
    processPath: '/usr/local/go/bin/go',
    protocol: 'TCP',
    destIP: '142.250.190.49',
    destPort: 443,
    resolvedDomain: 'proxy.golang.org',
    ruleId: 'rl-dev-pkgs',
    ruleName: 'Developer Tool Package Proxy',
    action: 'proxy',
    proxyId: 'px-clash',
    proxyName: 'Local Clash SOCKS5',
    state: 'closed',
    bytesIn: 524288,
    bytesOut: 3912,
    startedAt: Date.now() - 95000,
    endedAt: Date.now() - 21000,
  },
  {
    id: 'flow-04',
    pid: 8812,
    processName: 'Spotify',
    processPath: '/Applications/Spotify.app/Contents/MacOS/Spotify',
    protocol: 'TCP',
    destIP: '35.186.224.25',
    destPort: 443,
    resolvedDomain: 'spclient.wg.spotify.com',
    ruleId: 'rl-default-egress',
    ruleName: 'Default Cloud Exit Route',
    action: 'proxy',
    proxyId: 'px-wireguard',
    proxyName: 'WireGuard Exit SOCKS5',
    state: 'open',
    bytesIn: 4410290,
    bytesOut: 122880,
    startedAt: Date.now() - 280000,
  },
  {
    id: 'flow-05',
    pid: 9110,
    processName: 'AnalyticsAgent',
    processPath: '/usr/local/bin/agent',
    protocol: 'TCP',
    destIP: '198.51.100.22',
    destPort: 443,
    resolvedDomain: 'metrics.telemetry.tracking.io',
    ruleId: 'rl-block-telemetry',
    ruleName: 'Block Tracking & Ad Telemetry',
    action: 'block',
    proxyId: '',
    proxyName: 'Blocked',
    state: 'closed',
    bytesIn: 0,
    bytesOut: 0,
    startedAt: Date.now() - 8000,
    endedAt: Date.now() - 7950,
  },
]

let connLogs: ConnectionLogEntry[] = [
  {
    id: 'cl-1',
    timestamp: Date.now() - 8000,
    pid: 9110,
    processName: 'AnalyticsAgent',
    protocol: 'TCP',
    destIP: '198.51.100.22',
    destPort: 443,
    resolvedDomain: 'metrics.telemetry.tracking.io',
    action: 'block',
    proxyName: 'Blocked',
    ruleName: 'Block Tracking & Ad Telemetry',
    bytesIn: 0,
    bytesOut: 0,
    durationMs: 50,
  },
  {
    id: 'cl-2',
    timestamp: Date.now() - 21000,
    pid: 7421,
    processName: 'go',
    protocol: 'TCP',
    destIP: '142.250.190.49',
    destPort: 443,
    resolvedDomain: 'proxy.golang.org',
    action: 'proxy',
    proxyName: 'Local Clash SOCKS5',
    ruleName: 'Developer Tool Package Proxy',
    bytesIn: 524288,
    bytesOut: 3912,
    durationMs: 74000,
  },
  {
    id: 'cl-3',
    timestamp: Date.now() - 65000,
    pid: 3102,
    processName: 'git',
    protocol: 'TCP',
    destIP: '140.82.121.4',
    destPort: 443,
    resolvedDomain: 'github.com',
    action: 'proxy',
    proxyName: 'Local Clash SOCKS5',
    ruleName: 'Developer Tool Package Proxy',
    bytesIn: 1048576,
    bytesOut: 48920,
    durationMs: 4200,
  },
]

let activityLogs: ActivityLogEntry[] = [
  {
    id: 'act-1',
    timestamp: Date.now() - 300000,
    level: 'INFO',
    message: 'AnyProxyXa service initialized with 3 proxies and 5 rules',
  },
  {
    id: 'act-2',
    timestamp: Date.now() - 290000,
    level: 'INFO',
    message: 'Transparent NetworkExtension driver started successfully on en0',
  },
  {
    id: 'act-3',
    timestamp: Date.now() - 150000,
    level: 'INFO',
    message: 'DNS cache loaded: 43 host mappings registered',
  },
]

export const stubApp = {
  StartupError: async (): Promise<string> => '',
  ListProfiles: async (): Promise<ProfileListSummary> => ({
    names: allProfileNames,
    active: activeProfileData.name,
  }),
  ActiveProfile: async (): Promise<Profile> => ({ ...activeProfileData }),
  SwitchProfile: async (name: string): Promise<Profile> => {
    activeProfileData = { ...activeProfileData, name }
    return { ...activeProfileData }
  },
  CreateProfile: async (name: string): Promise<Profile> => {
    allProfileNames.push(name)
    activeProfileData = {
      id: 'prof-' + Date.now(),
      name,
      proxies: [],
      rules: [],
      defaultProxyId: '',
      trafficLoggingEnabled: true,
      dnsViaProxy: true,
      logEveryConnection: true,
      autoClearConnectionLogs: false,
    }
    return { ...activeProfileData }
  },
  DuplicateProfile: async (_src: string, newName: string): Promise<Profile> => {
    allProfileNames.push(newName)
    activeProfileData = { ...activeProfileData, id: 'prof-' + Date.now(), name: newName }
    return { ...activeProfileData }
  },
  RenameProfile: async (_oldName: string, newName: string): Promise<Profile> => {
    activeProfileData = { ...activeProfileData, name: newName }
    return { ...activeProfileData }
  },
  DeleteProfile: async (name: string): Promise<Profile> => {
    allProfileNames = allProfileNames.filter((n) => n !== name)
    activeProfileData.name = allProfileNames[0] || 'Default'
    return { ...activeProfileData }
  },
  ImportProfileJSON: async (raw: string): Promise<Profile> => {
    const parsed = JSON.parse(raw)
    activeProfileData = { ...parsed }
    if (!allProfileNames.includes(parsed.name)) {
      allProfileNames.push(parsed.name)
    }
    return { ...activeProfileData }
  },
  ExportProfileJSON: async (): Promise<string> => JSON.stringify(activeProfileData, null, 2),
  SaveProxy: async (px: ProxyConfig): Promise<Profile> => {
    const list = [...activeProfileData.proxies]
    const idx = list.findIndex((p) => p.id === px.id)
    if (idx >= 0) {
      list[idx] = px
    } else {
      list.push({ ...px, id: 'px-' + Date.now() })
    }
    activeProfileData = { ...activeProfileData, proxies: list }
    return { ...activeProfileData }
  },
  DeleteProxy: async (proxyId: string): Promise<Profile> => {
    activeProfileData = {
      ...activeProfileData,
      proxies: activeProfileData.proxies.filter((p) => p.id !== proxyId),
    }
    return { ...activeProfileData }
  },
  SetDefaultProxy: async (proxyId: string): Promise<Profile> => {
    activeProfileData = { ...activeProfileData, defaultProxyId: proxyId }
    return { ...activeProfileData }
  },
  TestProxy: async (proxyId: string, targetHost: string, targetPort: number): Promise<CheckReport> => {
    const px = activeProfileData.proxies.find((p) => p.id === proxyId) || activeProfileData.proxies[0]
    const steps: CheckStep[] = [
      { name: '1. Resolve Target', description: 'DNS resolution for target host', status: 'ok', durationMs: 12, detail: '142.250.190.49' },
      { name: '2. Connect Proxy Server', description: `TCP connect to ${px.host}:${px.port}`, status: 'ok', durationMs: 4, detail: 'TCP Handshake completed' },
      { name: '3. Handshake & Auth', description: `Protocol handshake (${px.type.toUpperCase()})`, status: 'ok', durationMs: 6, detail: 'Auth OK' },
      { name: '4. Proxy Relay Connection', description: `Relay connect to ${targetHost}:${targetPort}`, status: 'ok', durationMs: 18, detail: 'Connection established' },
      { name: '5. HTTP Probe', description: `HTTP HEAD request through relay tunnel`, status: 'ok', durationMs: 22, detail: 'HTTP 200 OK' },
    ]
    return {
      ok: true,
      target: `${targetHost}:${targetPort}`,
      proxyType: px.type,
      steps,
      summary: 'Proxy connection, relay and probe succeeded in 62ms',
      resolvedIP: '142.250.190.49',
      responseStatus: 'HTTP/1.1 200 OK',
      totalDurationMs: 62,
    }
  },
  SaveRule: async (r: ProxyRule): Promise<Profile> => {
    const list = [...activeProfileData.rules]
    const idx = list.findIndex((rule) => rule.id === r.id)
    if (idx >= 0) {
      list[idx] = r
    } else {
      list.push({ ...r, id: 'rl-' + Date.now() })
    }
    activeProfileData = { ...activeProfileData, rules: list }
    return { ...activeProfileData }
  },
  DeleteRule: async (ruleId: string): Promise<Profile> => {
    activeProfileData = {
      ...activeProfileData,
      rules: activeProfileData.rules.filter((r) => r.id !== ruleId),
    }
    return { ...activeProfileData }
  },
  ToggleRule: async (ruleId: string, enabled: boolean): Promise<Profile> => {
    const list = activeProfileData.rules.map((r) => (r.id === ruleId ? { ...r, enabled } : r))
    activeProfileData = { ...activeProfileData, rules: list }
    return { ...activeProfileData }
  },
  MoveRule: async (ruleId: string, delta: number): Promise<Profile> => {
    const list = [...activeProfileData.rules]
    const idx = list.findIndex((r) => r.id === ruleId)
    if (idx >= 0) {
      const target = idx + delta
      if (target >= 0 && target < list.length) {
        const tmp = list[idx]
        list[idx] = list[target]
        list[target] = tmp
      }
    }
    activeProfileData = { ...activeProfileData, rules: list }
    return { ...activeProfileData }
  },
  CloneRule: async (ruleId: string): Promise<Profile> => {
    const list = [...activeProfileData.rules]
    const idx = list.findIndex((r) => r.id === ruleId)
    if (idx >= 0) {
      const clone = { ...list[idx], id: 'rl-' + Date.now(), name: list[idx].name + ' (Copy)' }
      list.splice(idx + 1, 0, clone)
    }
    activeProfileData = { ...activeProfileData, rules: list }
    return { ...activeProfileData }
  },
  ImportRulesJSON: async (raw: string): Promise<Profile> => {
    const parsed = JSON.parse(raw) as ProxyRule[]
    activeProfileData = { ...activeProfileData, rules: [...activeProfileData.rules, ...parsed] }
    return { ...activeProfileData }
  },
  ExportRulesJSON: async (ruleIds: string[]): Promise<string> => {
    const rules = ruleIds.length > 0
      ? activeProfileData.rules.filter((r) => ruleIds.includes(r.id))
      : activeProfileData.rules
    return JSON.stringify(rules, null, 2)
  },
  StartEngine: async (): Promise<EngineStatus> => {
    engineStatusData = { ...engineStatusData, state: 'running', enabled: true }
    return { ...engineStatusData }
  },
  StopEngine: async (): Promise<EngineStatus> => {
    engineStatusData = { ...engineStatusData, state: 'stopped', enabled: false }
    return { ...engineStatusData }
  },
  SetEngineEnabled: async (enabled: boolean): Promise<EngineStatus> => {
    engineStatusData = { ...engineStatusData, enabled, state: enabled ? 'running' : 'paused' }
    return { ...engineStatusData }
  },
  Status: async (): Promise<EngineStatus> => ({ ...engineStatusData }),
  UpdateOptions: async (
    trafficLogging: boolean,
    dnsViaProxy: boolean,
    logEveryConn: boolean,
    autoClearLogs: boolean
  ): Promise<Profile> => {
    activeProfileData = {
      ...activeProfileData,
      trafficLoggingEnabled: trafficLogging,
      dnsViaProxy,
      logEveryConnection: logEveryConn,
      autoClearConnectionLogs: autoClearLogs,
    }
    return { ...activeProfileData }
  },
  SaveLogFilters: async (filters: LogFilter[]): Promise<Profile> => {
    activeProfileData = { ...activeProfileData, logFilters: filters }
    return { ...activeProfileData }
  },
  GetFlows: async (): Promise<Flow[]> => [...liveFlows],
  ClearClosedFlows: async (): Promise<number> => {
    const before = liveFlows.length
    liveFlows = liveFlows.filter((f) => f.state === 'open')
    return before - liveFlows.length
  },
  GetConnectionLogs: async (): Promise<ConnectionLogEntry[]> => [...connLogs],
  ClearConnectionLogs: async (): Promise<void> => {
    connLogs = []
  },
  GetActivityLogs: async (): Promise<ActivityLogEntry[]> => [...activityLogs],
  ClearActivityLogs: async (): Promise<void> => {
    activityLogs = []
  },
  SimulateFlow: async (
    _pid: number,
    processName: string,
    destIP: string,
    domain: string,
    port: number,
    _isUDP: boolean
  ): Promise<string> => {
    const id = 'flow-' + Date.now()
    liveFlows.unshift({
      id,
      pid: 9999,
      processName,
      processPath: `/usr/bin/${processName}`,
      protocol: 'TCP',
      destIP,
      destPort: port,
      resolvedDomain: domain,
      ruleId: 'rl-sim',
      ruleName: 'Simulated Rule',
      action: 'proxy',
      proxyId: 'px-clash',
      proxyName: 'Local Clash SOCKS5',
      state: 'open',
      bytesIn: 1024,
      bytesOut: 512,
      startedAt: Date.now(),
    })
    return `proxy:${id}`
  },
  PickProcessExecutable: async (): Promise<string> => '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
}

// Attach to window.go.main.App for preview mode
if (typeof window !== 'undefined') {
  ;(window as unknown as { go: { main: { App: typeof stubApp } } }).go = {
    main: { App: stubApp },
  }
}

