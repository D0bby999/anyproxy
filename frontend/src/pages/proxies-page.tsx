// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { BuyProxies } from '../components/buy-proxies'
import { parseProxy } from '../lib/parse-proxy'
import { t } from '../i18n'
import type { CheckReport, ProxyConfig } from '../types'

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
  onTestProxy: (px: ProxyConfig) => Promise<CheckReport | undefined>
}) {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<ProxyConfig | null>(null)
  const [error, setError] = useState('')
  const [checking, setChecking] = useState('')
  const [results, setResults] = useState<Record<string, CheckReport>>({})
  const [bulkText, setBulkText] = useState('')
  const [bulkBusy, setBulkBusy] = useState(false)
  const [bulkMessage, setBulkMessage] = useState('')

  const handleOpenAdd = () => {
    setError('')
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
    setError('')
    setEditing({ ...p, type: p.type.toLowerCase() as ProxyConfig['type'] })
    setShowModal(true)
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing) return
    try {
      await onSaveProxy(editing)
      setResults((prev) => {
        const next = { ...prev }
        delete next[editing.id]
        return next
      })
      setShowModal(false)
    } catch (err) {
      setError(String(err))
    }
  }

  const handleBulkAdd = async () => {
    const lines = bulkText.split(/\r?\n/).map((line) => line.trim())
    setBulkText('')
    setBulkMessage('')
    setBulkBusy(true)
    let added = 0
    const failed: number[] = []
    try {
      for (const [index, line] of lines.entries()) {
        if (!line) continue
        const proxy = parseProxy(line)
        if (!proxy) { failed.push(index + 1); continue }
        try { await onSaveProxy(proxy); added++ } catch { failed.push(index + 1) }
      }
      setBulkMessage([added ? `${t.proxies.bulkAdded}: ${added}` : '', failed.length ? `${t.proxies.bulkFailed}: ${failed.join(', ')}` : ''].filter(Boolean).join(' · '))
    } finally { setBulkBusy(false) }
  }

  return (
    <>
      <PageHead title={t.proxies.title} sub={t.proxies.sub}>
        <BuyProxies />
        <button className="btn primary nodrag" onClick={handleOpenAdd}>
          <Icon name="plus" />
          {t.proxies.newBtn}
        </button>
      </PageHead>

      <div className="pagebody">
        <section className="card pad proxy-import">
          <div>
            <h2>{t.proxies.bulkTitle}</h2>
            <p>{t.proxies.bulkDesc}</p>
          </div>
          <textarea className="input mono proxy-paste" rows={3} value={bulkText} onChange={(e) => setBulkText(e.target.value)} placeholder={t.proxies.bulkPlaceholder} aria-label={t.proxies.bulkTitle} spellCheck={false} autoComplete="off" />
          <div className="proxy-import-foot">
            <button type="button" className="btn primary" disabled={bulkBusy || !bulkText.trim()} onClick={handleBulkAdd} aria-busy={bulkBusy}><Icon name="plus" />{bulkBusy ? t.proxies.bulkAdding : t.proxies.bulkAdd}</button>
            <span className="muted">{t.proxies.bulkSecret}</span>
          </div>
          {bulkMessage && <p className="proxy-import-result" role="status">{bulkMessage}</p>}
        </section>
        <div className="card fill proxy-list">
          <div className="tablewrap">
            {proxies.length === 0 ? (
              <EmptyState
                icon="proxy"
                title={t.proxies.emptyTitle}
                text={t.proxies.emptyText}
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
                    <th className="col-actions">{t.proxies.actions}</th>
                  </tr>
                </thead>
                <tbody>
                  {proxies.map((p) => {
                    const isDefault = p.id === defaultProxyId
                    return (
                      <tr key={p.id} className={isDefault ? 'active' : ''}>
                        <td className="cellname col-name">
                          <b className="selectable" title={p.name}>
                            {p.name}
                          </b>
                          <span className="mono" title={`${p.type}://${p.host}:${p.port}`}>
                            {p.type}://{p.host}:{p.port}
                          </span>
                        </td>
                        <td className="col-type">
                          <span className="chip">
                            <span className="chiptext">{p.type.toUpperCase()}</span>
                          </span>
                        </td>
                        <td className="col-server">
                          <span className="mono selectable ellipsis-text" title={`${p.host}:${p.port}`}>
                            {p.host}:{p.port}
                          </span>
                        </td>
                        <td className="col-auth">
                          {p.username ? (
                            <span className="chip accent" title={`User: ${p.username}`}>
                              <span className="dot" />
                              <span className="chiptext">{t.proxies.authProvided}</span>
                            </span>
                          ) : (
                            <span className="faint">—</span>
                          )}
                        </td>
                        <td className="col-latency">
                          {results[p.id] ? (
                            <span className={`chip ${(results[p.id].success ?? results[p.id].ok) ? 'ok' : 'danger'}`} title={results[p.id].summary}>
                              <span className="dot" />
                              <span className="chiptext">{(results[p.id].success ?? results[p.id].ok) ? `${results[p.id].avgMs ?? results[p.id].totalDurationMs ?? 0} ms` : t.messages.testFailed}</span>
                            </span>
                          ) : <span className="faint">{t.proxies.unchecked}</span>}
                        </td>
                        <td className="col-actions">
                          <span className="row">
                            <button
                              className="iconbtn sm"
                              disabled={!!checking}
                              aria-busy={checking === p.id}
                              onClick={async () => {
                                setChecking(p.id)
                                try {
                                  const report = await onTestProxy(p)
                                  if (report) setResults((prev) => ({ ...prev, [p.id]: report }))
                                } finally { setChecking('') }
                              }}
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
                              <span className="chip ok" title={t.proxies.active}>
                                <span className="dot" />
                                <span className="chiptext">{t.proxies.active}</span>
                              </span>
                            )}
                            <button
                              className="iconbtn sm"
                              onClick={() => handleEdit(p)}
                              title={t.actions.edit}
                              aria-label={t.actions.edit}
                            >
                              <Icon name="pencil" />
                            </button>
                            <button
                              className="iconbtn sm"
                              onClick={() => { if (window.confirm(`${t.proxies.confirmDelete}\n${p.name}`)) void onDeleteProxy(p.id) }}
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
                  aria-label={t.actions.cancel}
                >
                  <Icon name="close" />
                </button>
              </div>

              <div className="dialog-body">
                {error && <p role="alert">{error}</p>}
                <div className="field">
                  <label>{t.proxies.name}</label>
                  <input
                    className="input"
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                    placeholder={t.proxies.namePlaceholder}
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
                  </select>
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 3 }}>
                    <label>{t.proxies.host}</label>
                    <input
                      className="input mono"
                      value={editing.host}
                      onChange={(e) => setEditing({ ...editing, host: e.target.value })}
                      placeholder="127.0.0.1"
                      required
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>{t.proxies.port}</label>
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
                    <label>{t.proxies.username}</label>
                    <input
                      className="input"
                      value={editing.username || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, username: e.target.value })
                      }
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>{t.proxies.password}</label>
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
