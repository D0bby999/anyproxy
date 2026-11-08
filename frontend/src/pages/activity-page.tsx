// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { t } from '../i18n'
import type { ActivityLogEntry } from '../types'

export function ActivityPage({
  logs,
  onClearLogs,
}: {
  logs: ActivityLogEntry[]
  onClearLogs: () => Promise<void>
}) {
  const [filter, setFilter] = useState('')

  const filtered = logs.filter(
    (l) => !filter || l.message.toLowerCase().includes(filter.toLowerCase())
  )

  return (
    <>
      <PageHead title={t.nav.activity} sub="Engine lifecycle, driver events, and diagnostic traces.">
        <div className="row" style={{ gap: 8 }}>
          <input
            className="input"
            style={{ width: 180, height: 28 }}
            placeholder={t.actions.search}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <button className="btn sm" onClick={onClearLogs}>
            {t.actions.clear}
          </button>
        </div>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th style={{ width: 120 }}>Time</th>
                  <th style={{ width: 80 }}>Level</th>
                  <th>Message</th>
                </tr>
              </thead>
              <tbody>
                {filtered.length === 0 ? (
                  <tr>
                    <td colSpan={3} style={{ textAlign: 'center', padding: 24 }} className="muted">
                      No activity logs recorded.
                    </td>
                  </tr>
                ) : (
                  filtered.map((l) => (
                    <tr key={l.id}>
                      <td className="muted mono">
                        {new Date(l.timestamp).toLocaleTimeString()}
                      </td>
                      <td>
                        <span
                          className={`chip ${
                            l.level === 'ERROR'
                              ? 'danger'
                              : l.level === 'WARN'
                              ? 'warn'
                              : 'ok'
                          }`}
                        >
                          {l.level}
                        </span>
                      </td>
                      <td className="mono selectable">{l.message}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </>
  )
}
