#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

cd "$PROJECT_ROOT/backend"

export GOPROXY=https://goproxy.cn,direct

GO_REQUIRED="1.25"
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
        echo "ERROR: need Go $GO_REQUIRED+, current: $(go version 2>/dev/null || echo 'not found')"
        echo "Run init-ubuntu.sh first or install Go manually"
        exit 1
    fi
fi

echo "Starting MySQL Ops Platform Backend..."
go run cmd/main.go
