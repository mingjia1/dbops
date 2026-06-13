#!/bin/bash
# 在10.1.81.41上建立MySQL相关包的本地仓库

HOST="10.1.81.41"
PACKET_DIR="/opt/packet"

echo "=== 连接到 $HOST 并下载安装包 ==="

# 使用heredoc通过SSH执行命令
ssh root@$HOST << 'ENDSSH'
set -e

cd /opt/packet

echo "当前目录: $(pwd)"
echo "现有文件:"
ls -lh

echo ""
echo "=== 下载MySQL 8.0安装包 ==="

# MySQL APT Repository配置包
if [ ! -f "mysql-apt-config_0.8.29-1_all.deb" ]; then
    echo "下载 mysql-apt-config..."
    wget -q https://dev.mysql.com/get/mysql-apt-config_0.8.29-1_all.deb || \
    wget -q https://repo.mysql.com/mysql-apt-config_0.8.29-1_all.deb
fi

# Percona XtraBackup 8.0
if [ ! -f "percona-xtrabackup-80_8.0.35-30-1.jammy_amd64.deb" ]; then
    echo "下载 percona-xtrabackup-80..."
    wget -q https://downloads.percona.com/downloads/Percona-XtraBackup-LATEST/Percona-XtraBackup-8.0.35-30/binary/debian/jammy/x86_64/percona-xtrabackup-80_8.0.35-30-1.jammy_amd64.deb || \
    wget -q https://repo.percona.com/pxb-80/apt/pool/main/p/percona-xtrabackup-80/percona-xtrabackup-80_8.0.35-30-1.jammy_amd64.deb
fi

# MySQL Server 8.0 (从Ubuntu官方源下载常用的包)
echo "下载 MySQL Server 8.0 核心包..."
apt-get download mysql-server-8.0 mysql-client-8.0 mysql-common libmysqlclient21 2>/dev/null || true

echo ""
echo "=== 下载完成，当前包列表 ==="
ls -lh *.deb 2>/dev/null || echo "没有找到.deb文件"

echo ""
echo "=== 创建包索引 ==="
dpkg-scanpackages . /dev/null | gzip -9c > Packages.gz 2>/dev/null || \
    echo "dpkg-scanpackages未安装，跳过索引创建"

echo ""
echo "=== 最终目录内容 ==="
ls -lh

ENDSSH

echo ""
echo "✅ 41主机本地包仓库设置完成"
