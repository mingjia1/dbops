#!/bin/bash

echo "=== 更新16号Agent ==="
ssh root@10.1.81.16 "pkill -9 -f dbops-agent; pkill -9 mysqld; rm -rf /data/mysql/3306; sleep 2; rm -f /opt/dbops-agent/agent"
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.16:/opt/dbops-agent/
ssh root@10.1.81.16 "chmod +x /opt/dbops-agent/agent && cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3
curl -s http://10.1.81.16:9090/health | grep -q ok && echo "✓ 16 OK" || echo "✗ 16 Failed"

echo ""
echo "=== 更新17号Agent ==="
ssh root@10.1.81.17 "pkill -9 -f dbops-agent; pkill -9 mysqld; rm -rf /data/mysql/3307; sleep 2; rm -f /opt/dbops-agent/agent"
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.17:/opt/dbops-agent/
ssh root@10.1.81.17 "chmod +x /opt/dbops-agent/agent && cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3
curl -s http://10.1.81.17:9090/health | grep -q ok && echo "✓ 17 OK" || echo "✗ 17 Failed"

echo ""
echo "=== 更新18号Agent ==="
ssh root@10.1.81.18 "pkill -9 -f dbops-agent; pkill -9 mysqld; rm -rf /data/mysql/3308; sleep 2; rm -f /opt/dbops-agent/agent"
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.18:/opt/dbops-agent/
ssh root@10.1.81.18 "chmod +x /opt/dbops-agent/agent && cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3
curl -s http://10.1.81.18:9090/health | grep -q ok && echo "✓ 18 OK" || echo "✗ 18 Failed"

echo ""
echo "✓ 所有Agent已更新"
