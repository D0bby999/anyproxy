// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { t } from '../i18n'
import type { Flow, ConnectionLogEntry } from '../types'

export function ConnectionsPage({
  flows,
  logs,
  onClearClosed,
  onClearLogs,
}: {
  flows: Flow[]
  logs: ConnectionLogEntry[]
  onClearClosed: () => Promise<void>
  onClearLogs: () => Promise<void>
}) {
  const [mode, setMode] = useState<'live' | 'history'>('live')
  const [filter, setFilter] = useState('')

  const activeFlows = flows.filter((f) => f.state === 'open')
  const filteredFlows = flows.filter(
    (f) =>
      !filter ||
      f.processName.toLowerCase().includes(filter.toLowerCase()) ||
      f.destIP.includes(filter) ||
      f.resolvedDomain.toLowerCase().includes(filter.toLowerCase())
  )

  const filteredLogs = logs.filter(
    (l) =>
      !filter ||
      l.processName.toLowerCase().includes(filter.toLowerCase()) ||
      l.destIP.includes(filter) ||
      l.resolvedDomain.toLowerCase().includes(filter.toLowerCase())
  )

  return (
    <>
      <PageHead title={t.connections.title} sub={t.connections.sub}>
        <div className="row" style={{ gap: 8 }}>
          <div className="row" style={{ background: 'var(--surface-hover)', borderRadius: 'var(--r-md)', padding: 2 }}>
            <button
              className={`btn sm ${mode === 'live' ? 'primary' : ''}`}
              onClick={() => setMode('live')}
            >
              {t.connections.live} ({activeFlows.length})
            </button>
            <button
              className={`btn sm ${mode === 'history' ? 'primary' : ''}`}
              onClick={() => setMode('history')}
            >
              {t.connections.history} ({logs.length})
            </button>
          </div>

          <input
            className="input"
            style={{ width: 160, height: 28 }}
            placeholder={t.actions.search}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />

          <button
            className="btn sm"
            onClick={mode === 'live' ? onClearClosed : onClearLogs}
          >
            {t.actions.clear}
          </button>
        </div>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="table-wrap">
            {mode === 'live' ? (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t.connections.process}</th>
                    <th>{t.connections.dest}</th>
                    <th>{t.connections.proto}</th>
                    <th>{t.connections.action}</th>
                    <th>{t.connections.sent}</th>
                    <th>{t.connections.received}</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredFlows.length === 0 ? (
                    <tr>
                      <td colSpan={7} style={{ textAlign: 'center', padding: 24 }} className="muted">
                        No active network flows captured.
                      </td>
                    </tr>
                  ) : (
                    filteredFlows.map((f) => (
                      <tr key={f.id}>
                        <td>
                          <strong>{f.processName}</strong>{' '}
                          <span className="muted mono">({f.pid})</span>
                        </td>
                        <td className="mono">
                          {f.resolvedDomain || f.destIP}:{f.destPort}
                        </td>
                        <td>
                          <span className="chip">{f.protocol}</span>
                        </td>
                        <td>
                          <span
                            className={`chip ${
                              f.action === 'proxy'
                                ? 'accent'
                                : f.action === 'block'
                                ? 'danger'
                                : 'ok'
                            }`}
                          >
                            {f.action.toUpperCase()}
                            {f.proxyName ? ` -> ${f.proxyName}` : ''}
                          </span>
                        </td>
                        <td className="mono">{f.bytesOut} B</td>
                        <td className="mono">{f.bytesIn} B</td>
                        <td>
                          <span className={`chip ${f.state === 'open' ? 'ok' : ''}`}>
                            {f.state.toUpperCase()}
                          </span>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>{t.connections.process}</th>
                    <th>{t.connections.dest}</th>
                    <th>{t.connections.proto}</th>
                    <th>{t.connections.action}</th>
                    <th>Rule</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredLogs.length === 0 ? (
                    <tr>
                      <td colSpan={6} style={{ textAlign: 'center', padding: 24 }} className="muted">
                        No connection history logs yet.
                      </td>
                    </tr>
                  ) : (
                    filteredLogs.map((l) => (
                      <tr key={l.id}>
                        <td className="muted mono">
                          {new Date(l.timestamp).toLocaleTimeString()}
                        </td>
                        <td>
                          <strong>{l.processName}</strong>{' '}
                          <span className="muted mono">({l.pid})</span>
                        </td>
                        <td className="mono">
                          {l.resolvedDomain || l.destIP}:{l.destPort}
                        </td>
                        <td>
                          <span className="chip">{l.protocol}</span>
                        </td>
                        <td>
                          <span
                            className={`chip ${
                              l.action === 'proxy'
                                ? 'accent'
                                : l.action === 'block'
                                ? 'danger'
                                : 'ok'
                            }`}
                          >
                            {l.action.toUpperCase()}
                            {l.proxyName ? ` via ${l.proxyName}` : ''}
                          </span>
                        </td>
                        <td className="muted">{l.ruleName || 'Default'}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>
    </>
  )
}
