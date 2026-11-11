// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ProxyConfig } from '../types'

// Parse the two paste formats without including credentials in errors or generated names.
export function parseProxy(line: string): ProxyConfig | null {
  const input = line.trim()
  const match = /^(?:(https?|socks5):\/\/)?(?:([^\s@/]+)@)?(\[[^\]\s]+\]|[^:\s/@?#]+):(\d{1,5})(?::([^:\s]*):(.*))?\/?$/i.exec(input)
  if (!match) return null
  const [, scheme, userInfo, rawHost, rawPort, colonUser, colonPassword] = match
  if (scheme && !['http', 'socks5'].includes(scheme.toLowerCase())) return null
  if (userInfo && colonUser !== undefined) return null
  const port = Number(rawPort)
  if (port < 1 || port > 65535) return null
  let username = colonUser ?? ''
  let password = colonPassword ?? ''
  if (userInfo) {
    const split = userInfo.indexOf(':')
    if (split < 0) return null
    try {
      username = decodeURIComponent(userInfo.slice(0, split))
      password = decodeURIComponent(userInfo.slice(split + 1))
    } catch { return null }
  }
  const host = rawHost.startsWith('[') ? rawHost.slice(1, -1) : rawHost
  if (/[\u0000-\u001f\u007f]/.test(username + password)) return null
  if (scheme?.toLowerCase() === 'socks5' && (new TextEncoder().encode(username).length > 255 || new TextEncoder().encode(password).length > 255)) return null
  return {
    id: '', name: `${host}:${port}`, type: (scheme?.toLowerCase() || 'http') as ProxyConfig['type'],
    host, port, username, password, sendDomainToProxy: true,
  }
}
