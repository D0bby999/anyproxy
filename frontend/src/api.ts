// SPDX-License-Identifier: MIT

import { stubApp } from './preview/stub'
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

interface WailsApp {
  StartupError(): Promise<string>
  ListProfiles(): Promise<ProfileListSummary>
  ActiveProfile(): Promise<Profile>
  SwitchProfile(name: string): Promise<Profile>
  CreateProfile(name: string): Promise<Profile>
  DuplicateProfile(srcName: string, newName: string): Promise<Profile>
  RenameProfile(oldName: string, newName: string): Promise<Profile>
  DeleteProfile(name: string): Promise<Profile>
  ImportProfileJSON(raw: string, makeActive: boolean): Promise<Profile>
  ExportProfileJSON(name: string, includePasswords: boolean): Promise<string>
  SaveProxy(px: ProxyConfig): Promise<Profile>
  DeleteProxy(proxyId: string): Promise<Profile>
  SetDefaultProxy(proxyId: string): Promise<Profile>
  TestProxy(proxyId: string, targetHost: string, targetPort: number): Promise<CheckReport>
  SaveRule(r: ProxyRule): Promise<Profile>
  DeleteRule(ruleId: string): Promise<Profile>
  ToggleRule(ruleId: string, enabled: boolean): Promise<Profile>
  MoveRule(ruleId: string, delta: number): Promise<Profile>
  CloneRule(ruleId: string): Promise<Profile>
  ImportRulesJSON(raw: string): Promise<Profile>
  ExportRulesJSON(ruleIds: string[]): Promise<string>
  StartEngine(): Promise<EngineStatus>
  StopEngine(): Promise<EngineStatus>
  SetEngineEnabled(enabled: boolean): Promise<EngineStatus>
  Status(): Promise<EngineStatus>
  UpdateOptions(
    trafficLogging: boolean,
    dnsViaProxy: boolean,
    logEveryConn: boolean,
    autoClearLogs: boolean
  ): Promise<Profile>
  SaveLogFilters(filters: LogFilter[]): Promise<Profile>
  GetFlows(limit: number): Promise<Flow[]>
  ClearClosedFlows(): Promise<number>
  GetConnectionLogs(limit: number): Promise<ConnectionLogEntry[]>
  ClearConnectionLogs(): Promise<void>
  GetActivityLogs(limit: number): Promise<ActivityLogEntry[]>
  ClearActivityLogs(): Promise<void>
  SimulateFlow(
    pid: number,
    processName: string,
    destIP: string,
    domain: string,
    port: number,
    isUDP: boolean
  ): Promise<string>
  PickProcessExecutable(): Promise<string>
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: WailsApp
      }
    }
    runtime?: {
      EventsOn?: (eventName: string, callback: (...args: unknown[]) => void) => () => void
    }
  }
}

function getApp(): WailsApp {
  if (typeof window !== 'undefined' && window.go?.main?.App) {
    return window.go.main.App
  }
  return stubApp as unknown as WailsApp
}

export const api = {
  getApp,

  subscribeEvent(topic: string, callback: (...args: unknown[]) => void): () => void {
    if (typeof window !== 'undefined' && window.runtime?.EventsOn) {
      return window.runtime.EventsOn(topic, callback)
    }
    return () => {}
  },
}

