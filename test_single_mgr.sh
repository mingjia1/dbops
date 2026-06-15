#!/bin/bash
cd d:/test_tmple/new_dbops/dbops

echo "=== 停止17号Agent和MySQL ==="
ssh root@10.1.81.17 "pkill -9 -f dbops-agent; pkill -9 mysqld; rm -rf /data/mysql/3307; sleep 2; rm -f /opt/dbops-agent/agent"

echo "=== 传输新Agent ==="
scp agent/agent root@10.1.81.17:/opt/dbops-agent/

echo "=== 启动Agent ==="
ssh root@10.1.81.17 "chmod +x /opt/dbops-agent/agent && cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
sleep 3

echo "=== 验证Agent ==="
curl -s http://10.1.81.17:9090/health | grep -q ok && echo "✓ Agent OK" || echo "✗ Agent Failed"

echo ""
echo "=== 部署MGR单节点测试 ==="
curl -X POST http://10.1.81.17:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16" \
  -d '{
    "config": {
      "mysql_version": "8.0.32",
      "port": 3307,
      "data_dir": "/data/mysql/3307",
      "mysql_user": "root",
      "mysql_pass": "root123",
      "install_type": "mgr",
      "is_primary": true,
      "group_name": "test-final-uuid-99999999",
      "local_address": "10.1.81.17:33071",
      "seeds": "10.1.81.17:33071"
    }
  }'

echo ""
echo ""
echo "=== 等待部署完成 ==="
sleep 20

echo "=== 验证MySQL启动 ==="
ssh root@10.1.81.17 "ps aux | grep mysqld | grep 3307"

echo ""
echo "=== 验证MGR状态 ==="
ssh root@10.1.81.17 "mysql -h127.0.0.1 -P3307 -uroot -proot123 -e 'SELECT MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members;' 2>&1"
