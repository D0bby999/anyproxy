// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
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
  const [error, setError] = useState('')

  const handleOpenAdd = () => {
    setError('')
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
    setError('')
    setEditing({ ...r, action: r.action.toLowerCase() as ProxyRule['action'], protocol: r.protocol.toLowerCase() as ProxyRule['protocol'] })
    setShowModal(true)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing) return
    try {
      await onSaveRule(editing)
      setShowModal(false)
    } catch (err) {
      setError(String(err))
    }
  }

  return (
    <>
      <PageHead title={t.rules.title} sub={t.rules.sub}>
        <button className="btn primary nodrag" onClick={handleOpenAdd}>
          <Icon name="plus" />
          {t.rules.newBtn}
        </button>
      </PageHead>

      <div className="pagebody">
        <div className="card fill">
          <div className="tablewrap">
            {rules.length === 0 ? (
              <EmptyState
                icon="rules"
                title={t.rules.emptyTitle}
                text={t.rules.emptyText}
              >
                <button className="btn primary" onClick={handleOpenAdd}>
                  <Icon name="plus" />
                  {t.rules.newBtn}
                </button>
              </EmptyState>
            ) : (
              <table className="table rules">
                <thead>
                  <tr>
                    <th className="col-no">{t.rules.order}</th>
                    <th className="col-name">{t.rules.name}</th>
                    <th className="col-apps">{t.rules.applications}</th>
                    <th className="col-targets">{t.rules.targets}</th>
                    <th className="col-ports">{t.rules.ports}</th>
                    <th className="col-proto">{t.rules.protocol}</th>
                    <th className="col-action">{t.rules.action}</th>
                    <th className="col-actions" />
                  </tr>
                </thead>
                <tbody>
                  {rules.map((r, idx) => {
                    const matchedProxy = proxies.find((p) => p.id === r.proxyConfigId)
                    const action = r.action.toLowerCase()
                    return (
                      <tr key={r.id}>
                        <td className="col-no">{idx + 1}</td>
                        <td className="cellname col-name">
                          <b className="selectable" title={r.name}>
                            {r.name}
                          </b>
                          <span className="sub">
                            {r.enabled ? t.rules.enabled : t.rules.disabled}
                          </span>
                        </td>
                        <td className="col-apps">
                          <span className="mono selectable ellipsis-text" title={r.processNames}>
                            {r.processNames || '*'}
                          </span>
                        </td>
                        <td className="col-targets">
                          <span className="mono selectable ellipsis-text" title={r.targetHosts}>
                            {r.targetHosts || '*'}
                          </span>
                        </td>
                        <td className="col-ports">
                          <span className="mono selectable ellipsis-text" title={r.targetPorts}>
                            {r.targetPorts || '*'}
                          </span>
                        </td>
                        <td className="col-proto">
                          <span className="chip">
                            <span className="chiptext">{r.protocol.toUpperCase()}</span>
                          </span>
                        </td>
                        <td className="col-action">
                          <span
                            className={`chip ${
                              action === 'proxy'
                                ? 'accent'
                                : action === 'block'
                                ? 'danger'
                                : 'ok'
                            }`}
                            title={
                              action === 'proxy' && matchedProxy
                                ? `Proxy: ${matchedProxy.name}`
                                : r.action.toUpperCase()
                            }
                          >
                            <span className="dot" />
                            <span className="chiptext">
                              {r.action.toUpperCase()}
                              {action === 'proxy' && matchedProxy
                                ? `: ${matchedProxy.name}`
                                : ''}
                            </span>
                          </span>
                        </td>
                        <td className="col-actions">
                          <span className="row">
                            <button
                              className="btn sm"
                              onClick={() => onToggleRule(r.id, !r.enabled)}
                              title={r.enabled ? t.rules.disable : t.rules.enable}
                            >
                              {r.enabled ? t.actions.disable : t.actions.enable}
                            </button>
                            <button
                              className="iconbtn sm"
                              disabled={idx === 0}
                              onClick={() => onMoveRule(r.id, -1)}
                              title={t.actions.moveUp}
                              aria-label={t.actions.moveUp}
                            >
                              <Icon name="arrow-up" />
                            </button>
                            <button
                              className="iconbtn sm"
                              disabled={idx === rules.length - 1}
                              onClick={() => onMoveRule(r.id, 1)}
                              title={t.actions.moveDown}
                              aria-label={t.actions.moveDown}
                            >
                              <Icon name="arrow-down" />
                            </button>
                            <button
                              className="iconbtn sm"
                              onClick={() => onCloneRule(r.id)}
                              title={t.actions.clone}
                              aria-label={t.actions.clone}
                            >
                              <Icon name="copy" />
                            </button>
                            <button
                              className="iconbtn sm"
                              onClick={() => handleEdit(r)}
                              title={t.actions.edit}
                              aria-label={t.actions.edit}
                            >
                              <Icon name="pencil" />
                            </button>
                            <button
                              className="iconbtn sm"
                              onClick={() => { if (window.confirm(`${t.rules.confirmDelete}\n${r.name}`)) void onDeleteRule(r.id) }}
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
                <h2>{editing.id ? t.actions.edit : t.rules.newBtn}</h2>
                <button
                  type="button"
                  className="iconbtn sm"
                  onClick={() => setShowModal(false)}
                  aria-label={t.actions.cancel}
                >
                  <Icon name="close" />
                </button>
              </div>

              <div className="dialog-body">
                {error && <p role="alert">{error}</p>}
                <div className="field">
                  <label>{t.rules.name}</label>
                  <input
                    className="input"
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                    placeholder={t.rules.namePlaceholder}
                    required
                  />
                </div>

                <div className="field">
                  <label>{t.rules.applications} ({t.rules.processHint})</label>
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
                    <label>{t.rules.targets} ({t.rules.targetHint})</label>
                    <input
                      className="input mono"
                      value={editing.targetHosts}
                      onChange={(e) =>
                        setEditing({ ...editing, targetHosts: e.target.value })
                      }
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>{t.rules.ports} ({t.rules.portHint})</label>
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
                      <option value="both">{t.rules.both}</option>
                      <option value="tcp">{t.rules.tcp}</option>
                      <option value="udp">{t.rules.udp}</option>
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
                      <option value="direct">{t.rules.direct}</option>
                      <option value="proxy">{t.rules.proxy}</option>
                      <option value="block">{t.rules.block}</option>
                    </select>
                  </div>
                </div>

                {editing.action === 'proxy' && (
                  <div className="field">
                    <label>{t.rules.selectProxy}</label>
                    <select
                      className="select"
                      value={editing.proxyConfigId || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, proxyConfigId: e.target.value })
                      }
                    >
                      <option value="">{t.rules.defaultProxy}</option>
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
