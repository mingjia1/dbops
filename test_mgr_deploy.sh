#!/bin/bash

echo "=== 部署3节点MGR集群 ==="
curl -X POST http://localhost:8080/api/v1/deployments/mgr \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mgr-test-cluster",
    "cluster_id": "mgr-test-001",
    "primary_host": "10.1.81.16",
    "primary_port": 3306,
    "replica_port": 3307,
    "secondary_hosts": [
      {
        "host": "10.1.81.17",
        "port": 3307,
        "agent_port": 9090
      },
      {
        "host": "10.1.81.18",
        "port": 3308,
        "agent_port": 9090
      }
    ],
    "group_mode": "single_primary",
    "mysql_user": "root",
    "mysql_password": "root123",
    "pseudo_mode": false
  }'

echo ""
echo ""
echo "=== 等待部署完成 ==="
sleep 30

echo "=== 检查MGR状态 ==="
echo "--- Node 1 (10.1.81.16:3306) ---"
mysql -h10.1.81.16 -P3306 -uroot -proot123 -e "SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members;"

echo ""
echo "--- Node 2 (10.1.81.17:3307) ---"
mysql -h10.1.81.17 -P3307 -uroot -proot123 -e "SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members;"

echo ""
echo "--- Node 3 (10.1.81.18:3308) ---"
mysql -h10.1.81.18 -P3308 -uroot -proot123 -e "SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members;"
