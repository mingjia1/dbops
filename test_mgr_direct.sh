#!/bin/bash

echo "=== 直接通过Agent部署MGR节点 ==="

# Node 1 (Primary) - 10.1.81.16:3306
echo "--- 部署主节点 (10.1.81.16:3306) ---"
curl -X POST http://10.1.81.16:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16" \
  -d '{
    "mysql_version": "8.0.32",
    "port": 3306,
    "data_dir": "/data/mysql/3306",
    "user": "root",
    "password": "root123",
    "config": {
      "install_type": "mgr",
      "is_primary": true,
      "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "local_address": "10.1.81.16:33061",
      "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
  }' 2>&1 | tail -3

sleep 5

# Node 2 (Replica) - 10.1.81.17:3307
echo ""
echo "--- 部署副本节点1 (10.1.81.17:3307) ---"
curl -X POST http://10.1.81.17:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16" \
  -d '{
    "mysql_version": "8.0.32",
    "port": 3307,
    "data_dir": "/data/mysql/3307",
    "user": "root",
    "password": "root123",
    "config": {
      "install_type": "mgr",
      "is_primary": false,
      "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "local_address": "10.1.81.17:33071",
      "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
  }' 2>&1 | tail -3

sleep 5

# Node 3 (Replica) - 10.1.81.18:3308
echo ""
echo "--- 部署副本节点2 (10.1.81.18:3308) ---"
curl -X POST http://10.1.81.18:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16" \
  -d '{
    "mysql_version": "8.0.32",
    "port": 3308,
    "data_dir": "/data/mysql/3308",
    "user": "root",
    "password": "root123",
    "config": {
      "install_type": "mgr",
      "is_primary": false,
      "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "local_address": "10.1.81.18:33081",
      "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
  }' 2>&1 | tail -3

echo ""
echo "=== 等待部署完成 ==="
sleep 45

echo ""
echo "=== 检查MGR集群状态 ==="
ssh root@10.1.81.16 "mysql -h127.0.0.1 -P3306 -uroot -proot123 -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members;' 2>&1"

echo ""
echo "=== 验证MGR插件加载 ==="
ssh root@10.1.81.16 "mysql -h127.0.0.1 -P3306 -uroot -proot123 -e 'SHOW PLUGINS LIKE \"group_replication\";' 2>&1"

echo ""
echo "=== 检查Agent日志 ==="
echo "--- Node 1 stderr ---"
ssh root@10.1.81.16 "tail -20 /opt/dbops-agent/stderr.log"
