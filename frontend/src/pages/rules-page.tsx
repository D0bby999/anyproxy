// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { t } from '../i18n'
import type { ProxyRule, ProxyConfig } from '../types'

export function RulesPage({
  rules,
  proxies,
  onSaveRule,
  onDeleteRule,
  onToggleRule,
  onMoveRule,
  onCloneRule,
}: {
  rules: ProxyRule[]
  proxies: ProxyConfig[]
  onSaveRule: (r: ProxyRule) => Promise<void>
  onDeleteRule: (id: string) => Promise<void>
  onToggleRule: (id: string, enabled: boolean) => Promise<void>
  onMoveRule: (id: string, delta: number) => Promise<void>
  onCloneRule: (id: string) => Promise<void>
}) {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<ProxyRule | null>(null)

  const handleOpenAdd = () => {
    setEditing({
      id: '',
      name: '',
      enabled: true,
      processNames: '*',
      targetHosts: '*',
      targetPorts: '*',
      protocol: 'both',
      action: 'direct',
    })
    setShowModal(true)
  }

  const handleEdit = (r: ProxyRule) => {
    setEditing({ ...r })
    setShowModal(true)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing) return
    await onSaveRule(editing)
    setShowModal(false)
  }

  return (
    <>
      <PageHead title={t.rules.title} sub={t.rules.sub}>
        <button className="btn primary" onClick={handleOpenAdd}>
          + {t.rules.newBtn}
        </button>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th style={{ width: 40 }}>{t.rules.order}</th>
                  <th>{t.rules.name}</th>
                  <th>{t.rules.applications}</th>
                  <th>{t.rules.targets}</th>
                  <th>{t.rules.ports}</th>
                  <th>{t.rules.protocol}</th>
                  <th>{t.rules.action}</th>
                  <th>Thao tác</th>
                </tr>
              </thead>
              <tbody>
                {rules.length === 0 ? (
                  <tr>
                    <td colSpan={8} style={{ textAlign: 'center', padding: 24 }} className="muted">
                      No rules configured. Connections follow the Default Proxy.
                    </td>
                  </tr>
                ) : (
                  rules.map((r, idx) => {
                    const matchedProxy = proxies.find((p) => p.id === r.proxyConfigId)
                    return (
                      <tr key={r.id}>
                        <td className="muted mono">{idx + 1}</td>
                        <td>
                          <strong>{r.name}</strong>
                        </td>
                        <td className="mono">{r.processNames || '*'}</td>
                        <td className="mono">{r.targetHosts || '*'}</td>
                        <td className="mono">{r.targetPorts || '*'}</td>
                        <td>
                          <span className="chip">{r.protocol.toUpperCase()}</span>
                        </td>
                        <td>
                          <span
                            className={`chip ${
                              r.action === 'proxy'
                                ? 'accent'
                                : r.action === 'block'
                                ? 'danger'
                                : 'ok'
                            }`}
                          >
                            {r.action.toUpperCase()}
                            {r.action === 'proxy' && matchedProxy
                              ? `: ${matchedProxy.name}`
                              : ''}
                          </span>
                        </td>
                        <td>
                          <div className="row">
                            <button
                              className="btn sm"
                              onClick={() => onToggleRule(r.id, !r.enabled)}
                            >
                              {r.enabled ? 'Disable' : 'Enable'}
                            </button>
                            <button
                              className="btn sm"
                              disabled={idx === 0}
                              onClick={() => onMoveRule(r.id, -1)}
                            >
                              ↑
                            </button>
                            <button
                              className="btn sm"
                              disabled={idx === rules.length - 1}
                              onClick={() => onMoveRule(r.id, 1)}
                            >
                              ↓
                            </button>
                            <button className="btn sm" onClick={() => onCloneRule(r.id)}>
                              Copy
                            </button>
                            <button className="btn sm" onClick={() => handleEdit(r)}>
                              {t.actions.edit}
                            </button>
                            <button
                              className="btn sm danger"
                              onClick={() => onDeleteRule(r.id)}
                            >
                              ✕
                            </button>
                          </div>
                        </td>
                      </tr>
                    )
                  })
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
                <h2>{editing.id ? t.actions.edit : t.rules.newBtn}</h2>
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
                  <label>{t.rules.name}</label>
                  <input
                    className="input"
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                    required
                  />
                </div>

                <div className="field">
                  <label>{t.rules.applications} (e.g. curl, chrome, * for all)</label>
                  <input
                    className="input mono"
                    value={editing.processNames}
                    onChange={(e) =>
                      setEditing({ ...editing, processNames: e.target.value })
                    }
                  />
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 2 }}>
                    <label>{t.rules.targets} (IP, domain, CIDR)</label>
                    <input
                      className="input mono"
                      value={editing.targetHosts}
                      onChange={(e) =>
                        setEditing({ ...editing, targetHosts: e.target.value })
                      }
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>{t.rules.ports}</label>
                    <input
                      className="input mono"
                      value={editing.targetPorts}
                      onChange={(e) =>
                        setEditing({ ...editing, targetPorts: e.target.value })
                      }
                    />
                  </div>
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 1 }}>
                    <label>{t.rules.protocol}</label>
                    <select
                      className="select"
                      value={editing.protocol}
                      onChange={(e) =>
                        setEditing({
                          ...editing,
                          protocol: e.target.value as ProxyRule['protocol'],
                        })
                      }
                    >
                      <option value="both">Both TCP & UDP</option>
                      <option value="tcp">TCP only</option>
                      <option value="udp">UDP only</option>
                    </select>
                  </div>

                  <div className="field" style={{ flex: 1 }}>
                    <label>{t.rules.action}</label>
                    <select
                      className="select"
                      value={editing.action}
                      onChange={(e) =>
                        setEditing({
                          ...editing,
                          action: e.target.value as ProxyRule['action'],
                        })
                      }
                    >
                      <option value="direct">Direct</option>
                      <option value="proxy">Proxy</option>
                      <option value="block">Block</option>
                    </select>
                  </div>
                </div>

                {editing.action === 'proxy' && (
                  <div className="field">
                    <label>Target Proxy Server</label>
                    <select
                      className="select"
                      value={editing.proxyConfigId || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, proxyConfigId: e.target.value })
                      }
                    >
                      <option value="">Default Proxy</option>
                      {proxies.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name} ({p.type} {p.host}:{p.port})
                        </option>
                      ))}
                    </select>
                  </div>
                )}
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
