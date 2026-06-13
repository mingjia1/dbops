#!/bin/bash
echo "=== 检查MySQL进程 ==="
ssh root@10.1.81.16 "ps aux | grep mysqld | grep 3306 | grep -v grep"

echo ""
echo "=== 测试socket连接 ==="
ssh root@10.1.81.16 "mysql -S /data/mysql/3306/mysql.sock -e 'SELECT VERSION()' 2>&1"

echo ""
echo "=== 测试TCP连接(无密码) ==="
ssh root@10.1.81.16 "mysql -h 127.0.0.1 -P 3306 -uroot -e 'SELECT VERSION()' 2>&1"

echo ""
echo "=== 测试TCP连接(有密码) ==="
ssh root@10.1.81.16 "mysql -h 127.0.0.1 -P 3306 -uroot -proot -e 'SELECT VERSION()' 2>&1"

echo ""
echo "=== 测试mysqladmin ping ==="
ssh root@10.1.81.16 "MYSQL_PWD=root mysqladmin -h 127.0.0.1 -P 3306 -uroot ping 2>&1"
