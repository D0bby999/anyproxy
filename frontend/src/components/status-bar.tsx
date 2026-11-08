// SPDX-License-Identifier: MIT
import React from 'react'
import { t } from '../i18n'
import type { EngineStatus, Profile } from '../types'

export function StatusBar({
  engine,
  profile,
  connRate = 0,
}: {
  engine: EngineStatus | null
  profile: Profile | null
  connRate?: number
}) {
  const isRunning = engine?.state === 'running'
  const isStarting = engine?.state === 'starting'

  return (
    <footer className="statusbar">
      <div className="stat-item">
        <span
          className={`stat-dot ${isRunning ? 'ok' : isStarting ? 'warn' : 'off'}`}
        />
        <span>
          {t.status.engine}:{' '}
          <strong>
            {isRunning
              ? t.engine.running
              : isStarting
              ? t.engine.starting
              : t.engine.stopped}
          </strong>
        </span>
      </div>

      <div className="stat-item">
        <span>
          {t.status.profile}:{' '}
          <strong>{profile?.name || 'Default'}</strong>
        </span>
      </div>

      <div className="stat-item">
        <span>
          {t.status.proxies}:{' '}
          <strong>{profile?.proxies?.length ?? 0}</strong>
        </span>
      </div>

      <div className="stat-item">
        <span>
          {t.status.rules}:{' '}
          <strong>{profile?.rules?.length ?? 0}</strong>
        </span>
      </div>

      <span className="spacer" />

      <div className="stat-item">
        <span>
          {t.status.rate}:{' '}
          <strong>{connRate} conn/s</strong>
        </span>
      </div>
    </footer>
  )
}
