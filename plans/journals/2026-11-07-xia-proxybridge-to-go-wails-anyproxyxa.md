---
title: Xia ProxyBridge to Go + Wails (AnyProxyXa)
date: 2026-11-07
summary: Complete 6-phase Go + Wails port of InterceptSuite/ProxyBridge aligned with AnyProfile architecture
---

# Xia ProxyBridge to Go + Wails (AnyProxyXa)

Complete 6-phase Go + Wails port of [InterceptSuite/ProxyBridge](https://github.com/InterceptSuite/ProxyBridge) (`cb897b017a88a142ec803c3af138b0ad75c2ae12`, MIT License) into the AnyTools desktop ecosystem at `github.com/D0bby999/anyproxyxa`.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.

## Completed Milestones

1. **Phase 1: Project Skeleton, Persistence & Vault**
   - Configured Go 1.24 module, `wails.json`, `Makefile`, and `NOTICE` MIT attributions.
   - Owner-only paths (`0700`/`0600`) via `internal/paths`.
   - Encrypted secrets vault with OS Keychain & Credential Manager backend via `internal/secrets`.
   - 100% lossless `.pbprofile` JSON codec via `internal/pbprofile`.
   - Modern pure-Go SQLite store (`modernc.org/sqlite`) via `internal/store`.

2. **Phase 2: DNS Cache & Precompiled Rule Engine**
   - Pure-Go RFC 1035 DNS packet parser, `/etc/hosts` loader, and 32k-entry LRU domain map via `internal/dnsmap`.
   - Priority-ordered rule engine with process globs, IP/CIDR/wildcard matchers, domain globs, port ranges, and loopback auto-direct bypass via `internal/rules`.

3. **Phase 3: Upstream Client, SOCKS5 UDP Relay & Flow Tracker**
   - Universal upstream client supporting SOCKS5, SOCKS4, HTTP CONNECT, and HTTPS with remote DNS (`socks5h`) via `internal/netproxy`.
   - Full RFC 1928 `UDP ASSOCIATE` client and local UDP forwarder via `internal/netproxy/udp.go`.
   - Interactive 5-step proxy diagnostic checker via `internal/netproxy/checker.go`.
   - 400-entry thread-safe live flow tracker and deduplicated connection logger via `internal/flows`.

4. **Phase 4: macOS NetworkExtension Driver & Diagnostics**
   - Driver abstraction interface and loopback/mock driver via `internal/driver`.
   - Swift NetworkExtension System Extension (`macos/extension/`) combining `NETransparentProxyProvider` and `NEDNSProxyProvider`.
   - Darwin control IPC, diagnostics, and loopback fallback via `internal/driver/darwin.go`.

5. **Phase 5: Windows WinDivert Driver & Service**
   - Pure-Go `internal/driver/windows` using `golang.org/x/sys/windows` LazyDLL for `WinDivert.dll` and `iphlpapi.dll`.
   - IPv4/IPv6 TCP/UDP raw packet decoder and bidirectional NAT/conntrack table.
   - Windows Service (`windows/svc`) implementation for elevated background interception.
   - Cross-compilation verified with `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...`.

6. **Phase 6: Service Coordinator, Headless CLI & Wails React GUI**
   - Application coordinator `internal/service/service.go` managing store, vault, rules, DNS, and drivers.
   - Dual-mode CLI `internal/cli/cli.go` supporting `--profile`, `--list-profiles`, `--verbose 0..3`, `--test-proxy`, and graceful signal handling (`SIGINT`/`SIGTERM`).
   - Wails v2 desktop entrypoint (`main.go`, `app.go`, `app_bindings.go`, `menu_darwin.go`, `menu_windows.go`).
   - React 19 + TypeScript frontend following the dealt `framed-sheet` design direction (`IBM Plex Sans` + `IBM Plex Mono`, responsive layout, tabular numerical alignment).
   - Real-browser preview harness `frontend/preview.html` verified via `design-taste-frontend/scripts/inspect.mjs --app` and `impeccable detect`.
   - Automated tests: 100% pass across Go (`go test -race ./...`) and Frontend (`npm test`).
