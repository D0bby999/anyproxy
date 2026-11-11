// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { t } from '../i18n'
import type { ProxyRule, ProxyConfig } from '../types'

function parseAppTokens(raw: string): string[] {
  return (raw || '')
    .split(';')
    .flatMap((x) => x.split(','))
    .map((x) => x.trim())
    .filter((x) => x && x !== '*')
}

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
  defaultProxyId = '',
  knownApps = [],
  onSaveRule,
  onDeleteRule,
  onToggleRule,
  onMoveRule,
  onCloneRule,
  onSetDefaultProxy,
  onPickExecutable,
  onImportRules,
  onExportRules,
}: {
  rules: ProxyRule[]
  proxies: ProxyConfig[]
  defaultProxyId?: string
  knownApps?: string[]
  onSaveRule: (r: ProxyRule) => Promise<void>
  onDeleteRule: (id: string) => Promise<void>
  onToggleRule: (id: string, enabled: boolean) => Promise<void>
  onMoveRule: (id: string, delta: number) => Promise<void>
  onCloneRule: (id: string) => Promise<void>
  onSetDefaultProxy?: (proxyId: string) => Promise<void>
  onPickExecutable?: () => Promise<string>
  onImportRules?: (raw: string) => Promise<void>
  onExportRules?: () => Promise<void>
}) {
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<ProxyRule | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [importText, setImportText] = useState('')
  const [importError, setImportError] = useState('')

  const quickAppCandidates = Array.from(
    new Set([
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
      excludeProcess: false,
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
      processNames: r.processNames || '*',
      excludeProcess: Boolean(r.excludeProcess),
      action: r.action.toLowerCase() as ProxyRule['action'],
      protocol: r.protocol.toLowerCase() as ProxyRule['protocol'],
    })
    setShowModal(true)
  }

  const handleToggleAppToken = (appName: string) => {
    if (!appName) return
    setEditing((prev) => {
      if (!prev) return prev
      const cur = parseAppTokens(prev.processNames)
      const exists = cur.includes(appName)
      const next = exists ? cur.filter((x) => x !== appName) : [...cur, appName]
      const autoName = prev.excludeProcess
        ? `${t.rules.exceptPrefix}: ${appName}`
        : `Proxy ${appName}`
      return {
        ...prev,
        processNames: next.join('; '),
        name: !exists && !prev.name.trim() ? autoName : prev.name,
      }
    })
  }

  const handleBrowseExecutable = async () => {
    if (!onPickExecutable || !editing) return
    const path = await onPickExecutable()
    const extracted = extractAppNameFromPath(path)
    if (!extracted) return
    setEditing((prev) => {
      if (!prev) return prev
      const cur = parseAppTokens(prev.processNames)
      if (!cur.includes(extracted)) cur.push(extracted)
      const autoName = prev.excludeProcess
        ? `${t.rules.exceptPrefix}: ${extracted}`
        : `Proxy ${extracted}`
      return {
        ...prev,
        processNames: cur.join('; '),
        name: prev.name.trim() ? prev.name : autoName,
      }
    })
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing || saving) return
    if (editing.excludeProcess && parseAppTokens(editing.processNames).length === 0) {
      setError(t.rules.excludeRequired)
      return
    }
    const payload =
      !editing.excludeProcess && !(editing.processNames || '').trim()
        ? { ...editing, processNames: '*' }
        : editing
    setSaving(true)
    setError('')
    try {
      await onSaveRule(payload)
      setShowModal(false)
    } catch (err) {
      setError(String(err))
    } finally {
      setSaving(false)
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
                          <span
                            className="mono selectable ellipsis-text"
                            title={r.excludeProcess ? `${t.rules.exceptPrefix}: ${r.processNames}` : r.processNames}
                          >
                            {r.excludeProcess ? `${t.rules.exceptPrefix}: ${r.processNames || '*'}` : (r.processNames || '*')}
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
        <RuleEditorDialog
          editing={editing}
          rules={rules}
          proxies={proxies}
          defaultProxyId={defaultProxyId}
          quickAppCandidates={quickAppCandidates}
          error={error}
          saving={saving}
          onChange={setEditing}
          onToggleApp={handleToggleAppToken}
          onSetDefaultProxy={onSetDefaultProxy}
          onBrowseExecutable={onPickExecutable ? handleBrowseExecutable : undefined}
          onSave={handleSave}
          onClose={() => setShowModal(false)}
        />
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

function RuleEditorDialog({
  editing,
  rules,
  proxies,
  defaultProxyId,
  quickAppCandidates,
  error,
  saving,
  onChange,
  onToggleApp,
  onSetDefaultProxy,
  onBrowseExecutable,
  onSave,
  onClose,
}: {
  editing: ProxyRule
  rules: ProxyRule[]
  proxies: ProxyConfig[]
  defaultProxyId: string
  quickAppCandidates: string[]
  error: string
  saving: boolean
  onChange: (next: ProxyRule) => void
  onToggleApp: (appName: string) => void
  onSetDefaultProxy?: (proxyId: string) => Promise<void>
  onBrowseExecutable?: () => Promise<void>
  onSave: (e: React.FormEvent) => Promise<void>
  onClose: () => void
}) {
  const backdropRef = useRef<HTMLDivElement>(null)
  const nameRef = useRef<HTMLInputElement>(null)
  const appsRef = useRef<HTMLInputElement>(null)
  const title = editing.id ? t.rules.editTitle : t.rules.newBtn

  useEffect(() => {
    nameRef.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      const all = document.querySelectorAll('.backdrop')
      if (all[all.length - 1] === backdropRef.current) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const selectedTokens = parseAppTokens(editing.processNames)
  const rawProcs = (editing.processNames || '').trim()
  const appMode: 'all' | 'include' | 'exclude' = editing.excludeProcess
    ? 'exclude'
    : rawProcs === '*'
      ? 'all'
      : 'include'
  const matchedProxy = proxies.find((p) => p.id === editing.proxyConfigId)

  const editIdx = editing.id ? rules.findIndex((r) => r.id === editing.id) : -1
  const trailingRules = editIdx >= 0 ? rules.slice(editIdx + 1) : []
  const hasCatchAllDirectAfter = trailingRules.some(
    (r) =>
      r.enabled &&
      !r.excludeProcess &&
      (r.processNames || '*').trim() === '*' &&
      (r.targetHosts || '*').trim() === '*' &&
      (r.targetPorts || '*').trim() === '*' &&
      r.protocol.toLowerCase() === 'both' &&
      r.action.toLowerCase() === 'direct',
  )
  const showFallbackWarn =
    Boolean(defaultProxyId) &&
    Boolean(onSetDefaultProxy) &&
    editing.action === 'proxy' &&
    appMode !== 'all' &&
    !hasCatchAllDirectAfter

  const handleSwitchFallbackDirect = async () => {
    if (!onSetDefaultProxy) return
    // Keep the rule pinned to the current active proxy before clearing the global fallback.
    if (!editing.proxyConfigId && defaultProxyId) {
      onChange({ ...editing, proxyConfigId: defaultProxyId })
    }
    await onSetDefaultProxy('')
  }

  const protoLabel = editing.protocol === 'tcp' ? 'TCP' : editing.protocol === 'udp' ? 'UDP' : 'TCP & UDP'
  const appsSummary =
    appMode === 'all'
      ? t.rules.allApps
      : appMode === 'exclude'
        ? selectedTokens.length > 0
          ? `${t.rules.exceptPrefix}: ${selectedTokens.join(', ')}`
          : t.rules.matchExclude
        : selectedTokens.length > 0
          ? selectedTokens.join(', ')
          : t.rules.matchInclude
  const hostsSummary = (editing.targetHosts || '').trim() || '*'
  const portsSummary = (editing.targetPorts || '').trim() || '*'
  const actionChipClass = editing.action === 'proxy' ? 'accent' : editing.action === 'block' ? 'danger' : 'ok'
  const actionSummary =
    editing.action === 'proxy'
      ? `${t.rules.actionProxy}: ${matchedProxy ? matchedProxy.name : t.rules.defaultProxy}`
      : editing.action === 'block'
        ? t.rules.actionBlock
        : t.rules.actionDirect

  const protocols: { id: ProxyRule['protocol']; label: string }[] = [
    { id: 'both', label: t.rules.both },
    { id: 'tcp', label: t.rules.tcp },
    { id: 'udp', label: t.rules.udp },
  ]

  const actions: { id: ProxyRule['action']; label: string }[] = [
    { id: 'proxy', label: t.rules.actionProxy },
    { id: 'direct', label: t.rules.actionDirect },
    { id: 'block', label: t.rules.actionBlock },
  ]

  return createPortal(
    <div
      className="backdrop"
      ref={backdropRef}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <form
        className="dialog ruledialog"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onSubmit={(e) => void onSave(e)}
      >
        <div className="dialog-head">
          <Icon name="rules" />
          <h2>{title}</h2>
          <span className="spacer" />
          <button
            type="button"
            className="iconbtn sm"
            onClick={onClose}
            aria-label={t.actions.close}
            title={t.actions.close}
          >
            <Icon name="close" />
          </button>
        </div>

        <div className="dialog-body">
          {error && (
            <div className="banner danger" role="alert">
              <Icon name="warn" />
              <span className="grow">{error}</span>
            </div>
          )}

          <div className="field">
            <label htmlFor="rule-name">{t.rules.name}</label>
            <input
              ref={nameRef}
              id="rule-name"
              className="input"
              value={editing.name}
              onChange={(e) => onChange({ ...editing, name: e.target.value })}
              placeholder={t.rules.namePlaceholder}
              required
            />
          </div>

          <div className="rule-group">
            <div className="field">
              <div className="rule-field-head" style={{ alignItems: 'center' }}>
                <label htmlFor="rule-apps">{t.rules.applications}</label>
                {onBrowseExecutable && (
                  <button
                    type="button"
                    className="btn sm"
                    onClick={() => void onBrowseExecutable()}
                  >
                    <Icon name="folder" />
                    {t.rules.browseApp}
                  </button>
                )}
              </div>
              <div className="seg rule-seg-full" role="group" aria-label={t.rules.appMatchMode}>
                <button
                  type="button"
                  aria-pressed={appMode === 'all'}
                  onClick={() => onChange({ ...editing, processNames: '*', excludeProcess: false })}
                >
                  {t.rules.allApps}
                </button>
                <button
                  type="button"
                  aria-pressed={appMode === 'include'}
                  onClick={() => {
                    onChange({
                      ...editing,
                      excludeProcess: false,
                      processNames: rawProcs === '*' ? '' : editing.processNames,
                    })
                    appsRef.current?.focus()
                  }}
                >
                  {t.rules.matchInclude}
                </button>
                <button
                  type="button"
                  aria-pressed={appMode === 'exclude'}
                  onClick={() => {
                    onChange({
                      ...editing,
                      excludeProcess: true,
                      processNames: rawProcs === '*' ? '' : editing.processNames,
                    })
                    appsRef.current?.focus()
                  }}
                >
                  {t.rules.matchExclude}
                </button>
              </div>
              <input
                ref={appsRef}
                id="rule-apps"
                className="input mono"
                value={editing.processNames}
                onChange={(e) => {
                  const val = e.target.value
                  onChange({
                    ...editing,
                    processNames: val,
                    excludeProcess: appMode === 'exclude' ? true : false,
                  })
                }}
                placeholder={appMode === 'exclude' ? t.rules.excludeHint : t.rules.processHint}
                spellCheck={false}
              />
              <div className="quick-chip-bar" role="group" aria-label={t.rules.quickApps}>
                {quickAppCandidates.map((appName) => {
                  const active = selectedTokens.includes(appName)
                  return (
                    <button
                      key={appName}
                      type="button"
                      className={`chip-btn mono${active ? ' active' : ''}`}
                      aria-pressed={active}
                      onClick={() => onToggleApp(appName)}
                    >
                      {appName}
                    </button>
                  )
                })}
              </div>
            </div>

            <div className="rule-grid">
              <div className="field">
                <div className="rule-field-head">
                  <label htmlFor="rule-targets">{t.rules.targets}</label>
                  <span className="hint">{t.rules.targetHint}</span>
                </div>
                <input
                  id="rule-targets"
                  className="input mono"
                  value={editing.targetHosts}
                  onChange={(e) => onChange({ ...editing, targetHosts: e.target.value })}
                  placeholder="*"
                  spellCheck={false}
                />
              </div>

              <div className="field">
                <div className="rule-field-head">
                  <label htmlFor="rule-ports">{t.rules.ports}</label>
                  <span className="hint">{t.rules.portHint}</span>
                </div>
                <input
                  id="rule-ports"
                  className="input mono"
                  value={editing.targetPorts}
                  onChange={(e) => onChange({ ...editing, targetPorts: e.target.value })}
                  placeholder="*"
                  spellCheck={false}
                />
              </div>
            </div>

            <div className="field">
              <span id="rule-proto-label" className="field-label">{t.rules.protocol}</span>
              <div className="seg rule-seg-full" role="group" aria-labelledby="rule-proto-label">
                {protocols.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    aria-pressed={editing.protocol === p.id}
                    onClick={() => onChange({ ...editing, protocol: p.id })}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div className="rule-group">
            <div className={editing.action === 'proxy' ? 'rule-route-grid' : 'field'}>
              <div className="field">
                <span id="rule-action-label" className="field-label">{t.rules.action}</span>
                <div className="seg rule-seg-full" role="group" aria-labelledby="rule-action-label">
                  {actions.map((a) => (
                    <button
                      key={a.id}
                      type="button"
                      aria-pressed={editing.action === a.id}
                      onClick={() => onChange({ ...editing, action: a.id })}
                    >
                      <span className={`rule-action-dot ${a.id}`} aria-hidden="true" />
                      {a.label}
                    </button>
                  ))}
                </div>
              </div>

              {editing.action === 'proxy' && (
                <div className="field">
                  <label htmlFor="rule-proxy">{t.rules.selectProxy}</label>
                  <select
                    id="rule-proxy"
                    className="select"
                    value={editing.proxyConfigId || ''}
                    onChange={(e) => onChange({ ...editing, proxyConfigId: e.target.value })}
                  >
                    <option value="">{t.rules.defaultProxy}</option>
                    {proxies.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} ({p.type.toUpperCase()} {p.host}:{p.port})
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>

            {editing.action === 'proxy' && proxies.length === 0 && (
              <span className="hint">{t.rules.noProxiesHint}</span>
            )}

            {showFallbackWarn && (
              <div className="rule-fallback-note">
                <Icon name="warn" />
                <span className="grow">{t.rules.fallbackProxyWarn}</span>
                <button
                  type="button"
                  className="btn sm"
                  onClick={() => void handleSwitchFallbackDirect()}
                >
                  {t.rules.switchFallbackDirect}
                </button>
              </div>
            )}
          </div>

          <div className="rule-preview" aria-live="polite">
            <span className="rule-preview-match mono">
              {appsSummary} · {hostsSummary}:{portsSummary} · {protoLabel}
            </span>
            <span className={`chip ${actionChipClass}`}>
              <span className="dot" />
              <span className="chiptext">{actionSummary}</span>
            </span>
          </div>
        </div>

        <div className="dialog-foot">
          <button type="button" className="btn" onClick={onClose}>
            {t.actions.cancel}
          </button>
          <button type="submit" className="btn primary" disabled={saving} aria-busy={saving}>
            {t.actions.save}
          </button>
        </div>
      </form>
    </div>,
    document.body,
  )
}
