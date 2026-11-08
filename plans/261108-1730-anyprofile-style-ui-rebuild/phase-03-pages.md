---
phase: 3
title: "Pages"
status: pending
effort: "4h"
---

# Phase 3 — Pages

Each page = `PageHead` (title, sub, actions) + `pagebody` with one `card fill` holding a table, like
[proxies-page.tsx](file:///Users/dobby/Heroin/anyprofile/frontend/src/pages/proxies-page.tsx). Read
`impeccable` `reference/craft-floor.md` before writing UI code.

## Sidebar (`components/sidebar.tsx`)

- Brand row (LogoMark + AnyProxyXa).
- Primary button: **Enable proxy** / **Disable proxy** (`SetEngineEnabled`), shows *Starting…* while pending.
- **Default proxy** `ProxySelect` → `SetDefaultProxy` (quick switch, works from any page). Option "Direct" when no default.
- Nav: Connections (live flow count, `live` tone), Activity, Proxies (count), Rules (enabled count).
- Section **Profiles**: one nav item per profile, active one `aria-current`, row `•••` (Rename, Duplicate, Export, Delete), `+ New profile`.
- Footer: Settings; engine chip (dot ok/warn/off + backend name; click → Settings when the extension is not installed, with the System Settings approval hint).

## Pages

| Page | Head actions | Body |
|---|---|---|
| `connections-page.tsx` | Segmented `Live / Log`, search field, Filters (dialog → `SaveLogFilters`), Clear (`ClearClosedFlows` or `ClearConnectionLogs`, confirm) | Live: process (PID), destination (domain + IP:port), proto chip, action chip, proxy, age, ↑/↓ bytes. Log: time, process, destination, action/via — the ProxyBridge one-line format. Row menu: **Create rule for this app** → rule dialog prefilled |
| `activity-page.tsx` | Search, Clear (confirm) | Time, level chip, message (mono, selectable). Dev-only "Simulate flow" button when on the stub |
| `proxies-page.tsx` | `+ New proxy` | Name, type, `host:port` (mono), auth yes/no, health pill, default star. Row menu: Test, Edit, Set as default, Delete. `proxy-dialog.tsx` (type, host, port, user, password — never prefilled with the stored password, send-domain-to-proxy, description). `proxy-test-dialog.tsx`: target host/port, the 5 steps with ok/fail per step and latency |
| `rules-page.tsx` | Import, Export (selected or all), `+ New rule` | Order #, enable `Switch`, name, apps, hosts, ports, protocol chip, action chip (+ proxy name). Row menu: Edit, Clone, Move up/down, Delete. `rule-dialog.tsx` with "Pick app…" (`PickProcessExecutable`) and the ProxyBridge "When … Do …" layout |
| `settings-page.tsx` | — | AnyProfile `settings` grid with sticky group nav: **Traffic** (logging, DNS via proxy, log every connection, auto-clear → `UpdateOptions`), **Appearance** (theme system/light/dark, language EN/VI), **Profiles** (Import JSON with "make active", Export with "include passwords" off by default + warning), **Version** (`CheckForUpdates`, notes, download link), **About** (version, NOTICE/license, ProxyBridge credit) |

## Tests (one file per page, replacing the old `App.test.tsx`)

Cover every row of the parity checklist in `plan.md` against `preview/stub.ts`:
switch profile; create/rename/duplicate/delete profile; add/edit/test/delete proxy; quick switch default; add/toggle/move/clone/delete rule; import/export rules; Live/Log toggle + Clear; filters save; options toggle; check updates; language switch changes a visible label; theme switch sets `data-theme`.

## Done when

- `npm run typecheck && npm test && npm run build` pass.
- Every checklist row ticked in `plan.md`.

