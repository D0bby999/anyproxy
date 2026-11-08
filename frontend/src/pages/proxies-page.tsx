// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { t } from '../i18n'
import type { ProxyConfig } from '../types'

export function ProxiesPage({
  proxies,
  defaultProxyId,
  onSaveProxy,
  onDeleteProxy,
  onSetDefault,
  onTestProxy,
}: {
  proxies: ProxyConfig[]
  defaultProxyId: string
  onSaveProxy: (px: ProxyConfig) => Promise<void>
  onDeleteProxy: (id: string) => Promise<void>
  onSetDefault: (id: string) => Promise<void>
  onTestProxy: (px: ProxyConfig) => void
}) {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<ProxyConfig | null>(null)

  const handleOpenAdd = () => {
    setEditing({
      id: '',
      name: '',
      type: 'socks5',
      host: '127.0.0.1',
      port: 1080,
      username: '',
      password: '',
      sendDomainToProxy: true,
    })
    setShowModal(true)
  }

  const handleEdit = (p: ProxyConfig) => {
    setEditing({ ...p })
    setShowModal(true)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing) return
    await onSaveProxy(editing)
    setShowModal(false)
  }

  return (
    <>
      <PageHead title={t.proxies.title} sub={t.proxies.sub}>
        <button className="btn primary nodrag" onClick={handleOpenAdd}>
          <Icon name="plus" />
          {t.proxies.newBtn}
        </button>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="tablewrap">
            {proxies.length === 0 ? (
              <EmptyState
                icon="proxy"
                title="Chưa có máy chủ Proxy nào"
                text="Thêm proxy để bắt đầu chuyển hướng lưu lượng mạng cho các ứng dụng."
              >
                <button className="btn primary" onClick={handleOpenAdd}>
                  <Icon name="plus" />
                  {t.proxies.newBtn}
                </button>
              </EmptyState>
            ) : (
              <table className="table proxies">
                <thead>
                  <tr>
                    <th className="col-name">{t.proxies.name}</th>
                    <th className="col-type">{t.proxies.type}</th>
                    <th className="col-server">{t.proxies.server}</th>
                    <th className="col-auth">{t.proxies.auth}</th>
                    <th className="col-latency">{t.proxies.latency}</th>
                    <th className="col-actions" />
                  </tr>
                </thead>
                <tbody>
                  {proxies.map((p) => {
                    const isDefault = p.id === defaultProxyId
                    return (
                      <tr key={p.id} className={isDefault ? 'active' : ''}>
                        <td className="cellname">
                          <b className="selectable" title={p.name}>
                            {p.name}
                          </b>
                          <span className="mono" title={`${p.type}://${p.host}:${p.port}`}>
                            {p.type}://{p.host}:{p.port}
                          </span>
                        </td>
                        <td>
                          <span className="chip">
                            <span className="chiptext">{p.type.toUpperCase()}</span>
                          </span>
                        </td>
                        <td>
                          <span className="mono selectable ellipsis-text" title={`${p.host}:${p.port}`}>
                            {p.host}:{p.port}
                          </span>
                        </td>
                        <td>
                          {p.username ? (
                            <span className="chip accent" title={`User: ${p.username}`}>
                              <span className="dot" />
                              <span className="chiptext">User / Pass</span>
                            </span>
                          ) : (
                            <span className="faint">—</span>
                          )}
                        </td>
                        <td>
                          {p.latencyMs !== undefined ? (
                            <span className={`chip ${p.latencyMs < 300 ? 'ok' : 'warn'}`}>
                              <span className="dot" />
                              <span className="chiptext">{p.latencyMs} ms</span>
                            </span>
                          ) : (
                            <span className="faint">—</span>
                          )}
                        </td>
                        <td className="col-actions">
                          <span className="row">
                            <button
                              className="iconbtn sm"
                              onClick={() => onTestProxy(p)}
                              title={t.actions.test}
                              aria-label={t.actions.test}
                            >
                              <Icon name="refresh" />
                            </button>
                            {!isDefault ? (
                              <button
                                className="btn sm"
                                onClick={() => onSetDefault(p.id)}
                                title={t.actions.setActive}
                              >
                                {t.actions.setActive}
                              </button>
                            ) : (
                              <span className="chip ok" title="Proxy mặc định">
                                <span className="dot" />
                                <span className="chiptext">Active</span>
                              </span>
                            )}
                            <button
                              className="iconbtn sm"
                              onClick={() => handleEdit(p)}
                              title={t.actions.edit}
                              aria-label={t.actions.edit}
                            >
                              <Icon name="copy" />
                            </button>
                            <button
                              className="iconbtn sm"
                              onClick={() => onDeleteProxy(p.id)}
                              title={t.actions.delete}
                              aria-label={t.actions.delete}
                            >
                              <Icon name="trash" />
                            </button>
                          </span>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>

      {showModal && editing && (
        <div className="dialog-backdrop">
          <div className="dialog-box">
            <form onSubmit={handleSave}>
              <div className="dialog-head">
                <h2>{editing.id ? t.actions.edit : t.proxies.newBtn}</h2>
                <button
                  type="button"
                  className="iconbtn sm"
                  onClick={() => setShowModal(false)}
                >
                  <Icon name="close" />
                </button>
              </div>

              <div className="dialog-body">
                <div className="field">
                  <label>{t.proxies.name}</label>
                  <input
                    className="input"
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                    placeholder="VD: Clash Local / SOCKS5 Singapore"
                    required
                  />
                </div>

                <div className="field">
                  <label>{t.proxies.type}</label>
                  <select
                    className="select"
                    value={editing.type}
                    onChange={(e) =>
                      setEditing({ ...editing, type: e.target.value as ProxyConfig['type'] })
                    }
                  >
                    <option value="socks5">SOCKS5</option>
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                    <option value="socks4">SOCKS4</option>
                  </select>
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 3 }}>
                    <label>Host</label>
                    <input
                      className="input mono"
                      value={editing.host}
                      onChange={(e) => setEditing({ ...editing, host: e.target.value })}
                      placeholder="127.0.0.1"
                      required
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>Port</label>
                    <input
                      className="input mono"
                      type="number"
                      value={editing.port}
                      onChange={(e) =>
                        setEditing({ ...editing, port: parseInt(e.target.value, 10) || 0 })
                      }
                      required
                    />
                  </div>
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 1 }}>
                    <label>Username (tùy chọn)</label>
                    <input
                      className="input"
                      value={editing.username || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, username: e.target.value })
                      }
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>Password (tùy chọn)</label>
                    <input
                      className="input"
                      type="password"
                      value={editing.password || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, password: e.target.value })
                      }
                    />
                  </div>
                </div>
              </div>

              <div className="dialog-foot">
                <button
                  type="button"
                  className="btn"
                  onClick={() => setShowModal(false)}
                >
                  {t.actions.cancel}
                </button>
                <button type="submit" className="btn primary">
                  {t.actions.save}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
