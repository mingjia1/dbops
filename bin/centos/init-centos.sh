#!/bin/bash
# CentOS/RHEL 7+ 一键初始化脚本 - MySQL 运维平台环境准备
# 注意: 本脚本逐一安装组件，单个步骤失败不会中断后续安装

echo "==================================="
echo "MySQL 运维平台 - CentOS/RHEL 初始化"
echo "==================================="

# 检测 CentOS 版本
OS_VERSION=""
if [ -f /etc/centos-release ]; then
    OS_VERSION=$(cat /etc/centos-release | grep -oP '[0-9]+\.[0-9]+' | head -1 | cut -d. -f1)
elif [ -f /etc/redhat-release ]; then
    OS_VERSION=$(cat /etc/redhat-release | grep -oP '[0-9]+\.[0-9]+' | head -1 | cut -d. -f1)
else
    echo "不支持的操作系统，仅支持 CentOS/RHEL"
    exit 1
fi

echo "检测到 CentOS/RHEL $OS_VERSION"
PKG_MGR="yum"
[ "$OS_VERSION" -ge 8 ] && PKG_MGR="dnf"

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then
    echo "请使用 root 权限运行此脚本: sudo bash $0"
    exit 1
fi

YUM_OPTS="--skip-broken"
YUM_INSTALL="$PKG_MGR install -y $YUM_OPTS"

# 更新系统（跳过有问题的仓库，防止 XML 解析错误中断）
echo ""
echo "[1/9] 更新系统包..."
$PKG_MGR install -y epel-release $YUM_OPTS 2>&1 | grep -v "primary.xml error" || echo "epel-release 已安装或不可用"
$PKG_MGR update -y $YUM_OPTS 2>&1 | grep -v "primary.xml error" || echo "部分仓库更新失败（已跳过）"

# 安装基础工具
echo ""
echo "[2/9] 安装基础工具..."
$PKG_MGR install -y curl wget git vim-enhanced net-tools lsof tar bzip2 $YUM_OPTS || echo "部分基础工具安装失败"

# 安装 Go（从国内镜像下载 tarball）
echo ""
echo "[3/9] 安装 Go 1.23+..."
GO_VERSION="1.25.10"
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
    if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 25 ]; ]; then
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
echo "[4/9] 安装 Node.js 18.x..."
if ! command -v node &> /dev/null; then
    if [ "$OS_VERSION" -ge 8 ]; then
        $PKG_MGR module enable -y nodejs:18 2>/dev/null || true
        $PKG_MGR install -y nodejs $YUM_OPTS || echo "Node.js 安装失败"
    else
        curl -fsSL https://rpm.nodesource.com/setup_18.x | bash - 2>&1 | tail -5 || echo "NodeSource 仓库配置失败"
        $PKG_MGR install -y nodejs $YUM_OPTS || echo "Node.js 安装失败"
    fi
    echo "Node.js installed: $(node --version 2>/dev/null || echo '未安装')"
    echo "npm installed: $(npm --version 2>/dev/null || echo '未安装')"
else
    echo "Node.js already installed: $(node --version)"
fi

# 安装 MySQL 8.0
echo ""
echo "[5/9] 安装 MySQL 8.0 Server..."
if ! command -v mysqld &> /dev/null; then
    if [ "$OS_VERSION" -ge 8 ]; then
        $PKG_MGR install -y mysql-server mysql $YUM_OPTS || echo "MySQL 安装失败"
    else
        rpm -Uvh https://dev.mysql.com/get/mysql80-community-release-el7-7.noarch.rpm 2>&1 | tail -3 || echo "MySQL 仓库配置失败"
        $PKG_MGR install -y mysql-community-server mysql-community-client $YUM_OPTS || echo "MySQL 安装失败"
    fi
    systemctl enable mysqld 2>/dev/null || echo "mysqld enable 失败"
    systemctl start mysqld 2>/dev/null || echo "mysqld start 失败"
    echo "MySQL installed: $(mysql --version 2>/dev/null || echo '未安装')"
else
    echo "MySQL already installed: $(mysqld --version)"
fi

# 安装 Percona XtraBackup 8.0
echo ""
echo "[6/9] 安装 Percona XtraBackup 8.0..."
if ! command -v xtrabackup &> /dev/null; then
    rpm -Uvh https://repo.percona.com/yum/percona-release-latest.noarch.rpm 2>&1 | tail -3 || echo "Percona 仓库配置失败，跳过 XtraBackup"
    if command -v percona-release &> /dev/null; then
        percona-release enable-only tools release 2>/dev/null || true
        $PKG_MGR install -y percona-xtrabackup-80 $YUM_OPTS || echo "XtraBackup 安装失败"
    fi
    echo "XtraBackup installed: $(xtrabackup --version 2>&1 | head -1 || echo '未安装')"
else
    echo "XtraBackup already installed: $(xtrabackup --version 2>&1 | head -1)"
fi

# 安装 Gcc 编译工具链
echo ""
echo "[7/9] 安装编译工具链..."
$PKG_MGR groupinstall -y "Development Tools" $YUM_OPTS || $PKG_MGR install -y gcc gcc-c++ make $YUM_OPTS || echo "编译工具链安装失败"


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
echo "注意：以上标记"未安装"的组件可按需手动安装。"
echo "下一步："
echo "1. 克隆项目: git clone <repo-url>"
echo "2. 进入项目: cd dbops"
echo "3. 安装依赖: make all"
echo "4. 启动服务: bash bin/centos/start-all.sh"
echo ""
