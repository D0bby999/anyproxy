// SPDX-License-Identifier: MIT

export type ThemeChoice = 'system' | 'light' | 'dark'
const KEY = 'anyproxy.theme'
const LEGACY_KEY = 'anyproxyxa.theme'

export function readTheme(storage: Pick<Storage, 'getItem'> = localStorage): ThemeChoice {
  const v = storage.getItem(KEY) ?? storage.getItem(LEGACY_KEY)
  return v === 'light' || v === 'dark' || v === 'system' ? v : 'system'
}

export function resolveTheme(choice: ThemeChoice, prefersDark: boolean): 'light' | 'dark' {
  if (choice === 'system') return prefersDark ? 'dark' : 'light'
  return choice
}

export function applyTheme(choice: ThemeChoice): () => void {
  try {
    localStorage.setItem(KEY, choice)
  } catch {}
  const mq = window.matchMedia('(prefers-color-scheme: dark)')
  const set = () => {
    document.documentElement.dataset.theme = resolveTheme(choice, mq.matches)
  }
  set()
  mq.addEventListener('change', set)
  return () => mq.removeEventListener('change', set)
}
