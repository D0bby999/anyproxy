// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { Icon } from './icons'
import { t } from '../i18n'
import { formatProxyLocation, type LeakReport } from '../types'

export function LeakCheckDialog({
  onCheckLeak,
  onClose,
}: {
  onCheckLeak: () => Promise<LeakReport>
  onClose: () => void
}) {
  const [loading, setLoading] = useState(true)
  const [report, setReport] = useState<LeakReport | null>(null)
  const [error, setError] = useState('')

  const runCheck = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await onCheckLeak()
      setReport(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void runCheck()
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const overallClass =
    report?.overall === 'protected'
      ? 'ok'
      : report?.overall === 'warning'
        ? 'warn'
        : 'danger'

  const overallLabel =
    report?.overall === 'protected'
      ? t.leak.protected
      : report?.overall === 'warning'
        ? t.leak.warning
        : t.leak.unprotected

  const loc = report ? formatProxyLocation(report.country, report.exitIp) : ''

  return createPortal(
    <div
      className="backdrop"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className="dialog leakdialog"
        role="dialog"
        aria-modal="true"
        aria-label={t.leak.title}
      >
        <div className="dialog-head">
          <Icon name="shield" />
          <h2>{t.leak.title}</h2>
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

          {loading && !report ? (
            <div className="leak-loading">
              <Icon name="refresh" className="icon sm spin" />
              <span>{t.leak.checking}</span>
            </div>
          ) : report ? (
            <>
              <div className={`leak-summary is-${overallClass}`}>
                <div className="leak-summary-left">
                  <span className={`chip ${overallClass}`}>
                    <span className="dot" />
                    <span className="chiptext">{overallLabel}</span>
                  </span>
                  {report.proxyName && (
                    <span className="leak-summary-meta">
                      {report.proxyName}
                      {report.proxyType ? ` (${report.proxyType})` : ''}
                    </span>
                  )}
                </div>
                {loc && (
                  <div className="leak-exit-ip">
                    <span className="leak-exit-label">{t.leak.exitIp}</span>
                    <b className="mono selectable">{loc}</b>
                  </div>
                )}
              </div>

              <div className="leak-list" role="list">
                {report.items.map((item) => {
                  const stClass = item.status === 'ok' ? 'ok' : item.status === 'warn' ? 'warn' : 'danger'
                  return (
                    <div key={item.id} className="leak-row" role="listitem">
                      <span className={`leak-dot ${stClass}`} aria-hidden="true" />
                      <div className="leak-row-copy">
                        <b>{item.name}</b>
                        <span>{item.detail}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </>
          ) : null}

          <p className="hint leak-note">{t.leak.noDirectProbeNote}</p>
        </div>

        <div className="dialog-foot">
          <button
            type="button"
            className="btn"
            disabled={loading}
            aria-busy={loading}
            onClick={() => void runCheck()}
          >
            <Icon name="refresh" />
            {t.leak.recheck}
          </button>
          <button type="button" className="btn primary" onClick={onClose}>
            {t.actions.close}
          </button>
        </div>
      </div>
    </div>,
    document.body
  )
}
