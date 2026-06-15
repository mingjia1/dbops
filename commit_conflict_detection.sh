#!/bin/bash
cd d:/test_tmple/new_dbops/dbops

echo "=== 添加修改的文件 ==="
git add platform-backend/internal/services/cluster_deploy_service.go

echo ""
echo "=== 提交 ==="
git commit -m "feat: add port conflict detection and auto-resolution for MGR deployment

- Add checkPortAndPathConflicts() to detect port conflicts with existing instances
- Add findAvailablePort() to find next available port on host
- Add autoResolveConflicts() to automatically resolve conflicts before deployment
- Integrate conflict detection into DeployMGR() flow
- Log conflict resolution actions for audit

Features:
- Query existing instances from database (paginated, limit 1000)
- Group instances by host for efficient lookup
- Check if requested port is already in use
- Suggest next available port starting from 3306
- Auto-update node configuration if conflict detected
- Add 'resource conflict detection' step in deployment progress

Note: DataDir conflict detection is TODO (requires Instance model enhancement)

This prevents deployment failures due to port conflicts and improves user experience"

echo ""
echo "=== 推送到远程 ==="
git push origin master

echo ""
echo "=== 最近提交 ==="
git log -3 --oneline
