#!/usr/bin/env bash
# mysqlchk.sh — MySQL health check script for Keepalived
# Usage: mysqlchk.sh <port> [user] [password]
#
# Returns 0 if MySQL is healthy, 1 otherwise.
# Keepalived uses the exit code to decide VIP failover.

MYSQL_PORT="${1:-3306}"
MYSQL_USER="${2:-monitor}"
MYSQL_PASS="${3:-}"

if [ -z "$MYSQL_PASS" ]; then
    mysqladmin -h 127.0.0.1 -P "$MYSQL_PORT" -u "$MYSQL_USER" ping > /dev/null 2>&1
else
    mysqladmin -h 127.0.0.1 -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" ping > /dev/null 2>&1
fi

exit $?
