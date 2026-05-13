#!/bin/bash
#
# start-frontend.sh - Start Vue frontend dev server on Linux
# Usage: ./start-frontend.sh
#

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$PROJECT_DIR/frontend"
PID_FILE="$PROJECT_DIR/.frontend.pid"
PORT=${FRONTEND_PORT:-3000}

# Validate frontend directory
if [ ! -d "$FRONTEND_DIR" ]; then
    echo "[ERROR] Frontend directory not found at $FRONTEND_DIR"
    exit 1
fi

if [ ! -f "$FRONTEND_DIR/package.json" ]; then
    echo "[ERROR] package.json not found in $FRONTEND_DIR"
    exit 1
fi

# Validate node_modules
if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
    echo "[ERROR] node_modules not found. Run: cd frontend && npm install"
    exit 1
fi

# Check if already running
if [ -f "$PID_FILE" ]; then
    EXISTING_PID=$(cat "$PID_FILE")
    if kill -0 "$EXISTING_PID" 2>/dev/null; then
        echo "[ERROR] Frontend dev server is already running (PID: $EXISTING_PID)"
        echo "        Run ./stop-frontend.sh first."
        exit 1
    else
        echo "[WARN] Stale PID file found. Cleaning up."
        rm -f "$PID_FILE"
    fi
fi

cd "$FRONTEND_DIR"
echo "[INFO] Starting frontend dev server on port $PORT ..."

# Start Vite in background
npx vite --port "$PORT" &
VITE_PID=$!
echo "$VITE_PID" > "$PID_FILE"

# Wait and verify
sleep 3

if kill -0 "$VITE_PID" 2>/dev/null; then
    echo "[SUCCESS] Frontend dev server started (PID: $VITE_PID)"
    echo "         URL:  http://localhost:$PORT"
    echo "         API proxies to: http://localhost:8000"
else
    echo "[ERROR] Frontend process exited immediately."
    rm -f "$PID_FILE"
    exit 1
fi
