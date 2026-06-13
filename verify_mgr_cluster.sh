#!/bin/bash
echo "=== 1. 为16:3306设置密码 ==="
ssh root@10.1.81.16 "mysql -S /data/mysql/3306/mysql.sock -e \"ALTER USER 'root'@'localhost' IDENTIFIED BY 'root'; CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY 'root'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES;\""
echo "✓ 16:3306密码设置完成"

echo ""
echo "=== 2. 为17:3307设置密码 ==="
ssh root@10.1.81.17 "mysql -S /data/mysql/3307/mysql.sock -e \"ALTER USER 'root'@'localhost' IDENTIFIED BY 'root'; CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY 'root'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES;\""
echo "✓ 17:3307密码设置完成"

echo ""
echo "=== 3. 为17:3308设置密码 ==="
ssh root@10.1.81.17 "mysql -S /data/mysql/3308/mysql.sock -e \"ALTER USER 'root'@'localhost' IDENTIFIED BY 'root'; CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY 'root'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES;\""
echo "✓ 17:3308密码设置完成"

echo ""
echo "=== 4. 验证MGR集群状态 ==="
echo "--- 16:3306 ---"
ssh root@10.1.81.16 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3306 -uroot -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE FROM performance_schema.replication_group_members'"

echo ""
echo "--- 17:3307 ---"
ssh root@10.1.81.17 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3307 -uroot -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE FROM performance_schema.replication_group_members'"

echo ""
echo "--- 17:3308 ---"
ssh root@10.1.81.17 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3308 -uroot -e 'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE FROM performance_schema.replication_group_members'"
