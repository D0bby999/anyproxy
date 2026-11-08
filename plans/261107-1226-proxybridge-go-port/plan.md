---
title: "Port InterceptSuite/ProxyBridge to Go + Wails (AnyProxyXa)"
description: "Port ProxyBridge universal transparent proxy client to Go + Wails v2 on macOS and Windows following AnyProfile architecture."
status: completed
priority: P1
effort: "24h"
branch: main
tags: [feature, backend, frontend, infra, security]
blockedBy: []
blocks: []
created: 2026-11-07
---

# Port InterceptSuite/ProxyBridge to Go + Wails (AnyProxyXa)

## Overview

Port [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge) (`cb897b017a88a142ec803c3af138b0ad75c2ae12`, MIT License) into the AnyTools desktop ecosystem (`github.com/D0bby999/anyproxyxa`) in Go + Wails v2 + React/TypeScript, matching [`anyprofile`](file:///Users/dobby/Heroin/anyprofile/app.go) conventions (`modernc.org/sqlite`, OS Keychain/Credential Manager `secrets.Vault`, owner-only `0700`/`0600` files, preview harness, and dual GUI/CLI binary).

## Source Manifest & Attribution

- **Source Repository**: `https://github.com/InterceptSuite/ProxyBridge`
- **Resolved Commit SHA**: `cb897b017a88a142ec803c3af138b0ad75c2ae12`
- **License**: MIT License (`Copyright (c) 2025 Anof-cyber/InterceptSuite` — preserved in `NOTICE` and third-party attributions)
- **Reference Local Architecture**: `/Users/dobby/Heroin/anyprofile` (`github.com/D0bby999/anyprofile`)

## Dependency & Component Matrix

| Component | Source (`ProxyBridge`) | Local Equivalent (`anyprofile`) | Status | Port Strategy |
|---|---|---|---|---|
| App Shell & IPC | Swift AppKit (macOS), Win32 C GUI (Win) | Wails v2 (`app.go`, `app_bindings.go`) | `EXISTS` | Reuse Wails v2 + React/TS pattern from `anyprofile` |
| Persistence & Secrets | `UserDefaults` / plaintext `.pbprofile` | `internal/store` (SQLite) + `internal/secrets` (`Vault`) | `EXISTS` | Primary store in SQLite + Keychain Vault; 100% lossless `.pbprofile` & rules JSON import/export |
| TCP Proxy Client | `NWTCPConnection` / C `pb_socks5.c`, `pb_http.c` | `internal/netproxy` (`upstream.go`, `socks.go`, `relay.go`) | `EXISTS` | Extend `internal/netproxy` with `SendDomainToProxy` (`socks5h` / `CONNECT host:port`) |
| SOCKS5 UDP Relay | Swift `UDPAssociation` / C `udp_relay_server` | Refused in `anyprofile/internal/netproxy/socks.go` | `NEW` | Implement RFC 1928 `UDP ASSOCIATE` client & local UDP relay in Go (`internal/netproxy/udp.go`) |
| 5-Step Proxy Checker | `ProxyCheckerView.swift` / `ProxyBridge_TestProxyConfigEx` | `internal/netproxy/check.go` (GeoIP check) | `NEW` | Add 5-step diagnostic checker (TCP reach, tunnel+auth, HTTP page, 3x latency, UDP ASSOCIATE probe) |
| Rule Engine & Matcher | `ProxyRule.Matcher` (Swift) / `pb_rules.c` (C) | None | `NEW` | Pure Go `internal/rules` pre-compiled matcher (process glob, IP/CIDR/range/wildcard, domain glob, port range, auto-direct overrides) |
| DNS Snoop Map | `DNSProxyProvider.swift` + `DNSMapStore` / `pb_dns.c` | None | `NEW` | Pure Go `internal/dnsmap` (RFC 1035 A/AAAA/CNAME parser + `/etc/hosts` + 32k LRU IP->domain cache) |
| Flow & Log Tracker | `PB_FLOW` (`GetFlows`) + dedup/filter logs | None | `NEW` | Pure Go `internal/flows` (400 live flows, byte counters, dedup vs `LogEveryConnection`, include/exclude `LogFilters`) |
| macOS Interception | `extension/` (`NETransparentProxyProvider` + `NEDNSProxyProvider`) | Developer ID signing (`25C8SNXS6Q`) in `Makefile` | `ADAPT` | Keep thin Swift `.systemextension` (`macos/extension/`) + Go `internal/driver/darwin` bridge |
| Windows Interception | `WinDivert` + `iphlpapi` + `ProxyBridgeSvc.exe` (C) | None | `NEW` | Pure Go `internal/driver/windows` using `golang.org/x/sys/windows` (`LazyDLL` for `WinDivert.dll` + `iphlpapi.dll` + `windows/svc`) |

## Decision Matrix (Approved in Challenge Gate)

| # | Decision | Source's Way | Local Way (`AnyProfile`) | Risk | Approved Choice |
|---|---|---|---|---|---|
| 1 | Target OS (v1) | macOS + Windows + Linux | macOS (arm64) + Windows (amd64) | Low | **macOS + Windows** (100% aligned with `anyprofile`) |
| 2 | macOS Interception | Swift `NETransparentProxyProvider` + `NEDNSProxyProvider` | Go host + native signed helper in `Contents/` | Medium | **Thin Swift `.systemextension` + Go Wails App/CLI** |
| 3 | Windows Interception | C DLL (`WinDivert`) + C Windows Service | Pure Go (`CGO_ENABLED=0` cross-vetted) | Medium | **Pure Go `WinDivert` (`LazyDLL`) + Go Windows Service (`svc`)** |
| 4 | Storage & Credentials | Plaintext `.pbprofile` / `UserDefaults` | SQLite (`modernc.org/sqlite`) + `secrets.Vault` | Low | **SQLite + `secrets.Vault` + 100% `.pbprofile` import/export** |

## Risk Score & Rollback Strategy

- **Risk Score**: **Low-Medium (2 critical assumptions resolved)**:
  1. macOS `NetworkExtension` requires app bundle placement in `/Applications` and System Extension entitlements; mitigated by providing a mock/loopback driver for `wails dev` and `frontend/preview.html` so UI and core Go tests run without kernel extensions.
  2. Windows `WinDivert` requires Administrator / `LocalSystem` service; mitigated by isolating `WinDivert` inside `internal/driver/windows` behind the `driver.Engine` interface and running unit tests against recorded packets.
- **Rollback Strategy**: Each phase is self-contained with its own `go test` / `npm test` gate. Stopping the engine or quitting the app unloads the tunnel/WinDivert handle cleanly so OS traffic returns to direct routing immediately.

## Goals

| # | Goal | Priority |
|---|------|----------|
| 1 | Scaffold Go + Wails v2 repo (`anyproxyxa`) with SQLite, Keychain `secrets.Vault`, and lossless `.pbprofile` import/export | P1 |
| 2 | Implement pure Go Rule Engine (`internal/rules`) and DNS Snoop Cache (`internal/dnsmap`) with 100% parity with ProxyBridge | P1 |
| 3 | Implement Go TCP/UDP Proxy Relay (`SOCKS5` CONNECT + `UDP ASSOCIATE`, `HTTP` CONNECT), Live Flow Tracker, and 5-step Proxy Checker | P1 |
| 4 | Integrate macOS thin Swift `NETransparentProxyProvider` / `NEDNSProxyProvider` extension with Go IPC controller | P1 |
| 5 | Implement Windows pure Go `WinDivert` + `iphlpapi` packet interceptor and `LocalSystem` service over Named Pipe IPC | P1 |
| 6 | Deliver Wails v2 React/TS GUI (Connections, Live Connections, Rules, Proxies, Profiles, Logs), `preview.html` harness, and headless CLI | P1 |

## Phases

| # | Phase | Status |
|---|-------|--------|
| 1 | [Foundation, Storage, Vault & .pbprofile Compatibility](./phase-01-start.md) | Completed |
| 2 | [Rule Engine, Matcher & DNS Snoop Cache](./phase-02-rule-engine-and-dns-cache.md) | Completed |
| 3 | [TCP/UDP Proxy Relay, Flow Tracker & 5-Step Proxy Checker](./phase-03-proxy-relay-and-checker.md) | Completed |
| 4 | [macOS Driver: Thin Swift NetworkExtension + Go IPC Bridge](./phase-04-macos-network-extension-driver.md) | Completed |
| 5 | [Windows Driver: Pure Go WinDivert Engine & Windows Service](./phase-05-windows-windivert-service-driver.md) | Completed |
| 6 | [Wails v2 Desktop GUI, Dual-Mode CLI & Packaging](./phase-06-wails-gui-and-cli.md) | Completed |

## Success Criteria

- [x] `go test ./internal/... -count=1` passes on macOS and cross-vets cleanly with `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...`
- [x] `.pbprofile` files and rule JSON files exported from upstream ProxyBridge (macOS/Windows/Linux, including `MacOS/tests/pbtest.pbprofile`) import and export losslessly
- [x] Rule matcher passes all upstream test vectors from `MacOS/tests/logic/matcher_tests.swift` and `Windows/tests/test_rules_match.c`
- [x] DNS parser and IP->Domain cache pass all upstream test vectors from `MacOS/tests/logic/dnsmap_tests.swift` and `Windows/tests/test_dns.c`
- [x] TCP (`SOCKS5` / `HTTP` CONNECT) and UDP (`SOCKS5` UDP ASSOCIATE) relays forward traffic, enforce per-rule `LocalhostViaProxy` and per-proxy `SendDomainToProxy`, and report live byte counters
- [x] Wails GUI renders Connections, Live Connections, Proxy Rules, Proxy Servers, Proxy Checker, and Profile switcher, verified in `frontend/preview.html` and `npm test`

<!-- slug: proxybridge-go-port -->
