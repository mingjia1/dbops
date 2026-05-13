#!/bin/bash
#
# stop.sh - Stop Django backend server
# Usage: ./stop.sh
#

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$PROJECT_DIR/.django.pid"

if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "[INFO] Stopping Django server (PID: $PID)..."
        kill "$PID" 2>/dev/null || true

        # Wait for graceful shutdown (up to 10 seconds)
        for i in $(seq 1 10); do
            if ! kill -0 "$PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done

        # Force kill if still running
        if kill -0 "$PID" 2>/dev/null; then
            echo "[WARN] Graceful shutdown timed out. Forcing kill..."
            kill -9 "$PID" 2>/dev/null || true
        fi

        rm -f "$PID_FILE"
        echo "[SUCCESS] Django server stopped."
    else
        echo "[WARN] Process $PID not found. Cleaning up PID file."
        rm -f "$PID_FILE"

        # Clean up any orphaned gunicorn processes
        REMAINING=$(pgrep -f "gunicorn.*mysql_dba_platform" 2>/dev/null || true)
        if [ -n "$REMAINING" ]; then
            echo "[INFO] Cleaning up orphaned gunicorn processes..."
            pkill -f "gunicorn.*mysql_dba_platform" 2>/dev/null || true
        fi
    fi
else
    echo "[WARN] No PID file found at $PID_FILE"

    # Try to find and kill any running gunicorn for this project
    REMAINING=$(pgrep -f "gunicorn.*mysql_dba_platform" 2>/dev/null || true)
    if [ -n "$REMAINING" ]; then
        echo "[INFO] Found orphaned gunicorn process(es). Cleaning up..."
        pkill -f "gunicorn.*mysql_dba_platform" 2>/dev/null || true
        echo "[SUCCESS] Orphaned processes stopped."
    else
        echo "[INFO] No Django server process found."
    fi
fi
