---
phase: 2
title: "Shared components and data hooks"
status: pending
effort: "2h"
---

# Phase 2 — Shared components and data hooks

## Components (port from AnyProfile, MIT header, AnyProxyXa strings via `t`)

| File | Source | Notes |
|---|---|---|
| `components/dialog.tsx` | [dialog.tsx](file:///Users/dobby/Heroin/anyprofile/frontend/src/components/dialog.tsx) | Esc closes, focus trap, `role="dialog"` |
| `components/confirm.tsx` | confirm.tsx | Used for every delete/clear |
| `components/pop-menu.tsx`, `row-menu.tsx` | same | `•••` menus for proxy, rule, profile rows |
| `components/toast.tsx` | same | Success/error after every mutation; errors show the backend message |
| `components/empty-state.tsx` | same | No proxies / no rules / no connections |
| `components/health-pill.tsx` | same | Proxy latency: ok < 300 ms, warn < 1000 ms, bad otherwise, unknown if never tested |
| `components/proxy-select.tsx` | same | Sidebar quick switch and rule dialog proxy field |
| `components/switch.tsx` | new, ~20 lines | `role="switch"` for rule enable and Settings toggles |

## Hooks

| File | Job |
|---|---|
| `hooks/use-profile.ts` | Holds `ProfileListSummary` + active `Profile`; wraps every profile/proxy/rule mutation as `run(fn)` that sets the returned `Profile` and toasts errors |
| `hooks/use-engine.ts` | `Status()` on mount, subscribes to the existing engine event, start/stop/toggle |
| `hooks/use-traffic.ts` | Polls `GetFlows`, `GetConnectionLogs`, `GetActivityLogs` every 1 s only while the matching page is visible; computes conn/s for the status bar |
| `lib/format.ts` | bytes, durations, `HH:MM:SS` timestamps (port AnyProfile's `format.ts` parts that apply) |

Reuse the existing event names and polling intervals of the old `App.tsx` (read from git: `git show HEAD:frontend/src/App.tsx`) so backend behaviour is unchanged.

## Done when

- Unit tests: `format.test.ts`, `use-profile` error toast path, `health-pill` thresholds, `confirm` resolves true/false.
- `npm run typecheck && npm test` pass.

