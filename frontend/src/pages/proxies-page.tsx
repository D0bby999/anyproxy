// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useEffect, useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { BuyProxies } from '../components/buy-proxies'
import { parseProxy } from '../lib/parse-proxy'
import { t } from '../i18n'
import { formatProxyLocation, type CheckReport, type ProxyConfig } from '../types'

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
  onSaveProxy: (px: ProxyConfig, opts?: { silent?: boolean }) => Promise<ProxyConfig | void>
  onDeleteProxy: (id: string) => Promise<void>
  onSetDefault: (id: string) => Promise<void>
  onTestProxy: (px: ProxyConfig, opts?: { silent?: boolean }) => Promise<CheckReport | undefined>
}) {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<ProxyConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [checking, setChecking] = useState('')
  const [pingingAll, setPingingAll] = useState(false)
  const [results, setResults] = useState<Record<string, CheckReport>>({})
  const [bulkText, setBulkText] = useState('')
  const [bulkBusy, setBulkBusy] = useState(false)
  const [bulkMessage, setBulkMessage] = useState('')
  const [defaultScheme, setDefaultScheme] = useState<'auto' | 'http' | 'https' | 'socks5'>('auto')
  const [confirmDelete, setConfirmDelete] = useState<ProxyConfig | null>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (confirmDelete) { setConfirmDelete(null); return }
      if (showModal && !saving) setShowModal(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [confirmDelete, showModal, saving])

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
    if (!editing || saving) return
    const host = editing.host.trim()
    const payload: ProxyConfig = {
      ...editing,
      host,
      name: editing.name.trim() || `${host}:${editing.port}`,
    }
    setSaving(true)
    setError('')
    try {
      const saved = await onSaveProxy(payload)
      setResults((prev) => {
        const next = { ...prev }
        delete next[editing.id]
        return next
      })
      const target = saved || payload
      if (target.id) {
        setChecking(target.id)
        try {
          const report = await onTestProxy(target)
          if (report) setResults((prev) => ({ ...prev, [target.id]: report }))
        } catch {
          // Proxy is saved; check error is already surfaced by onTestProxy toast.
        } finally {
          setChecking('')
        }
      }
      setShowModal(false)
    } catch (err) {
      setError(String(err))
    } finally {
      setSaving(false)
    }
  }

  const checkProxiesConcurrently = async (list: ProxyConfig[]): Promise<CheckReport[]> => {
    const targets = list.filter((p) => Boolean(p.id))
    const reports: CheckReport[] = []
    let cursor = 0
    const concurrency = Math.min(4, targets.length)
    const workers = Array.from({ length: concurrency }, async () => {
      while (cursor < targets.length) {
        const p = targets[cursor++]
        setChecking(p.id)
        try {
          const report = await onTestProxy(p, { silent: true })
          if (report) {
            setResults((prev) => ({ ...prev, [p.id]: report }))
            reports.push(report)
          }
        } catch {
          // Individual check error already handled by caller/toast
        }
      }
    })
    await Promise.all(workers)
    return reports
  }

  const handlePingAll = async () => {
    if (pingingAll || proxies.length === 0) return
    setPingingAll(true)
    try {
      await checkProxiesConcurrently(proxies)
    } finally {
      setChecking('')
      setPingingAll(false)
    }
  }

  const handleBulkAdd = async () => {
    const lines = bulkText.split(/\r?\n/).map((line) => line.trim())
    setBulkText('')
    setBulkMessage('')
    setBulkBusy(true)
    let added = 0
    const failed: number[] = []
    const savedList: ProxyConfig[] = []
    try {
      for (const [index, line] of lines.entries()) {
        if (!line) continue
        const proxy = parseProxy(line, defaultScheme)
        if (!proxy) { failed.push(index + 1); continue }
        try {
          const saved = await onSaveProxy(proxy, { silent: true })
          added++
          savedList.push(saved || proxy)
        } catch {
          failed.push(index + 1)
        }
      }

      const checkedReports = await checkProxiesConcurrently(savedList)

      let summaryText = added ? `${t.proxies.bulkAdded}: ${added}` : ''
      if (added === 1 && checkedReports.length === 1) {
        const rep = checkedReports[0]
        const success = rep.success ?? rep.ok
        const ms = rep.avgMs ?? rep.totalDurationMs
        summaryText += success ? ` (${ms} ms)` : ` (${t.messages.testFailed})`
      } else if (checkedReports.length > 1) {
        const successCount = checkedReports.filter((r) => r.success ?? r.ok).length
        summaryText += ` (${successCount}/${checkedReports.length} ${t.engine.running.toLowerCase()})`
      }

      setBulkMessage([summaryText, failed.length ? `${t.proxies.bulkFailed}: ${failed.join(', ')}` : ''].filter(Boolean).join(' · '))
    } finally {
      setChecking('')
      setBulkBusy(false)
    }
  }

  return (
    <>
      <PageHead title={t.proxies.title} sub={t.proxies.sub}>
        {proxies.length > 0 && (
          <button
            type="button"
            className={`btn nodrag${pingingAll ? ' busy' : ''}`}
            aria-busy={pingingAll}
            disabled={pingingAll || !!checking}
            onClick={() => void handlePingAll()}
          >
            <Icon name="refresh" />
            {t.engine.pingAll}
          </button>
        )}
        {defaultProxyId && (
          <button
            type="button"
            className="btn nodrag"
            onClick={() => void onSetDefault('')}
            title={t.engine.direct}
          >
            {t.engine.direct}
          </button>
        )}
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
            <button type="button" className={`btn primary${bulkBusy ? ' busy' : ''}`} disabled={bulkBusy || !bulkText.trim()} onClick={handleBulkAdd} aria-busy={bulkBusy}><Icon name={bulkBusy ? 'refresh' : 'plus'} />{bulkBusy ? t.proxies.bulkAdding : t.proxies.bulkAdd}</button>
            <label className="row" style={{ fontSize: 'var(--t-xs)', gap: 6 }}>
              {t.proxies.type}
              <select className="select" style={{ width: 130, height: 30 }} value={defaultScheme} disabled={bulkBusy} onChange={(e) => setDefaultScheme(e.target.value as 'auto' | 'http' | 'https' | 'socks5')}>
                <option value="auto">{t.proxies.schemeAuto}</option>
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
                <option value="socks5">SOCKS5</option>
              </select>
            </label>
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
                <div className="row">
                  <BuyProxies className="btn buy" />
                  <button className="btn primary" onClick={handleOpenAdd}>
                    <Icon name="plus" />
                    {t.proxies.newBtn}
                  </button>
                </div>
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
                    const rep = results[p.id]
                    const country = rep?.country || p.country
                    const exitIp = rep?.exitIp || p.exitIp
                    const loc = formatProxyLocation(country, exitIp)
                    return (
                      <tr
                        key={p.id}
                        className={`clickable-row ${isDefault ? 'active' : ''}`}
                        onClick={() => void onSetDefault(isDefault ? '' : p.id)}
                      >
                        <td className="cellname col-name">
                          <b className="selectable" title={p.name}>
                            {p.name}
                          </b>
                        </td>
                        <td className="col-type">
                          <span className="chip">
                            <span className="chiptext">{p.type.toUpperCase()}</span>
                          </span>
                        </td>
                        <td className="col-server">
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }}>
                            <span className="mono selectable ellipsis-text" title={`${p.host}:${p.port}`}>
                              {p.host}:{p.port}
                            </span>
                            {loc && (
                              <span
                                className="muted mono selectable ellipsis-text"
                                title={exitIp ? `${country} · ${exitIp}` : country}
                                style={{ fontSize: 11 }}
                              >
                                {loc}
                              </span>
                            )}
                          </div>
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
                        <td className="col-latency" onClick={(e) => e.stopPropagation()}>
                          <span className="row" style={{ justifyContent: 'flex-end' }}>
                            {results[p.id] ? (
                              <span className={`chip ${(results[p.id].success ?? results[p.id].ok) ? 'ok' : 'danger'}`} title={results[p.id].summary}>
                                <span className="dot" />
                                <span className="chiptext">{(results[p.id].success ?? results[p.id].ok) ? `${results[p.id].avgMs ?? results[p.id].totalDurationMs ?? 0} ms` : t.messages.testFailed}</span>
                              </span>
                            ) : <span className="faint" title={t.proxies.unchecked}>{t.proxies.unchecked}</span>}
                            <button
                              className={`iconbtn sm${checking === p.id ? ' busy' : ''}`}
                              disabled={!!checking || pingingAll}
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
                          </span>
                        </td>
                        <td className="col-actions" onClick={(e) => e.stopPropagation()}>
                          <span className="row" style={{ justifyContent: 'flex-end' }}>
                            {!isDefault ? (
                              <button
                                type="button"
                                className="btn sm"
                                onClick={() => void onSetDefault(p.id)}
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
                              type="button"
                              className="iconbtn sm"
                              onClick={() => handleEdit(p)}
                              title={t.actions.edit}
                              aria-label={t.actions.edit}
                            >
                              <Icon name="pencil" />
                            </button>
                            <button
                              type="button"
                              className="iconbtn sm danger"
                              onClick={() => setConfirmDelete(p)}
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
          <div className="dialog-box" role="dialog" aria-modal="true" aria-label={editing.id ? t.actions.edit : t.proxies.newBtn}>
            <form onSubmit={handleSave}>
              <div className="dialog-head">
                <h2>{editing.id ? t.actions.edit : t.proxies.newBtn}</h2>
                <button
                  type="button"
                  className="iconbtn sm"
                  disabled={saving}
                  onClick={() => setShowModal(false)}
                  aria-label={t.actions.cancel}
                >
                  <Icon name="close" />
                </button>
              </div>

              <div className="dialog-body">
                {error && <div className="banner danger" role="alert"><Icon name="warn" /><span>{error}</span></div>}
                <div className="field">
                  <label>{t.proxies.name}</label>
                  <input
                    className="input"
                    disabled={saving}
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                    placeholder={t.proxies.namePlaceholder}
                  />
                </div>

                <div className="field">
                  <label>{t.proxies.type}</label>
                  <select
                    className="select"
                    disabled={saving}
                    value={editing.type}
                    onChange={(e) =>
                      setEditing({ ...editing, type: e.target.value as ProxyConfig['type'] })
                    }
                  >
                    <option value="socks5">SOCKS5</option>
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                  </select>
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 3 }}>
                    <label>{t.proxies.host}</label>
                    <input
                      className="input mono"
                      disabled={saving}
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
                      min={1}
                      max={65535}
                      disabled={saving}
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
                      disabled={saving}
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
                      disabled={saving}
                      value={editing.password || ''}
                      onChange={(e) =>
                        setEditing({ ...editing, password: e.target.value })
                      }
                    />
                  </div>
                </div>
              </div>

              <div className="dialog-foot">
                {editing.id && (
                  <button
                    type="button"
                    className="btn danger"
                    disabled={saving}
                    style={{ marginRight: 'auto' }}
                    onClick={() => {
                      const target = editing
                      setShowModal(false)
                      setConfirmDelete(target)
                    }}
                  >
                    <Icon name="trash" />
                    {t.actions.delete}
                  </button>
                )}
                <button
                  type="button"
                  className="btn"
                  disabled={saving}
                  onClick={() => setShowModal(false)}
                >
                  {t.actions.cancel}
                </button>
                <button type="submit" className={`btn primary${saving ? ' busy' : ''}`} aria-busy={saving} disabled={saving}>
                  <Icon name={saving ? 'refresh' : 'plus'} />
                  {!editing.id
                    ? (saving ? t.proxies.addSubmitting : t.proxies.addSubmit)
                    : (saving ? t.proxies.saving : t.proxies.saveAndCheck)}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {confirmDelete && (
        <div className="dialog-backdrop" onClick={() => setConfirmDelete(null)}>
          <div className="dialog-box" role="dialog" aria-modal="true" aria-labelledby="confirm-delete-proxy-title" onClick={(e) => e.stopPropagation()}>
            <div className="dialog-head">
              <h2 id="confirm-delete-proxy-title">{t.proxies.confirmDelete}</h2>
              <button type="button" className="iconbtn sm" onClick={() => setConfirmDelete(null)} aria-label={t.actions.cancel}>
                <Icon name="close" />
              </button>
            </div>
            <div className="dialog-body">
              <p>{t.proxies.confirmDeleteBody(confirmDelete.name || `${confirmDelete.host}:${confirmDelete.port}`)}</p>
            </div>
            <div className="dialog-foot">
              <button type="button" className="btn" autoFocus onClick={() => setConfirmDelete(null)}>
                {t.actions.cancel}
              </button>
              <button
                type="button"
                className="btn danger"
                onClick={() => {
                  const id = confirmDelete.id
                  setConfirmDelete(null)
                  setResults((prev) => {
                    const next = { ...prev }
                    delete next[id]
                    return next
                  })
                  void onDeleteProxy(id)
                }}
              >
                <Icon name="trash" />
                {t.actions.delete}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
