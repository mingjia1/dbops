#!/bin/bash
# 完整的MGR集群部署测试脚本

BACKEND="http://localhost:8080"

echo "================================================"
echo "步骤1: 检查Backend健康状态"
echo "================================================"
curl -s $BACKEND/health | jq .

echo ""
echo "================================================"
echo "步骤2: 检查所有Agent健康状态"
echo "================================================"
for host in 16 17 18; do
  echo -n "10.1.81.$host: "
  curl -s http://10.1.81.$host:9090/health | jq -r '.data.status'
done

echo ""
echo "================================================"
echo "步骤3: 直接通过Agent API部署3节点MGR集群"
echo "================================================"

TOKEN="dev-agent-token-CHANGE-ME-at-least-16"

# 部署主节点 (10.1.81.16:3306)
echo "部署主节点 10.1.81.16:3306..."
curl -X POST http://10.1.81.16:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "config": {
      "port": 3306,
      "data_dir": "/data/mysql/3306",
      "mysql_user": "root",
      "mysql_pass": "root123",
      "install_type": "mgr",
      "is_primary": true,
      "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "local_address": "10.1.81.16:33061",
      "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
  }' | jq .

echo ""
echo "等待主节点启动..."
sleep 40

# 部署副本节点1 (10.1.81.17:3307)
echo ""
echo "部署副本节点1 10.1.81.17:3307..."
curl -X POST http://10.1.81.17:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "config": {
      "port": 3307,
      "data_dir": "/data/mysql/3307",
      "mysql_user": "root",
      "mysql_pass": "root123",
      "install_type": "mgr",
      "is_primary": false,
      "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "local_address": "10.1.81.17:33071",
      "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
  }' | jq .

echo ""
echo "等待副本节点1启动..."
sleep 40

# 部署副本节点2 (10.1.81.18:3308)
echo ""
echo "部署副本节点2 10.1.81.18:3308..."
curl -X POST http://10.1.81.18:9090/agent/tasks/deploy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "config": {
      "port": 3308,
      "data_dir": "/data/mysql/3308",
      "mysql_user": "root",
      "mysql_pass": "root123",
      "install_type": "mgr",
      "is_primary": false,
      "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "local_address": "10.1.81.18:33081",
      "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
  }' | jq .

echo ""
echo "等待副本节点2启动..."
sleep 40

echo ""
echo "================================================"
echo "步骤4: 验证MGR集群状态"
echo "================================================"

echo "检查主节点 10.1.81.16:3306:"
ssh root@10.1.81.16 "mysql -h127.0.0.1 -P3306 -uroot -proot123 -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members ORDER BY MEMBER_HOST;' 2>&1 | grep -v Warning"

echo ""
echo "检查副本节点1 10.1.81.17:3307:"
ssh root@10.1.81.17 "mysql -h127.0.0.1 -P3307 -uroot -proot123 -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members ORDER BY MEMBER_HOST;' 2>&1 | grep -v Warning"

echo ""
echo "检查副本节点2 10.1.81.18:3308:"
ssh root@10.1.81.18 "mysql -h127.0.0.1 -P3308 -uroot -proot123 -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members ORDER BY MEMBER_HOST;' 2>&1 | grep -v Warning"

echo ""
echo "================================================"
echo "完成！"
echo "================================================"
