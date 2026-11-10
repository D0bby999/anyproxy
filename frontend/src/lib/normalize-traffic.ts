import type { ConnectionLogEntry, Flow } from '../types'

// Native Go and the preview fixture use different field names for traffic records.
export function normalizeFlow(raw: any): Flow {
  const proxyName = raw.proxyName ?? raw.proxyLabel ?? ''
  return {
    ...raw,
    destIP: raw.destIP ?? raw.destIp ?? '',
    destPort: raw.destPort ?? raw.port ?? 0,
    resolvedDomain: raw.resolvedDomain ?? raw.host ?? '',
    protocol: raw.protocol ?? (raw.isUdp ? 'UDP' : 'TCP'),
    action: (raw.action ?? (proxyName ? 'proxy' : 'direct')).toLowerCase(),
    proxyName,
    state: raw.state ?? (raw.closed ? 'closed' : 'open'),
    bytesIn: raw.bytesIn ?? raw.received ?? 0,
    bytesOut: raw.bytesOut ?? raw.sent ?? 0,
  }
}

export function normalizeConnectionLog(raw: any): ConnectionLogEntry {
  return {
    ...raw,
    destIP: raw.destIP ?? raw.destIp ?? '',
    destPort: raw.destPort ?? raw.port ?? 0,
    resolvedDomain: raw.resolvedDomain ?? raw.host ?? '',
    action: (raw.action ?? 'direct').toLowerCase(),
    ruleName: raw.ruleName ?? raw.ruleId ?? '',
  }
}
