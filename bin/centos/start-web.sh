#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# 优先使用编译好的静态文件，否则使用 npm run dev
if [ -d "$PROJECT_ROOT/frontend/build" ]; then
    echo "Starting MySQL Ops Platform Web Console (static build)..."
    cd "$PROJECT_ROOT/frontend/build"
    python3 -m http.server 3000
elif [ -d "$PROJECT_ROOT/frontend/dist" ]; then
    echo "Starting MySQL Ops Platform Web Console (dist)..."
    cd "$PROJECT_ROOT/frontend/dist"
    python3 -m http.server 3000
else
    echo "Starting MySQL Ops Platform Web Console (npm run dev)..."
    cd "$PROJECT_ROOT/frontend"
    npm run dev
fi
