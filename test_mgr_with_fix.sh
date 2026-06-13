#!/bin/bash
echo "=== 1. 停止Agent和MySQL ==="
ssh root@10.1.81.16 "killall -9 agent mysqld 2>/dev/null; rm -rf /data/mysql/3306"
ssh root@10.1.81.17 "killall -9 agent mysqld 2>/dev/null; rm -rf /data/mysql/3307 /data/mysql/3308"

echo ""
echo "=== 2. 部署新Agent ==="
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.16:/opt/dbops-agent/
scp d:/test_tmple/new_dbops/dbops/agent/agent root@10.1.81.17:/opt/dbops-agent/

echo ""
echo "=== 3. 启动Agent ==="
ssh root@10.1.81.16 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"
ssh root@10.1.81.17 "cd /opt/dbops-agent && nohup ./agent > stdout.log 2> stderr.log &"

sleep 5

echo ""
echo "=== 4. 验证Agent ==="
curl -s http://10.1.81.16:9090/health | grep -q ok && echo "✓ 16 Agent OK"
curl -s http://10.1.81.17:9090/health | grep -q ok && echo "✓ 17 Agent OK"

echo ""
echo "=== 5. 部署MGR集群 ==="
cd d:/test_tmple/new_dbops/dbops
go run test_mgr_3nodes.go

echo ""
echo "=== 6. 验证MGR状态 ==="
sleep 3
echo "--- 主节点 (16:3306) ---"
ssh root@10.1.81.16 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3306 -uroot -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE FROM performance_schema.replication_group_members'"

echo ""
echo "--- 副本1 (17:3307) ---"
ssh root@10.1.81.17 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3307 -uroot -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE FROM performance_schema.replication_group_members'"

echo ""
echo "--- 副本2 (17:3308) ---"
ssh root@10.1.81.17 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3308 -uroot -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE FROM performance_schema.replication_group_members'"

echo ""
echo "=== 完成 ==="
