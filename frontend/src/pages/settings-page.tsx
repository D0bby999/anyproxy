// SPDX-License-Identifier: MIT
import React, { useState } from 'react'
import { PageHead } from '../components/page-head'
import { Icon } from '../components/icons'
import { t, setLanguage, getLanguage } from '../i18n'
import { applyTheme, readTheme, type ThemeChoice } from '../lib/theme'
import type { Profile } from '../types'

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
}) {
  const [includePasswords, setIncludePasswords] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [importText, setImportText] = useState('')
  const [importError, setImportError] = useState('')

  const currentTheme = readTheme()
  const currentLang = getLanguage()

  const handleThemeChange = (theme: ThemeChoice) => {
    applyTheme(theme)
    window.location.reload()
  }

  const handleLangChange = (lang: 'en' | 'vi') => {
    setLanguage(lang)
    window.location.reload()
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

      <div className="pagebody" style={{ maxWidth: 760 }}>
        {profile && onSwitchProfile && (
          <div className="card pad">
            <div className="row" style={{ justifyContent: 'space-between' }}>
              <h3>{t.settings.profiles}</h3>
              <div className="row" style={{ gap: 6 }}>
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

            <div className="field" style={{ marginTop: 4 }}>
              <label>{t.settings.activeProfileLabel}</label>
              <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}>
                <select
                  className="select"
                  style={{ flex: 1, minWidth: 160 }}
                  value={profile.name}
                  onChange={(e) => onSwitchProfile(e.target.value)}
                >
                  {profiles.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
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
                    className="btn sm"
                    onClick={() => onDeleteProfile(profile.name)}
                  >
                    <Icon name="trash" />
                    {t.settings.deleteBtn}
                  </button>
                )}
              </div>
            </div>

            {onExportProfile && (
              <label className="row" style={{ cursor: 'pointer', fontSize: 12 }}>
                <input
                  type="checkbox"
                  checked={includePasswords}
                  onChange={(e) => setIncludePasswords(e.target.checked)}
                />
                <span className="muted">{t.settings.includePasswords}</span>
              </label>
            )}
          </div>
        )}

        <div className="card pad">
          <h3>{t.settings.appearance}</h3>
          <div className="row" style={{ gap: 24, marginTop: 8 }}>
            <div className="field" style={{ flex: 1 }}>
              <label>{t.settings.theme}</label>
              <select
                className="select"
                defaultValue={currentTheme}
                onChange={(e) => handleThemeChange(e.target.value as ThemeChoice)}
              >
                <option value="system">{t.settings.system}</option>
                <option value="light">{t.settings.light}</option>
                <option value="dark">{t.settings.dark}</option>
              </select>
            </div>

            <div className="field" style={{ flex: 1 }}>
              <label>{t.settings.language}</label>
              <select
                className="select"
                defaultValue={currentLang}
                onChange={(e) => handleLangChange(e.target.value as 'en' | 'vi')}
              >
                <option value="en">English</option>
                <option value="vi">Tiếng Việt</option>
              </select>
            </div>
          </div>
        </div>

        {profile && (
          <div className="card pad">
            <h3>{t.settings.traffic}</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 8 }}>
              <label className="row" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={profile.trafficLoggingEnabled}
                  onChange={(e) =>
                    onUpdateOptions(
                      e.target.checked,
                      profile.dnsViaProxy,
                      profile.logEveryConnection,
                      profile.autoClearConnectionLogs
                    )
                  }
                />
                <span>{t.settings.trafficLogging}</span>
              </label>

              <label className="row" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={profile.dnsViaProxy}
                  onChange={(e) =>
                    onUpdateOptions(
                      profile.trafficLoggingEnabled,
                      e.target.checked,
                      profile.logEveryConnection,
                      profile.autoClearConnectionLogs
                    )
                  }
                />
                <span>{t.settings.dnsViaProxy}</span>
              </label>

              <label className="row" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={profile.logEveryConnection}
                  onChange={(e) =>
                    onUpdateOptions(
                      profile.trafficLoggingEnabled,
                      profile.dnsViaProxy,
                      e.target.checked,
                      profile.autoClearConnectionLogs
                    )
                  }
                />
                <span>{t.settings.logEvery}</span>
              </label>
            </div>
          </div>
        )}

        <div className="card pad">
          <h3>{t.settings.version}</h3>
          <p className="muted" style={{ margin: 0, fontSize: 'var(--t-sm)' }}>
            AnyProxy v1.0.0
          </p>
          <div style={{ marginTop: 8 }}>
            <button className="btn sm" onClick={onCheckUpdates}>
              {t.settings.checkUpdates}
            </button>
          </div>
        </div>
      </div>

      {showImportModal && (
        <div className="dialog-backdrop">
          <div className="dialog-box">
            <form onSubmit={handleImportSubmit}>
              <div className="dialog-head">
                <h2>{t.settings.importProfileTitle}</h2>
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
                  placeholder={t.settings.importProfilePlaceholder}
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
