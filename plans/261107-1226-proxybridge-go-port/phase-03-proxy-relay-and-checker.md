---
title: "Phase 3: TCP/UDP Proxy Relay, Flow Tracker & 5-Step Proxy Checker"
status: completed
---

# Phase 3: TCP/UDP Proxy Relay, Flow Tracker & 5-Step Proxy Checker

## Context Links

- Local reference: [`anyprofile/internal/netproxy/upstream.go`](file:///Users/dobby/Heroin/anyprofile/internal/netproxy/upstream.go), [`socks.go`](file:///Users/dobby/Heroin/anyprofile/internal/netproxy/socks.go), [`relay.go`](file:///Users/dobby/Heroin/anyprofile/internal/netproxy/relay.go), [`pipe.go`](file:///Users/dobby/Heroin/anyprofile/internal/netproxy/pipe.go)
- Upstream references:
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/AppProxyProvider.swift` (SOCKS5 TCP/UDP & HTTP CONNECT relay, lines 970–1600)
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/ProxyBridge/ProxyCheckerView.swift`
  - `/tmp/ProxyBridge/Windows/src/pb_socks5.c`, `pb_http.c`, `pb_relay.c`, `pb_proxy.c`
  - `/tmp/ProxyBridge/Linux/src/pb_relay.c`

## Overview

- **Priority**: P1
- **Status**: Completed
- **Description**: Build the Go upstream proxy dialer & local TCP/UDP relay (`internal/netproxy`), the 5-step diagnostic Proxy Checker (`TestProxyEx`), and the Live Connections + Connection/Activity Log tracker (`internal/flows`).

## Key Insights

- `anyprofile/internal/netproxy` already provides clean `Upstream`, `bufConn`, `pipe`, and `DialTunnel` primitives, but refuses `UDP ASSOCIATE` by design. For `anyproxyxa`, we need full RFC 1928 `SOCKS5` `UDP ASSOCIATE` support!
- In `SOCKS5` `UDP ASSOCIATE`:
  - Client opens a TCP control connection to the SOCKS5 proxy, authenticates (none `0x00` or user/pass `0x02`), and sends `CMD = 0x03 (UDP ASSOCIATE)` with `0.0.0.0:0`.
  - Server replies with `BND.ADDR` and `BND.PORT`. If `BND.ADDR` is `0.0.0.0`, `::`, or empty, reuse the TCP control connection's proxy host IP.
  - Each UDP datagram sent to the relay port is prefixed with `[RSV=0x00, 0x00][FRAG=0x00][ATYP][DST.ADDR][DST.PORT][DATA]`. Replies carry the same header, which is stripped before handing the payload back to the application.
  - The TCP control connection must be monitored; when it closes or errors, the UDP association is torn down (or reconnected on Windows/Linux). Idle UDP associations are reaped after 120 seconds.
- When a rule routes UDP to an `HTTP` proxy, HTTP proxies cannot carry raw UDP: upstream ProxyBridge logs a one-time warning (`warnOnce`) and sends that UDP flow `DIRECT`.
- When a rule routes TCP/UDP to a proxy ID that no longer exists (or fails handshake): TCP connection handshake failure closes/resets the connection rather than silently leaking via direct IP.
- Stalled proxies must not starve other proxies: cap concurrent in-flight handshakes to 48 per upstream proxy (20s handshake timeout).

## Requirements

- [x] Implement `internal/netproxy`:
  - `Upstream` with `ID`, `Name`, `Type` (`SOCKS5` | `HTTP`), `Host`, `Port`, `Username`, `Password`, `SendDomainToProxy`.
  - `DialTCP(ctx, destIP, destDomain, destPort)`: uses `destDomain` when `SendDomainToProxy && destDomain != ""` (`ATYP=0x03` for SOCKS5, `CONNECT domain:port` for HTTP), otherwise `destIP`. Preserves any trailing bytes after HTTP `200` or SOCKS5 reply via `bufConn`.
  - `UDPAssociation`: establishes and maintains `SOCKS5` `UDP ASSOCIATE`, caches 64 destination headers per session, tracks `sent`/`received` bytes, monitors the TCP control socket, and reaps idle associations.
  - `CheckProxyEx(ctx, up, targetHost, targetPort, emit func(StepResult))`: 5-step proxy checker matching upstream ProxyBridge:
    1. TCP reachability to proxy `host:port` (with latency ms).
    2. Proxy tunnel + authentication handshake to `targetHost:targetPort`.
    3. HTTP request/response probe over tunnel (status line / bytes read).
    4. 3x sequential tunnel setup latency measurement (min/avg/max ms).
    5. SOCKS5 `UDP ASSOCIATE` probe (for SOCKS5 proxies; reported as warning if unsupported since many SSH/SOCKS proxies omit UDP).
- [x] Implement `internal/flows`:
  - `Tracker`: thread-safe live flow table (`GetFlows(max int) []Flow` up to 400 active + recently closed flows, with `ID`, `PID`, `ProcessName`, `DestIP`, `Host`, `Port`, `IsUDP`, `IsIPv6`, `ProxyID`, `ProxyLabel`, `RuleID`, `AgeMs`, `Sent`, `Received`, `Closed`).
  - `LogManager`:
    - Connection log ring buffer (up to 2000 entries; auto-clears at 500 when `AutoClearLogs` is on).
    - Deduplication mode (default: one entry per `(process, dest, port, proto, action)`) vs `LogEveryConnection` mode (every new TCP connection and every UDP flow after 30s silence).
    - `LogFilters` evaluator (`EXCLUDE` evaluated first, then `INCLUDE` if any exist; case-insensitive globs on process, IP/host, port, protocol, action).
    - Activity log ring buffer (up to 1000 entries with timestamp, level, message).

## Architecture

```
internal/
├── netproxy/
│   ├── upstream.go     # Upstream config, SOCKS5 & HTTP CONNECT TCP dialer, per-proxy handshake semaphore
│   ├── udp.go          # SOCKS5 UDP ASSOCIATE session, RFC 1928 datagram framing/deframing, idle reaper
│   ├── checker.go      # 5-step streaming Proxy Checker (TCP, Auth/Tunnel, HTTP probe, 3x Latency, UDP probe)
│   ├── pipe.go         # Bidirectional TCP copy with live atomic byte counters (sent/received)
│   └── netproxy_test.go# In-process fake SOCKS5 (TCP+UDP) & HTTP proxies + checker tests
└── flows/
    ├── tracker.go      # Live flow table (active + greyed closed flows, byte counters)
    ├── logs.go         # Connection & Activity log ring buffers, dedup, 30s UDP window, LogFilters
    └── flows_test.go   # Unit tests for flow lifecycle, dedup, and include/exclude filters
```

## Related Code Files

- **Create**:
  - `/Users/dobby/Heroin/anyproxyxa/internal/netproxy/upstream.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/netproxy/udp.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/netproxy/checker.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/netproxy/pipe.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/netproxy/netproxy_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/flows/tracker.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/flows/logs.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/flows/flows_test.go`

## Implementation Steps

1. Implement `internal/netproxy/upstream.go` and `pipe.go` with atomic byte counter callbacks (`onSent`, `onRecv`) so live connections update in real time.
2. Implement `internal/netproxy/udp.go` for RFC 1928 `SOCKS5` `UDP ASSOCIATE`, including exact-length reply reading (`ATYP` `0x01`, `0x03`, `0x04`), `0.0.0.0`/`::` bind-address substitution, UDP header cache, and TCP control channel disconnect watcher.
3. Implement `internal/netproxy/checker.go` for the 5-step diagnostic check with streaming line callbacks.
4. Implement `internal/flows/tracker.go` and `internal/flows/logs.go` for Live Connections, Connection Log deduplication, and `LogFilters`.
5. Write comprehensive tests in `netproxy_test.go` (using in-process SOCKS5 TCP+UDP and HTTP CONNECT test servers) and `flows_test.go`.

## Todo

- [x] Implement `SOCKS5` and `HTTP` CONNECT TCP dialer with `SendDomainToProxy` and per-upstream semaphore
- [x] Implement `SOCKS5` `UDP ASSOCIATE` client and datagram framing/deframing
- [x] Implement 5-step `CheckProxyEx` diagnostic runner
- [x] Implement `flows.Tracker` and `flows.LogManager` (dedup, 30s UDP window, `LogFilters`, auto-clear)
- [x] Pass `go test -race ./internal/netproxy/... ./internal/flows/... -count=1`

## Success Criteria

- `go test -race ./internal/netproxy/... ./internal/flows/... -count=1` passes against in-process SOCKS5 (TCP + UDP ASSOCIATE, with and without auth) and HTTP CONNECT servers.
- Live flow byte counters accurately match payload bytes sent and received.

## Risk Assessment & Security Considerations

- **No Silent Leak on Handshake Failure**: If a `PROXY` rule matches and the proxy refuses connection or authentication fails, the connection is closed with an error — never fallen back to direct.
- **Server-First Protocol Safety**: `readSOCKS5Reply` and HTTP CONNECT header reader read only the exact proxy header and preserve any buffered server banner bytes via `bufConn`.
