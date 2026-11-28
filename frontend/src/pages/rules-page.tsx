// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { PageHead } from '../components/page-head'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { t } from '../i18n'
import { formatProxyLocation, type ProxyRule, type ProxyConfig, type RuleSimulationResult } from '../types'

function parseAppTokens(raw: string): string[] {
  return (raw || '')
    .split(';')
    .flatMap((x) => x.split(','))
    .map((x) => x.trim())
    .filter((x) => x && x !== '*')
}

function extractAppTokensFromPath(fullPath: string): string[] {
  if (!fullPath) return []
  const appMatch = fullPath.match(/([^/\\]+)\.app(?:[/\\]|$)/i)
  if (appMatch && appMatch[1]) {
    const appName = appMatch[1].trim()
    return [`*/${appName}.app/*`, `${appName}*`]
  }
  const base = fullPath.split(/[/\\]/).filter(Boolean).pop() || ''
  if (/\.exe$/i.test(base)) {
    const stem = base.replace(/\.exe$/i, '').trim()
    return stem ? [`${stem}*`] : []
  }
  return base ? [base] : []
}

type AppChipGroup = { label: string; tokens: string[] }

function groupAppTokens(tokens: string[]): AppChipGroup[] {
  const consumed = new Set<number>()
  const groups: AppChipGroup[] = []

  for (let i = 0; i < tokens.length; i++) {
    if (consumed.has(i)) continue
    const tok = tokens[i]

    // 1. Match a known multi-token preset starting at or containing this token, preserving insertion order.
    let matchedPreset = false
    for (const preset of QUICK_APP_PRESETS) {
      const pTokens = parseAppTokens(preset.value)
      if (pTokens.length <= 1 || !pTokens.includes(tok)) continue
      const indices: number[] = []
      for (const pt of pTokens) {
        const idx = tokens.findIndex((t, j) => !consumed.has(j) && t === pt)
        if (idx >= 0) indices.push(idx)
      }
      if (indices.length === pTokens.length) {
        indices.forEach((idx) => consumed.add(idx))
        groups.push({
          label: preset.label.replace(/\s*\([^)]*\)$/, ''),
          tokens: pTokens,
        })
        matchedPreset = true
        break
      }
    }
    if (matchedPreset) continue

    // 2. Collapse */AppName.app/* (+ optional AppName*) into one clean AppName chip.
    const appBundle = tok.match(/^\*\/([^/\\]+)\.app\/\*$/i)
    if (appBundle && appBundle[1]) {
      const appName = appBundle[1]
      consumed.add(i)
      const paired = [tok]
      const starIdx = tokens.findIndex(
        (t, j) => !consumed.has(j) && t.toLowerCase() === `${appName.toLowerCase()}*`
      )
      if (starIdx >= 0) {
        consumed.add(starIdx)
        paired.push(tokens[starIdx])
      }
      groups.push({ label: appName, tokens: paired })
      continue
    }

    consumed.add(i)
    groups.push({ label: tok, tokens: [tok] })
  }
  return groups
}

function formatAppSummary(raw: string): string {
  const tokens = parseAppTokens(raw)
  if (tokens.length === 0) return '*'
  return groupAppTokens(tokens)
    .map((g) => g.label)
    .join(', ')
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
  const groups = groupAppTokens(parseAppTokens(r.processNames))
  const subject =
    r.excludeProcess && groups.length > 0
      ? `${t.rules.exceptPrefix}: ${groups[0].label}`
      : groups.length > 0
        ? groups[0].label
        : r.targetHosts !== '*'
          ? r.targetHosts.split(/[;,]/)[0].trim()
          : r.targetPorts !== '*'
            ? `:${r.targetPorts.split(/[;,]/)[0].trim()}`
            : t.rules.allApps
  return `${verb} ${subject}`.trim()
}

type QuickAppOption = { label: string; value: string; detected?: boolean }

const QUICK_APP_PRESETS: QuickAppOption[] = [
  { label: 'Google Chrome (+ Helper)', value: '*/Google Chrome.app/*; Google Chrome*; *com.google.Chrome*' },
  { label: 'Google Drive (+ Helper)', value: '*/Google Drive.app/*; Google Drive*; *drivefs*' },
  { label: 'AnyProfile (+ Browser)', value: '*/AnyProfile.app/*; AnyProfile*; org.chromium.Chromium*' },
  { label: 'Safari (+ Helper)', value: '*/Safari.app/*; Safari*; com.apple.Safari*; *com.apple.WebKit.Networking*' },
  { label: 'Cursor (+ Helper)', value: '*/Cursor.app/*; Cursor*' },
  { label: 'Telegram (+ Helper)', value: '*/Telegram.app/*; Telegram*; *ru.keepcoder.Telegram*' },
  { label: 'Muse (Meta)', value: '*/Muse.app/*; Muse*; com.meta.endo*' },
  { label: 'League of Legends (+ Game/Riot)', value: '*/League of Legends.app/*; League of Legends*; *LeagueofLegends*; *com.riotgames*' },
  { label: 'curl', value: 'curl' },
  { label: 'git', value: 'git' },
  { label: 'node', value: 'node' },
]

type StarterTemplateId = 'proxy-browser' | 'exclude-app' | 'bypass-lan' | 'block-host'
type ActionFilter = 'all' | 'proxy' | 'direct' | 'block'

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
  onSimulateRule,
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
  onSimulateRule?: (
    processName: string,
    host: string,
    port: number,
    protocol: string
  ) => Promise<RuleSimulationResult>
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
  const [confirmDelete, setConfirmDelete] = useState<ProxyRule | null>(null)

  const [search, setSearch] = useState('')
  const [actionFilter, setActionFilter] = useState<ActionFilter>('all')
  const [dragId, setDragId] = useState<string | null>(null)
  const [dropTarget, setDropTarget] = useState<{ id: string; pos: 'before' | 'after' } | null>(null)
  const [showSim, setShowSim] = useState(false)
  const [simApp, setSimApp] = useState('Google Chrome')
  const [simHost, setSimHost] = useState('api.example.com')
  const [simPort, setSimPort] = useState('443')
  const [simProto, setSimProto] = useState<'tcp' | 'udp'>('tcp')
  const [simRunning, setSimRunning] = useState(false)
  const [simResult, setSimResult] = useState<RuleSimulationResult | null>(null)

  const handleRunSim = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    if (!onSimulateRule || simRunning) return
    setSimRunning(true)
    try {
      const parsedPort = parseInt(simPort, 10)
      const res = await onSimulateRule(simApp, simHost, Number.isFinite(parsedPort) ? parsedPort : 443, simProto)
      setSimResult(res)
    } finally {
      setSimRunning(false)
    }
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (confirmDelete) {
        setConfirmDelete(null)
        return
      }
      if (showImportModal) setShowImportModal(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [confirmDelete, showImportModal])

  const presetValues = new Set(QUICK_APP_PRESETS.map((p) => p.value))
  const detectedApps: QuickAppOption[] = knownApps
    .filter((a) => a && a !== 'local-client' && a !== 'AnyProxy' && !presetValues.has(a))
    .map((a) => ({ label: a, value: a, detected: true }))
  const quickAppCandidates: QuickAppOption[] = [...detectedApps, ...QUICK_APP_PRESETS].slice(0, 20)

  const defaultProxy = proxies.find((p) => p.id === defaultProxyId)
  const isFiltered = search.trim().length > 0 || actionFilter !== 'all'

  const actionCounts = useMemo(() => {
    const counts = { all: rules.length, proxy: 0, direct: 0, block: 0 }
    for (const r of rules) {
      const a = r.action.toLowerCase()
      if (a === 'proxy') counts.proxy++
      else if (a === 'direct') counts.direct++
      else if (a === 'block') counts.block++
    }
    return counts
  }, [rules])

  const filteredRules = useMemo(() => {
    const words = search
      .trim()
      .toLowerCase()
      .split(/\s+/)
      .filter(Boolean)
    return rules.filter((r) => {
      const act = r.action.toLowerCase()
      if (actionFilter !== 'all' && act !== actionFilter) return false
      if (words.length === 0) return true
      const px = proxies.find((p) => p.id === r.proxyConfigId)
      const haystack = [
        r.name,
        r.processNames,
        r.excludeProcess ? t.rules.exceptPrefix : '',
        r.targetHosts,
        r.targetPorts,
        r.protocol,
        r.action,
        px?.name || '',
        px ? `${px.host}:${px.port}` : '',
        !r.proxyConfigId && act === 'proxy' ? `${t.rules.defaultProxy} ${defaultProxy?.name || ''}` : '',
      ]
        .join(' ')
        .toLowerCase()
      return words.every((w) => haystack.includes(w))
    })
  }, [rules, proxies, defaultProxy, search, actionFilter])

  const openEditorWith = (
    draft: ProxyRule,
    mode: 'all' | 'include' | 'exclude',
    advanced: boolean
  ) => {
    setError('')
    setAppMode(mode)
    setAppDraft('')
    setShowAdvanced(advanced)
    setEditing(draft)
    setShowModal(true)
  }

  const handleOpenAdd = () => {
    openEditorWith(
      {
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
        localhostViaProxy: false,
      },
      'include',
      false
    )
  }

  const handleOpenTemplate = (templateId: StarterTemplateId) => {
    if (templateId === 'proxy-browser') {
      openEditorWith(
        {
          id: '',
          name: '',
          enabled: true,
          processNames: '*/Google Chrome.app/*; Google Chrome*; *com.google.Chrome*',
          excludeProcess: false,
          targetHosts: '',
          targetPorts: '',
          protocol: 'both',
          action: 'proxy',
          proxyConfigId: '',
          localhostViaProxy: false,
        },
        'include',
        false
      )
      return
    }
    if (templateId === 'exclude-app') {
      openEditorWith(
        {
          id: '',
          name: '',
          enabled: true,
          processNames: '',
          excludeProcess: true,
          targetHosts: '',
          targetPorts: '',
          protocol: 'both',
          action: 'proxy',
          proxyConfigId: '',
          localhostViaProxy: false,
        },
        'exclude',
        false
      )
      return
    }
    if (templateId === 'bypass-lan') {
      openEditorWith(
        {
          id: '',
          name: t.rules.templateBypassLan,
          enabled: true,
          processNames: '',
          excludeProcess: false,
          targetHosts: '10.0.0.0/8; 172.16.0.0/12; 192.168.0.0/16; *.local',
          targetPorts: '',
          protocol: 'both',
          action: 'direct',
          proxyConfigId: '',
          localhostViaProxy: false,
        },
        'all',
        true
      )
      return
    }
    openEditorWith(
      {
        id: '',
        name: '',
        enabled: true,
        processNames: '',
        excludeProcess: false,
        targetHosts: '*.tracking.example; *.ads.example',
        targetPorts: '',
        protocol: 'both',
        action: 'block',
        proxyConfigId: '',
        localhostViaProxy: false,
      },
      'all',
      true
    )
  }

  const handleEdit = (r: ProxyRule) => {
    const initialMode: 'all' | 'include' | 'exclude' = r.excludeProcess
      ? 'exclude'
      : (r.processNames || '*').trim() === '*'
        ? 'all'
        : 'include'
    const normalizedProtocol = r.protocol.toLowerCase() as ProxyRule['protocol']
    const hasAdvanced =
      initialMode === 'all' ||
      r.action.toLowerCase() !== 'proxy' ||
      Boolean(r.localhostViaProxy) ||
      (r.targetHosts || '*').trim() !== '*' ||
      (r.targetPorts || '*').trim() !== '*' ||
      normalizedProtocol !== 'both'
    openEditorWith(
      {
        ...r,
        processNames: initialMode === 'all' ? '' : r.processNames || '',
        targetHosts: (r.targetHosts || '').trim() === '*' ? '' : r.targetHosts,
        targetPorts: (r.targetPorts || '').trim() === '*' ? '' : r.targetPorts,
        excludeProcess: Boolean(r.excludeProcess),
        localhostViaProxy: Boolean(r.localhostViaProxy),
        action: r.action.toLowerCase() as ProxyRule['action'],
        protocol: normalizedProtocol,
      },
      initialMode,
      hasAdvanced
    )
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

  const handleRemoveApps = (tokensToRemove: string[]) => {
    const removeSet = new Set(tokensToRemove)
    setEditing((prev) => {
      if (!prev) return prev
      const next = parseAppTokens(prev.processNames).filter((x) => !removeSet.has(x))
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
    const extracted = extractAppTokensFromPath(path)
    if (extracted.length === 0) return
    handleAddApps(extracted)
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
    if (effectiveMode === 'include' && mergedTokens.length === 0) {
      setError(t.rules.includeRequired)
      return
    }
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
      localhostViaProxy: editing.action === 'proxy' ? Boolean(editing.localhostViaProxy) : false,
    }
    const hasCatchAllRule = rules.some(
      (r) =>
        r.id !== editing.id &&
        r.enabled &&
        !r.excludeProcess &&
        (r.processNames || '*').trim() === '*' &&
        (r.targetHosts || '*').trim() === '*' &&
        (r.targetPorts || '*').trim() === '*' &&
        r.protocol.toLowerCase() === 'both'
    )
    // "Only these apps" -> Proxy targeting the same proxy as Default Route is a no-op
    // unless Default Route is switched to Direct; pin the rule's proxy so it stays active.
    const shouldSwitchDefaultToDirect =
      !hasCatchAllRule &&
      effectiveMode === 'include' &&
      mergedTokens.length > 0 &&
      normalized.action === 'proxy' &&
      Boolean(defaultProxyId) &&
      Boolean(onSetDefaultProxy) &&
      (!normalized.proxyConfigId || normalized.proxyConfigId === defaultProxyId)
    const finalRule: ProxyRule = shouldSwitchDefaultToDirect
      ? { ...normalized, proxyConfigId: normalized.proxyConfigId || defaultProxyId }
      : normalized
    const payload: ProxyRule = (finalRule.name || '').trim()
      ? finalRule
      : { ...finalRule, name: defaultRuleName(finalRule) }
    setSaving(true)
    setError('')
    try {
      await onSaveRule(payload)
      if (shouldSwitchDefaultToDirect && onSetDefaultProxy) {
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

  const handleDropReorder = async (sourceId: string, targetId: string, pos: 'before' | 'after') => {
    if (isFiltered || sourceId === targetId) return
    const fromIdx = rules.findIndex((r) => r.id === sourceId)
    const rawTargetIdx = rules.findIndex((r) => r.id === targetId)
    if (fromIdx < 0 || rawTargetIdx < 0) return
    let toIdx = pos === 'before' ? rawTargetIdx : rawTargetIdx + 1
    if (fromIdx < toIdx) toIdx -= 1
    const steps = toIdx - fromIdx
    if (steps === 0) return
    const dir = steps > 0 ? 1 : -1
    for (let i = 0; i < Math.abs(steps); i++) {
      await onMoveRule(sourceId, dir)
    }
  }

  const actionFilters: { id: ActionFilter; label: string; count: number }[] = [
    { id: 'all', label: t.rules.filterAll, count: actionCounts.all },
    { id: 'proxy', label: t.rules.actionProxy, count: actionCounts.proxy },
    { id: 'direct', label: t.rules.actionDirect, count: actionCounts.direct },
    { id: 'block', label: t.rules.actionBlock, count: actionCounts.block },
  ]

  return (
    <>
      <PageHead title={t.rules.title} sub={t.rules.sub}>
        {onSimulateRule && (
          <button
            type="button"
            className={`btn nodrag${showSim ? ' active-toggle' : ''}`}
            aria-pressed={showSim}
            onClick={() => {
              const next = !showSim
              setShowSim(next)
              if (!next) setSimResult(null)
            }}
          >
            <Icon name="zap" />
            {t.rules.testBtn}
          </button>
        )}
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
          {rules.length > 0 && (
            <div className="rules-toolbar">
              <div className="seg" role="group" aria-label={t.rules.action}>
                {actionFilters.map((f) => (
                  <button
                    key={f.id}
                    type="button"
                    aria-pressed={actionFilter === f.id}
                    onClick={() => setActionFilter(f.id)}
                  >
                    {f.id !== 'all' && <span className={`rule-action-dot ${f.id}`} aria-hidden="true" />}
                    <span>{f.label}</span>
                    <span className="seg-count">{f.count}</span>
                  </button>
                ))}
              </div>
              <div className="search-field">
                <Icon name="search" className="icon sm search-field-icon" />
                <input
                  className="input search-field-input"
                  type="search"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={t.rules.searchPlaceholder}
                  aria-label={t.rules.searchPlaceholder}
                />
                {search && (
                  <button
                    type="button"
                    className="search-field-clear"
                    onClick={() => setSearch('')}
                    aria-label={t.actions.clear}
                    title={t.actions.clear}
                  >
                    <Icon name="close" className="icon xs" />
                  </button>
                )}
              </div>
            </div>
          )}

          {showSim && onSimulateRule && (
            <form className="rule-sim-bar" onSubmit={(e) => void handleRunSim(e)}>
              <div className="rule-sim-inputs">
                <input
                  className="input rule-sim-app"
                  value={simApp}
                  onChange={(e) => setSimApp(e.target.value)}
                  placeholder={t.rules.simApp}
                  aria-label={t.rules.simApp}
                  list="rule-sim-apps-list"
                  spellCheck={false}
                />
                <datalist id="rule-sim-apps-list">
                  {quickAppCandidates.map((opt) => (
                    <option key={opt.value} value={opt.label.replace(/\s*\([^)]*\)$/, '')} />
                  ))}
                </datalist>
                <input
                  className="input mono rule-sim-host"
                  value={simHost}
                  onChange={(e) => setSimHost(e.target.value)}
                  placeholder={t.rules.simHost}
                  aria-label={t.rules.simHost}
                  spellCheck={false}
                />
                <input
                  className="input mono rule-sim-port"
                  value={simPort}
                  onChange={(e) => setSimPort(e.target.value)}
                  placeholder={t.rules.simPort}
                  aria-label={t.rules.simPort}
                  inputMode="numeric"
                />
                <div className="seg rule-sim-proto" role="group" aria-label={t.rules.protocol}>
                  <button
                    type="button"
                    aria-pressed={simProto === 'tcp'}
                    onClick={() => setSimProto('tcp')}
                  >
                    TCP
                  </button>
                  <button
                    type="button"
                    aria-pressed={simProto === 'udp'}
                    onClick={() => setSimProto('udp')}
                  >
                    UDP
                  </button>
                </div>
                <button
                  type="submit"
                  className="btn sm primary"
                  disabled={simRunning}
                  aria-busy={simRunning}
                >
                  {t.rules.simRun}
                </button>
              </div>
              {simResult && (
                <div className="rule-sim-result" aria-live="polite">
                  <span className="rule-sim-match-label">
                    {simResult.viaDefault
                      ? t.rules.simDefault
                      : t.rules.simMatched(simResult.ruleIndex, simResult.ruleName || '')}
                  </span>
                  <span
                    className={`chip ${
                      simResult.action === 'proxy'
                        ? 'accent'
                        : simResult.action === 'block'
                          ? 'danger'
                          : 'ok'
                    }`}
                  >
                    <span className="dot" />
                    <span className="chiptext">
                      {simResult.action === 'proxy'
                        ? `${t.rules.actionProxy}: ${simResult.proxyName || t.rules.defaultProxy}`
                        : simResult.action === 'block'
                          ? t.rules.actionBlock
                          : t.rules.actionDirect}
                    </span>
                  </span>
                  {simResult.failClosed && (
                    <span className="chip danger">
                      <span className="chiptext">{t.rules.simFailClosed}</span>
                    </span>
                  )}
                  <span className="rule-sim-reason">{simResult.reason}</span>
                </div>
              )}
            </form>
          )}

          <div className="tablewrap">
            {rules.length === 0 ? (
              <EmptyState
                icon="rules"
                title={t.rules.emptyTitle}
                text={t.rules.emptyText}
              >
                <div className="rule-templates">
                  <button type="button" className="btn sm" onClick={() => handleOpenTemplate('proxy-browser')}>
                    <Icon name="globe" className="icon xs" />
                    {t.rules.templateProxyBrowser}
                  </button>
                  <button type="button" className="btn sm" onClick={() => handleOpenTemplate('exclude-app')}>
                    <Icon name="rules" className="icon xs" />
                    {t.rules.templateExcludeApp}
                  </button>
                  <button type="button" className="btn sm" onClick={() => handleOpenTemplate('bypass-lan')}>
                    <Icon name="shield" className="icon xs" />
                    {t.rules.templateBypassLan}
                  </button>
                  <button type="button" className="btn sm" onClick={() => handleOpenTemplate('block-host')}>
                    <Icon name="ban" className="icon xs" />
                    {t.rules.templateBlockHost}
                  </button>
                </div>
                <button className="btn primary" onClick={handleOpenAdd}>
                  <Icon name="plus" />
                  {t.rules.newBtn}
                </button>
              </EmptyState>
            ) : filteredRules.length === 0 ? (
              <EmptyState
                icon="search"
                title={t.rules.noFilterMatches}
                text={t.rules.noFilterMatchesText}
              >
                <button
                  type="button"
                  className="btn"
                  onClick={() => {
                    setSearch('')
                    setActionFilter('all')
                  }}
                >
                  {t.rules.clearFilters}
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
                  {filteredRules.map((r) => {
                    const realIdx = rules.findIndex((item) => item.id === r.id)
                    const matchedProxy = proxies.find((p) => p.id === r.proxyConfigId)
                    const action = r.action.toLowerCase()
                    const isDropBefore = dropTarget?.id === r.id && dropTarget.pos === 'before'
                    const isDropAfter = dropTarget?.id === r.id && dropTarget.pos === 'after'
                    const actionLabel =
                      action === 'proxy'
                        ? matchedProxy
                          ? `Proxy: ${matchedProxy.name}`
                          : defaultProxy
                            ? `${t.rules.defaultProxy} · ${defaultProxy.name}`
                            : t.rules.defaultProxy
                        : action === 'block'
                          ? t.rules.actionBlock
                          : t.rules.actionDirect
                    return (
                      <tr
                        key={r.id}
                        draggable={!isFiltered}
                        onDragStart={(e) => {
                          if (isFiltered) return
                          setDragId(r.id)
                          e.dataTransfer.effectAllowed = 'move'
                          e.dataTransfer.setData('text/plain', r.id)
                        }}
                        onDragOver={(e) => {
                          if (isFiltered || !dragId || dragId === r.id) return
                          e.preventDefault()
                          const rect = e.currentTarget.getBoundingClientRect()
                          const pos = e.clientY - rect.top < rect.height / 2 ? 'before' : 'after'
                          setDropTarget({ id: r.id, pos })
                        }}
                        onDragLeave={() => {
                          if (dropTarget?.id === r.id) setDropTarget(null)
                        }}
                        onDrop={(e) => {
                          e.preventDefault()
                          const src = dragId || e.dataTransfer.getData('text/plain')
                          const pos = dropTarget?.pos || 'before'
                          setDragId(null)
                          setDropTarget(null)
                          if (src) void handleDropReorder(src, r.id, pos)
                        }}
                        onDragEnd={() => {
                          setDragId(null)
                          setDropTarget(null)
                        }}
                        className={[
                          'clickable-row',
                          !r.enabled ? 'is-disabled' : '',
                          simResult && !simResult.viaDefault && simResult.ruleId === r.id ? 'is-sim-match' : '',
                          dragId === r.id ? 'is-dragging' : '',
                          isDropBefore ? 'drop-before' : '',
                          isDropAfter ? 'drop-after' : '',
                        ]
                          .filter(Boolean)
                          .join(' ')}
                        onClick={() => handleEdit(r)}
                      >
                        <td className="col-no" title={isFiltered ? t.rules.reorderDisabledFiltered : t.rules.dragToReorder}>
                          <span className="rule-order-cell">
                            {!isFiltered && <Icon name="grip" className="icon xs rule-grip" />}
                            <span>{realIdx + 1}</span>
                          </span>
                        </td>
                        <td className="cellname col-name">
                          <b className="selectable" title={r.name}>
                            {r.name}
                          </b>
                          <span className="sub">
                            <span>{r.enabled ? t.rules.enabled : t.rules.disabled}</span>
                            {r.localhostViaProxy && action === 'proxy' && (
                              <span className="rule-inline-tag">{t.rules.localhostBadge}</span>
                            )}
                          </span>
                        </td>
                        <td className="col-apps">
                          <span
                            className={`mono selectable ellipsis-text${r.excludeProcess ? ' rule-exclude-text' : ''}`}
                            title={r.excludeProcess ? `${t.rules.exceptPrefix}: ${r.processNames}` : r.processNames}
                          >
                            {r.excludeProcess
                              ? `${t.rules.exceptPrefix}: ${formatAppSummary(r.processNames)}`
                              : formatAppSummary(r.processNames)}
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
                            title={actionLabel}
                          >
                            <span className="dot" />
                            <span className="chiptext">{actionLabel}</span>
                          </span>
                        </td>
                        <td className="col-actions" onClick={(e) => e.stopPropagation()}>
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
                              disabled={isFiltered || realIdx === 0}
                              onClick={() => void onMoveRule(r.id, -1)}
                              title={isFiltered ? t.rules.reorderDisabledFiltered : t.actions.moveUp}
                              aria-label={t.actions.moveUp}
                            >
                              <Icon name="arrow-up" />
                            </button>
                            <button
                              className="iconbtn sm"
                              disabled={isFiltered || realIdx === rules.length - 1}
                              onClick={() => void onMoveRule(r.id, 1)}
                              title={isFiltered ? t.rules.reorderDisabledFiltered : t.actions.moveDown}
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
                              onClick={() => setConfirmDelete(r)}
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
                  {!isFiltered && (
                    <tr className={`rule-default-row${simResult?.viaDefault ? ' is-sim-match' : ''}`}>
                      <td className="col-no">∞</td>
                      <td className="cellname col-name">
                        <b>{t.rules.defaultRouteLabel}</b>
                        <span className="sub">{t.rules.defaultRouteSub}</span>
                      </td>
                      <td className="col-apps">
                        <span className="mono muted">*</span>
                      </td>
                      <td className="col-targets">
                        <span className="mono muted">*</span>
                      </td>
                      <td className="col-ports">
                        <span className="mono muted">*</span>
                      </td>
                      <td className="col-proto">
                        <span className="chip">
                          <span className="chiptext">BOTH</span>
                        </span>
                      </td>
                      <td className="col-action" colSpan={2}>
                        <div className="rule-default-control">
                          {onSetDefaultProxy ? (
                            <select
                              className="select rule-default-select"
                              aria-label={t.rules.defaultRouteLabel}
                              value={defaultProxyId || ''}
                              onChange={(e) => void onSetDefaultProxy(e.target.value)}
                            >
                              <option value="">{t.engine.direct}</option>
                              {proxies.map((p) => (
                                <option key={p.id} value={p.id}>
                                  {t.rules.actionProxy}: {p.name} ({p.type.toUpperCase()} {p.host}:{p.port})
                                </option>
                              ))}
                            </select>
                          ) : (
                            <span className={`chip ${defaultProxy ? 'accent' : 'ok'}`}>
                              <span className="dot" />
                              <span className="chiptext">
                                {defaultProxy ? `${t.rules.actionProxy}: ${defaultProxy.name}` : t.engine.direct}
                              </span>
                            </span>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
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
          onRemoveApps={handleRemoveApps}
          onCommitAppDraft={handleCommitAppDraft}
          onBrowseExecutable={onPickExecutable ? handleBrowseExecutable : undefined}
          onSave={handleSave}
          onClose={() => setShowModal(false)}
        />
      )}

      {confirmDelete && (
        <div className="dialog-backdrop" onClick={() => setConfirmDelete(null)}>
          <div
            className="dialog-box"
            role="dialog"
            aria-modal="true"
            aria-labelledby="confirm-delete-rule-title"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="dialog-head">
              <h2 id="confirm-delete-rule-title">{t.rules.confirmDelete}</h2>
              <button
                type="button"
                className="iconbtn sm"
                onClick={() => setConfirmDelete(null)}
                aria-label={t.actions.close}
              >
                <Icon name="close" />
              </button>
            </div>
            <div className="dialog-body">
              <p>{t.rules.confirmDeleteBody(confirmDelete.name || defaultRuleName(confirmDelete))}</p>
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
                  void onDeleteRule(id)
                }}
              >
                <Icon name="trash" />
                {t.actions.delete}
              </button>
            </div>
          </div>
        </div>
      )}

      {showImportModal && (
        <div className="dialog-backdrop" onClick={() => setShowImportModal(false)}>
          <div
            className="dialog-box"
            role="dialog"
            aria-modal="true"
            aria-labelledby="import-rules-title"
            onClick={(e) => e.stopPropagation()}
          >
            <form onSubmit={handleImportSubmit}>
              <div className="dialog-head">
                <h2 id="import-rules-title">{t.rules.importTitle}</h2>
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
                {importError && (
                  <div className="banner danger" role="alert">
                    <Icon name="warn" />
                    <span className="grow">{importError}</span>
                  </div>
                )}
                <textarea
                  className="input mono"
                  rows={6}
                  style={{ height: 'auto', padding: 10 }}
                  value={importText}
                  onChange={(e) => setImportText(e.target.value)}
                  placeholder={t.rules.importPlaceholder}
                  autoFocus
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
  onRemoveApps,
  onCommitAppDraft,
  onBrowseExecutable,
  onSave,
  onClose,
}: {
  editing: ProxyRule
  appMode: 'all' | 'include' | 'exclude'
  appDraft: string
  showAdvanced: boolean
  proxies: ProxyConfig[]
  defaultProxyId: string
  quickAppCandidates: QuickAppOption[]
  error: string
  saving: boolean
  onChange: (next: ProxyRule) => void
  onAppModeChange: (mode: 'all' | 'include' | 'exclude') => void
  onAppDraftChange: (val: string) => void
  onToggleAdvanced: () => void
  onAddApps: (tokens: string[]) => void
  onRemoveApps: (tokens: string[]) => void
  onCommitAppDraft: () => void
  onBrowseExecutable?: () => Promise<void>
  onSave: (e: React.FormEvent) => Promise<void>
  onClose: () => void
}) {
  const backdropRef = useRef<HTMLDivElement>(null)
  const nameRef = useRef<HTMLInputElement>(null)
  const appsRef = useRef<HTMLInputElement>(null)
  const title = editing.id ? t.rules.editTitle : t.rules.newBtn

  useEffect(() => {
    if (appMode !== 'all' && appsRef.current) {
      appsRef.current.focus()
    } else {
      nameRef.current?.focus()
    }
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
  const selectedGroups = groupAppTokens(selectedTokens)
  const draftTokens = parseAppTokens(appDraft)
  const previewGroups = groupAppTokens([
    ...selectedTokens,
    ...draftTokens.filter((tok) => !selectedTokens.includes(tok)),
  ])
  const selectedLower = selectedGroups.map((g) => g.label.toLowerCase().replace(/\*+/g, '').trim())
  const availableQuickApps = quickAppCandidates.filter((preset) => {
    const tokens = parseAppTokens(preset.value)
    if (tokens.length === 0) return false
    const presetBase = preset.label.replace(/\s*\([^)]*\)$/, '').toLowerCase().trim()
    if (selectedLower.includes(presetBase)) return false
    return tokens.some((tok) => !selectedTokens.includes(tok))
  })
  const detectedQuickApps = availableQuickApps.filter((a) => a.detected)
  const commonQuickApps = availableQuickApps.filter((a) => !a.detected)
  const quickPills = availableQuickApps.slice(0, 6)

  const targetsValue = editing.targetHosts
  const portsValue = editing.targetPorts
  const matchedProxy = proxies.find((p) => p.id === editing.proxyConfigId)
  const defaultProxy = proxies.find((p) => p.id === defaultProxyId)
  const effectiveExclude = appMode === 'exclude'

  const protoLabel = editing.protocol === 'tcp' ? 'TCP' : editing.protocol === 'udp' ? 'UDP' : 'TCP & UDP'
  const previewLabels = previewGroups.map((g) => g.label)
  const appsSummary = effectiveExclude
    ? previewLabels.length > 0
      ? `${t.rules.exceptPrefix}: ${previewLabels.join(', ')}`
      : t.rules.matchExclude
    : previewLabels.length > 0
      ? previewLabels.join(', ')
      : t.rules.allApps
  const hostsSummary = (editing.targetHosts || '').trim() || '*'
  const portsSummary = (editing.targetPorts || '').trim() || '*'
  const actionChipClass = editing.action === 'proxy' ? 'accent' : editing.action === 'block' ? 'danger' : 'ok'
  const actionSummary =
    editing.action === 'proxy'
      ? `${t.rules.actionProxy}: ${
          matchedProxy
            ? matchedProxy.name
            : defaultProxy
              ? `${t.rules.defaultProxy} (${defaultProxy.name})`
              : t.rules.defaultProxy
        }`
      : editing.action === 'block'
        ? t.rules.actionBlock
        : t.rules.actionDirect

  const advancedParts: string[] = []
  if (editing.action !== 'proxy') advancedParts.push(actionVerb(editing.action))
  if (appMode === 'all') advancedParts.push(t.rules.allApps)
  if ((editing.name || '').trim()) advancedParts.push(editing.name.trim())
  if (hostsSummary !== '*') advancedParts.push(hostsSummary)
  if (portsSummary !== '*') advancedParts.push(`:${portsSummary}`)
  if (editing.protocol !== 'both') advancedParts.push(protoLabel)
  if (editing.action === 'proxy' && editing.localhostViaProxy) advancedParts.push(t.rules.localhostBadge)
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

          <div className="rule-group">
            <div className="field">
              <label htmlFor="rule-apps" className="field-label">{t.rules.applications}</label>
              <div className="seg rule-seg-full" role="group" aria-label={t.rules.appMatchMode}>
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
                {(showAdvanced || appMode === 'all') && (
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
                )}
              </div>

              {appMode === 'all' ? (
                <span className="hint">{t.rules.noAppsSelected}</span>
              ) : (
                <>
                  {editing.action === 'proxy' && (
                    <span className="hint">
                      {appMode === 'exclude' ? t.rules.excludeModeHint : t.rules.includeModeHint}
                    </span>
                  )}
                  <div className="rule-apps-row">
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
                          if (picked) onAddApps(parseAppTokens(picked))
                        }}
                      >
                        <option value="">{t.rules.pickAppPlaceholder}</option>
                        {detectedQuickApps.length > 0 && (
                          <optgroup label={t.rules.detectedAppsGroup}>
                            {detectedQuickApps.map((opt) => (
                              <option key={opt.value} value={opt.value}>
                                {opt.label}
                              </option>
                            ))}
                          </optgroup>
                        )}
                        {commonQuickApps.length > 0 && (
                          <optgroup label={t.rules.commonAppsGroup}>
                            {commonQuickApps.map((opt) => (
                              <option key={opt.value} value={opt.value}>
                                {opt.label}
                              </option>
                            ))}
                          </optgroup>
                        )}
                      </select>
                    )}
                  </div>

                  <div className="selected-chip-bar" role="list" aria-label={t.rules.selectedApps}>
                    {selectedGroups.length === 0 ? (
                      <span className="hint">
                        {appMode === 'exclude' ? t.rules.excludeRequired : t.rules.includeRequired}
                      </span>
                    ) : (
                      selectedGroups.map((group) => (
                        <span
                          key={group.tokens.join(';')}
                          className={`app-chip${appMode === 'exclude' ? ' is-exclude' : ''}`}
                          role="listitem"
                        >
                          <span className="app-chip-name" title={group.tokens.join('; ')}>
                            {group.label}
                          </span>
                          <button
                            type="button"
                            className="app-chip-remove"
                            onClick={() => onRemoveApps(group.tokens)}
                            aria-label={`${t.rules.removeApp} ${group.label}`}
                            title={`${t.rules.removeApp} ${group.label}`}
                          >
                            <Icon name="close" className="icon xs" />
                          </button>
                        </span>
                      ))
                    )}
                  </div>

                  {quickPills.length > 0 && (
                    <div className="rule-suggest-row" aria-label={t.rules.suggestedApps}>
                      <span className="rule-suggest-label">{t.rules.suggestedApps}:</span>
                      {quickPills.map((opt) => (
                        <button
                          key={opt.value}
                          type="button"
                          className="rule-suggest-pill"
                          onClick={() => onAddApps(parseAppTokens(opt.value))}
                          title={opt.value}
                        >
                          <span>+ {opt.label.replace(/\s*\([^)]*\)$/, '')}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </>
              )}
            </div>
          </div>

          {editing.action === 'proxy' && (
            <div className="rule-group">
              <div className="field">
                <label htmlFor="rule-proxy">{t.rules.selectProxy}</label>
                <select
                  id="rule-proxy"
                  className="select"
                  value={editing.proxyConfigId || ''}
                  onChange={(e) => onChange({ ...editing, proxyConfigId: e.target.value })}
                >
                  <option value="">
                    {defaultProxy ? `${t.rules.defaultProxy} (${defaultProxy.name})` : t.rules.defaultProxy}
                  </option>
                  {proxies.map((p) => {
                    const loc = formatProxyLocation(p.country, p.exitIp)
                    return (
                      <option key={p.id} value={p.id}>
                        {loc ? `${loc} · ` : ''}
                        {p.name} ({p.type.toUpperCase()} {p.host}:{p.port})
                      </option>
                    )
                  })}
                </select>
              </div>
              {proxies.length === 0 && <span className="hint">{t.rules.noProxiesHint}</span>}
            </div>
          )}

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

                {editing.action !== 'proxy' && (
                  <div className="field">
                    <span className="field-label">{t.rules.routeOutcome}</span>
                    <div className="rule-outcome">
                      {editing.action === 'block' ? t.rules.blockOutcome : t.rules.directOutcome}
                    </div>
                  </div>
                )}
              </div>
              <div className="rule-grid">
                <div className="field">
                  <div className="rule-field-head">
                    <label htmlFor="rule-targets">{t.rules.targets}</label>
                    <div className="rule-mini-presets">
                      <button
                        type="button"
                        className="rule-mini-chip"
                        onClick={() => onChange({ ...editing, targetHosts: '' })}
                      >
                        {t.rules.presetAny}
                      </button>
                      <button
                        type="button"
                        className="rule-mini-chip"
                        onClick={() =>
                          onChange({
                            ...editing,
                            targetHosts: '10.0.0.0/8; 172.16.0.0/12; 192.168.0.0/16; *.local',
                          })
                        }
                      >
                        {t.rules.presetLan}
                      </button>
                    </div>
                  </div>
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
                  <div className="rule-field-head">
                    <label htmlFor="rule-ports">{t.rules.ports}</label>
                    <div className="rule-mini-presets">
                      <button
                        type="button"
                        className="rule-mini-chip"
                        onClick={() => onChange({ ...editing, targetPorts: '' })}
                      >
                        {t.rules.presetAny}
                      </button>
                      <button
                        type="button"
                        className="rule-mini-chip"
                        onClick={() => onChange({ ...editing, targetPorts: '80, 443' })}
                      >
                        {t.rules.presetWebPorts}
                      </button>
                      <button
                        type="button"
                        className="rule-mini-chip"
                        onClick={() => onChange({ ...editing, targetPorts: '443' })}
                      >
                        {t.rules.presetHttpsPort}
                      </button>
                      <button
                        type="button"
                        className="rule-mini-chip"
                        onClick={() => onChange({ ...editing, targetPorts: '53' })}
                      >
                        {t.rules.presetDnsPort}
                      </button>
                    </div>
                  </div>
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

              {editing.action === 'proxy' && (
                <div className="rule-switch-row">
                  <div className="settings-row-copy">
                    <label htmlFor="rule-localhost-proxy" className="settings-row-title">
                      {t.rules.localhostViaProxy}
                    </label>
                    <span id="rule-localhost-proxy-desc" className="hint">
                      {t.rules.localhostViaProxyHint}
                    </span>
                  </div>
                  <button
                    id="rule-localhost-proxy"
                    type="button"
                    role="switch"
                    aria-checked={Boolean(editing.localhostViaProxy)}
                    aria-describedby="rule-localhost-proxy-desc"
                    className="switch"
                    onClick={() =>
                      onChange({ ...editing, localhostViaProxy: !editing.localhostViaProxy })
                    }
                  />
                </div>
              )}
            </div>
          )}
        </div>

        <div className="rule-preview" aria-live="polite">
          <span className="rule-preview-when">{t.rules.previewWhen}</span>
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
    document.body
  )
}
