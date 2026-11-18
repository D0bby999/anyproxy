#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Smoke test for the macOS Network Extension path - the code that only runs in
# a signed, entitled build and therefore never runs under `make dev`.
#
# It checks, in order:
#   1. the app really selected backend=macos-ne (not the macos-sysproxy fallback)
#   2. traffic through 127.0.0.1:10800 exits with a different ip than direct
#   3. that stays true after quick stop/start cycles, which is where v1.0.9
#      came back with an empty config and silently proxied nothing
#
# Usage: ./scripts/ne-smoke.sh [path/to/AnyProxy.app]
# Requires an already-built entitled app: make build-ne APP_PROVISION_PROFILE=... EXT_PROVISION_PROFILE=...
set -uo pipefail

APP="${1:-build/bin/AnyProxy.app}"
BIN="$APP/Contents/MacOS/AnyProxy"
PORT="${ANYPROXY_PORT:-10800}"
IP_URL="${ANYPROXY_IP_URL:-https://api.ipify.org}"
ROUNDS="${ANYPROXY_SMOKE_ROUNDS:-3}"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
info() { printf '  %s\n' "$*"; }

[[ -x "$BIN" ]] || fail "no executable at $BIN (build it with: make build-ne)"

direct=$(curl -fsS --max-time 10 "$IP_URL") || fail "could not read the direct exit ip"
info "direct exit ip: $direct"

engine_pid=""
cleanup() {
  if [[ -n "$engine_pid" ]] && kill -0 "$engine_pid" 2>/dev/null; then
    kill -TERM "$engine_pid" 2>/dev/null || true
    wait "$engine_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

wait_for_port() {
  for _ in $(seq 1 60); do
    if nc -z 127.0.0.1 "$PORT" 2>/dev/null; then return 0; fi
    sleep 0.25
  done
  return 1
}

start_engine() {
  ANYPROXY_DRIVER=ne "$BIN" --verbose 0 &
  engine_pid=$!
  wait_for_port || fail "inbound proxy never listened on 127.0.0.1:$PORT"
}

stop_engine() {
  kill -TERM "$engine_pid" 2>/dev/null || true
  for _ in $(seq 1 40); do
    kill -0 "$engine_pid" 2>/dev/null || { engine_pid=""; return 0; }
    sleep 0.25
  done
  kill -9 "$engine_pid" 2>/dev/null || true
  engine_pid=""
  fail "the engine did not stop within 10s"
}

check_round() {
  local label="$1" got
  got=$(curl -fsS --max-time 15 -x "socks5h://127.0.0.1:$PORT" "$IP_URL") \
    || fail "$label: request through the proxy failed"
  info "$label: exit ip $got"
  [[ "$got" != "$direct" ]] \
    || fail "$label: exit ip is still the direct ip ($direct), the proxy is not applied"
}

if nc -z 127.0.0.1 "$PORT" 2>/dev/null; then
  fail "something is already listening on 127.0.0.1:$PORT - quit the running AnyProxy (and its tunnel) first"
fi

info "starting the engine with ANYPROXY_DRIVER=ne"
start_engine

status=$(ANYPROXY_DRIVER=ne "$BIN" --status) || fail "--status failed"
info "status: $status"
case "$status" in
  *backend=macos-ne*) ;;
  *) fail "backend is not macos-ne ($status) - this run is NOT exercising the Network Extension" ;;
esac

check_round "first start"

for round in $(seq 1 "$ROUNDS"); do
  info "restart cycle $round/$ROUNDS"
  stop_engine
  start_engine
  check_round "after restart $round"
done

printf 'PASS: Network Extension path serves traffic through the proxy across %d restarts\n' "$ROUNDS"
