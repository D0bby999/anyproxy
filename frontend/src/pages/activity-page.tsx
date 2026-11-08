// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
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
      <PageHead
        title={t.nav.activity}
        sub="Nhật ký hoạt động của Driver Engine, sự kiện hệ thống và theo dõi kết nối."
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
                title="Chưa có nhật ký hoạt động nào"
                text="Các thông báo trạng thái của trình điều khiển mạng sẽ được ghi nhận tại đây."
              />
            ) : (
              <table className="table activity">
                <thead>
                  <tr>
                    <th className="col-time">Thời gian</th>
                    <th className="col-level">Mức độ</th>
                    <th className="col-msg">Nội dung thông điệp</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((l) => (
                    <tr key={l.id}>
                      <td className="muted mono col-time">
                        {new Date(l.timestamp).toLocaleTimeString()}
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
