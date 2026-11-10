// SPDX-License-Identifier: MIT
import React from 'react'
import { PageHead } from '../components/page-head'
import { t, setLanguage, getLanguage } from '../i18n'
import { applyTheme, readTheme, type ThemeChoice } from '../lib/theme'
import type { Profile } from '../types'

export function SettingsPage({
  profile,
  onUpdateOptions,
  onCheckUpdates,
}: {
  profile: Profile | null
  onUpdateOptions: (
    trafficLogging: boolean,
    dnsViaProxy: boolean,
    logEveryConn: boolean,
    autoClearLogs: boolean
  ) => Promise<void>
  onCheckUpdates: () => Promise<void>
}) {
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

  return (
    <>
      <PageHead title={t.settings.title} sub={t.settings.sub} />

      <div className="pagebody" style={{ maxWidth: 760 }}>
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
    </>
  )
}
