#!/bin/bash
cd d:/test_tmple/new_dbops/dbops

echo "=== 添加MGR修复代码 ==="
git add agent/internal/executor/task_executor.go

echo ""
echo "=== 提交 ==="
git commit -m "feat: implement MGR plugin loading and initialization

- Add MGR configuration parameters parsing (install_type, is_primary, group_name, local_address, seeds)
- Add MGR-specific MySQL startup parameters (plugin-load-add, group_replication_*)
- Implement initializeMGR() function to:
  * Create replication user with proper privileges
  * Bootstrap group on primary node
  * Start group replication on replica nodes
  * Verify MGR member state is ONLINE
- Initialize MGR after MySQL health check passes

Changes:
- Read MGR config from request: install_type, is_primary, group_name, local_address, seeds
- Add --plugin-load-add=group_replication.so to MySQL startup
- Add group_replication_group_name, local_address, seeds to startup params
- Create repl user with REPLICATION SLAVE, CONNECTION_ADMIN, BACKUP_ADMIN, GROUP_REPLICATION_STREAM
- Use CHANGE REPLICATION SOURCE TO for recovery channel
- Bootstrap primary with SET GLOBAL group_replication_bootstrap_group=ON
- Wait 3s and verify MEMBER_STATE=ONLINE

Next: Add port and file path conflict detection"

echo ""
echo "=== 推送到远程 ==="
git push origin master

echo ""
echo "=== 最近提交 ==="
git log -1 --oneline
