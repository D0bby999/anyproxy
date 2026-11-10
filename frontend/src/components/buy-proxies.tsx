// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState } from 'react'
import { api } from '../api'
import { t } from '../i18n'
import { providers, type ProviderType } from '../lib/proxy-providers'
import { Icon } from './icons'

const types: ProviderType[] = ['residential', 'isp', 'mobile', 'datacenter']

export function BuyProxies() {
  const [open, setOpen] = useState(false)
  const [type, setType] = useState<ProviderType | ''>('')
  const dialog = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    if (open) dialog.current?.showModal()
  }, [open])

  return <>
    <button type="button" className="btn nodrag" onClick={() => setOpen(true)}><Icon name="cart" />{t.proxies.buy}</button>
    {open && <dialog ref={dialog} className="provider-dialog" aria-labelledby="buy-proxies-title" onClose={() => setOpen(false)}>
      <div className="dialog-head">
        <h2 id="buy-proxies-title">{t.proxies.buy}</h2>
        <button type="button" className="iconbtn sm" aria-label={t.actions.cancel} onClick={() => dialog.current?.close()}><Icon name="close" /></button>
      </div>
      <div className="dialog-body">
        <p className="provider-guide">{t.proxies.buyGuide}</p>
        <p className="provider-disclosure">{t.proxies.buyDisclosure}</p>
        <div className="provider-filters" role="group" aria-label={t.proxies.type}>
          {(['', ...types] as const).map((kind) => <button key={kind || 'all'} type="button" className={`btn sm ${type === kind ? 'primary' : ''}`} aria-pressed={type === kind} onClick={() => setType(kind)}>{kind ? t.proxies[kind] : t.proxies.filterAll}</button>)}
        </div>
        <ul className="provider-list">
          {providers.filter((p) => !type || p.types.includes(type)).map((p) => <li key={p.name}>
            <button type="button" className="provider-link" onClick={() => api.openExternal(p.url)}>
              <span className="provider-name">{p.name}</span>
              <span className="provider-types">{p.types.map((kind) => t.proxies[kind]).join(' · ')}</span>
              <Icon name="chevron" className="icon sm" />
            </button>
          </li>)}
        </ul>
      </div>
      <div className="dialog-foot"><button type="button" className="btn" onClick={() => dialog.current?.close()}>{t.actions.cancel}</button></div>
    </dialog>}
  </>
}
