#!/bin/bash
# Ubuntu 22.04 一键初始化脚本 - MySQL 运维平台环境准备
set -e

echo "==================================="
echo "MySQL 运维平台 - Ubuntu 22.04 初始化"
echo "==================================="

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then
  echo "请使用 root 权限运行此脚本: sudo bash $0"
  exit 1
fi

# 更新系统
echo ""
echo "[1/8] 更新系统包..."
apt update

# 安装基础工具
echo ""
echo "[2/8] 安装基础工具..."
apt install -y curl wget git vim net-tools lsof software-properties-common

# 安装 Go（从国内镜像下载 tarball，比 golang.org 快数十倍）
echo ""
echo "[3/8] 安装 Go 1.25+..."
GO_VERSION="1.25.11"
GO_MIRRORS=(
    "https://mirrors.ustc.edu.cn/golang"
    "https://mirrors.huaweicloud.com/go"
    "https://mirrors.aliyun.com/golang"
    "https://go.dev/dl"
)

# 检查当前 Go 版本（需要 >= 1.25，因为依赖要求 go 1.25+）
INSTALL_GO=false
if command -v go &> /dev/null; then
    cur=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
    major="${cur%.*}"; minor="${cur#*.}"
    if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 25 ]; }; then
        echo "当前 Go ${cur} 版本过旧，升级到 ${GO_VERSION}..."
        INSTALL_GO=true
    else
        echo "Go already installed: $(go version)"
    fi
else
    echo "Go 未安装，安装 ${GO_VERSION}..."
    INSTALL_GO=true
fi

if [ "$INSTALL_GO" = true ]; then
    TAR_FILE="go${GO_VERSION}.linux-amd64.tar.gz"
    # 优先使用当前目录已有的 tarball，避免重复下载
    if [ -f "./${TAR_FILE}" ]; then
        echo "  使用本地文件: ./${TAR_FILE}"
        cp "./${TAR_FILE}" "/tmp/${TAR_FILE}"
    elif [ -f "/tmp/${TAR_FILE}" ]; then
        echo "  使用本地文件: /tmp/${TAR_FILE}"
    else
        DOWNLOAD_OK=false
        for mirror in "${GO_MIRRORS[@]}"; do
            url="${mirror}/go${GO_VERSION}.linux-amd64.tar.gz"
            echo "  尝试下载: ${url}"
            wget -q --timeout=10 "${url}" -O "/tmp/${TAR_FILE}" && { DOWNLOAD_OK=true; break; } || true
        done
        if [ "$DOWNLOAD_OK" != true ]; then
            echo "[WARN] 所有镜像下载失败，保留当前 Go $(go version 2>/dev/null | head -1)"
            echo "       可手动下载后重试:"
            echo "       wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
            echo "       bash $0"
            INSTALL_GO=false
        fi
    fi
    if [ "$INSTALL_GO" = true ]; then
        rm -rf /usr/local/go
        tar -C /usr/local -xzf "/tmp/${TAR_FILE}"
        rm "/tmp/${TAR_FILE}"
    fi
fi

# 确保 /usr/local/go/bin 在 PATH 中（优先级高于旧版系统 Go）
if [ -x /usr/local/go/bin/go ]; then
    # 创建软链到 /usr/local/bin（默认在 PATH 中且优先于 /usr/bin）
    ln -sf /usr/local/go/bin/go /usr/local/bin/go
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt 2>/dev/null || true
    export PATH="/usr/local/go/bin:$PATH"
fi
if ! grep -q '/usr/local/go/bin' /etc/profile 2>/dev/null; then
    echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/profile
fi
echo 'export GOPROXY=https://goproxy.cn,direct' >> /etc/profile 2>/dev/null || true
export GOPROXY=https://goproxy.cn,direct
echo "Go version: $(go version)"

# 安装 Node.js
echo ""
echo "[4/8] 安装 Node.js 18.x..."
if ! command -v node &> /dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
    apt install -y nodejs
    echo "Node.js installed: $(node --version)"
    echo "npm installed: $(npm --version)"
else
    echo "Node.js already installed: $(node --version)"
fi

# 安装 MySQL Server 和 Client
echo ""
echo "[5/8] 安装 MySQL Server 8.0..."
# if ! command -v mysqld &> /dev/null; then
#     apt install -y mysql-server-8.0 mysql-client-8.0
#     systemctl enable mysql
#     systemctl start mysql
#     echo "MySQL installed: $(mysql --version)"
# else
#     echo "MySQL already installed: $(mysqld --version)"
# fi

# 安装 Percona XtraBackup
echo ""
echo "[6/8] 安装 Percona XtraBackup 8.0..."
if ! command -v xtrabackup &> /dev/null; then
    wget -q https://repo.percona.com/apt/percona-release_latest.$(lsb_release -sc)_all.deb
    dpkg -i percona-release_latest.$(lsb_release -sc)_all.deb
    rm percona-release_latest.$(lsb_release -sc)_all.deb
    apt update
    apt install -y percona-xtrabackup-80
    echo "XtraBackup installed: $(xtrabackup --version 2>&1 | head -1)"
else
    echo "XtraBackup already installed: $(xtrabackup --version 2>&1 | head -1)"
fi



# 验证安装
echo ""
echo "==================================="
echo "验证安装结果"
echo "==================================="
echo "Go:         $(go version 2>&1 || echo '未安装')"
echo "Node.js:    $(node --version 2>&1 || echo '未安装')"
echo "npm:        $(npm --version 2>&1 || echo '未安装')"
echo "MySQL:      $(mysqld --version 2>&1 || echo '未安装')"
echo "XtraBackup: $(xtrabackup --version 2>&1 | head -1 || echo '未安装')"

echo ""
echo "==================================="
echo "初始化完成！"
echo "==================================="
echo ""
echo "下一步："
echo "1. 克隆项目: git clone <repo-url>"
echo "2. 进入项目: cd dbops"
echo "3. 安装依赖: make all"
echo "4. 启动服务: bash bin/start-all.sh"
echo ""
