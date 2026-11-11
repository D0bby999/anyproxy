// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useEffect, useState } from 'react'
import { PageHead } from '../components/page-head'
import { Icon } from '../components/icons'
import { t, setLanguage, getLanguage } from '../i18n'
import { applyTheme, readTheme, type ThemeChoice } from '../lib/theme'
import pkg from '../../package.json'
import type { Profile } from '../types'

function SwitchRow({
  id,
  label,
  description,
  checked,
  onToggle,
}: {
  id: string
  label: string
  description: string
  checked: boolean
  onToggle: (next: boolean) => void
}) {
  const descId = `${id}-desc`
  return (
    <div className="settings-row">
      <div className="settings-row-copy">
        <label htmlFor={id} className="settings-row-title">
          {label}
        </label>
        <span id={descId} className="hint">
          {description}
        </span>
      </div>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        aria-describedby={descId}
        className="switch"
        onClick={() => onToggle(!checked)}
      />
    </div>
  )
}

export function SettingsPage({
  profile,
  profiles = [],
  onUpdateOptions,
  onCheckUpdates,
  onSwitchProfile,
  onCreateProfile,
  onDuplicateProfile,
  onRenameProfile,
  onDeleteProfile,
  onImportProfile,
  onExportProfile,
  onOpenActivityLogs,
}: {
  profile: Profile | null
  profiles?: string[]
  onUpdateOptions: (
    trafficLogging: boolean,
    dnsViaProxy: boolean,
    logEveryConn: boolean,
    autoClearLogs: boolean
  ) => Promise<void>
  onCheckUpdates: () => Promise<void>
  onSwitchProfile?: (name: string) => void
  onCreateProfile?: () => void
  onDuplicateProfile?: (srcName: string) => void
  onRenameProfile?: (oldName: string) => void
  onDeleteProfile?: (name: string) => void
  onImportProfile?: (raw: string) => Promise<void>
  onExportProfile?: (includePasswords: boolean) => Promise<void>
  onOpenActivityLogs?: () => void
}) {
  const [themeChoice, setThemeChoice] = useState<ThemeChoice>(() => readTheme())
  const [includePasswords, setIncludePasswords] = useState(false)
  const [checkingUpdates, setCheckingUpdates] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [importText, setImportText] = useState('')
  const [importError, setImportError] = useState('')

  const currentLang = getLanguage()

  useEffect(() => {
    if (!showImportModal) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setShowImportModal(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [showImportModal])

  const handleThemeChange = (theme: ThemeChoice) => {
    setThemeChoice(theme)
    applyTheme(theme)
  }

  const handleLangChange = (lang: 'en' | 'vi') => {
    if (lang === currentLang) return
    setLanguage(lang)
    window.location.reload()
  }

  const handleCheckUpdates = async () => {
    if (checkingUpdates) return
    setCheckingUpdates(true)
    try {
      await onCheckUpdates()
    } finally {
      setCheckingUpdates(false)
    }
  }

  const handleImportSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!onImportProfile || !importText.trim()) return
    setImportError('')
    try {
      await onImportProfile(importText)
      setImportText('')
      setShowImportModal(false)
    } catch (err) {
      setImportError(String(err))
    }
  }

  return (
    <>
      <PageHead title={t.settings.title} sub={t.settings.sub} />

      <div className="pagebody">
        <div className="settings-grid">
          <div className="settings-col">
            {profile && (
              <section className="card settings-card" aria-labelledby="settings-traffic-h">
                <div className="settings-card-head">
                  <h2 id="settings-traffic-h">{t.settings.traffic}</h2>
                </div>
                <div className="settings-list">
                  <SwitchRow
                    id="opt-traffic-logging"
                    label={t.settings.trafficLogging}
                    description={t.settings.trafficLoggingDesc}
                    checked={profile.trafficLoggingEnabled}
                    onToggle={(next) =>
                      void onUpdateOptions(
                        next,
                        profile.dnsViaProxy,
                        profile.logEveryConnection,
                        profile.autoClearConnectionLogs
                      )
                    }
                  />
                  <SwitchRow
                    id="opt-dns-proxy"
                    label={t.settings.dnsViaProxy}
                    description={t.settings.dnsViaProxyDesc}
                    checked={profile.dnsViaProxy}
                    onToggle={(next) =>
                      void onUpdateOptions(
                        profile.trafficLoggingEnabled,
                        next,
                        profile.logEveryConnection,
                        profile.autoClearConnectionLogs
                      )
                    }
                  />
                  <SwitchRow
                    id="opt-log-every"
                    label={t.settings.logEvery}
                    description={t.settings.logEveryDesc}
                    checked={profile.logEveryConnection}
                    onToggle={(next) =>
                      void onUpdateOptions(
                        profile.trafficLoggingEnabled,
                        profile.dnsViaProxy,
                        next,
                        profile.autoClearConnectionLogs
                      )
                    }
                  />
                  <SwitchRow
                    id="opt-autoclear-logs"
                    label={t.settings.autoClearLogs}
                    description={t.settings.autoClearLogsDesc}
                    checked={profile.autoClearConnectionLogs}
                    onToggle={(next) =>
                      void onUpdateOptions(
                        profile.trafficLoggingEnabled,
                        profile.dnsViaProxy,
                        profile.logEveryConnection,
                        next
                      )
                    }
                  />
                </div>
              </section>
            )}

            <section className="card settings-card" aria-labelledby="settings-appearance-h">
              <div className="settings-card-head">
                <h2 id="settings-appearance-h">{t.settings.appearance}</h2>
              </div>

              <div className="field">
                <span className="field-label">{t.settings.theme}</span>
                <div className="seg settings-seg" role="group" aria-label={t.settings.theme}>
                  {(['system', 'light', 'dark'] as const).map((choice) => (
                    <button
                      key={choice}
                      type="button"
                      aria-pressed={themeChoice === choice}
                      onClick={() => handleThemeChange(choice)}
                    >
                      {t.settings[choice]}
                    </button>
                  ))}
                </div>
              </div>

              <div className="field">
                <span className="field-label">{t.settings.language}</span>
                <div className="seg settings-seg" role="group" aria-label={t.settings.language}>
                  <button
                    type="button"
                    aria-pressed={currentLang === 'en'}
                    onClick={() => handleLangChange('en')}
                  >
                    English
                  </button>
                  <button
                    type="button"
                    aria-pressed={currentLang === 'vi'}
                    onClick={() => handleLangChange('vi')}
                  >
                    Tiếng Việt
                  </button>
                </div>
              </div>
            </section>
          </div>

          <div className="settings-col">
            {profile && onSwitchProfile && (
              <section className="card settings-card" aria-labelledby="settings-profiles-h">
                <div className="settings-card-head">
                  <h2 id="settings-profiles-h">{t.settings.profiles}</h2>
                </div>

                <div className="field">
                  <label htmlFor="settings-profile-select">{t.settings.activeProfileLabel}</label>
                  <select
                    id="settings-profile-select"
                    className="select"
                    value={profile.name}
                    onChange={(e) => onSwitchProfile(e.target.value)}
                  >
                    {profiles.map((name) => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                  </select>
                </div>

                <div
                  className={`settings-btn-row ${
                    onDeleteProfile && profiles.length > 1 ? '' : 'three-col'
                  }`}
                >
                  {onCreateProfile && (
                    <button type="button" className="btn sm" onClick={onCreateProfile}>
                      <Icon name="plus" />
                      {t.nav.newProfile}
                    </button>
                  )}
                  {onDuplicateProfile && (
                    <button
                      type="button"
                      className="btn sm"
                      onClick={() => onDuplicateProfile(profile.name)}
                    >
                      <Icon name="copy" />
                      {t.settings.duplicateBtn}
                    </button>
                  )}
                  {onRenameProfile && (
                    <button
                      type="button"
                      className="btn sm"
                      onClick={() => onRenameProfile(profile.name)}
                    >
                      <Icon name="pencil" />
                      {t.settings.renameBtn}
                    </button>
                  )}
                  {onDeleteProfile && profiles.length > 1 && (
                    <button
                      type="button"
                      className="btn sm danger"
                      onClick={() => onDeleteProfile(profile.name)}
                    >
                      <Icon name="trash" />
                      {t.settings.deleteBtn}
                    </button>
                  )}
                </div>

                {(onImportProfile || onExportProfile) && (
                  <div className="settings-divider">
                    <span className="settings-subhead">{t.settings.portability}</span>
                    {onExportProfile && (
                      <SwitchRow
                        id="opt-include-passwords"
                        label={t.settings.includePasswords}
                        description={t.settings.includePasswordsDesc}
                        checked={includePasswords}
                        onToggle={setIncludePasswords}
                      />
                    )}
                    <div className="settings-btn-row">
                      {onImportProfile && (
                        <button
                          type="button"
                          className="btn sm"
                          onClick={() => {
                            setImportError('')
                            setShowImportModal(true)
                          }}
                        >
                          <Icon name="upload" />
                          {t.settings.importProfileBtn}
                        </button>
                      )}
                      {onExportProfile && (
                        <button
                          type="button"
                          className="btn sm"
                          onClick={() => void onExportProfile(includePasswords)}
                        >
                          <Icon name="download" />
                          {t.settings.exportProfileBtn}
                        </button>
                      )}
                    </div>
                  </div>
                )}
              </section>
            )}

            <section className="card settings-card" aria-labelledby="settings-version-h">
              <div className="settings-card-head">
                <h2 id="settings-version-h">{t.settings.version}</h2>
                <span className="chip mono">v{pkg.version}</span>
              </div>

              <div className="settings-row-copy">
                <span className="settings-version-title">{t.appName}</span>
                <span className="hint">{t.settings.aboutDesc}</span>
              </div>

              <div className="settings-btn-row">
                <button
                  type="button"
                  className="btn sm"
                  disabled={checkingUpdates}
                  aria-busy={checkingUpdates}
                  onClick={() => void handleCheckUpdates()}
                >
                  <Icon name="refresh" />
                  {t.settings.checkUpdates}
                </button>
                {onOpenActivityLogs && (
                  <button type="button" className="btn sm" onClick={onOpenActivityLogs}>
                    <Icon name="list" />
                    {t.nav.activity}
                  </button>
                )}
              </div>
            </section>
          </div>
        </div>
      </div>

      {showImportModal && (
        <div
          className="dialog-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) setShowImportModal(false)
          }}
        >
          <div
            className="dialog-box"
            role="dialog"
            aria-modal="true"
            aria-labelledby="import-profile-title"
          >
            <form onSubmit={handleImportSubmit}>
              <div className="dialog-head">
                <h2 id="import-profile-title">{t.settings.importProfileTitle}</h2>
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
                  className="input mono settings-import-textarea"
                  rows={6}
                  value={importText}
                  onChange={(e) => setImportText(e.target.value)}
                  placeholder={t.settings.importProfilePlaceholder}
                  aria-label={t.settings.importProfileTitle}
                  spellCheck={false}
                  autoComplete="off"
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
