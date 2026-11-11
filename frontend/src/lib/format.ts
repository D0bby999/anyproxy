// SPDX-License-Identifier: AGPL-3.0-or-later

/** Human-readable size, base 1024, matching AnyProfile format */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) n = 0
  if (n < 1024) return `${Math.round(n)} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

/** Format milliseconds or seconds duration */
export function formatDuration(ms: number): string {
  if (!ms || ms < 0) return '0s'
  const secs = Math.floor(ms / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const remSecs = secs % 60
  if (mins < 60) return `${mins}m ${remSecs}s`
  const hours = Math.floor(mins / 60)
  return `${hours}h ${mins % 60}m`
}

