#!/bin/bash
#
# MySQL 运维平台 - Agent 启动脚本 (含自动编译)
# 用法: bash start-agent.sh
# 功能:
#   1. 检查 Go 编译环境
#   2. 停止正在运行的旧 Agent 进程
#   3. 移除旧 Agent 二进制文件
#   4. 重新编译 Agent
#   5. 启动新 Agent (后台运行)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
LOG_DIR="$PROJECT_ROOT/logs"
AGENT_DIR="$PROJECT_ROOT/agent"
AGENT_BIN_DIR="$AGENT_DIR/bin"
AGENT_EXE="$AGENT_BIN_DIR/agent"
AGENT_PID_FILE="$LOG_DIR/agent.pid"
AGENT_LOG="$LOG_DIR/agent.log"
AGENT_ERR="$LOG_DIR/agent.err"
AGENT_PORT=9090

# 颜色输出
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
MAGENTA='\033[0;35m'
NC='\033[0m'

log_success() { echo -e "${GREEN}[成功]${NC} $1"; }
log_info()    { echo -e "${CYAN}[信息]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[警告]${NC} $1"; }
log_error()   { echo -e "${RED}[错误]${NC} $1"; }
log_step()    { echo -e "\n${MAGENTA}=== $1 ===${NC}"; }

mkdir -p "$LOG_DIR" "$AGENT_BIN_DIR"

# ===== 1. 加载 .env 配置 =====
log_step "1. 加载环境配置"
if [ -f "$PROJECT_ROOT/.env" ]; then
    while IFS='=' read -r key value; do
        key="${key#"${key%%[![:space:]]*}"}"
        [ -z "$key" ] && continue
        [ "${key#\#}" != "$key" ] && continue
        [ -z "${value+set}" ] && continue
        export "$key=$value"
    done < "$PROJECT_ROOT/.env"
    log_success ".env 配置文件已加载"
else
    log_warn ".env 文件不存在于 $PROJECT_ROOT/.env"
fi

# ===== 2. 检查 Go 编译环境 =====
log_step "2. 检查 Go 编译环境"

export GOPROXY=https://goproxy.cn,direct
GO_REQUIRED="1.22"

check_go_version() {
    local go_cmd="${1:-go}"
    local ver
    ver=$("$go_cmd" version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
    [ -z "$ver" ] && return 1
    local major="${ver%.*}"
    local minor="${ver#*.}"
    local req_major="${GO_REQUIRED%.*}"
    local req_minor="${GO_REQUIRED#*.}"
    if [ "$major" -gt "$req_major" ] || { [ "$major" -eq "$req_major" ] && [ "$minor" -ge "$req_minor" ]; }; then
        return 0
    fi
    return 1
}

if ! check_go_version "go"; then
    if [ -x "/usr/local/go/bin/go" ] && check_go_version "/usr/local/go/bin/go"; then
        export PATH="/usr/local/go/bin:$PATH"
        log_info "使用 /usr/local/go/bin/go: $(go version)"
    else
        log_error "需要 Go $GO_REQUIRED+, 当前: $(go version 2>/dev/null || echo '未安装')"
        log_error "请先运行 init-ubuntu.sh 或手动安装 Go"
        exit 1
    fi
else
    log_info "Go 环境就绪: $(go version)"
fi

# ===== 3. 停止正在运行的旧 Agent =====
log_step "3. 停止旧 Agent 进程"

# 先尝试通过 PID 文件停止
if [ -f "$AGENT_PID_FILE" ]; then
    OLD_PID=$(cat "$AGENT_PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        log_info "发现正在运行的 Agent (PID: $OLD_PID)，正在停止..."
        kill "$OLD_PID" 2>/dev/null
        sleep 2
        if kill -0 "$OLD_PID" 2>/dev/null; then
            log_warn "Agent 未响应 SIGTERM，强制终止..."
            kill -9 "$OLD_PID" 2>/dev/null
        fi
        log_success "旧 Agent 已停止"
    else
        log_info "PID 文件存在但进程已结束"
    fi
    rm -f "$AGENT_PID_FILE"
fi

# 也搜索 agent 二进制进程（防止 PID 文件丢失）
AGENT_PIDS=$(pgrep -f "$AGENT_EXE" 2>/dev/null || true)
if [ -n "$AGENT_PIDS" ]; then
    log_info "发现残留 Agent 进程 (PID: $(echo $AGENT_PIDS | tr '\n' ' '))，正在清理..."
    pkill -f "$AGENT_EXE" 2>/dev/null || true
    sleep 1
    log_success "残留进程已清理"
fi

# 检查端口是否已被占用
if command -v ss &>/dev/null; then
    if ss -tlnp 2>/dev/null | grep -q ":$AGENT_PORT "; then
        log_warn "端口 $AGENT_PORT 仍被占用，尝试强制释放..."
        fuser -k "${AGENT_PORT}/tcp" 2>/dev/null || true
        sleep 1
    fi
fi

# ===== 4. 移除旧 Agent 二进制 =====
log_step "4. 移除旧 Agent 二进制"
if [ -f "$AGENT_EXE" ]; then
    rm -f "$AGENT_EXE"
    log_success "已移除旧二进制: $AGENT_EXE"
else
    log_info "无旧二进制需要移除"
fi

# 同时清理 agent 目录下的旧二进制文件
for old_bin in "$AGENT_DIR/agent" "$AGENT_DIR/agent-linux" "$AGENT_DIR/agent-linux-amd64" \
               "$AGENT_DIR/agent_linux" "$AGENT_DIR/mysql-ops-agent-linux" \
               "$AGENT_DIR/mysql-ops-agent-linux-amd64" "$AGENT_DIR/dbops-agent-linux"; do
    if [ -f "$old_bin" ]; then
        rm -f "$old_bin"
        log_info "已清理旧二进制: $old_bin"
    fi
done

# ===== 5. 编译新 Agent =====
log_step "5. 编译新 Agent"
cd "$AGENT_DIR"
log_info "编译中: go build -o $AGENT_EXE ./cmd/main.go"

BUILD_OUTPUT=$(go build -o "$AGENT_EXE" ./cmd/main.go 2>&1) || {
    log_error "编译失败:"
    echo "$BUILD_OUTPUT"
    exit 1
}

if [ ! -f "$AGENT_EXE" ]; then
    log_error "编译完成后未找到二进制文件: $AGENT_EXE"
    exit 1
fi

log_success "编译成功: $AGENT_EXE ($(du -h "$AGENT_EXE" | cut -f1))"

# ===== 6. 启动新 Agent =====
log_step "6. 启动 Agent 服务 (端口 $AGENT_PORT)"

# 后台启动并将输出重定向到日志文件
"$AGENT_EXE" > "$AGENT_LOG" 2>"$AGENT_ERR" &
AGENT_PID=$!
echo "$AGENT_PID" > "$AGENT_PID_FILE"
log_info "Agent 进程已拉起 (PID: $AGENT_PID)"

# 等待端口就绪
log_info "等待端口 $AGENT_PORT 就绪..."
for i in $(seq 1 30); do
    if kill -0 "$AGENT_PID" 2>/dev/null; then
        if command -v ss &>/dev/null; then
            if ss -tlnp 2>/dev/null | grep -q ":$AGENT_PORT "; then
                log_success "Agent 已启动，监听端口 $AGENT_PORT"
                break
            fi
        fi
        # 也检查进程是否已退出（启动失败）
        sleep 1
    else
        log_error "Agent 进程在启动后已退出 (PID: $AGENT_PID)"
        log_error "请检查日志: $AGENT_LOG 和 $AGENT_ERR"
        tail -20 "$AGENT_ERR" 2>/dev/null | sed 's/^/  /'
        exit 1
    fi
done

echo ""
log_success "========================================"
log_success "  Agent 启动完成"
log_success "  进程 PID: $AGENT_PID"
log_success "  监听端口: $AGENT_PORT"
log_success "  日志目录: $LOG_DIR"
log_success "  查看日志: tail -f $AGENT_LOG"
log_success "========================================"
