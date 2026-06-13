#!/bin/bash
echo "=== 清理16 ==="
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306"
echo "16清理完成"

echo "=== 清理17 ==="
ssh root@10.1.81.17 "pkill -9 mysqld; rm -rf /data/mysql/3306"
echo "17清理完成"

echo "=== 清理18 ==="
ssh root@10.1.81.18 "pkill -9 mysqld; rm -rf /data/mysql/3306"
echo "18清理完成"

echo ""
echo "✅ 所有主机MySQL已清理"
