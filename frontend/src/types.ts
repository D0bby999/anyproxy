// SPDX-License-Identifier: MIT

export type ProxyType = 'socks5' | 'socks4' | 'http' | 'https'
export type RuleProtocol = 'tcp' | 'udp' | 'both'
export type RuleAction = 'proxy' | 'direct' | 'block'
export type EngineState = 'stopped' | 'starting' | 'running' | 'paused' | 'degraded' | 'error'

export interface ProxyConfig {
  id: string
  name: string
  type: ProxyType
  host: string
  port: number
  username?: string
  password?: string
  sendDomainToProxy?: boolean
  description?: string
  latencyMs?: number
  lastChecked?: number
}

export interface ProxyRule {
  id: string
  name: string
  enabled: boolean
  processNames: string
  targetHosts: string
  targetPorts: string
  protocol: RuleProtocol
  action: RuleAction
  proxyConfigId?: string
}

export interface LogFilter {
  id: string
  name?: string
  enabled?: boolean
  mode: 'include' | 'exclude' | 'INCLUDE' | 'EXCLUDE'
  process?: string
  host?: string
  port?: string
  protocol?: string
  action?: string
}

export interface Profile {
  id: string
  name: string
  proxies: ProxyConfig[]
  rules: ProxyRule[]
  defaultProxyId: string
  trafficLoggingEnabled: boolean
  dnsViaProxy: boolean
  logEveryConnection: boolean
  autoClearConnectionLogs: boolean
  logFilters?: LogFilter[]
}

export interface ProfileListSummary {
  names: string[]
  active: string
}

export interface EngineStatus {
  state: EngineState
  enabled: boolean
  backend: string
  extensionInstalled: boolean
  dnsProxyActive: boolean
  activeProxies: number
  activeRules: number
  activeFlows: number
  message?: string
}

export interface Flow {
  id: string
  pid: number
  processName: string
  processPath: string
  protocol: 'TCP' | 'UDP'
  destIP: string
  destPort: number
  resolvedDomain: string
  ruleId: string
  ruleName: string
  action: RuleAction
  proxyId: string
  proxyName: string
  state: 'open' | 'closed'
  bytesIn: number
  bytesOut: number
  startedAt: number
  endedAt?: number
}

export interface ConnectionLogEntry {
  id: string
  timestamp: number
  pid: number
  processName: string
  protocol: 'TCP' | 'UDP'
  destIP: string
  destPort: number
  resolvedDomain: string
  action: RuleAction
  proxyName: string
  ruleName: string
  bytesIn: number
  bytesOut: number
  durationMs: number
}

export interface ActivityLogEntry {
  id: string
  timestamp: number
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'
  message: string
}

export interface CheckStep {
  name: string
  description: string
  status: 'pending' | 'running' | 'ok' | 'failed'
  durationMs?: number
  detail?: string
}

export interface CheckReport {
  ok: boolean
  target: string
  proxyType: string
  steps: CheckStep[]
  summary: string
  resolvedIP?: string
  responseStatus?: string
  totalDurationMs: number
}

