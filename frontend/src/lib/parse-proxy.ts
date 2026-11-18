// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ProxyConfig } from '../types'

// Parse common proxy paste formats without including credentials in errors or generated names:
// - host:port[:user:pass]
// - host:port@user:pass
// - [scheme://][user:pass@]host:port[/][#CustomName]
export function parseProxy(line: string, defaultScheme: 'auto' | 'http' | 'https' | 'socks5' = 'auto'): ProxyConfig | null {
  let input = line.trim()
  if (!input) return null

  let customName = ''
  const hashIdx = input.indexOf('#')
  if (hashIdx >= 0) {
    try {
      customName = decodeURIComponent(input.slice(hashIdx + 1).trim())
    } catch {
      customName = input.slice(hashIdx + 1).trim()
    }
    input = input.slice(0, hashIdx).trim()
  }

  // Convert host:port@user:pass (when no @ precedes host:port) to host:port:user:pass
  const hostPortAtCreds = /^(?:(https?|socks5h?):\/\/)?(\[[^\]\s]+\]|[^:\s/@?#]+):(\d{1,5})@([^/\s]+)\/?$/i.exec(input)
  if (hostPortAtCreds) {
    const [, sch, h, p, creds] = hostPortAtCreds
    if (!creds.includes(':')) return null
    input = `${sch ? `${sch}://` : ''}${h}:${p}:${creds}`
  }

  const match = /^(?:(https?|socks5h?):\/\/)?(?:([^\s@/]+)@)?(\[[^\]\s]+\]|[^:\s/@?#]+):(\d{1,5})(?::([^:\s]*):(.*))?\/?$/i.exec(input)
  if (!match) return null
  const [, rawScheme, userInfo, rawHost, rawPort, colonUser, colonPassword] = match
  const schemeLower = rawScheme?.toLowerCase()
  const scheme = schemeLower === 'socks5h' ? 'socks5' : schemeLower
  if (scheme && !['http', 'https', 'socks5'].includes(scheme)) return null
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
  const resolvedScheme = (scheme || (defaultScheme === 'auto' ? 'http' : defaultScheme)) as ProxyConfig['type']
  if (resolvedScheme === 'socks5' && (new TextEncoder().encode(username).length > 255 || new TextEncoder().encode(password).length > 255)) return null
  return {
    id: '', name: customName || `${host}:${port}`, type: resolvedScheme,
    host, port, username, password, sendDomainToProxy: true,
  }
}
