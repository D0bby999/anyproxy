---
phase: 1
title: "Remove old UI, port design system, shell"
status: pending
effort: "2.5h"
---

# Phase 1 — Remove old UI, port design system, shell

## Steps

1. **Delete** `frontend/src/styles/app.css`, `frontend/src/App.tsx`, `frontend/src/App.test.tsx`. Keep `api.ts`, `types.ts`, `preview/stub.ts`, `main.tsx`.
2. **Styles** — copy from AnyProfile, trim to what AnyProxyXa uses, change the SPDX header to MIT:
   - `styles/tokens.css` ← [tokens.css](file:///Users/dobby/Heroin/anyprofile/frontend/src/styles/tokens.css) (drop `--tag-*`, `--agent-row*`; set button glow to `none`).
   - `styles/base.css` ← [base.css](file:///Users/dobby/Heroin/anyprofile/frontend/src/styles/base.css) (buttons, fields, chips, banners, icons).
   - `styles/shell.css` ← [shell.css](file:///Users/dobby/Heroin/anyprofile/frontend/src/styles/shell.css) (sidebar, pagehead, pagebody, settings grid, card, table, toasts, dialog). Drop welcome, engines, brand-tier, palette, mobile.
   - Add `.statusbar` (32 px, `--surface`, top hairline, `--t-xs`, tabular nums).
3. **Font + theme**: `npm i @fontsource-variable/inter`; import in `main.tsx`. Port `lib/theme.ts` (key `anyproxyxa.theme`), call `applyTheme(readTheme())` before render.
4. **i18n**: `i18n/en.ts`, `vi.ts`, `index.ts` (language from `localStorage` `anyproxyxa.lang`, default from `navigator.language`). Same `t.section.key` shape as AnyProfile. Port `i18n.test.ts` that checks both files have the same keys.
5. **Icons**: `components/icons.tsx` — AnyProfile's `Icon`/`LogoMark` pattern; icons needed: power, plug (connections), list (activity), proxy, rules, settings, plus, more, chevron, search, trash, filter, refresh, check, warn, arrow-up, arrow-down, copy, upload, download. New LogoMark for AnyProxyXa (simple bridge glyph, single accent).
6. **Window**: `main.go` → `TitleBar: mac.TitleBarHiddenInset()`; `Mac.Appearance` follows the system (omit forcing). Set `BackgroundColour` to `--bg` light (#ecebe7) to avoid a white flash.
7. **Shell**: `App.tsx` (< 200 lines): loads startup error, routes `page` state (`connections | activity | proxies | rules | settings`), renders `Sidebar`, `<main className="main">` with the page, `StatusBar`, toasts. Pages are placeholders in this phase.
8. **DESIGN.md** at repo root: tokens source, type scale, Inter, no glow, layout (sidebar 236 px, pagehead 64 px, statusbar 32 px), components list. **NOTICE**: add AnyProfile (owner relicensed to MIT) and AliasMode (Apache-2.0) attribution for `tokens.css`.

## Files

Delete: `frontend/src/App.tsx`, `frontend/src/App.test.tsx`, `frontend/src/styles/app.css`.
Create: `styles/{tokens,base,shell}.css`, `lib/theme.ts`, `i18n/{en,vi,index}.ts`, `i18n/i18n.test.ts`, `components/{icons,sidebar,page-head,status-bar}.tsx`, new `App.tsx`, `DESIGN.md`.
Modify: `main.tsx`, `main.go`, `NOTICE`, `frontend/package.json`.

## Done when

- `npm run typecheck && npm test` pass; preview at 1280×800 shows the AnyProfile-like shell in light and dark.
- `grep -rn "app.css\|proxy-switcher-pill" frontend/src` is empty.

