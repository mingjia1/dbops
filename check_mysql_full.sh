#!/bin/bash
echo "=== 检查MySQL进程 ==="
ssh root@10.1.81.16 "ps aux | grep mysqld | grep 3306 | grep -v grep"

if [ $? -eq 0 ]; then
    echo ""
    echo "=== MySQL正在运行，测试连接 ==="
    ssh root@10.1.81.16 "mysql -S /data/mysql/3306/mysql.sock -e 'SELECT VERSION()'"
    echo ""
    echo "=== 测试TCP连接(密码root) ==="
    ssh root@10.1.81.16 "MYSQL_PWD=root mysql -h 127.0.0.1 -P 3306 -uroot -e 'SELECT VERSION()'"
    echo ""
    echo "=== 测试mysqladmin ping ==="
    ssh root@10.1.81.16 "MYSQL_PWD=root mysqladmin -h 127.0.0.1 -P 3306 -uroot ping"
else
    echo "MySQL未运行"
fi
