#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
修复并重新部署MGR集群
解决密码设置和MGR插件问题
"""

import subprocess
import sys
import time

def run(cmd, timeout=30):
    """执行命令"""
    try:
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except Exception as e:
        return -1, "", str(e)

print("="*60)
print("  修复并重新部署MGR集群")
print("="*60)

# 步骤1: 清理所有节点
print("\n[1/4] 清理所有节点...")
for host in ['16', '17', '18']:
    print(f"  清理 10.1.81.{host}...")
    code, out, err = run(f'ssh root@10.1.81.{host} "pkill -9 mysqld; rm -rf /data/mysql/*; mkdir -p /data/mysql; chown mysql:mysql /data/mysql"')
    print(f"    {'OK' if code == 0 else '!'}")

time.sleep(3)

# 步骤2: 重新部署（使用Python requests更可靠）
print("\n[2/4] 重新部署MGR集群...")

import json
import tempfile

token = "dev-agent-token-CHANGE-ME-at-least-16"

# 主节点
print("  部署主节点 10.1.81.16:3306...")
payload1 = {
    "config": {
        "port": 3306,
        "data_dir": "/data/mysql/3306",
        "mysql_user": "root",
        "mysql_pass": "root123",
        "install_type": "mgr",
        "is_primary": True,
        "group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "local_address": "10.1.81.16:33061",
        "seeds": "10.1.81.16:33061,10.1.81.17:33071,10.1.81.18:33081"
    }
}

with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
    json.dump(payload1, f)
    tmpfile = f.name

code, out, err = run(f'curl -X POST http://10.1.81.16:9090/agent/tasks/deploy -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d @{tmpfile}', timeout=120)
import os
os.unlink(tmpfile)

print(f"    响应: {out[:200]}")
print("    等待60秒...")
time.sleep(60)

# 副本1
print("  部署副本节点1 10.1.81.17:3307...")
payload2 = payload1.copy()
payload2["config"]["port"] = 3307
payload2["config"]["data_dir"] = "/data/mysql/3307"
payload2["config"]["is_primary"] = False
payload2["config"]["local_address"] = "10.1.81.17:33071"

with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
    json.dump(payload2, f)
    tmpfile = f.name

code, out, err = run(f'curl -X POST http://10.1.81.17:9090/agent/tasks/deploy -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d @{tmpfile}', timeout=120)
os.unlink(tmpfile)

print(f"    响应: {out[:200]}")
print("    等待60秒...")
time.sleep(60)

# 副本2
print("  部署副本节点2 10.1.81.18:3308...")
payload3 = payload1.copy()
payload3["config"]["port"] = 3308
payload3["config"]["data_dir"] = "/data/mysql/3308"
payload3["config"]["is_primary"] = False
payload3["config"]["local_address"] = "10.1.81.18:33081"

with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
    json.dump(payload3, f)
    tmpfile = f.name

code, out, err = run(f'curl -X POST http://10.1.81.18:9090/agent/tasks/deploy -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d @{tmpfile}', timeout=120)
os.unlink(tmpfile)

print(f"    响应: {out[:200]}")
print("    等待60秒...")
time.sleep(60)

# 步骤3: 手动修复密码和MGR（如果Agent失败）
print("\n[3/4] 检查并修复...")

# 检查16号主节点
code, out, err = run('ssh root@10.1.81.16 "mysql -S /data/mysql/3306/mysql.sock -uroot -proot123 -e \'SELECT 1\' 2>&1"', timeout=10)
if 'Access denied' in out or 'Access denied' in err:
    print("  16号节点需要设置密码...")
    run('ssh root@10.1.81.16 "mysql -S /data/mysql/3306/mysql.sock -uroot -e \\"ALTER USER \'root\'@\'localhost\' IDENTIFIED BY \'root123\'; FLUSH PRIVILEGES;\\""')

    # 手动初始化MGR
    print("  16号节点手动初始化MGR...")
    run('ssh root@10.1.81.16 "mysql -S /data/mysql/3306/mysql.sock -uroot -proot123 -e \\"SET SQL_LOG_BIN=0; CREATE USER IF NOT EXISTS \'repl\'@\'%\' IDENTIFIED BY \'repl\'; GRANT REPLICATION SLAVE, CONNECTION_ADMIN, BACKUP_ADMIN, GROUP_REPLICATION_STREAM ON *.* TO \'repl\'@\'%\'; FLUSH PRIVILEGES; SET SQL_LOG_BIN=1; SET GLOBAL super_read_only=0; SET GLOBAL read_only=0; INSTALL PLUGIN group_replication SONAME \'group_replication.so\'; SET GLOBAL group_replication_bootstrap_group=ON; START GROUP_REPLICATION; SET GLOBAL group_replication_bootstrap_group=OFF;\\" 2>&1"', timeout=30)

# 步骤4: 验证
print("\n[4/4] 验证MGR集群状态...")
code, out, err = run('ssh root@10.1.81.16 "mysql -h127.0.0.1 -P3306 -uroot -proot123 -e \'SELECT MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members ORDER BY MEMBER_HOST;\' 2>&1 | grep -v Warning"', timeout=30)

print(f"\n{out}")

if 'ONLINE' in out:
    online_count = out.count('ONLINE')
    primary_count = out.count('PRIMARY')
    print(f"\n>>> 检测到 {online_count} 个ONLINE节点, {primary_count} 个PRIMARY")
    if online_count >= 1:
        print(">>> MGR集群至少部分成功！")
        sys.exit(0)

print("\n需要手动检查和修复。")
sys.exit(1)
