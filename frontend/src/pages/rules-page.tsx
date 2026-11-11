// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { t } from '../i18n'
import type { ProxyRule, ProxyConfig } from '../types'

function extractAppNameFromPath(fullPath: string): string {
  if (!fullPath) return ''
  const appMatch = fullPath.match(/([^/\\]+)\.app(?:[/\\]|$)/i)
  if (appMatch && appMatch[1]) return appMatch[1]
  const base = fullPath.split(/[/\\]/).filter(Boolean).pop() || ''
  return base.replace(/\.exe$/i, '')
}

export function RulesPage({
  rules,
  proxies,
  knownApps = [],
  onSaveRule,
  onDeleteRule,
  onToggleRule,
  onMoveRule,
  onCloneRule,
  onPickExecutable,
  onImportRules,
  onExportRules,
}: {
  rules: ProxyRule[]
  proxies: ProxyConfig[]
  knownApps?: string[]
  onSaveRule: (r: ProxyRule) => Promise<void>
  onDeleteRule: (id: string) => Promise<void>
  onToggleRule: (id: string, enabled: boolean) => Promise<void>
  onMoveRule: (id: string, delta: number) => Promise<void>
  onCloneRule: (id: string) => Promise<void>
  onPickExecutable?: () => Promise<string>
  onImportRules?: (raw: string) => Promise<void>
  onExportRules?: () => Promise<void>
}) {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<ProxyRule | null>(null)
  const [error, setError] = useState('')
  const [showImportModal, setShowImportModal] = useState(false)
  const [importText, setImportText] = useState('')
  const [importError, setImportError] = useState('')

  const quickAppCandidates = Array.from(
    new Set([
      '*',
      ...knownApps.filter((a) => a && a !== 'local-client' && a !== 'AnyProxy'),
      'Google Chrome',
      'Safari',
      'Telegram',
      'Cursor',
      'curl',
    ])
  ).slice(0, 8)

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
      action: 'proxy',
      proxyConfigId: '',
    })
    setShowModal(true)
  }

  const handleEdit = (r: ProxyRule) => {
    setError('')
    setEditing({
      ...r,
      action: r.action.toLowerCase() as ProxyRule['action'],
      protocol: r.protocol.toLowerCase() as ProxyRule['protocol'],
    })
    setShowModal(true)
  }

  const handleAddAppToken = (appName: string) => {
    if (!editing || !appName) return
    if (appName === '*') {
      setEditing({ ...editing, processNames: '*' })
      return
    }
    const cur = (editing.processNames || '')
      .split(';')
      .flatMap((x) => x.split(','))
      .map((x) => x.trim())
      .filter((x) => x && x !== '*')
    if (!cur.includes(appName)) {
      cur.push(appName)
    }
    setEditing({
      ...editing,
      processNames: cur.join('; '),
      name: editing.name.trim() ? editing.name : `Proxy ${appName}`,
    })
  }

  const handleBrowseExecutable = async () => {
    if (!onPickExecutable) return
    const path = await onPickExecutable()
    const extracted = extractAppNameFromPath(path)
    if (extracted) {
      handleAddAppToken(extracted)
    }
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

  const handleImportSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!onImportRules || !importText.trim()) return
    setImportError('')
    try {
      await onImportRules(importText)
      setImportText('')
      setShowImportModal(false)
    } catch (err) {
      setImportError(String(err))
    }
  }

  return (
    <>
      <PageHead title={t.rules.title} sub={t.rules.sub}>
        {onImportRules && (
          <button
            type="button"
            className="btn nodrag"
            onClick={() => {
              setImportError('')
              setShowImportModal(true)
            }}
          >
            <Icon name="upload" />
            {t.rules.importBtn}
          </button>
        )}
        {onExportRules && rules.length > 0 && (
          <button
            type="button"
            className="btn nodrag"
            onClick={() => void onExportRules()}
          >
            <Icon name="download" />
            {t.rules.exportBtn}
          </button>
        )}
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
                              action === 'proxy'
                                ? `Proxy: ${matchedProxy ? matchedProxy.name : t.rules.defaultProxy}`
                                : action.toUpperCase()
                            }
                          >
                            <span className="dot" />
                            <span className="chiptext">
                              {action === 'proxy'
                                ? matchedProxy
                                  ? `Proxy: ${matchedProxy.name}`
                                  : t.rules.defaultProxy
                                : action.toUpperCase()}
                            </span>
                          </span>
                        </td>
                        <td className="col-actions">
                          <span className="row">
                            <button
                              type="button"
                              role="switch"
                              aria-checked={r.enabled}
                              aria-label={`${r.enabled ? t.rules.disable : t.rules.enable}: ${r.name}`}
                              title={r.enabled ? t.rules.enabled : t.rules.disabled}
                              className={`switch ${r.enabled ? 'on' : ''}`}
                              onClick={() => void onToggleRule(r.id, !r.enabled)}
                            />
                            <button
                              className="iconbtn sm"
                              disabled={idx === 0}
                              onClick={() => void onMoveRule(r.id, -1)}
                              title={t.actions.moveUp}
                              aria-label={t.actions.moveUp}
                            >
                              <Icon name="arrow-up" />
                            </button>
                            <button
                              className="iconbtn sm"
                              disabled={idx === rules.length - 1}
                              onClick={() => void onMoveRule(r.id, 1)}
                              title={t.actions.moveDown}
                              aria-label={t.actions.moveDown}
                            >
                              <Icon name="arrow-down" />
                            </button>
                            <button
                              className="iconbtn sm"
                              onClick={() => void onCloneRule(r.id)}
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
                              type="button"
                              className="iconbtn sm danger"
                              onClick={() => void onDeleteRule(r.id)}
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
                  <div className="row" style={{ justifyContent: 'space-between' }}>
                    <label>
                      {t.rules.applications} ({t.rules.processHint})
                    </label>
                    {onPickExecutable && (
                      <button
                        type="button"
                        className="btn sm"
                        onClick={() => void handleBrowseExecutable()}
                      >
                        <Icon name="folder" />
                        {t.rules.browseApp}
                      </button>
                    )}
                  </div>
                  <input
                    className="input mono"
                    value={editing.processNames}
                    onChange={(e) =>
                      setEditing({ ...editing, processNames: e.target.value })
                    }
                  />
                  <div className="quick-chip-bar">
                    <span className="muted" style={{ fontSize: 11 }}>{t.rules.quickApps}</span>
                    {quickAppCandidates.map((appName) => (
                      <button
                        key={appName}
                        type="button"
                        className="chip-btn"
                        onClick={() => handleAddAppToken(appName)}
                      >
                        {appName}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="row" style={{ gap: 8 }}>
                  <div className="field" style={{ flex: 2 }}>
                    <label>
                      {t.rules.targets} ({t.rules.targetHint})
                    </label>
                    <input
                      className="input mono"
                      value={editing.targetHosts}
                      onChange={(e) =>
                        setEditing({ ...editing, targetHosts: e.target.value })
                      }
                    />
                  </div>
                  <div className="field" style={{ flex: 1 }}>
                    <label>
                      {t.rules.ports} ({t.rules.portHint})
                    </label>
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

      {showImportModal && (
        <div className="dialog-backdrop">
          <div className="dialog-box">
            <form onSubmit={handleImportSubmit}>
              <div className="dialog-head">
                <h2>{t.rules.importTitle}</h2>
                <button
                  type="button"
                  className="iconbtn sm"
                  onClick={() => setShowImportModal(false)}
                  aria-label={t.actions.cancel}
                >
                  <Icon name="close" />
                </button>
              </div>
              <div className="dialog-body">
                {importError && <p role="alert">{importError}</p>}
                <textarea
                  className="input mono"
                  rows={6}
                  style={{ height: 'auto', padding: 10 }}
                  value={importText}
                  onChange={(e) => setImportText(e.target.value)}
                  placeholder={t.rules.importPlaceholder}
                  required
                />
              </div>
              <div className="dialog-foot">
                <button
                  type="button"
                  className="btn"
                  onClick={() => setShowImportModal(false)}
                >
                  {t.actions.cancel}
                </button>
                <button type="submit" className="btn primary">
                  {t.actions.import}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
