#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Starting all MySQL Ops Platform services..."

# Start backend in background
bash "$SCRIPT_DIR/start-backend.sh" &
BACKEND_PID=$!
echo "Backend started (PID: $BACKEND_PID)"
sleep 2

# Start agent in background
bash "$SCRIPT_DIR/start-agent.sh" &
AGENT_PID=$!
echo "Agent started (PID: $AGENT_PID)"
sleep 2

# Start web console in background
bash "$SCRIPT_DIR/start-web.sh" &
WEB_PID=$!
echo "Web console started (PID: $WEB_PID)"

echo ""
echo "All services started!"
echo "Backend:     http://localhost:8080"
echo "Agent:       http://localhost:9090"
echo "Web Console: http://localhost:3000"
echo ""
echo "Press Ctrl+C to stop all services"

# Wait for any process to exit
wait -n
