// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
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
        <button className="btn primary" onClick={handleOpenAdd}>
          + {t.proxies.newBtn}
        </button>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t.proxies.name}</th>
                  <th>{t.proxies.type}</th>
                  <th>{t.proxies.server}</th>
                  <th>{t.proxies.auth}</th>
                  <th>{t.proxies.latency}</th>
                  <th>{t.proxies.actions}</th>
                </tr>
              </thead>
              <tbody>
                {proxies.length === 0 ? (
                  <tr>
                    <td colSpan={6} style={{ textAlign: 'center', padding: 24 }} className="muted">
                      No proxy servers configured. Click "+ Add Proxy" to create one.
                    </td>
                  </tr>
                ) : (
                  proxies.map((p) => (
                    <tr key={p.id}>
                      <td>
                        <strong>{p.name}</strong>
                        {p.id === defaultProxyId && (
                          <span className="chip ok" style={{ marginLeft: 8 }}>
                            Default
                          </span>
                        )}
                      </td>
                      <td>
                        <span className="chip">{p.type.toUpperCase()}</span>
                      </td>
                      <td className="mono">
                        {p.host}:{p.port}
                      </td>
                      <td>{p.username ? 'Yes' : 'None'}</td>
                      <td>
                        {p.latencyMs !== undefined ? (
                          <span className={`chip ${p.latencyMs < 300 ? 'ok' : 'warn'}`}>
                            {p.latencyMs} ms
                          </span>
                        ) : (
                          <span className="muted">-</span>
                        )}
                      </td>
                      <td>
                        <div className="row">
                          <button className="btn sm" onClick={() => onTestProxy(p)}>
                            {t.actions.test}
                          </button>
                          {p.id !== defaultProxyId && (
                            <button className="btn sm" onClick={() => onSetDefault(p.id)}>
                              {t.actions.setActive}
                            </button>
                          )}
                          <button className="btn sm" onClick={() => handleEdit(p)}>
                            {t.actions.edit}
                          </button>
                          <button className="btn sm danger" onClick={() => onDeleteProxy(p.id)}>
                            {t.actions.delete}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
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
                  className="btn sm"
                  onClick={() => setShowModal(false)}
                >
                  ✕
                </button>
              </div>

              <div className="dialog-body">
                <div className="field">
                  <label>{t.proxies.name}</label>
                  <input
                    className="input"
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
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
                    <label>Username</label>
                    <input
                      className="input"
                      value={editing.username || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, username: e.target.value })
                      }
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>Password</label>
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
