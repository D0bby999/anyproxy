// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { getLanguage, t } from '../i18n'
import { formatBytes } from '../lib/format'
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
        <div className="row nodrag connection-toolbar" style={{ gap: 8 }}>
          <div
            className="row"
            style={{
              background: 'var(--surface-hover)',
              borderRadius: 'var(--r-md)',
              padding: 2,
            }}
          >
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
            style={{ width: 180, height: 28 }}
            placeholder={t.actions.search}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />

          <button
            className="btn sm"
            onClick={mode === 'live' ? onClearClosed : onClearLogs}
            title={t.actions.clear}
          >
            <Icon name="trash" />
            {t.actions.clear}
          </button>
        </div>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="tablewrap">
            {mode === 'live' ? (
              filteredFlows.length === 0 ? (
                <EmptyState
                  icon="plug"
                  title={t.connections.emptyLive}
                  text={t.connections.emptyLiveText}
                />
              ) : (
                <table className="table flows">
                  <thead>
                    <tr>
                      <th className="col-proc">{t.connections.process}</th>
                      <th className="col-dest">{t.connections.dest}</th>
                      <th className="col-proto">{t.connections.proto}</th>
                      <th className="col-route">{t.connections.action}</th>
                      <th className="col-traffic">{t.connections.traffic}</th>
                      <th className="col-state">{t.connections.status}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredFlows.map((f) => (
                      <tr
                        key={f.id}
                        className={f.state === 'open' ? 'running' : ''}
                      >
                        <td className="cellname col-proc">
                          <b className="selectable" title={f.processName}>
                            {f.processName}
                          </b>
                          <span className="mono">PID: {f.pid}</span>
                        </td>
                        <td className="col-dest">
                          <span
                            className="mono selectable ellipsis-text"
                            title={`${f.resolvedDomain || f.destIP}:${f.destPort}`}
                          >
                            {f.resolvedDomain || f.destIP}:{f.destPort}
                          </span>
                        </td>
                        <td className="col-proto">
                          <span className="chip">
                            <span className="chiptext">{f.protocol}</span>
                          </span>
                        </td>
                        <td className="col-route">
                          <span
                            className={`chip ${
                              f.action === 'proxy'
                                ? 'accent'
                                : f.action === 'block'
                                ? 'danger'
                                : 'ok'
                            }`}
                            title={
                              f.action === 'proxy' && f.proxyName
                                ? `Proxy: ${f.proxyName}`
                                : f.action.toUpperCase()
                            }
                          >
                            <span className="dot" />
                            <span className="chiptext">
                              {f.action.toUpperCase()}
                              {f.proxyName ? ` → ${f.proxyName}` : ''}
                            </span>
                          </span>
                        </td>
                        <td className="col-traffic">
                          <div className="traffic-stack mono" title={`${t.connections.sentTitle}: ${f.bytesOut} B, ${t.connections.receivedTitle}: ${f.bytesIn} B`}>
                            <span className="tx" title={t.connections.sentTitle}>↑ {formatBytes(f.bytesOut)}</span>
                            <span className="rx" title={t.connections.receivedTitle}>↓ {formatBytes(f.bytesIn)}</span>
                          </div>
                        </td>
                        <td className="col-state">
                          <span
                            className={`chip ${
                              f.state === 'open' ? 'ok' : ''
                            }`}
                          >
                            <span className="dot" />
                            <span className="chiptext">
                              {f.state.toUpperCase()}
                            </span>
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )
            ) : filteredLogs.length === 0 ? (
              <EmptyState
                icon="list"
                title={t.connections.emptyHistory}
                text={t.connections.emptyHistoryText}
              />
            ) : (
              <table className="table connection-logs">
                <thead>
                  <tr>
                    <th className="col-time">{t.connections.time}</th>
                    <th className="col-proc">{t.connections.process}</th>
                    <th className="col-dest">{t.connections.dest}</th>
                    <th className="col-proto">{t.connections.proto}</th>
                    <th className="col-route">{t.connections.action}</th>
                    <th className="col-rule">{t.connections.rule}</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredLogs.map((l) => (
                    <tr key={l.id}>
                      <td className="muted mono col-time">
                        {new Date(l.timestamp).toLocaleTimeString(getLanguage() === 'vi' ? 'vi-VN' : 'en-US')}
                      </td>
                      <td className="cellname col-proc">
                        <b className="selectable" title={l.processName}>
                          {l.processName}
                        </b>
                        <span className="mono">PID: {l.pid}</span>
                      </td>
                      <td className="col-dest">
                        <span
                          className="mono selectable ellipsis-text"
                          title={`${l.resolvedDomain || l.destIP}:${l.destPort}`}
                        >
                          {l.resolvedDomain || l.destIP}:{l.destPort}
                        </span>
                      </td>
                      <td className="col-proto">
                        <span className="chip">
                          <span className="chiptext">{l.protocol}</span>
                        </span>
                      </td>
                      <td className="col-route">
                        <span
                          className={`chip ${
                            l.action === 'proxy'
                              ? 'accent'
                              : l.action === 'block'
                              ? 'danger'
                              : 'ok'
                          }`}
                          title={
                            l.action === 'proxy' && l.proxyName
                              ? `Proxy: ${l.proxyName}`
                              : l.action.toUpperCase()
                          }
                        >
                          <span className="dot" />
                          <span className="chiptext">
                            {l.action.toUpperCase()}
                            {l.proxyName ? ` → ${l.proxyName}` : ''}
                          </span>
                        </span>
                      </td>
                      <td className="col-rule">
                        <span
                          className="chip ellipsis-text"
                          title={l.ruleName || t.connections.defaultRule}
                        >
                          <span className="chiptext">
                            {l.ruleName || t.connections.defaultRule}
                          </span>
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>
    </>
  )
}
