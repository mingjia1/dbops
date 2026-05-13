#!/bin/bash
#
# start.sh - Start Django backend server
# Usage: ./start.sh
#
# Requires a Python virtual environment at <project_root>/venv/
#

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
VENV_DIR="$PROJECT_DIR/venv"
PID_FILE="$PROJECT_DIR/.django.pid"
LOG_FILE="$PROJECT_DIR/.django.log"
PORT=${DJANGO_PORT:-8000}
WORKERS=${GUNICORN_WORKERS:-4}

# Validate virtual environment
if [ ! -d "$VENV_DIR" ]; then
    echo "[ERROR] Virtual environment not found at: $VENV_DIR"
    echo "        Create one: python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt"
    exit 1
fi

if [ ! -f "$VENV_DIR/bin/activate" ]; then
    echo "[ERROR] Invalid virtual environment at: $VENV_DIR"
    exit 1
fi

# Check if already running
if [ -f "$PID_FILE" ]; then
    EXISTING_PID=$(cat "$PID_FILE")
    if kill -0 "$EXISTING_PID" 2>/dev/null; then
        echo "[ERROR] Django server is already running (PID: $EXISTING_PID)"
        echo "        Run ./stop.sh first or manually: kill $EXISTING_PID"
        exit 1
    else
        echo "[WARN] Stale PID file found. Cleaning up."
        rm -f "$PID_FILE"
    fi
fi

# Activate virtual environment
# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

# Verify Django is installed
if ! python -c "import django" 2>/dev/null; then
    echo "[ERROR] Django is not installed in the virtual environment."
    echo "        Run: pip install -r requirements.txt"
    exit 1
fi

cd "$PROJECT_DIR"

echo "[INFO] Starting Django server on 0.0.0.0:$PORT (workers: $WORKERS)..."

gunicorn mysql_dba_platform.wsgi:application \
    --bind "0.0.0.0:$PORT" \
    --workers "$WORKERS" \
    --pid "$PID_FILE" \
    --log-file "$LOG_FILE" \
    --daemon

sleep 1

if [ -f "$PID_FILE" ]; then
    STARTED_PID=$(cat "$PID_FILE")
    if kill -0 "$STARTED_PID" 2>/dev/null; then
        echo "[SUCCESS] Django server started (PID: $STARTED_PID)"
        echo "         Logs: $LOG_FILE"
        echo "         URL:  http://0.0.0.0:$PORT"
    else
        echo "[ERROR] Process exited immediately. Check logs: $LOG_FILE"
        rm -f "$PID_FILE"
        exit 1
    fi
else
    echo "[ERROR] Failed to start Django server. Check logs: $LOG_FILE"
    exit 1
fi
