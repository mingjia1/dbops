#!/bin/bash
# 在16主机建立本地包仓库

echo "=== 在16上建立本地包仓库 ==="

# 创建包目录
mkdir -p /opt/packet/mysql8.0
cd /opt/packet/mysql8.0

# 解压包
echo "解压包到 /opt/packet/mysql8.0 ..."
tar xzf /tmp/mysql_packages.tar.gz

echo ""
echo "=== 包列表 ==="
ls -lh *.deb | wc -l
echo "个包文件"
ls -lh *.deb

echo ""
echo "=== 创建包索引 ==="
dpkg-scanpackages . /dev/null 2>/dev/null | gzip -9c > Packages.gz
ls -lh Packages.gz

echo ""
echo "=== 配置本地APT源 ==="
echo "deb [trusted=yes] file:///opt/packet/mysql8.0 ./" > /etc/apt/sources.list.d/local-mysql.list
cat /etc/apt/sources.list.d/local-mysql.list

echo ""
echo "=== 更新APT缓存 ==="
apt-get update -qq 2>&1 | tail -3

echo ""
echo "=== 验证本地源可用 ==="
apt-cache policy mysql-common | head -5

echo ""
echo "✅ 16主机本地包仓库建立完成"
echo "路径: /opt/packet/mysql8.0"
ls -1 *.deb 2>/dev/null | wc -l | xargs echo "包数量:"
du -sh . | cut -f1 | xargs echo "总大小:"
