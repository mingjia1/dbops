#!/bin/bash
cd d:/test_tmple/new_dbops/dbops/agent
echo "=== 编译Agent ==="
go build -o agent.exe cmd/main.go
ls -lh agent.exe

echo ""
echo "=== 停止16/17/18上的Agent ==="
ssh root@10.1.81.16 "pkill -9 agent"
ssh root@10.1.81.17 "pkill -9 agent"
ssh root@10.1.81.18 "pkill -9 agent"

echo ""
echo "=== 传输新Agent到16/17/18 ==="
scp agent.exe root@10.1.81.16:/opt/dbops-agent/agent
scp agent.exe root@10.1.81.17:/opt/dbops-agent/agent
scp agent.exe root@10.1.81.18:/opt/dbops-agent/agent

echo ""
echo "=== 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && nohup ./agent > agent-clean.log 2>&1 &"
ssh root@10.1.81.17 "cd /opt/dbops-agent && nohup ./agent > agent-clean.log 2>&1 &"
ssh root@10.1.81.18 "cd /opt/dbops-agent && nohup ./agent > agent-clean.log 2>&1 &"

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
