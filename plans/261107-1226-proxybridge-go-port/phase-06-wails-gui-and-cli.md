---
title: "Phase 6: Wails v2 Desktop GUI, Dual-Mode CLI & Packaging"
status: todo
---

# Phase 6: Wails v2 Desktop GUI, Dual-Mode CLI & Packaging

## Context Links

- Local UI & app architecture references:
  - [`anyprofile/main.go`](file:///Users/dobby/Heroin/anyprofile/main.go), [`anyprofile/app.go`](file:///Users/dobby/Heroin/anyprofile/app.go), [`anyprofile/app_bindings.go`](file:///Users/dobby/Heroin/anyprofile/app_bindings.go)
  - [`anyprofile/frontend/src/App.tsx`](file:///Users/dobby/Heroin/anyprofile/frontend/src/App.tsx), `anyprofile/frontend/preview.html`
- Upstream UI & CLI references:
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/ProxyBridge/ContentView.swift`, `ProxySettingsView.swift`, `ProxyRulesView.swift`, `ProxyCheckerView.swift`, `CLIArguments.swift`, `CLIRunner.swift`
  - `/tmp/ProxyBridge/Windows/gui/main.c`, `ui/flows.h`, `ui/dialogs.h`, `Windows/cli/main.c`

## Overview

- **Priority**: P1
- **Status**: Done
- **Description**: Orchestrated all packages in `internal/service`, exposed Wails bindings (`app.go`, `app_bindings.go`), implemented the dual-mode CLI (`--profile`, `--list-profiles`, `--verbose 0..3`, `--test-proxy`, `--version`), and built the Wails v2 React + TypeScript desktop GUI with browser preview harness (`frontend/preview.html`).

## Key Insights

- Like upstream macOS ProxyBridge (`main.swift` + `CLIArguments.swift`), **one binary serves both GUI and CLI**:
  - Launched without CLI flags (from Finder/Dock/Start Menu): starts the Wails v2 desktop GUI.
  - Launched with `--profile <name|path|->`, `--list-profiles`, `--verbose <0-3>`, `--test-proxy <id>`, `--version`, or `--help`: runs headlessly in the terminal until `Ctrl+C`, which cleanly stops the engine and clears rules.
- Following the global UI rules (`impeccable` + `design-taste-frontend` + `frontend/preview.html` verification):
  - This is an **Operate surface** (network routing control plane, live flow monitor, rule table, proxy diagnostics): dense, scannable tabular layout (`font-variant-numeric: tabular-nums` on ports, PIDs, latencies, and byte counters), clear status badges (`PROXY`, `DIRECT`, `BLOCK`, `PAUSED`), instant multi-word keyboard filtering (`Ctrl+F` / `Cmd+F`), and zero decorative AI-slop cards/gradients.
  - Include `frontend/preview.html` with realistic fixtures (live connections, deduplicated connection log, rules, proxies, 5-step proxy checker stream) so we render and inspect the UI in a real browser at desktop viewport size (`1200x760`).

## Requirements

- [x] Implement `internal/service`:
  - Coordinates `store.Store`, `secrets.Vault`, `pbprofile`, `rules.Engine`, `dnsmap.Store`, `netproxy`, `flows.Tracker`, `flows.LogManager`, and `driver.Driver`.
  - Atomic profile switching (`SwitchProfile`, `CreateProfile`, `DuplicateProfile`, `RenameProfile`, `DeleteProfile`, `ImportProfile`, `ExportProfile`).
  - Proxy CRUD (`AddProxy`, `UpdateProxy`, `DeleteProxy`, `SetDefaultProxy`, `TestProxyEx` streaming events to frontend).
  - Rule CRUD (`AddRule`, `UpdateRule`, `DeleteRule`, `ToggleRule`, `MoveRule`, `ImportRulesJSON`, `ExportRulesJSON`).
  - Runtime controls (`StartEngine`, `StopEngine`, `SetEnabled` master pause switch, `SetTrafficLogging`, `SetLogEveryConnection`, `SetAutoClearLogs`, `SetLogFilters`, `ClearConnectionLogs`, `ClearClosedFlows`).
- [x] Implement Dual-Mode CLI (`internal/cli/cli.go` wired in `main.go`):
  - `--profile <name|path|->` (supports saved profile name, `.pbprofile` path, or `-` stdin).
  - `--list-profiles` (marks active profile with `*`).
  - `--verbose <0-3>` (`0` silent, `1` activity logs, `2` connection events, `3` both).
  - `--test-proxy <id|host:port>` (runs 5-step checker from CLI).
  - `--version`, `-h` / `--help`.
- [x] Implement Wails v2 Frontend (`frontend/src/`):
  - **Top Bar & Sidebar**: Profile selector + quick actions (New, Duplicate, Rename, Delete, Import/Export `.pbprofile`), Master Engine Switch (`Active` / `Paused` / `Stopped` + macOS Extension status notice), Search bar (`Cmd/Ctrl+F`), Navigation (`Connections`, `Live Flows`, `Proxy Rules`, `Proxy Servers`, `Activity Logs`, `Settings`).
  - **Connections View**: Real-time tabular stream (`Time`, `Protocol`, `Process`, `Destination / Domain`, `Port`, `Action / Proxy`), multi-word filter, `Log Filters` modal (`Include` / `Exclude` globs), `Log Every Connection` toggle, `Clear` button.
  - **Live Connections View**: Polls `GetFlows` every 1s only while tab is visible; displays `Process (PID)`, `Target (Domain/IP:Port)`, `Proto`, `Proxy`, `Duration`, `Sent`, `Received`; finished flows turn muted grey until `Clear Closed`.
  - **Proxy Rules View & Editor Modal**: Priority-ordered table (`#`, `Enabled`, `Name`, `Process`, `Target Hosts / Domains`, `Ports`, `Protocol`, `Action / Proxy`, `Localhost`), move up/down (disabled while search filter is active, matching upstream), clone, bulk checkbox select for JSON Export/Import, and rich Rule Editor with validation and native process file picker.
  - **Proxy Servers View & 5-Step Checker Modal**: Manage multiple `SOCKS5` / `HTTP` proxies, `SendDomainToProxy` toggle, Default Proxy badge, bulk paste (`netproxy.ParseList` from `anyprofile`), and interactive 5-step Proxy Checker modal streaming live step results (TCP reach, Auth/Tunnel, HTTP page, 3x Latency, SOCKS5 UDP ASSOCIATE).
  - **Browser Preview Harness (`frontend/preview.html`)**: Mock Wails bindings with realistic live flows, logs, rules, and proxy checker simulation for browser inspection.

## Architecture

```
anyproxyxa/
├── main.go                 # CLI flag detection -> cli.Run() OR wails.Run()
├── app.go                  # Wails App lifecycle, startup/beforeClose, event emitters
├── app_bindings.go         # Thin Wails-bound methods delegating to internal/service
├── menu_darwin.go          # Native macOS menu bar
├── menu_windows.go         # Windows menu
├── internal/
│   ├── cli/                # Headless CLI runner (--profile, --list-profiles, --verbose)
│   └── service/            # Core application service orchestrating store, vault, driver, checker
└── frontend/
    ├── index.html
    ├── preview.html        # Standalone browser preview harness with fixture bindings
    └── src/
        ├── App.tsx
        ├── api.ts
        ├── pages/          # ConnectionsPage, LiveFlowsPage, RulesPage, ProxiesPage, LogsPage
        └── components/     # RuleModal, ProxyModal, ProxyCheckerModal, LogFilterModal, ProfileBar
```

## Related Code Files

- **Create**:
  - `/Users/dobby/Heroin/anyproxyxa/internal/service/service.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/service/service_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/cli/cli.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/cli/cli_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/main.go`
  - `/Users/dobby/Heroin/anyproxyxa/app.go`
  - `/Users/dobby/Heroin/anyproxyxa/app_bindings.go`
  - `/Users/dobby/Heroin/anyproxyxa/menu_darwin.go`
  - `/Users/dobby/Heroin/anyproxyxa/menu_windows.go`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/package.json`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/tsconfig.json`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/vite.config.ts`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/index.html`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/preview.html`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/main.tsx`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/api.ts`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/App.tsx`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/pages/ConnectionsPage.tsx`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/pages/LiveFlowsPage.tsx`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/pages/RulesPage.tsx`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/pages/ProxiesPage.tsx`
  - `/Users/dobby/Heroin/anyproxyxa/frontend/src/pages/LogsPage.tsx`

## Implementation Steps

1. Implement `internal/service` connecting `store`, `secrets.Vault`, `pbprofile`, `rules`, `dnsmap`, `netproxy`, `flows`, and `driver`.
2. Implement `internal/cli` for `--profile`, `--list-profiles`, `--verbose`, `--test-proxy`, and signal-safe shutdown (`SIGINT`/`SIGTERM`).
3. Create `main.go`, `app.go`, `app_bindings.go`, `menu_darwin.go`, and `menu_windows.go`.
4. Build the React + TypeScript frontend (`frontend/src/`) following `impeccable` and `design-taste-frontend` Operate-screen standards (`tabular-nums`, keyboard navigation, zero AI tells) and wire `frontend/preview.html`.
5. Verify with `go test ./...`, `npm run typecheck`, `npm test`, and inspect `frontend/preview.html` in a real browser via `design-taste-frontend/scripts/inspect.mjs --app`.

## Todo

- [x] Implement `internal/service` and `internal/cli` with unit tests
- [x] Implement Wails `main.go`, `app.go`, `app_bindings.go`, and platform menus
- [x] Build React/TS frontend pages (`Connections`, `LiveFlows`, `Rules`, `Proxies`, `Logs`) and modals (`RuleModal`, `ProxyModal`, `ProxyCheckerModal`, `LogFilterModal`)
- [x] Create `frontend/preview.html` harness and visually verify at `1200x760`
- [x] Pass `make test` and `make lint` (`go test ./...`, `GOOS=windows go vet`, `npm run typecheck`, `npm test`)

## Success Criteria

- `go test ./... -count=1` and `cd frontend && npm run typecheck && npm test` pass.
- Running the binary with `--list-profiles` or `--profile <file.pbprofile>` works in CLI mode.
- Opening `frontend/preview.html` renders the complete UI cleanly and passes browser inspection.
