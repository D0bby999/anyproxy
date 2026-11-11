// SPDX-License-Identifier: AGPL-3.0-or-later
import React, { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { api } from '../api'
import { t } from '../i18n'
import { providerLogo, providers, providerTypes, type ProviderType } from '../lib/proxy-providers'
import { Icon } from './icons'

export function BuyProxies({ className = 'btn buy nodrag', compact = false }: { className?: string; compact?: boolean }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button
        type="button"
        className={className}
        onClick={() => setOpen(true)}
        title={t.proxies.buy}
        aria-label={compact ? t.proxies.buy : undefined}
      >
        <Icon name="cart" />
        {!compact && <span>{t.proxies.buy}</span>}
      </button>
      {open && <BuyProxyDialog onClose={() => setOpen(false)} />}
    </>
  )
}

export function BuyProxyDialog({ onClose }: { onClose: () => void }) {
  const [type, setType] = useState<ProviderType | ''>('')
  const ref = useRef<HTMLDivElement>(null)
  const shown = providers.filter((p) => !type || p.types.includes(type))

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      const all = document.querySelectorAll('.backdrop')
      if (all[all.length - 1] === ref.current) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return createPortal(
    <div className="backdrop" ref={ref} onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div className="dialog partnerdialog" role="dialog" aria-modal="true" aria-label={t.proxies.buy}>
        <div className="dialog-head">
          <Icon name="cart" />
          <h2>{t.proxies.buy}</h2>
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
          <p className="partner-guide muted">{t.partners.guide}</p>
          <div className="seg" role="group" aria-label={t.partners.filter}>
            {(['', ...providerTypes] as const).map((k) => (
              <button
                key={k || 'all'}
                type="button"
                aria-pressed={type === k}
                onClick={() => setType(k)}
              >
                {k ? t.partners.types[k] : t.partners.all}
              </button>
            ))}
          </div>
          <ul className="partnergrid">
            {shown.map((p) => {
              const logo = providerLogo(p)
              return (
                <li key={p.id}>
                  <button type="button" className="partner" onClick={() => api.openExternal(p.url)}>
                    {logo
                      ? <img className="partner-mark" src={logo} alt="" draggable={false} />
                      : <span className="partner-mark letter" aria-hidden="true">{p.name[0]}</span>}
                    <span className="partner-main">
                      <span className="partner-head">
                        <b>{p.name}</b>
                        <span className="partner-corner">
                          {p.recommended && <span className="chip accent">{t.partners.recommended}</span>}
                          <Icon name="external" className="icon sm" />
                        </span>
                      </span>
                      <span className="muted">{t.partners.desc[p.id]}</span>
                      <span className="partner-tags">
                        {p.types.map((k) => <span key={k} className="chip">{t.partners.types[k]}</span>)}
                      </span>
                    </span>
                  </button>
                </li>
              )
            })}
          </ul>
          <p className="partner-note faint">{t.partners.disclosure}</p>
        </div>

        <div className="dialog-foot">
          <button type="button" className="btn" onClick={onClose}>{t.actions.close}</button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
