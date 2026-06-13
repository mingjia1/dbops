#!/bin/bash
cd d:/test_tmple/new_dbops/dbops/agent
echo "=== 编译Agent ==="
GOOS=linux GOARCH=amd64 go build -o agent cmd/main.go
echo "编译完成"

ssh root@10.1.81.16 "pkill -9 agent; pkill -9 mysqld"
sleep 2
ssh root@10.1.81.16 "rm -rf /data/mysql/3306"

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
