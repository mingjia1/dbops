#!/bin/bash
cd d:/test_tmple/new_dbops/dbops/agent

echo "=== 编译Linux版本Agent ==="
GOOS=linux GOARCH=amd64 go build -o agent cmd/main.go
ls -lh agent

echo ""
echo "=== 停止Agent ==="
ssh root@10.1.81.16 "pkill -9 agent"

echo ""
echo "=== 传输Agent ==="
scp agent root@10.1.81.16:/opt/dbops-agent/

echo ""
echo "=== 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && chmod +x agent && nohup ./agent > agent-clean.log 2>&1 &"

sleep 3

echo ""
echo "=== 验证Agent ==="
curl -s http://10.1.81.16:9090/health

echo ""
echo "=== 清理MySQL ==="
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306"

echo ""
echo "✅ 准备完成"
