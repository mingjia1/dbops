#!/bin/bash
#
# stop-frontend.sh - Stop React web console dev server on Linux
# Usage: ./stop-frontend.sh
#

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$PROJECT_DIR/logs/frontend.pid"

if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "[INFO] Stopping frontend dev server (PID: $PID)..."
        kill "$PID" 2>/dev/null || true

        for i in $(seq 1 5); do
            if ! kill -0 "$PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done

        if kill -0 "$PID" 2>/dev/null; then
            kill -9 "$PID" 2>/dev/null || true
        fi

        rm -f "$PID_FILE"
        echo "[SUCCESS] Frontend dev server stopped."
    else
        echo "[WARN] Process $PID not found. Cleaning up."
        rm -f "$PID_FILE"
    fi
else
    echo "[WARN] No PID file found. Attempting to kill by port..."
    FUSER_PID=$(fuser 3000/tcp 2>/dev/null || true)
    if [ -n "$FUSER_PID" ]; then
        kill "$FUSER_PID" 2>/dev/null || true
        echo "[INFO] Process on port 3000 killed."
    else
        echo "[INFO] No process found on port 3000."
    fi
fi
