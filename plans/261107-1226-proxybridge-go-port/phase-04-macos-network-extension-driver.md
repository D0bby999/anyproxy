---
title: "Phase 4: macOS Driver: Thin Swift NetworkExtension + Go IPC Bridge"
status: completed
---

# Phase 4: macOS Driver: Thin Swift NetworkExtension + Go IPC Bridge

## Context Links

- Upstream macOS extension & host controller:
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/AppProxyProvider.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/DNSProxyProvider.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/DNSMapStore.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/FlowFailure.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/ProviderDiagnostics.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/ProxyBridge/ProxyBridgeViewModel.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/ProxyBridge/TunnelRepair.swift`
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/ProxyBridge/CLIRunner.swift`
- Local signing & bundle conventions:
  - [`anyprofile/Makefile`](file:///Users/dobby/Heroin/anyprofile/Makefile)

## Overview

- **Priority**: P1
- **Status**: Completed
- **Description**: Define the unified cross-platform `internal/driver.Driver` interface in Go, port the thin Swift `NETransparentProxyProvider` + `NEDNSProxyProvider` System Extension into `macos/extension/`, and implement `internal/driver/darwin` so the Go Wails app and CLI activate, configure, monitor, and repair the macOS Network Extension seamlessly.

## Key Insights

- On macOS 13+, transparent per-application TCP/UDP proxying (`NETransparentProxyProvider`) and system DNS snooping (`NEDNSProxyProvider`) live in a `.systemextension` bundle inside `Contents/Library/SystemExtensions/`.
- Upstream's Swift extension (`AppProxyProvider.swift` + `DNSProxyProvider.swift`) is already a self-contained JSON-RPC-like provider controlled via `NETunnelProviderSession.sendProviderMessage(Data)`:
  - `setProxyConfigs`: replaces stored proxy configs atomically.
  - `setRules`: replaces compiled rules atomically in one step (preventing half-built rule races).
  - `setTrafficLogging` / `setEnabled`: toggles traffic logging and master pause switch.
  - `getLogs` / `getFlows`: drains connection + activity logs and live proxied flows (`PB_FLOW` parity).
  - `clearConfig` / `clearRules`: wipes rules and in-memory proxy credentials on stop.
- We also extend `AppProxyProvider.swift` with `getFlows` (returning live active TCP/UDP relay byte counters and age) and `setEnabled` (master pause switch) so macOS has 100% feature parity with Windows `Live Connections` and `Enable ProxyBridge`!
- When running in `wails dev` or unit tests without System Extension entitlements, `internal/driver/darwin` falls back gracefully to a local loopback/mock driver and surfaces a clear status badge in the UI when the app is not running from `/Applications` or awaiting System Settings approval.

## Requirements

- [x] Define `internal/driver/driver.go` (`Driver` interface shared across `darwin`, `windows`, and `mock` test driver):
  - `Start(ctx) error`, `Stop() error`, `SetEnabled(bool) error`, `Status() EngineStatus`
  - `SyncProfile(proxies []ProxyConfig, rules []Rule, opts RuntimeOptions) error`
  - `GetFlows(max int) ([]flows.Flow, error)`
  - `DrainLogs() ([]flows.ConnectionEvent, []flows.ActivityEvent, error)`
- [x] Port and enhance `macos/extension/` (`AppProxyProvider.swift`, `DNSProxyProvider.swift`, `DNSMapStore.swift`, `FlowFailure.swift`, `ProviderDiagnostics.swift`, `main.swift`, `Info.plist`, entitlements):
  - Add `getFlows` action to `AppProxyProvider.handleAppMessage` to expose active TCP/UDP flows with `id`, `pid`, `process`, `dest`, `host`, `port`, `is_udp`, `proxy_id`, `age_ms`, `sent`, `received`.
  - Add `setEnabled` action to pause/resume rule evaluation for new flows without tearing down the tunnel.
  - Add `localhostViaProxy` per-rule support on macOS matching Windows/Linux.
- [x] Implement `internal/driver/darwin`:
  - Thin Objective-C/Swift C-bridge (`bridge_darwin.m` / Swift helper or CGo `NetworkExtension` / `SystemExtensions` bindings) to activate the system extension (`OSSystemExtensionRequest`), configure `NETransparentProxyManager` and `NEDNSProxyManager`, send JSON provider messages over `NETunnelProviderSession`, and run `TunnelRepair` watchdog (`#282`).
  - Provide a zero-entitlement fallback mode for `wails dev` and automated tests.

## Architecture

```
macos/
└── extension/                  # Com.anytools.anyproxyxa.extension (.systemextension)
    ├── AppProxyProvider.swift  # NETransparentProxyProvider + live flow byte counters + setEnabled
    ├── DNSProxyProvider.swift  # NEDNSProxyProvider + DNSMapStore
    ├── FlowFailure.swift
    ├── ProviderDiagnostics.swift
    └── Info.plist & entitlements
internal/driver/
├── driver.go                   # Platform-agnostic Driver interface + MockDriver
├── darwin.go                   # macOS driver implementation (build tag: darwin)
└── bridge_darwin.m             # NETunnelProviderSession / OSSystemExtensionRequest bridge
```

## Related Code Files

- **Create**:
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/driver.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/mock.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/darwin.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/darwin_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/AppProxyProvider.swift`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/DNSProxyProvider.swift`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/DNSMapStore.swift`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/FlowFailure.swift`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/ProviderDiagnostics.swift`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/main.swift`
  - `/Users/dobby/Heroin/anyproxyxa/macos/extension/Info.plist`

## Implementation Steps

1. Create `internal/driver/driver.go` and `internal/driver/mock.go` defining the `Driver` contract used by `internal/service`.
2. Port the Swift NetworkExtension files from `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/` into `macos/extension/`, updating bundle IDs to `dev.anyprofile.anyproxyxa` / `dev.anyprofile.anyproxyxa.extension` and adding `getFlows`, `setEnabled`, and `localhostViaProxy`.
3. Implement `internal/driver/darwin.go` to marshal `setProxyConfigs`, `setRules`, `setTrafficLogging`, `setEnabled`, `getLogs`, `getFlows`, and `clearConfig` messages to the extension, and merge drained logs/flows into `internal/flows`.
4. Add `Makefile` target to build and sign the `.systemextension` inside `build/bin/AnyProxyXa.app/Contents/Library/SystemExtensions/` when a Developer ID / provisioning profile with NetworkExtension capability is present, while keeping standard `make build` / `make dev` working out of the box.
5. Run Swift logic tests (`macos/tests/`) and Go driver tests (`go test ./internal/driver/...`).

## Todo

- [x] Define `internal/driver.Driver` and `MockDriver`
- [x] Port `macos/extension/*.swift` and add `getFlows`, `setEnabled`, and `localhostViaProxy`
- [x] Implement `internal/driver/darwin.go` controller and JSON message protocol
- [x] Wire `Makefile` bundle packaging and signing for macOS `.app` + `.systemextension`
- [x] Pass `go test ./internal/driver/... -count=1` and Swift logic tests

## Success Criteria

- `go test ./internal/driver/... -count=1` passes.
- Swift logic tests for `AppProxyProvider` rule matching, `DNSMapStore`, `FlowFailure`, and `getFlows` pass on macOS.

## Risk Assessment & Security Considerations

- **Loop Prevention**: Extension explicitly ignores flows where `sourceAppSigningIdentifier` is `dev.anyprofile.anyproxyxa` or `dev.anyprofile.anyproxyxa.extension`, and exempts configured proxy server endpoints.
- **Credential Hygiene**: `Stop()` sends `clearConfig` to the extension so proxy passwords never remain in extension memory after stopping.
