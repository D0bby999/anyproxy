// SPDX-License-Identifier: AGPL-3.0-or-later
import { en } from './en'
import { vi } from './vi'

export type Lang = 'en' | 'vi'
const KEY = 'anyproxy.lang'
const LEGACY_KEY = 'anyproxyxa.lang'

export function getLanguage(): Lang {
  try {
    const saved = localStorage.getItem(KEY) ?? localStorage.getItem(LEGACY_KEY)
    if (saved === 'vi' || saved === 'en') return saved
    if (typeof navigator !== 'undefined' && navigator.language?.startsWith('vi')) {
      return 'vi'
    }
  } catch {}
  return 'en'
}

export function setLanguage(lang: Lang) {
  try {
    localStorage.setItem(KEY, lang)
  } catch {}
}

const currentLang = getLanguage()
export const t = currentLang === 'vi' ? vi : en
export { en, vi }
