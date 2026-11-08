---
title: "Phase 5: Windows Driver: Pure Go WinDivert Engine & Windows Service"
status: completed
---

# Phase 5: Windows Driver: Pure Go WinDivert Engine & Windows Service

## Context Links

- Upstream Windows engine & service:
  - `/tmp/ProxyBridge/Windows/src/ProxyBridge.c` (WinDivert packet loop, IPv4/IPv6 TCP & UDP rewrite)
  - `/tmp/ProxyBridge/Windows/src/pb_conntrack.c` & `pb_sockmap.c` (connection tracking & reverse UDP index)
  - `/tmp/ProxyBridge/Windows/src/pb_process.c` (`GetExtendedTcpTable` / `GetExtendedUdpTable`, PID -> image name cache)
  - `/tmp/ProxyBridge/Windows/src/pb_relay.c` (local TCP/UDP relay on `34010` / `34011`)
  - `/tmp/ProxyBridge/Windows/src/service/pb_service.c` & `pb_proto.h` (`LocalSystem` service + Named Pipe IPC)
- Upstream Windows tests:
  - `/tmp/ProxyBridge/Windows/tests/test_conntrack.c`, `test_flows.c`, `test_proto.c`, `test_engine.c`

## Overview

- **Priority**: P1
- **Status**: Completed
- **Description**: Implement the Windows transparent packet interception engine and `LocalSystem` Windows Service in 100% pure Go (`CGO_ENABLED=0`) under `internal/driver/windows`, using `golang.org/x/sys/windows` (`LazyDLL` for `WinDivert.dll` and `iphlpapi.dll`) and `golang.org/x/sys/windows/svc`.

## Key Insights

- Upstream Windows ProxyBridge uses `WinDivert.dll` (`WinDivertOpen`, `WinDivertRecv`, `WinDivertSend`, `WinDivertShutdown`, `WinDivertClose`) to capture outbound TCP and UDP packets (`IPv4` + `IPv6`), resolves the owning PID via `GetExtendedTcpTable` / `GetExtendedUdpTable` + `QueryFullProcessImageNameW`, evaluates rules (`internal/rules`), and rewrites `PROXY`-matched packets to the local Go TCP/UDP relay (`34010` / `34011`).
- Three subtle upstream bugs fixed in `ProxyBridge.c` MUST be preserved in our Go port:
  1. **Per-Source-Port Decision Cache + SYN Reset** (`reset_stale_tcp_state`): Cache `(src_port) -> DIRECT | TRACKED` in an atomic bitmap so a 300 Mbps download (17,000 pkt/s) does not call `GetExtendedTcpTable` on every ACK/data packet; clear any cached state when a fresh outbound `SYN` (no `ACK`) arrives on a reused ephemeral port.
  2. **Proxy Endpoint Loop Exclusion** (`is_proxy_server_endpoint` / `is_proxy_udp_endpoint`): Never redirect packets whose destination is one of our configured upstream proxy TCP or UDP relay endpoints, even if their ephemeral source port collides with a stale entry.
  3. **Reverse UDP Connection Index**: Key inbound UDP relay responses by `(client_src_port)` and `(orig_dst_ip, orig_dst_port)` in O(1) so high-rate games/QUIC streams do not scan the connection table linearly.
- Like upstream `pb_service.c`, when installed as a Windows Service (`AnyProxyXaSvc`), the service runs as `LocalSystem` (holding the `WinDivert` handle) and exposes a Named Pipe (`\\.\pipe\AnyProxyXaCtl`) restricted via SDDL ACL to `SYSTEM`, `Administrators`, and the installing user SID, so the Wails GUI runs as a normal unprivileged user without UAC prompts on every launch. If the service is not installed and the user runs as Admin, the engine runs in-process directly.

## Requirements

- [x] Implement `internal/driver/windows/packet.go` (platform-independent IPv4/IPv6 + TCP/UDP packet header parser, checksum recalculator, and NAT address/port rewriter so unit tests run on macOS and Windows!).
- [x] Implement `internal/driver/windows/conntrack.go` (forward + reverse O(1) connection table, atomic 65536-port decision bitmap, SYN reset, 30s UDP flow validation).
- [x] Implement `internal/driver/windows/proc_windows.go` (`GetExtendedTcpTable`, `GetExtendedUdpTable`, `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)`, `QueryFullProcessImageNameW`, and bounded PID cache).
- [x] Implement `internal/driver/windows/windivert_windows.go` (`LazyDLL("WinDivert.dll")` bindings, packet worker loop, DNS UDP/53 response tap feeding `internal/dnsmap`, and integration with `internal/netproxy` TCP/UDP relays on ports `34010`/`34011`).
- [x] Implement `internal/driver/windows/service_windows.go` (`golang.org/x/sys/windows/svc` service + SDDL-secured Named Pipe IPC client/server).

## Architecture

```
internal/driver/windows/
├── packet.go            # Pure Go IPv4/IPv6 + TCP/UDP parser, rewriter & RFC 1071 checksums (tested on all OS)
├── conntrack.go         # O(1) forward/reverse connection tracking & 64k port decision bitmap (tested on all OS)
├── proc_windows.go      # iphlpapi GetExtendedTcp/UdpTable + PID->exe cache (windows build tag)
├── windivert_windows.go # WinDivert.dll LazyDLL packet loop + local TCP/UDP relay hookup (windows build tag)
├── service_windows.go   # Windows Service (windows/svc) + SDDL Named Pipe IPC (windows build tag)
└── packet_test.go       # Unit tests for IPv4/IPv6 TCP/UDP NAT rewrite, checksums, SYN reset, conntrack
```

## Related Code Files

- **Create**:
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows/packet.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows/conntrack.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows/packet_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows/proc_windows.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows/windivert_windows.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows/service_windows.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/driver/windows.go`

## Implementation Steps

1. Implement `packet.go` and `conntrack.go` without OS build tags so IPv4/IPv6 TCP/UDP packet rewriting, RFC 1071 checksum computation, SYN stale-port clearing, and proxy-endpoint loop prevention can be unit-tested on both macOS and Windows.
2. Implement `proc_windows.go` using `iphlpapi.dll` (`GetExtendedTcpTable` with `TCP_TABLE_OWNER_PID_ALL`, `GetExtendedUdpTable` with `UDP_TABLE_OWNER_PID`) and reusable buffers (`g_pidtbl_buf` pattern).
3. Implement `windivert_windows.go` binding `WinDivertOpen`, `WinDivertRecv`, `WinDivertSend`, `WinDivertShutdown`, and `WinDivertClose`, plus the local TCP (`127.0.0.1:34010`) and UDP (`0.0.0.0:34011` / `[::]:34011`) relay servers backed by `internal/netproxy` and `internal/flows`.
4. Implement `service_windows.go` so `AnyProxyXa.exe --service` runs as a `LocalSystem` Windows Service serving the Wails GUI over Named Pipe IPC, with automatic in-process fallback when run directly as Administrator.
5. Verify cross-compilation and `go vet` from macOS (`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...`) and run `packet_test.go`.

## Todo

- [x] Implement `packet.go` (IPv4/IPv6 TCP/UDP header parsing, NAT rewriting, RFC 1071 checksums)
- [x] Implement `conntrack.go` (forward/reverse maps, 64k port decision bitmap, SYN reset)
- [x] Implement `proc_windows.go` (`iphlpapi.dll` socket-to-PID resolver + PID cache)
- [x] Implement `windivert_windows.go` (`WinDivert.dll` packet loop + TCP/UDP relay integration)
- [x] Implement `service_windows.go` (`LocalSystem` service + Named Pipe IPC)
- [x] Pass `go test ./internal/driver/windows/...` and `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...`

## Success Criteria

- `go test -race ./internal/driver/windows/... -count=1` passes all conntrack, SYN-reset, and IPv4/IPv6 TCP/UDP packet rewrite + checksum unit tests.
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` compiles cleanly with zero CGO dependencies.

## Risk Assessment & Security Considerations

- **DLL Hijacking Prevention**: Load `WinDivert.dll` by absolute path from the executable's own directory (`filepath.Join(exeDir, "WinDivert.dll")`) and `iphlpapi.dll` / `kernel32.dll` via `windows.NewLazySystemDLL` (`LOAD_LIBRARY_SEARCH_System32`).
- **Named Pipe ACL**: Restrict the Windows Service pipe with SDDL + `PIPE_REJECT_REMOTE_CLIENTS` so network logons and unauthorized local users cannot control proxy routing.
