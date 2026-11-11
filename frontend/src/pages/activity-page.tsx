// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { getLanguage, t } from '../i18n'
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
      <PageHead
        title={t.nav.activity}
        sub={t.activity.sub}
      >
        <div className="row nodrag" style={{ gap: 8 }}>
          <input
            className="input"
            style={{ width: 200, height: 28 }}
            placeholder={t.actions.search}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <button className="btn sm" onClick={onClearLogs} title={t.actions.clear}>
            <Icon name="trash" />
            {t.actions.clear}
          </button>
        </div>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="tablewrap">
            {filtered.length === 0 ? (
              <EmptyState
                icon="list"
                title={t.activity.empty}
                text={t.activity.emptyText}
              />
            ) : (
              <table className="table activity">
                <thead>
                  <tr>
                    <th className="col-time">{t.connections.time}</th>
                    <th className="col-level">{t.activity.level}</th>
                    <th className="col-msg">{t.activity.message}</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((l) => (
                    <tr key={l.id}>
                      <td className="muted mono col-time">
                        {new Date(l.timestamp).toLocaleTimeString(getLanguage() === 'vi' ? 'vi-VN' : 'en-US')}
                      </td>
                      <td className="col-level">
                        <span
                          className={`chip ${
                            l.level === 'ERROR'
                              ? 'danger'
                              : l.level === 'WARN'
                              ? 'warn'
                              : 'ok'
                          }`}
                        >
                          <span className="dot" />
                          <span className="chiptext">{l.level}</span>
                        </span>
                      </td>
                      <td className="mono selectable col-msg" style={{ wordBreak: 'break-all' }}>
                        {l.message}
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
