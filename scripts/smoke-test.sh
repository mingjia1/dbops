#!/bin/bash
set -euo pipefail

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
JWT_TOKEN="${JWT_TOKEN:-}"
AGENT_TOKEN="${AGENT_TOKEN:-}"

log() {
  echo "[smoke-test] $*"
}

fail() {
  log "FAIL: $*"
  exit 1
}

pass() {
  log "PASS: $*"
}

check_http_ok() {
  local label="$1"
  local url="$2"
  local expected="${3:-200}"
  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" "$url" || true)
  if [ "$status" = "$expected" ]; then
    pass "$label -> $status"
  else
    fail "$label -> $status (expected $expected)"
  fi
}

check_json_field() {
  local label="$1"
  local url="$2"
  local field="$3"
  local expected="$4"
  local value
  value=$(curl -s "$url" | jq -r "$field // empty" 2>/dev/null || true)
  if [ "$value" = "$expected" ]; then
    pass "$label = $value"
  else
    fail "$label = $value (expected $expected)"
  fi
}

log "Running post-upgrade smoke tests against $BACKEND_URL"

check_http_ok "Backend health" "$BACKEND_URL/health/ready" 200
check_http_ok "Backend liveness" "$BACKEND_URL/health/live" 200
check_http_ok "Metrics endpoint" "$BACKEND_URL/metrics" 200
check_json_field "Backend status" "$BACKEND_URL/health/ready" ".data.status" "ready"

if [ -n "$JWT_TOKEN" ]; then
  check_http_ok "Hosts list" "$BACKEND_URL/api/v1/hosts" 200
  check_http_ok "Instances list" "$BACKEND_URL/api/v1/instances" 200
else
  log "SKIP: JWT_TOKEN not set, skipping authenticated API checks"
fi

if [ -n "$AGENT_TOKEN" ]; then
  log "SKIP: Agent smoke test requires host/agent details; run platform-health page instead"
fi

log "All smoke tests passed"
