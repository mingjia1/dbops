#!/bin/bash

echo "Stopping MySQL Ops Platform services..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Stop backend (binary + go run)
pkill -f "bin/platform" 2>/dev/null && echo "Backend (binary) stopped" || true
pkill -f "go run.*backend" 2>/dev/null && echo "Backend (go run) stopped" || true

# Stop agent (binary + go run)
pkill -f "bin/agent" 2>/dev/null && echo "Agent (binary) stopped" || true
pkill -f "go run.*agent" 2>/dev/null && echo "Agent (go run) stopped" || true

# Stop web console
pkill -f "npm run dev" 2>/dev/null && echo "Web console (npm) stopped" || true
pkill -f "python3 -m http.server 3000" 2>/dev/null && echo "Web console (static) stopped" || true

# Remove built binaries (仅保留服务运行期间存在)
BACKEND_BIN="$PROJECT_ROOT/backend/bin/platform"
AGENT_BIN="$PROJECT_ROOT/agent/bin/agent"
[ -f "$BACKEND_BIN" ] && rm -f "$BACKEND_BIN" && echo "Backend binary removed" || true
[ -f "$AGENT_BIN" ]   && rm -f "$AGENT_BIN"   && echo "Agent binary removed" || true

echo "All services stopped"
