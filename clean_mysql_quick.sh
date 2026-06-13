#!/bin/bash
echo "=== 清理16/17/18的MySQL ==="
ssh root@10.1.81.16 "pkill -9 mysqld; rm -rf /data/mysql/3306"
ssh root@10.1.81.17 "pkill -9 mysqld; rm -rf /data/mysql/3306"
ssh root@10.1.81.18 "pkill -9 mysqld; rm -rf /data/mysql/3306"
echo "清理完成"
