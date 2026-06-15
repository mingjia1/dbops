#!/bin/bash

echo "=== 强制停止所有Agent进程 ==="
ssh root@10.1.81.16 "pkill -9 -f 'dbops-agent/agent'; killall -9 agent 2>/dev/null || true" &
ssh root@10.1.81.17 "pkill -9 -f 'dbops-agent/agent'; killall -9 agent 2>/dev/null || true" &
ssh root@10.1.81.18 "pkill -9 -f 'dbops-agent/agent'; killall -9 agent 2>/dev/null || true" &
wait

sleep 5

echo ""
echo "=== 清理MySQL进程和数据 ==="
ssh root@10.1.81.16 "pkill -9 mysqld 2>/dev/null; rm -rf /data/mysql/3306" &
ssh root@10.1.81.17 "pkill -9 mysqld 2>/dev/null; rm -rf /data/mysql/3307" &
ssh root@10.1.81.18 "pkill -9 mysqld 2>/dev/null; rm -rf /data/mysql/3308" &
wait

echo ""
echo "=== 传输新版Agent ==="
ssh root@10.1.81.16 "rm -f /opt/dbops-agent/agent" &
ssh root@10.1.81.17 "rm -f /opt/dbops-agent/agent" &
ssh root@10.1.81.18 "rm -f /opt/dbops-agent/agent" &
wait

scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.16:/opt/dbops-agent/ &
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.17:/opt/dbops-agent/ &
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.18:/opt/dbops-agent/ &
wait

sleep 2

echo ""
echo "=== 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &" &
ssh root@10.1.81.17 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &" &
ssh root@10.1.81.18 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &" &
wait

sleep 6

echo ""
echo "=== 验证Agent健康状态 ==="
curl -s http://10.1.81.16:9090/health | grep -q '"status":"ok"' && echo "✓ 16 OK" || echo "✗ 16 Failed"
curl -s http://10.1.81.17:9090/health | grep -q '"status":"ok"' && echo "✓ 17 OK" || echo "✗ 17 Failed"
curl -s http://10.1.81.18:9090/health | grep -q '"status":"ok"' && echo "✓ 18 OK" || echo "✗ 18 Failed"

echo ""
echo "✓ 环境准备完成"
