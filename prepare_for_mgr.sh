#!/bin/bash
echo "=== 清理16/17/18的MySQL ==="
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306"
ssh root@10.1.81.17 "pkill -9 mysqld; rm -rf /data/mysql/3306"
ssh root@10.1.81.18 "pkill -9 mysqld; rm -rf /data/mysql/3306"

echo ""
echo "=== 完全重启Agent ==="
ssh root@10.1.81.16 "pkill -9 agent; sleep 2; cd /opt/dbops-agent && nohup ./agent > agent-clean.log 2>&1 &"
ssh root@10.1.81.17 "pkill -9 agent; sleep 2; cd /opt/dbops-agent && nohup ./agent > agent-clean.log 2>&1 &"
ssh root@10.1.81.18 "pkill -9 agent; sleep 2; cd /opt/dbops-agent && nohup ./agent > agent-clean.log 2>&1 &"

sleep 3

echo ""
echo "=== 验证Agent ==="
curl -s http://10.1.81.16:9090/health
echo ""
curl -s http://10.1.81.17:9090/health
echo ""
curl -s http://10.1.81.18:9090/health

echo ""
echo "✅ 准备就绪"
