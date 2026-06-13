#!/bin/bash
# MySQL Ops Platform - 完整集群安装流程测试
# 测试：主机注册 → 环境检查 → 工具安装 → 集群部署 → 验证

set -e
BASE_URL="http://127.0.0.1:8080"
TEST_PREFIX="test-$(date +%s)"

log() {
    echo -e "\n============================================"
    echo "【STEP $1】$2"
    echo "============================================"
}

api_call() {
    local method=$1
    local path=$2
    local data=$3
    local desc=$4
    
    echo "→ $desc"
    echo "  $method $path"
    
    if [ "$method" == "GET" ]; then
        response=$(curl -s -w "\n___STATUS___%{http_code}" "$BASE_URL$path" 2>/dev/null)
    else
        response=$(curl -s -w "\n___STATUS___%{http_code}" -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" "$BASE_URL$path" 2>/dev/null)
    fi
    
    status=$(echo "$response" | grep "___STATUS___" | tail -1)
    body=$(echo "$response" | sed '$d')
    
    code=$(echo "$status" | sed 's/___STATUS___//')
    
    echo "  HTTP Status: $code"
    if [ "$code" == "200" ] || [ "$code" == "201" ] || [ "$code" == "202" ]; then
        echo "  ✅ SUCCESS"
        return 0
    else
        echo "  ❌ FAILED: $body"
        return 1
    fi
}

# ==================== 步骤 1: 健康检查 ====================
log "1" "平台健康检查"
api_call "GET" "/health" "" "检查后端服务状态"
api_call "GET" "/api/v1/versions" "" "获取 MySQL 版本目录"

# ==================== 步骤 2: 注册测试主机 ====================
log "2" "注册测试主机 (模拟 3 节点)"

# 主机 1 - Master
api_call "POST" "/api/v1/hosts" "{
    \"name\": \"$TEST_PREFIX-master\",
    \"address\": \"192.168.1.100\",
    \"ssh_port\": 22,
    \"ssh_user\": \"root\",
    \"ssh_auth_method\": \"password\",
    \"agent_port\": 9090,
    \"os_type\": \"linux\",
    \"description\": \"Test master host\"
}" "注册 Master 主机"

# 主机 2 - Slave1
api_call "POST" "/api/v1/hosts" "{
    \"name\": \"$TEST_PREFIX-slave1\",
    \"address\": \"192.168.1.101\",
    \"ssh_port\": 22,
    \"ssh_user\": \"root\",
    \"agent_port\": 9090,
    \"os_type\": \"linux\"
}" "注册 Slave1 主机"

# 主机 3 - Slave2
api_call "POST" "/api/v1/hosts" "{
    \"name\": \"$TEST_PREFIX-slave2\",
    \"address\": \"192.168.1.102\",
    \"ssh_port\": 22,
    \"ssh_user\": \"root\",
    \"agent_port\": 9090,
    \"os_type\": \"linux\"
}" "注册 Slave2 主机"

# ==================== 步骤 3: 查询主机列表 ====================
log "3" "验证主机注册"
api_call "GET" "/api/v1/hosts?limit=100" "" "获取主机列表"

# ==================== 步骤 4: 环境检查 ====================
log "4" "执行环境检查"

api_call "POST" "/api/v1/env-checks" "{
    \"hosts\": [
        {\"host\": \"192.168.1.100\", \"port\": 22, \"username\": \"root\", \"password\": \"password\"},
        {\"host\": \"192.168.1.101\", \"port\": 22, \"username\": \"root\", \"password\": \"password\"},
        {\"host\": \"192.168.1.102\", \"port\": 22, \"username\": \"root\", \"password\": \"password\"}
    ]
}" "批量环境检查 (3 节点)"

# ==================== 步骤 5: 批量安装 Agent ====================
log "5" "批量安装 Agent"

api_call "POST" "/api/v1/hosts/batch-agent-action" "{
    \"host_ids\": [],
    \"action\": \"install\",
    \"hosts\": [
        {\"address\": \"192.168.1.100\", \"ssh_port\": 22, \"ssh_user\": \"root\", \"ssh_password\": \"password\"},
        {\"address\": \"192.168.1.101\", \"ssh_port\": 22, \"ssh_user\": \"root\", \"ssh_password\": \"password\"},
        {\"address\": \"192.168.1.102\", \"ssh_port\": 22, \"ssh_user\": \"root\", \"ssh_password\": \"password\"}
    ]
}" "批量安装 Agent (3 节点)"

# ==================== 步骤 6: 部署 HA 主从集群 ====================
log "6" "部署 HA 主从集群 (1 主 2 从)"

api_call "POST" "/api/v1/deployments/ha" "{
    \"cluster_id\": \"$TEST_PREFIX-ha-cluster\",
    \"name\": \"HA Test Cluster\",
    \"master_host\": \"192.168.1.100\",
    \"master_port\": 3306,
    \"replica_hosts\": [
        {\"host\": \"192.168.1.101\", \"port\": 3306},
        {\"host\": \"192.168.1.102\", \"port\": 3306}
    ],
    \"replication_user\": \"repl\",
    \"replication_password\": \"ReplPass#2024!\",
    \"mysql_version\": \"8.0.36\",
    \"root_password\": \"RootPass#2024!\",
    \"ssh_user\": \"root\",
    \"ssh_password\": \"password\"
}" "部署 HA 主从集群"

# ==================== 步骤 7: 查询部署进度 ====================
log "7" "查询部署进度"

api_call "GET" "/api/v1/deployments/$TEST_PREFIX-ha-cluster/progress" "" "获取 HA 部署进度"

# ==================== 步骤 8: 查询部署历史 ====================
log "8" "查询部署历史"

api_call "GET" "/api/v1/deployments/history?cluster_id=$TEST_PREFIX-ha-cluster" "" "获取部署历史"

# ==================== 步骤 9: 查询实例列表 ====================
log "9" "验证实例创建"

api_call "GET" "/api/v1/instances?limit=100" "" "获取实例列表"

# ==================== 步骤 10: 查询集群拓扑 ====================
log "10" "查询集群拓扑"

api_call "GET" "/api/v1/topology/clusters/$TEST_PREFIX-ha-cluster" "" "获取集群拓扑"
api_call "GET" "/api/v1/topology/clusters/$TEST_PREFIX-ha-cluster/graph" "" "获取拓扑图数据"

# ==================== 步骤 11: 部署 MGR 集群 ====================
log "11" "部署 MGR 集群 (多主模式)"

api_call "POST" "/api/v1/deployments/mgr" "{
    \"cluster_id\": \"$TEST_PREFIX-mgr-cluster\",
    \"name\": \"MGR Test Cluster\",
    \"cluster_type\": \"multi-primary\",
    \"nodes\": [
        {\"host\": \"192.168.1.110\", \"port\": 3306, \"role\": \"primary\"},
        {\"host\": \"192.168.1.111\", \"port\": 3306, \"role\": \"primary\"},
        {\"host\": \"192.168.1.112\", \"port\": 3306, \"role\": \"secondary\"}
    ],
    \"mysql_version\": \"8.0.36\",
    \"root_password\": \"RootPass#2024!\",
    \"ssh_user\": \"root\",
    \"ssh_password\": \"password\",
    \"group_name\": \"mgr_test_group\"
}" "部署 MGR 多主集群"

# ==================== 步骤 12: 部署 PXC 集群 ====================
log "12" "部署 PXC 集群"

api_call "POST" "/api/v1/deployments/pxc" "{
    \"cluster_id\": \"$TEST_PREFIX-pxc-cluster\",
    \"name\": \"PXC Test Cluster\",
    \"nodes\": [
        {\"host\": \"192.168.1.120\", \"port\": 3306},
        {\"host\": \"192.168.1.121\", \"port\": 3306},
        {\"host\": \"192.168.1.122\", \"port\": 3306}
    ],
    \"cluster_name\": \"pxc_test\",
    \"mysql_version\": \"8.0.36\",
    \"root_password\": \"RootPass#2024!\",
    \"sst_method\": \"xtrabackup-v2\",
    \"wsrep_port\": 4567,
    \"ssh_user\": \"root\",
    \"ssh_password\": \"password\"
}" "部署 PXC 集群"

# ==================== 步骤 13: 创建备份策略 ====================
log "13" "配置备份策略"

api_call "POST" "/api/v1/backup-policies" "{
    \"name\": \"$TEST_PREFIX-daily-backup\",
    \"instance_ids\": [],
    \"schedule\": \"0 2 * * *\",
    \"backup_type\": \"full\",
    \"method\": \"xtrabackup\",
    \"retention_days\": 7,
    \"storage_path\": \"/backup/mysql\"
}" "创建备份策略"

# ==================== 步骤 14: 执行手动备份 ====================
log "14" "执行手动备份"

api_call "POST" "/api/v1/backups" "{
    \"instance_id\": \"\",
    \"backup_type\": \"full\",
    \"method\": \"xtrabackup\",
    \"storage_path\": \"/backup/mysql/manual\"
}" "执行手动备份"

# ==================== 步骤 15: 查询备份历史 ====================
log "15" "查询备份历史"

api_call "GET" "/api/v1/backups?limit=100" "" "获取备份记录"

# ==================== 步骤 16: 创建告警规则 ====================
log "16" "配置监控告警"

api_call "POST" "/api/v1/alert-rules" "{
    \"name\": \"$TEST_PREFIX-high-cpu\",
    \"metric\": \"cpu_usage\",
    \"threshold\": 80,
    \"operator\": \">\",
    \"duration\": \"5m\",
    \"severity\": \"warning\",
    \"notification_channels\": [\"email\"]
}" "创建 CPU 告警规则"

api_call "POST" "/api/v1/alert-rules" "{
    \"name\": \"$TEST_PREFIX-replication-lag\",
    \"metric\": \"replication_lag\",
    \"threshold\": 60,
    \"operator\": \">\",
    \"duration\": \"2m\",
    \"severity\": \"critical\",
    \"notification_channels\": [\"email\", \"slack\"]
}" "创建复制延迟告警"

# ==================== 步骤 17: 查询告警历史 ====================
log "17" "查询告警历史"

api_call "GET" "/api/v1/alerts/history?limit=100" "" "获取告警历史"

# ==================== 步骤 18: 创建参数模板 ====================
log "18" "配置参数模板"

api_call "POST" "/api/v1/parameter-templates" "{
    \"name\": \"$TEST_PREFIX-high-perf\",
    \"description\": \"High performance template\",
    \"mysql_version\": \"8.0\",
    \"parameters\": {
        \"innodb_buffer_pool_size\": \"4G\",
        \"max_connections\": \"1000\",
        \"innodb_flush_log_at_trx_commit\": \"2\",
        \"innodb_io_capacity\": \"2000\"
    }
}" "创建高性能参数模板"

# ==================== 步骤 19: 应用参数模板 ====================
log "19" "应用参数模板"

api_call "POST" "/api/v1/parameter-templates/$TEST_PREFIX-high-perf/apply" "{
    \"instance_ids\": []
}" "应用参数模板到实例"

# ==================== 步骤 20: 查询审计日志 ====================
log "20" "查询审计日志"

api_call "GET" "/api/v1/audits?limit=50" "" "获取审计日志"

# ==================== 步骤 21: 查询任务列表 ====================
log "21" "查询任务列表"

api_call "GET" "/api/v1/tasks?limit=100" "" "获取任务列表"

# ==================== 步骤 22: 清理测试数据 ====================
log "22" "清理测试数据"

api_call "DELETE" "/api/v1/deployments/$TEST_PREFIX-ha-cluster" "" "删除 HA 集群"
api_call "DELETE" "/api/v1/deployments/$TEST_PREFIX-mgr-cluster" "" "删除 MGR 集群"
api_call "DELETE" "/api/v1/deployments/$TEST_PREFIX-pxc-cluster" "" "删除 PXC 集群"

# ==================== 完成 ====================
echo -e "\n============================================"
echo "【完成】完整集群安装流程测试完成"
echo "============================================"
echo ""
echo "测试覆盖:"
echo "  ✅ 主机注册 (3 节点)"
echo "  ✅ 环境检查"
echo "  ✅ Agent 安装"
echo "  ✅ HA 主从集群部署 (1 主 2 从)"
echo "  ✅ MGR 多主集群部署 (3 节点)"
echo "  ✅ PXC 集群部署 (3 节点)"
echo "  ✅ 备份策略配置"
echo "  ✅ 告警规则配置"
echo "  ✅ 参数模板配置"
echo "  ✅ 审计日志查询"
echo "  ✅ 任务管理"
echo "  ✅ 数据清理"
echo ""
echo "测试前缀：$TEST_PREFIX"
echo "测试时间：$(date)"
echo ""
