---
title: "Phase 1: Foundation, Storage, Vault & .pbprofile Compatibility"
status: completed
---

# Phase 1: Foundation, Storage, Vault & .pbprofile Compatibility

## Context Links

- Local reference: [`anyprofile/app.go`](file:///Users/dobby/Heroin/anyprofile/app.go), [`anyprofile/go.mod`](file:///Users/dobby/Heroin/anyprofile/go.mod), [`anyprofile/internal/paths`](file:///Users/dobby/Heroin/anyprofile/internal/paths), [`anyprofile/internal/secrets`](file:///Users/dobby/Heroin/anyprofile/internal/secrets), [`anyprofile/internal/store`](file:///Users/dobby/Heroin/anyprofile/internal/store)
- Upstream profile specs: `/tmp/ProxyBridge/MacOS/ProxyBridge/ProxyBridge/CLIProfile.swift`, `/tmp/ProxyBridge/Windows/gui/profile/profile.c`, `/tmp/ProxyBridge/Linux/profile/pb_profile.c`, `/tmp/ProxyBridge/MacOS/tests/pbtest.pbprofile`

## Overview

- **Priority**: P1
- **Status**: Completed
- **Description**: Bootstrap the `github.com/D0bby999/anyproxyxa` Go module, directory conventions (`0700` data dir, `0600` DB and vault), encrypted secret storage (`secrets.Vault` backed by macOS Keychain / Windows Credential Manager), SQLite schema (`modernc.org/sqlite`), and 100% lossless `.pbprofile` + rules JSON import/export.

## Key Insights

- Upstream ProxyBridge uses `.pbprofile` JSON files across Windows, macOS, and Linux, plus standalone rule-array JSON exports (`[ { "action": "DIRECT", ... } ]`).
- Upstream `.pbprofile` has slight casing/field variations across platforms (e.g., `ProxyConfigs[].Id` int vs UUID string on macOS, `TargetHosts` + `TargetDomains` split on Windows/Linux vs combined `;`-separated `targetHosts` on macOS, top-level legacy `LocalhostViaProxy` fallback). Our `internal/pbprofile` parser must accept all upstream variants and normalize them cleanly.
- Proxy passwords must never be stored in plaintext in SQLite or logs; they live in `secrets.Vault` (`secrets.vault` encrypted with AES-GCM, key in OS Keychain/CredMan), and are only written to `.pbprofile` when explicitly exported by the user (with `0600` file permissions).

## Requirements

- [x] Initialize `go.mod` (`github.com/D0bby999/anyproxyxa`), `wails.json`, `Makefile`, `LICENSE`, and `NOTICE` (with MIT attribution to `InterceptSuite/ProxyBridge`).
- [x] Implement `internal/paths` (`~/Library/Application Support/AnyProxyXa` on macOS, `%APPDATA%\AnyProxyXa` on Windows, overridable via `ANYPROXY_DATA_DIR` with absolute-path check).
- [x] Port `internal/secrets` (`Vault` + macOS Keychain / Windows Credential Manager) from `anyprofile`.
- [x] Implement `internal/store` (`modernc.org/sqlite`) with tables for `profiles`, `proxy_configs`, `proxy_rules`, `log_filters`, and `settings`.
- [x] Implement `internal/pbprofile` supporting full `.pbprofile` import/export (up to 4 MiB, 256 proxies, 4096 rules, control-char rejection) and standalone Rules JSON import/export.

## Architecture

```
internal/
├── paths/          # Data, DB, Vault, Logs, Profiles directories (0700/0600)
├── secrets/        # AES-GCM secrets.vault + Keychain (darwin) / CredMan (windows)
├── store/          # SQLite persistence for profiles, proxies, ordered rules, log filters
└── pbprofile/      # Lossless parser/serializer for .pbprofile and rules.json
```

## Related Code Files

- **Create**:
  - `/Users/dobby/Heroin/anyproxyxa/go.mod`
  - `/Users/dobby/Heroin/anyproxyxa/wails.json`
  - `/Users/dobby/Heroin/anyproxyxa/Makefile`
  - `/Users/dobby/Heroin/anyproxyxa/NOTICE`
  - `/Users/dobby/Heroin/anyproxyxa/umask_darwin.go`
  - `/Users/dobby/Heroin/anyproxyxa/umask_windows.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/paths/paths.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/secrets/vault.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/secrets/keychain_darwin.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/secrets/keychain_windows.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/store/store.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/store/store_test.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/pbprofile/pbprofile.go`
  - `/Users/dobby/Heroin/anyproxyxa/internal/pbprofile/pbprofile_test.go`

## Implementation Steps

1. Create `go.mod`, `wails.json`, `Makefile`, `NOTICE`, and `umask_*.go` following `anyprofile`.
2. Implement `internal/paths` and `internal/secrets` so all proxy passwords are keyed by `proxy:<uuid>` in `secrets.Vault`.
3. Create `internal/store` with SQLite migrations for:
   - `profiles` (`id`, `name` unique, `is_active`, `traffic_logging_enabled`, `dns_via_proxy`, `log_every_connection`, `auto_clear_logs`, `default_proxy_id`, `font_zoom`)
   - `proxy_configs` (`id`, `profile_id`, `legacy_int_id`, `name`, `type` [`SOCKS5`|`HTTP`], `host`, `port`, `username`, `send_domain_to_proxy`, `position`)
   - `proxy_rules` (`id`, `profile_id`, `name`, `process_names`, `target_hosts`, `target_domains`, `target_ports`, `protocol` [`TCP`|`UDP`|`BOTH`], `action` [`PROXY`|`DIRECT`|`BLOCK`], `proxy_config_id`, `enabled`, `localhost_via_proxy`, `position`)
   - `log_filters` (`id`, `profile_id`, `mode` [`INCLUDE`|`EXCLUDE`], `process`, `host`, `port`, `protocol`, `action`, `position`)
4. Implement `internal/pbprofile`:
   - `ParseProfile(r io.Reader)` & `MarshalProfile(p Profile)`: support Windows/Linux/macOS `.pbprofile` schemas, merging `TargetHosts` and `TargetDomains` seamlessly while preserving `TargetDomains` on export, handling top-level `LocalhostViaProxy` fallback, and enforcing upstream limits (max 4 MiB, 256 proxies, 4096 rules, no ASCII control characters `< 0x20` except `\t\r\n`).
   - `ParseRulesJSON(r io.Reader)` & `MarshalRulesJSON(rules []Rule)`: support macOS/Windows rule-array export format.
5. Write unit tests in `internal/store/store_test.go` and `internal/pbprofile/pbprofile_test.go` testing round-trip import/export with `pbtest.pbprofile` and edge cases.

## Todo

- [x] Scaffold `go.mod`, `wails.json`, `Makefile`, `NOTICE`, and `umask_*.go`
- [x] Implement `internal/paths` and `internal/secrets`
- [x] Implement `internal/store` SQLite schema and CRUD methods for profiles, proxies, rules, and log filters
- [x] Implement `internal/pbprofile` `.pbprofile` and rules JSON import/export with validation
- [x] Pass `go test ./internal/paths/... ./internal/secrets/... ./internal/store/... ./internal/pbprofile/... -count=1`

## Success Criteria

- `go test ./internal/... -count=1` passes.
- Importing `/tmp/ProxyBridge/MacOS/tests/pbtest.pbprofile` and Windows/Linux `.pbprofile` fixtures into SQLite and re-exporting preserves all proxy configs, rule order, `SendDomainToProxy`, `LocalhostViaProxy`, and `TargetDomains`.

## Risk Assessment & Security Considerations

- **Secret Protection**: Proxy passwords are stripped from `store` structs at rest and stored in `secrets.Vault`. Exported `.pbprofile` files are written with `0600` permissions.
- **Input Sanitization**: Reject `.pbprofile` inputs > 4 MiB or containing control characters in process/host/domain names to prevent log/terminal injection (matching upstream Linux security guarantee).
