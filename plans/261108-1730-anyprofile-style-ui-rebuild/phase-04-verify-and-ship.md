---
phase: 4
title: "Verify, package, document"
status: pending
effort: "1.5h"
---

# Phase 4 — Verify, package, document

1. **Inspect**: `npm run dev -- --port 5174` (daemon) → `node <design-taste-frontend>/scripts/inspect.mjs http://localhost:5174/preview.html --app`. 0 FAIL except the accepted font check; fix `tells.md` hits.
2. **Look**: agent-browser screenshots of all 5 pages + proxy dialog + test dialog at 1280×800 and 980×620, light/dark, EN/VI. Put AnyProfile's proxies page next to ours and fix spacing, type and colour drift. Run `impeccable` `critique`, then `polish`.
3. **Native**: `/Users/dobby/go/bin/wails build -platform darwin/arm64`, relaunch, check: hidden title bar + traffic lights clear of the brand row, window drags from sidebar/page head, every sidebar control clickable, `lsof -i :10800` listens, adding a proxy + quick switch works end to end (`curl -x socks5://127.0.0.1:10800 http://httpbin.org/ip`).
4. **Backend untouched**: `go test -race ./...` passes.
5. **Docs**: README "Using the app" section (Enable, Proxies, quick switch, Rules, Connections, language/theme); mark this plan `completed`.
6. **Commits** (conventional, one per phase): `refactor(ui): remove old frontend`, `feat(ui): anyprofile design system and shell`, `feat(ui): pages`, `docs: ui usage`.

## Done when

All success criteria in [plan.md](./plan.md) hold, with screenshots attached to the final report.

