#!/usr/bin/env bash
# mysqlchk.sh — MySQL health check for Keepalived (writable primary only)
# Usage: mysqlchk.sh <port> [user] [password]
#
# Exit 0 only when:
#   1) mysqld accepts connections
#   2) read_only=OFF and super_read_only=OFF (writable)
# Replica / offline nodes intentionally fail so VIP does not land on them.

MYSQL_PORT="${1:-3306}"
MYSQL_USER="${2:-monitor}"
MYSQL_PASS="${3:-}"

MYSQL_ARGS=(-h 127.0.0.1 -P "$MYSQL_PORT" -u "$MYSQL_USER" -N -B)
if [ -n "$MYSQL_PASS" ]; then
  export MYSQL_PWD="$MYSQL_PASS"
fi

# ping first (cheap)
if ! mysqladmin "${MYSQL_ARGS[@]}" ping >/dev/null 2>&1; then
  exit 1
fi

# writable check: both read_only flags must be OFF
# @@super_read_only may be missing on very old builds; treat as OFF if unknown.
RO=$(mysql "${MYSQL_ARGS[@]}" -e "SELECT CONCAT(@@global.read_only, ',', IFNULL(@@global.super_read_only, 0));" 2>/dev/null | tr -d '[:space:]')
if [ "$RO" != "0,0" ]; then
  exit 1
fi

exit 0
