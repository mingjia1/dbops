#!/bin/bash
cd d:/test_tmple/new_dbops/dbops

for host in 16 17 18; do
  echo "=== 部署Agent到10.1.81.$host ==="
  ssh root@10.1.81.$host "pkill -9 -f dbops-agent; pkill -9 mysqld; rm -rf /data/mysql/*; sleep 2; rm -f /opt/dbops-agent/agent"
  scp agent/agent root@10.1.81.$host:/opt/dbops-agent/
  ssh root@10.1.81.$host "chmod +x /opt/dbops-agent/agent && cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
  sleep 2
  curl -s http://10.1.81.$host:9090/health | grep -q ok && echo "✓ 10.1.81.$host OK" || echo "✗ 10.1.81.$host Failed"
  echo ""
done

echo "✓ 所有Agent已部署"
