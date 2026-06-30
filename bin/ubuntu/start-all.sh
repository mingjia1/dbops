#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"

# Load .env safely (line by line, no shell eval)
if [ -f "$PROJECT_ROOT/.env" ]; then
    while IFS='=' read -r key value; do
        key="${key#"${key%%[![:space:]]*}"}"
        [ -z "$key" ] && continue
        [ "${key#\#}" != "$key" ] && continue
        [ -z "${value+set}" ] && continue
        export "$key=$value"
    done < "$PROJECT_ROOT/.env"
    echo "Loaded .env config"
fi

echo "Starting all MySQL Ops Platform services..."

# Start backend in background (output captured to log)
bash "$SCRIPT_DIR/start-backend.sh" > "$LOG_DIR/backend.log" 2>&1 &
BACKEND_PID=$!
echo "Backend started (PID: $BACKEND_PID)"
sleep 5

# Start agent in background (output captured to log)
bash "$SCRIPT_DIR/start-agent.sh" > "$LOG_DIR/agent.log" 2>&1 &
AGENT_PID=$!
echo "Agent started (PID: $AGENT_PID)"
sleep 3

# Start web console in background
bash "$SCRIPT_DIR/start-web.sh" &
WEB_PID=$!
echo "Web console started (PID: $WEB_PID)"

# Check backend port
sleep 3
if ss -tlnp 2>/dev/null | grep -q ':8080 '; then
    echo "Backend is listening on port 8080"
else
    echo "WARNING: Backend NOT listening on 8080"
    echo "  Check logs: $LOG_DIR/backend.log"
    BACKEND_STATUS=$(ps --no-headers -p $BACKEND_PID 2>/dev/null || echo "exited")
    echo "  Process status: $BACKEND_STATUS"
fi

echo ""
echo "All services started!"
echo "Backend:     http://localhost:8080"
echo "Agent:       http://localhost:9090"
echo "Web Console: http://localhost:3000"
echo ""
echo "Log directory: $LOG_DIR"
echo "Press Ctrl+C to stop all services"

wait -n
