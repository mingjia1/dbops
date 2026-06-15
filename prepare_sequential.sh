#!/bin/bash

echo "=== 16号机器 ==="
ssh root@10.1.81.16 "pkill -9 -f dbops-agent; sleep 2; rm -f /opt/dbops-agent/agent"
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.16:/opt/dbops-agent/
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306; cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3
curl -s http://10.1.81.16:9090/health | grep -q ok && echo "✓ 16 OK" || echo "✗ 16 Failed"

echo ""
echo "=== 17号机器 ==="
ssh root@10.1.81.17 "pkill -9 -f dbops-agent; sleep 2; rm -f /opt/dbops-agent/agent"
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.17:/opt/dbops-agent/
ssh root@10.1.81.17 "pkill -9 mysqld; rm -rf /data/mysql/3307; cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3
curl -s http://10.1.81.17:9090/health | grep -q ok && echo "✓ 17 OK" || echo "✗ 17 Failed"

echo ""
echo "=== 18号机器 ==="
ssh root@10.1.81.18 "pkill -9 -f dbops-agent; sleep 2; rm -f /opt/dbops-agent/agent"
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.18:/opt/dbops-agent/
ssh root@10.1.81.18 "pkill -9 mysqld; rm -rf /data/mysql/3308; cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3
curl -s http://10.1.81.18:9090/health | grep -q ok && echo "✓ 18 OK" || echo "✗ 18 Failed"

echo ""
echo "✓ 环境准备完成"
