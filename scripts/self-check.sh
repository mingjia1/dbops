#!/bin/bash
set -euo pipefail

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
JWT_TOKEN="${JWT_TOKEN:-}"
AGENT_TOKEN="${AGENT_TOKEN:-}"
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"
REPORT_DIR="${REPORT_DIR:-/var/log/dbops-platform}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
REPORT_FILE="${REPORT_DIR}/selfcheck-${TIMESTAMP}.txt"

mkdir -p "$REPORT_DIR"

log() {
  echo "[selfcheck] $*"
}

fail() {
  log "FAIL: $*"
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
    return 0
  else
    fail "$label -> $status (expected $expected)"
    return 1
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
    return 0
  else
    fail "$label = $value (expected $expected)"
    return 1
  fi
}

check_disk_space() {
  local path="${1:-/}"
  local min_gb="${2:-10}"
  local usage
  usage=$(df -h "$path" | awk 'NR==2 {print $5}' | sed 's/%//')
  local available
  available=$(df -h "$path" | awk 'NR==2 {print $4}')
  if [ "$usage" -lt 90 ]; then
    pass "Disk space on $path: ${available} available (${usage}% used)"
    return 0
  else
    fail "Disk space on $path: ${available} available (${usage}% used) - LOW SPACE"
    return 1
  fi
}

send_alert() {
  local subject="$1"
  local body="$2"
  if [ -n "$ALERT_WEBHOOK" ]; then
    curl -s -X POST -H 'Content-Type: application/json' \
      -d "{\"text\":\"${subject}\n\n${body}\"}" \
      "$ALERT_WEBHOOK" || true
  fi
  if [ -n "$ALERT_EMAIL" ]; then
    echo "$body" | mail -s "$subject" "$ALERT_EMAIL" 2>/dev/null || true
  fi
}

log "=========================================="
log "DBOps Platform Daily Self-Check"
log "Time: $(date)"
log "Backend: $BACKEND_URL"
log "=========================================="

FAILED=0
WARNINGS=()

# 1. Health endpoints
log "--- Health Checks ---"
check_http_ok "Backend health" "$BACKEND_URL/health/ready" 200 || FAILED=$((FAILED+1))
check_http_ok "Backend liveness" "$BACKEND_URL/health/live" 200 || FAILED=$((FAILED+1))
check_http_ok "Metrics endpoint" "$BACKEND_URL/metrics" 200 || FAILED=$((FAILED+1))

# 2. Fine-grained health
log "--- Dependency Health ---"
if check_http_ok "Redis health" "$BACKEND_URL/health/redis" 200; then
  :
else
  WARNINGS+=("Redis may be down or disabled")
fi
if check_http_ok "ClickHouse health" "$BACKEND_URL/health/clickhouse" 200; then
  :
else
  WARNINGS+=("ClickHouse may be down or disabled")
fi
check_http_ok "Agents health" "$BACKEND_URL/health/agents" 200 || FAILED=$((FAILED+1))

# 3. API availability
log "--- API Checks ---"
if [ -n "$JWT_TOKEN" ]; then
  check_http_ok "Hosts API" "$BACKEND_URL/api/v1/hosts" 200 || FAILED=$((FAILED+1))
  check_http_ok "Instances API" "$BACKEND_URL/api/v1/instances" 200 || FAILED=$((FAILED+1))
  check_http_ok "Tasks API" "$BACKEND_URL/api/v1/tasks" 200 || FAILED=$((FAILED+1))
else
  log "SKIP: JWT_TOKEN not set, skipping authenticated API checks"
fi

# 4. Disk space
log "--- Disk Space ---"
check_disk_space "/" 10 || FAILED=$((FAILED+1))
check_disk_space "/tmp" 2 || WARNINGS+=("Low /tmp space")

# 5. Failed tasks check
log "--- Recent Tasks ---"
if [ -n "$JWT_TOKEN" ]; then
  TASKS_RESPONSE=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" "$BACKEND_URL/api/v1/tasks?limit=20" 2>/dev/null || echo '{"data":[]}')
  FAILED_TASKS=$(echo "$TASKS_RESPONSE" | jq -r '.data[] | select(.status=="failed" or .status=="error") | .id' 2>/dev/null || true)
  if [ -n "$FAILED_TASKS" ]; then
    FAILED_COUNT=$(echo "$FAILED_TASKS" | wc -l)
    fail "Found $FAILED_COUNT failed tasks in recent history"
    WARNINGS+=("Failed tasks: $(echo "$FAILED_TASKS" | head -5 | tr '\n' ' ')")
  else
    pass "No failed tasks in recent history"
  fi
fi

# 6. Summary
log "=========================================="
log "Self-Check Summary"
log "=========================================="
if [ "$FAILED" -eq 0 ]; then
  pass "All critical checks passed"
else
  fail "$FAILED critical check(s) failed"
fi

if [ ${#WARNINGS[@]} -gt 0 ]; then
  log "Warnings:"
  for warning in "${WARNINGS[@]}"; do
    log "  - $warning"
  done
fi

# 7. Generate report
cat > "$REPORT_FILE" <<EOF
DBOps Platform Self-Check Report
==================================
Time: $(date)
Backend: $BACKEND_URL

Critical Failures: $FAILED
Warnings: ${#WARNINGS[@]}

EOF
if [ ${#WARNINGS[@]} -gt 0 ]; then
  echo "Warnings:" >> "$REPORT_FILE"
  for warning in "${WARNINGS[@]}"; do
    echo "  - $warning" >> "$REPORT_FILE"
  done
  echo "" >> "$REPORT_FILE"
fi
echo "Recent failed tasks: ${FAILED_TASKS:-none}" >> "$REPORT_FILE"
log "Report saved to $REPORT_FILE"

# 8. Send alerts if needed
if [ "$FAILED" -gt 0 ] || [ ${#WARNINGS[@]} -gt 0 ]; then
  ALERT_BODY="DBOps Platform Self-Check Report\n\nTime: $(date)\nCritical Failures: $FAILED\nWarnings: ${#WARNINGS[@]}\n\n"
  ALERT_BODY+="Recent failed tasks: ${FAILED_TASKS:-none}\n"
  if [ ${#WARNINGS[@]} -gt 0 ]; then
    ALERT_BODY+="\nWarnings:\n"
    for warning in "${WARNINGS[@]}"; do
      ALERT_BODY+="  - $warning\n"
    done
  fi
  send_alert "DBOps Platform Self-Check: ${FAILED} failures" "$ALERT_BODY"
fi

log "=========================================="
if [ "$FAILED" -gt 0 ]; then
  log "RESULT: FAILED ($FAILED critical issues)"
  exit 1
else
  log "RESULT: PASSED"
  exit 0
fi
