#!/bin/bash
cd d:/test_tmple/new_dbops/dbops/agent
echo "=== 编译 ==="
GOOS=linux GOARCH=amd64 go build -o agent cmd/main.go
ls -lh agent

echo ""
echo "=== 停止服务 ==="
ssh root@10.1.81.16 "pkill -9 agent; pkill -9 mysqld; rm -rf /data/mysql/3306"
sleep 2

echo ""
echo "=== 传输Agent ==="
scp agent root@10.1.81.16:/opt/dbops-agent/

echo ""
echo "=== 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && nohup ./agent > agent.log 2>&1 &"
sleep 4

echo ""
echo "=== 测试部署 ==="
cd ../platform-backend
go run ../test_agent_deploy_16.go
