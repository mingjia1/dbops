#!/bin/bash
#
# start-frontend.sh - Start React web console on port 3000 (force kill if occupied)
#

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$PROJECT_DIR/web-console"
LOG_DIR="$PROJECT_DIR/logs"
PID_FILE="$LOG_DIR/frontend.pid"
LOG_FILE="$LOG_DIR/frontend.log"
PORT=3000
mkdir -p "$LOG_DIR"

echo "========================================"
echo "Starting React Web Console on Port $PORT"
echo "========================================"
echo

# Kill any process using port 3000
echo "[1/2] Checking port $PORT..."
PID_USING_PORT=$(lsof -ti:$PORT 2>/dev/null || true)

if [ -n "$PID_USING_PORT" ]; then
    echo "[INFO] Found process using port $PORT (PID: $PID_USING_PORT)"
    kill -9 $PID_USING_PORT 2>/dev/null || true
    echo "[SUCCESS] Killed process $PID_USING_PORT"
    sleep 2
fi

echo "[INFO] Port $PORT is now available"
echo

# Validate frontend directory
if [ ! -f "$FRONTEND_DIR/package.json" ]; then
    echo "[ERROR] package.json not found in $FRONTEND_DIR"
    exit 1
fi

# Start Frontend
echo "[2/2] Starting React web console..."
cd "$FRONTEND_DIR"

# Start Vite on port 3000
VITE_PORT=$PORT npx vite --port "$PORT" --host 0.0.0.0 > "$LOG_FILE" 2>&1 &
VITE_PID=$!
echo "$VITE_PID" > "$PID_FILE"

# Wait for startup
sleep 6

# Verify frontend started
if kill -0 "$VITE_PID" 2>/dev/null; then
    echo "[SUCCESS] React web console started on port $PORT (PID: $VITE_PID)"
else
    echo "[ERROR] Failed to start frontend"
    cat "$LOG_FILE"
    rm -f "$PID_FILE"
    exit 1
fi

echo
echo "========================================"
echo "Frontend: http://localhost:$PORT"
echo "Log: $LOG_FILE"
echo "========================================"
