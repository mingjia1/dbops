#!/bin/bash
echo "=== 1. 更新16的Agent ==="
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.16:/opt/dbops-agent/
ssh root@10.1.81.16 "killall -9 agent; cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"

echo ""
echo "=== 2. 更新17的Agent ==="
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.17:/opt/dbops-agent/
ssh root@10.1.81.17 "killall -9 agent; cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"

sleep 4

echo ""
echo "=== 3. 验证Agent ==="
curl -s http://10.1.81.16:9090/health | grep -q ok && echo "✓ 16 Agent OK"
curl -s http://10.1.81.17:9090/health | grep -q ok && echo "✓ 17 Agent OK"

echo ""
echo "=== 4. 清理旧MySQL ==="
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306"
ssh root@10.1.81.17 "pkill -9 mysqld; rm -rf /data/mysql/3307 /data/mysql/3308"

sleep 2
echo "✓ 准备完成"
