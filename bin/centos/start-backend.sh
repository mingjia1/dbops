#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

cd "$PROJECT_ROOT/backend"

# 国内网络需设置 Go 模块代理
export GOPROXY=https://goproxy.cn,direct

# Go 版本检查 — 需要 1.21+
GO_REQUIRED="1.21"
check_go_version() {
    local go_cmd="${1:-go}"
    local ver
    ver=$("$go_cmd" version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
    if [ -z "$ver" ]; then return 1; fi
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
        echo "Using Go at /usr/local/go/bin/go ($(go version))"
    else
        echo "ERROR: 需要 Go $GO_REQUIRED+，当前 go version: $(go version 2>/dev/null || echo 'not found')"
        echo "请先运行 init-centos.sh 安装 Go，或手动安装后重试"
        echo "  快速安装: wget -q https://go.dev/dl/go1.21.6.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz && export PATH=/usr/local/go/bin:\$PATH"
        exit 1
    fi
fi

echo "Building backend..."
make build

BINARY="bin/platform"
if [ -f "$BINARY" ]; then
    echo "Starting MySQL Ops Platform Backend (binary)..."
    exec ./$BINARY
else
    echo "Starting MySQL Ops Platform Backend (go run)..."
    go run cmd/main.go
fi
