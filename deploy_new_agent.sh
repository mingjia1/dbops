#!/bin/bash
echo "=== 停止Agent ==="
ssh root@10.1.81.16 "killall -9 agent"
ssh root@10.1.81.17 "killall -9 agent"
sleep 3

echo "=== 传输Agent ==="
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.16:/opt/dbops-agent/
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.17:/opt/dbops-agent/

echo "=== 清理MySQL ==="
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306"
ssh root@10.1.81.17 "pkill -9 mysqld; rm -rf /data/mysql/3307 /data/mysql/3308"

echo "=== 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
ssh root@10.1.81.17 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 5

echo "=== 验证Agent ==="
curl -s http://10.1.81.16:9090/health | grep -q ok && echo "✓ 16 OK" || echo "✗ 16 Failed"
curl -s http://10.1.81.17:9090/health | grep -q ok && echo "✓ 17 OK" || echo "✗ 17 Failed"

echo "✓ 准备完成"
