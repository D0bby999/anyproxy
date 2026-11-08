// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
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
        <div className="row nodrag" style={{ gap: 8 }}>
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
                  title="Chưa có luồng kết nối trực tiếp nào"
                  text="Khi ProxyBridge được bật và các ứng dụng gửi gói tin mạng, các luồng kết nối sẽ hiển thị theo thời gian thực tại đây."
                />
              ) : (
                <table className="table flows">
                  <thead>
                    <tr>
                      <th>{t.connections.process}</th>
                      <th>{t.connections.dest}</th>
                      <th>{t.connections.proto}</th>
                      <th>{t.connections.action}</th>
                      <th>{t.connections.sent}</th>
                      <th>{t.connections.received}</th>
                      <th className="col-actions">Trạng thái</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredFlows.map((f) => (
                      <tr
                        key={f.id}
                        className={f.state === 'open' ? 'running' : ''}
                      >
                        <td className="cellname">
                          <b className="selectable" title={f.processName}>
                            {f.processName}
                          </b>
                          <span className="mono">PID: {f.pid}</span>
                        </td>
                        <td>
                          <span className="cellproxy">
                            <span className="cc">{f.protocol}</span>
                            <span className="mono selectable" title={f.destIP}>
                              {f.resolvedDomain || f.destIP}:{f.destPort}
                            </span>
                          </span>
                        </td>
                        <td>
                          <span className="chip">
                            <span className="chiptext">{f.protocol}</span>
                          </span>
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
                            <span className="dot" />
                            <span className="chiptext">
                              {f.action.toUpperCase()}
                              {f.proxyName ? ` → ${f.proxyName}` : ''}
                            </span>
                          </span>
                        </td>
                        <td className="mono">{f.bytesOut} B</td>
                        <td className="mono">{f.bytesIn} B</td>
                        <td className="col-actions">
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
                title="Chưa có nhật ký kết nối nào"
                text="Lịch sử các phiên kết nối đã hoàn thành sẽ được lưu trữ và hiển thị tại đây."
              />
            ) : (
              <table className="table connection-logs">
                <thead>
                  <tr>
                    <th style={{ width: 110 }}>Thời gian</th>
                    <th>{t.connections.process}</th>
                    <th>{t.connections.dest}</th>
                    <th>{t.connections.proto}</th>
                    <th>{t.connections.action}</th>
                    <th>Quy tắc áp dụng</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredLogs.map((l) => (
                    <tr key={l.id}>
                      <td className="muted mono">
                        {new Date(l.timestamp).toLocaleTimeString()}
                      </td>
                      <td className="cellname">
                        <b className="selectable">{l.processName}</b>
                        <span className="mono">PID: {l.pid}</span>
                      </td>
                      <td>
                        <span className="cellproxy">
                          <span className="mono selectable">
                            {l.resolvedDomain || l.destIP}:{l.destPort}
                          </span>
                        </span>
                      </td>
                      <td>
                        <span className="chip">
                          <span className="chiptext">{l.protocol}</span>
                        </span>
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
                          <span className="dot" />
                          <span className="chiptext">
                            {l.action.toUpperCase()}
                            {l.proxyName ? ` qua ${l.proxyName}` : ''}
                          </span>
                        </span>
                      </td>
                      <td className="muted">
                        <span className="chip">{l.ruleName || 'Default'}</span>
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
