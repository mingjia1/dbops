#!/bin/bash
cd d:/test_tmple/new_dbops/dbops/agent
GOOS=linux GOARCH=amd64 go build -o agent cmd/main.go
echo "✓ 编译完成"

ssh root@10.1.81.16 "killall -9 agent mysqld 2>/dev/null; rm -rf /data/mysql/3306"
sleep 2

scp -q agent root@10.1.81.16:/opt/dbops-agent/
ssh root@10.1.81.16 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 4

curl -s http://10.1.81.16:9090/health | grep -q ok && echo "✓ Agent启动"

cd ../platform-backend
go run ../test_agent_deploy_16.go
