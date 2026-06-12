#!/bin/bash
# 测试集群完整移除流程：备份→验证→销毁

API_BASE="http://localhost:8080/api"
TOKEN=""  # 需要填入有效的JWT token

# 1. 获取集群信息
echo "=== 1. 获取 ha57 集群信息 ==="
CLUSTER_ID=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_BASE/deployments" | jq -r '.data[] | select(.name | contains("ha57")) | .deployment_id' | head -1)

if [ -z "$CLUSTER_ID" ]; then
    echo "❌ 未找到 ha57 集群"
    exit 1
fi

echo "✓ 集群ID: $CLUSTER_ID"

# 2. 获取集群实例
echo -e "\n=== 2. 获取集群实例 ==="
INSTANCES=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_BASE/instances?cluster_id=$CLUSTER_ID")
echo "$INSTANCES" | jq -r '.data[] | "\(.id) - \(.name) (\(.connection.host):\(.connection.port))"'

INSTANCE_IDS=$(echo "$INSTANCES" | jq -r '.data[].id')
INSTANCE_COUNT=$(echo "$INSTANCE_IDS" | wc -w)
echo "✓ 找到 $INSTANCE_COUNT 个实例"

# 3. 为每个实例执行备份
echo -e "\n=== 3. 执行完整备份 ==="
for INST_ID in $INSTANCE_IDS; do
    echo "备份实例: $INST_ID"
    BACKUP_RESULT=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"instance_id\":\"$INST_ID\",\"backup_type\":\"full\"}" \
        "$API_BASE/backups/execute")

    BACKUP_ID=$(echo "$BACKUP_RESULT" | jq -r '.data.id // empty')
    BACKUP_STATUS=$(echo "$BACKUP_RESULT" | jq -r '.data.status // empty')

    if [ "$BACKUP_STATUS" = "success" ]; then
        echo "  ✓ 备份成功: $BACKUP_ID"
    else
        echo "  ❌ 备份失败: $(echo "$BACKUP_RESULT" | jq -r '.message')"
        echo "  详情: $(echo "$BACKUP_RESULT" | jq '.')"
        exit 1
    fi
done

# 4. 验证备份文件
echo -e "\n=== 4. 验证备份可用性 ==="
for INST_ID in $INSTANCE_IDS; do
    LATEST_BACKUP=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_BASE/backups?instance_id=$INST_ID&limit=1")
    BACKUP_PATH=$(echo "$LATEST_BACKUP" | jq -r '.data[0].file_path // empty')
    BACKUP_SIZE=$(echo "$LATEST_BACKUP" | jq -r '.data[0].file_size // 0')
    CHECKSUM=$(echo "$LATEST_BACKUP" | jq -r '.data[0].checksum // empty')

    if [ -n "$BACKUP_PATH" ] && [ "$BACKUP_SIZE" -gt 0 ] && [ -n "$CHECKSUM" ]; then
        echo "  ✓ 实例 $INST_ID 备份验证通过"
        echo "    路径: $BACKUP_PATH"
        echo "    大小: $BACKUP_SIZE bytes"
        echo "    校验: $CHECKSUM"
    else
        echo "  ❌ 实例 $INST_ID 备份验证失败"
        exit 1
    fi
done

# 5. 执行集群销毁
echo -e "\n=== 5. 销毁集群 ==="
read -p "确认销毁集群 $CLUSTER_ID? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "取消销毁操作"
    exit 0
fi

DESTROY_RESULT=$(curl -s -X DELETE -H "Authorization: Bearer $TOKEN" "$API_BASE/deployments/$CLUSTER_ID")
DESTROY_STATUS=$(echo "$DESTROY_RESULT" | jq -r '.data.status // empty')

if [ "$DESTROY_STATUS" = "destroyed" ]; then
    echo "✓ 集群销毁成功"
    echo "$(echo "$DESTROY_RESULT" | jq -r '.data.message')"
else
    echo "❌ 集群销毁失败"
    echo "$(echo "$DESTROY_RESULT" | jq '.')"
    exit 1
fi

echo -e "\n=== 完成 ==="
echo "集群 $CLUSTER_ID 已完成备份并成功销毁"
