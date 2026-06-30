#!/bin/bash
#
# MySQL 运维平台 - 停止所有服务
# 用法: bash stop-services.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
LOG_DIR="$PROJECT_ROOT/logs"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_success() { echo -e "${GREEN}[成功]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[警告]${NC} $1"; }

echo "Stopping MySQL Ops Platform services..."

# Stop Agent (compiled binary)
AGENT_BIN="$PROJECT_ROOT/agent/bin/agent"
AGENT_PID_FILE="$LOG_DIR/agent.pid"
if [ -f "$AGENT_PID_FILE" ]; then
    OLD_PID=$(cat "$AGENT_PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        kill "$OLD_PID" 2>/dev/null
        sleep 1
        kill -0 "$OLD_PID" 2>/dev/null && kill -9 "$OLD_PID" 2>/dev/null
        log_success "Agent 已停止 (PID: $OLD_PID)"
    else
        log_warn "Agent PID 文件存在但进程已结束"
    fi
    rm -f "$AGENT_PID_FILE"
else
    pkill -f "$AGENT_BIN" 2>/dev/null && log_success "Agent 已停止" || log_warn "Agent 未运行"
fi

# Stop Agent (legacy go run)
pkill -f "go run.*agent/cmd/main.go" 2>/dev/null && log_success "Agent (go run) 已停止" || true

# Stop Backend
BACKEND_BIN="$PROJECT_ROOT/backend/bin/platform"
BACKEND_PID_FILE="$LOG_DIR/backend.pid"
if [ -f "$BACKEND_PID_FILE" ]; then
    OLD_PID=$(cat "$BACKEND_PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        kill "$OLD_PID" 2>/dev/null
        sleep 1
        kill -0 "$OLD_PID" 2>/dev/null && kill -9 "$OLD_PID" 2>/dev/null
        log_success "Backend 已停止 (PID: $OLD_PID)"
    fi
    rm -f "$BACKEND_PID_FILE"
else
    pkill -f "go run.*backend/cmd/main.go" 2>/dev/null && log_success "Backend 已停止" || log_warn "Backend 未运行"
fi

# Stop web console
pkill -f "npm run dev" 2>/dev/null && log_success "Web console 已停止" || log_warn "Web console 未运行"

echo ""
log_success "所有服务已停止"
