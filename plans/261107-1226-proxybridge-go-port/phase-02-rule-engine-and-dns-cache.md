---
title: "Phase 2: Rule Engine, Matcher & DNS Snoop Cache"
status: todo
---

# Phase 2: Rule Engine, Matcher & DNS Snoop Cache

## Context Links

- Upstream rule matchers:
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/AppProxyProvider.swift` (`ProxyRule.Matcher`, lines 10–385)
  - `/tmp/ProxyBridge/Windows/src/pb_rules.c` & `/tmp/ProxyBridge/Linux/src/pb_rules.c`
- Upstream DNS snoopers:
  - `/tmp/ProxyBridge/MacOS/ProxyBridge/extension/DNSProxyProvider.swift` & `DNSMapStore.swift`
  - `/tmp/ProxyBridge/Windows/src/pb_dns.c` & `/tmp/ProxyBridge/Linux/src/pb_dns.c`
- Upstream test suites:
  - `/tmp/ProxyBridge/MacOS/tests/logic/matcher_tests.swift`
  - `/tmp/ProxyBridge/MacOS/tests/logic/dnsmap_tests.swift`
  - `/tmp/ProxyBridge/Windows/tests/test_rules_match.c`, `test_dns.c`, `test_hosts.c`

## Overview

- **Priority**: P1
- **Status**: Todo
- **Description**: Build the pure-Go pre-compiled Rule Engine (`internal/rules`) and DNS Snoop / IP-to-Domain Cache (`internal/dnsmap`) with 100% behavioral parity with upstream ProxyBridge across macOS, Windows, and Linux.

## Key Insights

- Rules are evaluated on every new TCP connection and UDP flow in list order (**first match wins**). Pre-compiling rules into a `CompiledMatcher` when rules are loaded avoids string splitting, lowercasing, and CIDR/range parsing on the hot path.
- `*.example.com` in domain patterns must also match the apex domain `example.com` (upstream behavior in `AppProxyProvider.swift` line 80: `let apex = lowered.hasPrefix("*.") ? String(lowered.dropFirst(2)) : nil`).
- Empty segments in semicolon/comma lists (e.g. `"Safari;"`) must be skipped so a trailing `;` never acts as a wildcard.
- Certain addresses and ports **always** override `PROXY` to `DIRECT` (while still allowing `BLOCK` or `DIRECT` rules):
  - IPv4 Broadcast (`255.255.255.255`, `x.x.x.255` when applicable), Multicast (`224.0.0.0/4`), Link-local/APIPA (`169.254.0.0/16`), Unspecified (`0.0.0.0/8`), DHCP ports (`UDP 67, 68`).
  - IPv6 Multicast (`ff00::/8`), Link-local (`fe80::/10`), Site-local (`fec0::/10`), Unspecified (`::`), DHCPv6 ports (`UDP 546, 547`).
  - Loopback (`127.0.0.0/8`, `::1`, `::ffff:127.0.0.0/104`): bypassed to `DIRECT` for `PROXY` rules **unless** the matched rule has `LocalhostViaProxy == true` (and DNS to a loopback stub on port 53 is never proxied).

## Requirements

- [x] Implement `internal/rules`:
  - Process matching: `;` or `,` separated globs (`*`, `?`) against process executable filename, full path (when pattern contains `/` or `\`), or macOS bundle identifier (`com.apple.Safari`), case-insensitive on macOS/Windows.
  - Host/IP matching: `*`, exact IPv4/IPv6 (normalized bytes), IPv4 octet wildcards (`192.168.*.*`, `10.1-5.*.*`), IPv6 prefix wildcards (`2001:db8:*`), IPv4/IPv6 CIDR (`10.0.0.0/8`, `2001:db8::/32`), IPv4/IPv6 ranges (`10.10.1.1-10.10.255.255`, `2001:db8::1-2001:db8::ff`).
  - Domain matching: glob patterns (`github.com`, `*.github.com`, `api?.example.org`) matched against all domains associated with the destination IP in `dnsmap` (or direct hostname when known), with `*.domain` also matching apex `domain`.
  - Port matching: `;` or `,` separated ports and ranges (`80;443;8000-9000`).
  - Protocol matching (`TCP`, `UDP`, `BOTH`) and Action resolution (`PROXY`, `DIRECT`, `BLOCK`) with mandatory direct-routing overrides and `LocalhostViaProxy`.
- [x] Implement `internal/dnsmap`:
  - RFC 1035 wire-format DNS response parser supporting `A` (type 1), `AAAA` (type 28), and `CNAME` (type 5) chains chained from the question section, with pointer-compression loop protection (max 128 hops) and hostname character validation.
  - `/etc/hosts` (and `C:\Windows\System32\drivers\etc\hosts`) parser at startup.
  - Thread-safe bounded map (32,768 IP entries, oldest-evicted LRU) mapping IP (`netip.Addr`) -> `[]string` (domains, most recent first for `SendDomainToProxy`, all kept for domain rule matching).

## Architecture

```
internal/
├── dnsmap/
│   ├── parser.go       # RFC 1035 DNS response parser (QNAME -> CNAME chain -> A/AAAA)
│   ├── store.go        # 32,768-entry LRU IP -> domain store + hosts file loader
│   └── dnsmap_test.go  # Ported tests from dnsmap_tests.swift & test_dns.c
└── rules/
    ├── matcher.go      # Pre-compiled Rule & Engine (glob, IP/CIDR/range, domain, port)
    ├── bypass.go       # Mandatory direct-routing checks (multicast/link-local/DHCP/loopback)
    └── matcher_test.go # Ported tests from matcher_tests.swift & test_rules_match.c
```

## Related Code Files

- **Create**:
  - `/Users/dobby/Heroin/anyproxyxa/internal/dnsmap/parser.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/dnsmap/store.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/dnsmap/dnsmap_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/rules/matcher.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/rules/bypass.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/rules/matcher_test.go`

## Implementation Steps

1. Implement `internal/dnsmap/parser.go` to parse raw DNS response packets, validate question ownership of `CNAME`/`A`/`AAAA` records, and reject malformed or injection-attempting domain labels.
2. Implement `internal/dnsmap/store.go` with `Record(domain string, ips []netip.Addr)`, `DomainsForIP(ip netip.Addr) []string`, `PrimaryDomain(ip netip.Addr) string`, and `LoadHostsFile(path string)`.
3. Implement `internal/rules/bypass.go` for automatic `DIRECT` overrides (broadcast, multicast, link-local, APIPA, DHCP/DHCPv6, and per-rule loopback logic).
4. Implement `internal/rules/matcher.go` with `CompileRule(Rule) (*CompiledRule, error)` and `Engine.Evaluate(FlowContext) Decision`.
5. Port all test cases from `/tmp/ProxyBridge/MacOS/tests/logic/matcher_tests.swift`, `dnsmap_tests.swift`, and `/tmp/ProxyBridge/Windows/tests/test_rules_match.c`, `test_dns.c`, `test_hosts.c`.

## Todo

- [x] Implement RFC 1035 DNS response parser with CNAME chaining and compression loop protection
- [x] Implement 32k-capacity IP->Domain store and system `hosts` file loader
- [x] Implement pre-compiled rule matcher (process, IP/CIDR/range/wildcard, domain, port, protocol)
- [x] Implement automatic direct-routing bypass rules and `LocalhostViaProxy` handling
- [x] Port and pass all upstream matcher and DNS test suites

## Success Criteria

- `go test -race ./internal/dnsmap/... ./internal/rules/... -count=1` passes 100% of upstream test vectors.
- Benchmark `BenchmarkRuleEvaluate` runs in `< 500 ns/op` with `0 allocs/op` on the pre-compiled path.

## Risk Assessment & Security Considerations

- **DNS Poisoning / Spoofed Answers**: Only learn records in the Answer section whose owner name matches the Question QNAME or a CNAME chained from it (matching upstream Linux `pb_dns.c` security invariant).
- **Host Injection**: Validate all learned domain names (`[a-zA-Z0-9._-]`, length `1..253`, no spaces/CRLF/control chars) before storing or forwarding to an HTTP `CONNECT` or SOCKS5 proxy.
