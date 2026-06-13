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
apt install -y curl wget git vim net-tools lsof

# 安装 Go
echo ""
echo "[3/8] 安装 Go 1.21+..."
if ! command -v go &> /dev/null; then
    GO_VERSION="1.21.6"
    wget -q https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    rm go${GO_VERSION}.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    export PATH=$PATH:/usr/local/go/bin
    echo "Go installed: $(go version)"
else
    echo "Go already installed: $(go version)"
fi

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
if ! command -v mysqld &> /dev/null; then
    apt install -y mysql-server-8.0 mysql-client-8.0
    systemctl enable mysql
    systemctl start mysql
    echo "MySQL installed: $(mysql --version)"
else
    echo "MySQL already installed: $(mysqld --version)"
fi

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

# 安装 Docker (可选)
echo ""
echo "[7/8] 安装 Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    systemctl enable docker
    systemctl start docker
    echo "Docker installed: $(docker --version)"
else
    echo "Docker already installed: $(docker --version)"
fi

# 安装 Docker Compose (可选)
echo ""
echo "[8/8] 安装 Docker Compose..."
if ! command -v docker-compose &> /dev/null; then
    COMPOSE_VERSION="2.24.0"
    curl -L "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    echo "Docker Compose installed: $(docker-compose --version)"
else
    echo "Docker Compose already installed: $(docker-compose --version)"
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
echo "Docker:     $(docker --version 2>&1 || echo '未安装')"
echo "Docker Compose: $(docker-compose --version 2>&1 || echo '未安装')"

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
