#!/bin/bash
cd d:/test_tmple/new_dbops/dbops/agent

echo "=== 编译Linux版本Agent ==="
GOOS=linux GOARCH=amd64 go build -o agent cmd/main.go
ls -lh agent

echo ""
echo "=== 停止16/17/18上的旧Agent ==="
ssh root@10.1.81.16 "pkill -9 agent"
ssh root@10.1.81.17 "pkill -9 agent"
ssh root@10.1.81.18 "pkill -9 agent"

echo ""
echo "=== 传输新Agent ==="
scp agent root@10.1.81.16:/opt/dbops-agent/
scp agent root@10.1.81.17:/opt/dbops-agent/
scp agent root@10.1.81.18:/opt/dbops-agent/

echo ""
echo "=== 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && chmod +x agent && nohup ./agent > agent-clean.log 2>&1 &"
ssh root@10.1.81.17 "cd /opt/dbops-agent && chmod +x agent && nohup ./agent > agent-clean.log 2>&1 &"
ssh root@10.1.81.18 "cd /opt/dbops-agent && chmod +x agent && nohup ./agent > agent-clean.log 2>&1 &"

sleep 3

echo ""
echo "=== 验证Agent状态 ==="
curl -s http://10.1.81.16:9090/health
echo ""
curl -s http://10.1.81.17:9090/health
echo ""
curl -s http://10.1.81.18:9090/health

echo ""
echo "✅ Agent更新完成"
