#!/bin/bash
cd d:/test_tmple/new_dbops/dbops

echo "=== 添加修改的文件 ==="
git add agent/internal/executor/task_executor.go
git add docs/FULL_FLOW_TEST_REPORT.md

echo ""
echo "=== 提交 ==="
git commit -m "fix: resolve MySQL deployment password and health check issues

- Change default host from 'localhost' to '127.0.0.1' to force IPv4
- Add password retry logic via socket connection when health check fails
- Fix DEBUG logging to show received mysql_user and mysql_pass
- Verify single instance and 3-node deployment success

Related issues:
- localhost resolved to IPv6 ::1 but MySQL binds to IPv4
- Password setting command succeeded but didn't take effect
- Health check failed due to wrong host resolution

Test results:
- Single MySQL instance: deployed in 12-17s
- 3-node deployment: all instances running
- Password authentication: TCP connection works
- MGR plugin: not loaded (next step)"

echo ""
echo "=== 推送到远程 ==="
git push origin master

echo ""
echo "=== 最近的提交 ==="
git log -1 --oneline
