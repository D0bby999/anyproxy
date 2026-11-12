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

function actionVerb(action: ProxyRule['action']): string {
  return action === 'block'
    ? t.rules.actionBlock
    : action === 'direct'
      ? t.rules.actionDirect
      : t.rules.actionProxy
}

function defaultRuleName(r: ProxyRule): string {
  const verb = actionVerb(r.action)
  const tokens = parseAppTokens(r.processNames)
  const subject =
    r.excludeProcess && tokens.length > 0
      ? `${t.rules.exceptPrefix}: ${tokens[0]}`
      : tokens.length > 0
        ? tokens[0]
        : r.targetHosts !== '*'
          ? r.targetHosts.split(/[;,]/)[0].trim()
          : r.targetPorts !== '*'
            ? `:${r.targetPorts.split(/[;,]/)[0].trim()}`
            : t.rules.allApps
  return `${verb} ${subject}`.trim()
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
  const [appMode, setAppMode] = useState<'all' | 'include' | 'exclude'>('all')
  const [appDraft, setAppDraft] = useState('')
  const [showAdvanced, setShowAdvanced] = useState(false)
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
  ).slice(0, 12)

  const handleOpenAdd = () => {
    setError('')
    setAppMode('all')
    setAppDraft('')
    setShowAdvanced(false)
    setEditing({
      id: '',
      name: '',
      enabled: true,
      processNames: '',
      excludeProcess: false,
      targetHosts: '',
      targetPorts: '',
      protocol: 'both',
      action: 'proxy',
      proxyConfigId: '',
    })
    setShowModal(true)
  }

  const handleEdit = (r: ProxyRule) => {
    setError('')
    const initialMode: 'all' | 'include' | 'exclude' = r.excludeProcess
      ? 'exclude'
      : (r.processNames || '*').trim() === '*'
        ? 'all'
        : 'include'
    const normalizedProtocol = r.protocol.toLowerCase() as ProxyRule['protocol']
    const hasAdvanced =
      r.action.toLowerCase() !== 'proxy' ||
      Boolean(r.proxyConfigId) ||
      ((r.targetHosts || '*').trim() !== '*') ||
      ((r.targetPorts || '*').trim() !== '*') ||
      normalizedProtocol !== 'both'
    setAppMode(initialMode)
    setAppDraft('')
    setShowAdvanced(hasAdvanced)
    setEditing({
      ...r,
      processNames: initialMode === 'all' ? '' : (r.processNames || ''),
      targetHosts: (r.targetHosts || '').trim() === '*' ? '' : r.targetHosts,
      targetPorts: (r.targetPorts || '').trim() === '*' ? '' : r.targetPorts,
      excludeProcess: Boolean(r.excludeProcess),
      action: r.action.toLowerCase() as ProxyRule['action'],
      protocol: normalizedProtocol,
    })
    setShowModal(true)
  }

  const handleAddApps = (tokens: string[]) => {
    const clean = tokens.map((x) => x.trim()).filter((x) => x && x !== '*')
    if (clean.length === 0) return
    const wasAll = appMode === 'all'
    if (wasAll) setAppMode('include')
    setEditing((prev) => {
      if (!prev) return prev
      const cur = wasAll ? [] : parseAppTokens(prev.processNames)
      const next = [...cur]
      for (const appName of clean) {
        if (!next.includes(appName)) next.push(appName)
      }
      return {
        ...prev,
        processNames: next.join('; '),
      }
    })
  }

  const handleRemoveApp = (appName: string) => {
    setEditing((prev) => {
      if (!prev) return prev
      const next = parseAppTokens(prev.processNames).filter((x) => x !== appName)
      if (next.length === 0 && appMode === 'include') {
        setAppMode('all')
      }
      return {
        ...prev,
        processNames: next.join('; '),
      }
    })
  }

  const handleCommitAppDraft = () => {
    const tokens = parseAppTokens(appDraft)
    if (tokens.length > 0) handleAddApps(tokens)
    setAppDraft('')
  }

  const handleBrowseExecutable = async () => {
    if (!onPickExecutable || !editing) return
    const path = await onPickExecutable()
    const extracted = extractAppNameFromPath(path)
    if (!extracted) return
    handleAddApps([extracted])
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing || saving) return
    const draftTokens = parseAppTokens(appDraft)
    const baseTokens = appMode === 'all' ? [] : parseAppTokens(editing.processNames)
    const mergedTokens = [...baseTokens]
    for (const token of draftTokens) {
      if (!mergedTokens.includes(token)) mergedTokens.push(token)
    }
    const effectiveMode: 'all' | 'include' | 'exclude' =
      appMode === 'all' && mergedTokens.length > 0 ? 'include' : appMode
    if (effectiveMode === 'exclude' && mergedTokens.length === 0) {
      setError(t.rules.excludeRequired)
      return
    }
    if (draftTokens.length > 0) {
      setAppMode(effectiveMode)
      setEditing((prev) => (prev ? { ...prev, processNames: mergedTokens.join('; ') } : prev))
      setAppDraft('')
    }
    const normalized: ProxyRule = {
      ...editing,
      excludeProcess: effectiveMode === 'exclude',
      processNames: effectiveMode === 'all' || mergedTokens.length === 0 ? '*' : mergedTokens.join('; '),
      targetHosts: (editing.targetHosts || '').trim() || '*',
      targetPorts: (editing.targetPorts || '').trim() || '*',
    }
    const payload: ProxyRule = (normalized.name || '').trim()
      ? normalized
      : { ...normalized, name: defaultRuleName(normalized) }
    setSaving(true)
    setError('')
    try {
      await onSaveRule(payload)
      if (effectiveMode === 'exclude' && payload.action === 'proxy' && defaultProxyId && onSetDefaultProxy) {
        await onSetDefaultProxy('')
      }
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
          appMode={appMode}
          appDraft={appDraft}
          showAdvanced={showAdvanced}
          rules={rules}
          proxies={proxies}
          defaultProxyId={defaultProxyId}
          quickAppCandidates={quickAppCandidates}
          error={error}
          saving={saving}
          onChange={setEditing}
          onAppModeChange={setAppMode}
          onAppDraftChange={setAppDraft}
          onToggleAdvanced={() => setShowAdvanced((v) => !v)}
          onAddApps={handleAddApps}
          onRemoveApp={handleRemoveApp}
          onCommitAppDraft={handleCommitAppDraft}
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
  appMode,
  appDraft,
  showAdvanced,
  rules,
  proxies,
  defaultProxyId,
  quickAppCandidates,
  error,
  saving,
  onChange,
  onAppModeChange,
  onAppDraftChange,
  onToggleAdvanced,
  onAddApps,
  onRemoveApp,
  onCommitAppDraft,
  onSetDefaultProxy,
  onBrowseExecutable,
  onSave,
  onClose,
}: {
  editing: ProxyRule
  appMode: 'all' | 'include' | 'exclude'
  appDraft: string
  showAdvanced: boolean
  rules: ProxyRule[]
  proxies: ProxyConfig[]
  defaultProxyId: string
  quickAppCandidates: string[]
  error: string
  saving: boolean
  onChange: (next: ProxyRule) => void
  onAppModeChange: (mode: 'all' | 'include' | 'exclude') => void
  onAppDraftChange: (val: string) => void
  onToggleAdvanced: () => void
  onAddApps: (tokens: string[]) => void
  onRemoveApp: (appName: string) => void
  onCommitAppDraft: () => void
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
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      const all = document.querySelectorAll('.backdrop')
      if (all[all.length - 1] === backdropRef.current) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const selectedTokens = appMode === 'all' ? [] : parseAppTokens(editing.processNames)
  const draftTokens = parseAppTokens(appDraft)
  const previewTokens = [
    ...selectedTokens,
    ...draftTokens.filter((tok) => !selectedTokens.includes(tok)),
  ]
  const availableQuickApps = quickAppCandidates.filter((a) => !selectedTokens.includes(a))
  const targetsValue = editing.targetHosts
  const portsValue = editing.targetPorts
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
  const effectiveExclude = appMode === 'exclude'
  const hasAppFilter = effectiveExclude || previewTokens.length > 0
  const showFallbackWarn =
    Boolean(defaultProxyId) &&
    Boolean(onSetDefaultProxy) &&
    editing.action === 'proxy' &&
    hasAppFilter &&
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
  const appsSummary = effectiveExclude
    ? previewTokens.length > 0
      ? `${t.rules.exceptPrefix}: ${previewTokens.join(', ')}`
      : t.rules.matchExclude
    : previewTokens.length > 0
      ? previewTokens.join(', ')
      : t.rules.allApps
  const hostsSummary = (editing.targetHosts || '').trim() || '*'
  const portsSummary = (editing.targetPorts || '').trim() || '*'
  const actionChipClass = editing.action === 'proxy' ? 'accent' : editing.action === 'block' ? 'danger' : 'ok'
  const actionSummary =
    editing.action === 'proxy'
      ? `${t.rules.actionProxy}: ${matchedProxy ? matchedProxy.name : t.rules.defaultProxy}`
      : editing.action === 'block'
        ? t.rules.actionBlock
        : t.rules.actionDirect

  const advancedParts: string[] = []
  if (editing.action !== 'proxy') advancedParts.push(actionVerb(editing.action))
  else if (editing.proxyConfigId && matchedProxy) advancedParts.push(matchedProxy.name)
  if (hostsSummary !== '*') advancedParts.push(hostsSummary)
  if (portsSummary !== '*') advancedParts.push(`:${portsSummary}`)
  if (editing.protocol !== 'both') advancedParts.push(protoLabel)
  const advancedSummary =
    advancedParts.length > 0 ? advancedParts.join(' · ') : t.rules.advancedDefaultHint

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
        className={`dialog ruledialog${showAdvanced ? ' advanced-open' : ''}`}
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
            />
          </div>

          <div className="rule-group">
            <div className="field">
              <label htmlFor="rule-apps" className="field-label">{t.rules.applications}</label>
              <div className="seg rule-seg-full" role="group" aria-label={t.rules.appMatchMode}>
                <button
                  type="button"
                  aria-pressed={appMode === 'all'}
                  onClick={() => {
                    onAppModeChange('all')
                    onChange({ ...editing, excludeProcess: false })
                  }}
                >
                  {t.rules.allApps}
                </button>
                <button
                  type="button"
                  aria-pressed={appMode === 'include'}
                  onClick={() => {
                    onAppModeChange('include')
                    onChange({ ...editing, excludeProcess: false })
                    appsRef.current?.focus()
                  }}
                >
                  {t.rules.matchInclude}
                </button>
                <button
                  type="button"
                  aria-pressed={appMode === 'exclude'}
                  onClick={() => {
                    onAppModeChange('exclude')
                    onChange({ ...editing, excludeProcess: true })
                    appsRef.current?.focus()
                  }}
                >
                  {t.rules.matchExclude}
                </button>
              </div>

              <div className="rule-apps-row">
                <div className="rule-app-input-wrap">
                  <input
                    ref={appsRef}
                    id="rule-apps"
                    className="input"
                    value={appDraft}
                    onChange={(e) => onAppDraftChange(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ',' || e.key === ';') {
                        if (appDraft.trim()) {
                          e.preventDefault()
                          onCommitAppDraft()
                        }
                      }
                    }}
                    onBlur={() => {
                      if (appDraft.trim()) onCommitAppDraft()
                    }}
                    placeholder={appMode === 'exclude' ? t.rules.excludeHint : t.rules.processHint}
                    spellCheck={false}
                  />
                  <button
                    type="button"
                    className="iconbtn sm"
                    disabled={!appDraft.trim()}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={onCommitAppDraft}
                    aria-label={t.rules.addAppBtn}
                    title={t.rules.addAppBtn}
                  >
                    <Icon name="plus" className="icon sm" />
                  </button>
                </div>
                {availableQuickApps.length > 0 && (
                  <select
                    className="select rule-app-picker"
                    aria-label={t.rules.quickApps}
                    value=""
                    onChange={(e) => {
                      const picked = e.target.value
                      if (picked) onAddApps([picked])
                    }}
                  >
                    <option value="">{t.rules.pickAppPlaceholder}</option>
                    {availableQuickApps.map((appName) => (
                      <option key={appName} value={appName}>
                        {appName}
                      </option>
                    ))}
                  </select>
                )}
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
              <div className="selected-chip-bar" role="list" aria-label={t.rules.selectedApps}>
                {selectedTokens.length === 0 ? (
                  <span className="hint">
                    {appMode === 'exclude' ? t.rules.excludeRequired : t.rules.noAppsSelected}
                  </span>
                ) : (
                  selectedTokens.map((appName) => (
                    <span key={appName} className="app-chip" role="listitem">
                      <span className="app-chip-name" title={appName}>{appName}</span>
                      <button
                        type="button"
                        className="app-chip-remove"
                        onClick={() => onRemoveApp(appName)}
                        aria-label={`${t.rules.removeApp} ${appName}`}
                        title={`${t.rules.removeApp} ${appName}`}
                      >
                        <Icon name="close" className="icon xs" />
                      </button>
                    </span>
                  ))
                )}
              </div>
            </div>
          </div>

          <button
            type="button"
            className="rule-advanced-toggle"
            aria-expanded={showAdvanced}
            aria-controls="rule-advanced-panel"
            onClick={onToggleAdvanced}
          >
            <Icon name="chevron" className="icon sm chevron" />
            <span>{t.rules.advancedSection}</span>
            <span className="rule-advanced-summary">{advancedSummary}</span>
          </button>

          {showAdvanced && (
            <div id="rule-advanced-panel" className="rule-group">
              <div className="rule-route-grid">
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

                {editing.action === 'proxy' ? (
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
                ) : (
                  <div className="field">
                    <span className="field-label">{t.rules.routeOutcome}</span>
                    <div className="rule-outcome">
                      {editing.action === 'block' ? t.rules.blockOutcome : t.rules.directOutcome}
                    </div>
                  </div>
                )}
              </div>

              {editing.action === 'proxy' && proxies.length === 0 && (
                <span className="hint">{t.rules.noProxiesHint}</span>
              )}

              <div className="rule-grid">
                <div className="field">
                  <label htmlFor="rule-targets">{t.rules.targets}</label>
                  <input
                    id="rule-targets"
                    className="input mono"
                    value={targetsValue}
                    onChange={(e) => onChange({ ...editing, targetHosts: e.target.value })}
                    placeholder={t.rules.targetHint}
                    spellCheck={false}
                  />
                </div>

                <div className="field">
                  <label htmlFor="rule-ports">{t.rules.ports}</label>
                  <input
                    id="rule-ports"
                    className="input mono"
                    value={portsValue}
                    onChange={(e) => onChange({ ...editing, targetPorts: e.target.value })}
                    placeholder={t.rules.portHint}
                    spellCheck={false}
                  />
                </div>
              </div>

              <div className="rule-proto-row">
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
          )}
        </div>

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

        <div className="rule-preview" aria-live="polite">
          <span className="rule-preview-match mono">
            {appsSummary} · {hostsSummary}:{portsSummary} · {protoLabel}
          </span>
          <span className={`chip ${actionChipClass}`}>
            <span className="dot" />
            <span className="chiptext">{actionSummary}</span>
          </span>
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
