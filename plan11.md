# plan11.md — MySQL Ops Platform 里程碑开发计划

> 基于 `new_feature.txt` 七大目标，结合现有代码库（Go后端 + Agent + WebConsole）现状，按 superpowers+openspec+getstock 模式进行里程碑拆分。

---

## 现状摘要

| 模块 | 已有 | 缺失 |
|------|------|------|
| 集群部署（三段式） | `ClusterDeployService` 伪模式 + 部分真实模式，支持 MHA/MGR/PXC | 缺真正的"模板复刻"引擎（Phase2 并发批量复刻）、参数化配置渲染 |
| 插件体系 | Agent 有 `executor` 包可执行各类任务，无插件接口定义 | 缺 Plugin Interface、插件注册/发现机制、架构/中间件插件规范 |
| 凭据管理 | `utils/crypto.go` AES-256-GCM 加解密可用，`InstanceConnection.PasswordEncrypted` 存储字段已有 | 缺统一"凭据保险箱"服务、标准账号体系（root/repl/monitor）、集群化装配阶段的密码拉齐逻辑 |
| 拓扑管理 | `TopologyService` 可读取/推理拓扑图，`InstanceTopology` 模型已有 | 缺拓扑状态机（角色切换后自动更新）、拓扑变更事件通知 |
| 实时进度 | `DeploymentProgress` 内存进度跟踪已有，前端有 WebSocket 基础 | 缺 gRPC 双向流（后端↔Agent）、结构化日志解析、Redis 消息总线 |
| 端口/目录避让 | Agent `EnvironmentChecker` 检查资源 | 缺 `ss -tlnp` 端口扫描、目录隔离规则（`/data/mysql_<port>/`）、Cgroup 注入 |
| 多版本适配 | `VersionCatalog` 完整，`InstallFromURL` 版本无关安装可用 | 缺 APT 源动态配置、中继服务器下载、Go Template 配置渲染（gtid/mariadbd 差异） |
| 生命周期管理 | `UpgradeExecutor`（原地+逻辑+滚动）、`FailoverService`、`SwitchService` 有基础 | 缺扩缩容自动加入集群、节点重建流程、集群销毁/重建完整工作流 |

---

## 里程碑总览

```
M1  插件体系与凭据保险箱        [基础层]     ~2 周
M2  模板复刻部署引擎            [核心部署]   ~3 周
M3  gRPC 实时通道与消息总线     [可观测性]   ~2 周
M4  端口避让与资源隔离          [Agent 增强]  ~1 周
M5  多版本软件分发              [分发层]     ~1.5 周
M6  集群全生命周期管理          [运维层]     ~3 周
M7  前端联调与端到端验证        [交付层]     ~2 周
```

---

## M1：插件体系与凭据保险箱

> 目标：定义插件接口，实现凭据统一加密存储与标准账号管理，为后续所有里程碑提供基础。

### M1.1 Plugin Interface 定义

**文件**: `platform-backend/internal/plugins/plugin.go`

```
type PluginType string // "kernel" | "arch" | "middleware"

type Plugin interface {
    Name() string
    Type() PluginType
    Version() string
    // Prepare 校验前置条件（主机连通性、端口、依赖工具）
    Prepare(ctx context.Context, env PluginEnv) error
    // Execute 执行部署/配置动作
    Execute(ctx context.Context, env PluginEnv, params map[string]interface{}) (*PluginResult, error)
    // PostExecute Execute 成功后调用，用于扩缩容/升级场景补充动作
    PostExecute(ctx context.Context, env PluginEnv) error
    // Rollback 执行失败时回滚
    Rollback(ctx context.Context, env PluginEnv) error
    // Teardown 销毁集群时清理
    Teardown(ctx context.Context, env PluginEnv) error
    // Join 将新节点加入已有集群（扩容场景）
    Join(ctx context.Context, env PluginEnv, newNode PluginNode) error
    // Leave 将节点踢出集群拓扑（缩容场景）
    Leave(ctx context.Context, env PluginEnv, node PluginNode) error
}

type PluginEnv struct {
    ClusterID string
    Nodes     []PluginNode
    Credentials CredentialSet
}

type PluginNode struct {
    HostID    string
    Address   string
    AgentPort int
    MySQLPort int
    Role      string
    DataDir   string
    Basedir   string
}

type CredentialSet struct {
    RootPassword  string
    ReplUser      string
    ReplPassword  string
    MonitorUser   string
    MonitorPassword string
}
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/plugin.go` — 定义 `Plugin` 接口（含 Join/Leave/PostExecute）、`PluginEnv`、`PluginResult` 类型
- [ ] 创建 `platform-backend/internal/plugins/registry.go` — 插件注册表，按名称+类型查找插件实例
- [ ] 创建 `platform-backend/internal/plugins/executor.go` — 插件执行器，统一调用 Prepare→Execute→PostExecute→Rollback 流程
- [ ] 修改 `platform-backend/internal/repositories/migrator.go` — 添加 `plugin_registry` 表迁移

### M1.2 Kernel Plugin（内核插件接口）

**文件**: `platform-backend/internal/plugins/kernel/mysql_core.go`

```
// mysql-core 实现 Plugin 接口
// Prepare: 校验 Agent 连通、目标端口空闲、磁盘空间足够
// Execute: 通过 AgentClient 下发安装+初始化+账号创建任务
// Rollback: 停服+删除 datadir
// Teardown: 停服+删除 datadir+删除 systemd service
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/kernel/mysql_core.go`
- [ ] 创建 `platform-backend/internal/plugins/kernel/percona_core.go`（复用 mysql_core，差异：包名/认证插件）
- [ ] 创建 `platform-backend/internal/plugins/kernel/mariadb_core.go`（差异：`mariadbd` 命令、gtid_domain_id）
- [ ] 每个 kernel plugin 的 `Execute` 核心流程：`install_binary → render_config → init_datadir → create_os_user → create_systemd_service → start_instance → setup_accounts(root, repl, monitor)`

### M1.3 凭据保险箱（Credential Vault）

**文件**: `platform-backend/internal/services/credential_vault.go`

```
type CredentialVault struct {
    repo     *repositories.CredentialRepository
    encKey   string
}

type ClusterCredential struct {
    ID            string
    ClusterID     string
    AccountType   string // "root" | "repl" | "monitor"
    Username      string
    PasswordEnc   string // AES-256-GCM encrypted
    CreatedAt     time.Time
    RotatedAt     *time.Time
}

// GetCredential 获取并解密指定集群的某类账号密码
// SetCredential 加密并存储
// RotateClusterCredentials 集群化装配阶段强制轮转：生成新密码，下发到所有节点，更新存储
// SyncCredentialToNode 将凭据推送到单个 Agent 节点
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/models/credential.go` — `ClusterCredential` 模型
- [ ] 创建 `platform-backend/internal/repositories/credential_repository.go` — CRUD + 按 clusterID+accountType 查询
- [ ] 创建 `platform-backend/internal/services/credential_vault.go` — Vault 服务实现
- [ ] 修改 `platform-backend/internal/repositories/migrator.go` — 添加 credential 表迁移
- [ ] 修改 `ClusterDeployService` — 部署前从 Vault 获取凭据，部署后存储凭据

### M1.4 Agent 侧账号管理

**文件**: `agent/internal/executor/account_manager.go`

```
// AccountManager 在目标主机上管理 MySQL 账号
// SetupRootAccount(rootPassword) — ALTER USER / SET PASSWORD
// SetupReplAccount(replUser, replPassword) — GRANT REPLICATION SLAVE
// SetupMonitorAccount(monitorUser, monitorPassword) — GRANT PROCESS, REPLICATION CLIENT, SELECT
// RotatePassword(user, newPassword) — 密码轮转，原子生效
```

**任务清单**:
- [ ] 创建 `agent/internal/executor/account_manager.go`
- [ ] 创建 `agent/internal/executor/account_manager_test.go`
- [ ] 在 Agent HTTP 路由中注册 `/api/v1/accounts/setup` 和 `/api/v1/accounts/rotate` 端点

---

## M2：模板复刻部署引擎

> 目标：实现"三段式部署工作流"的核心引擎——单点模板构建 → 参数化并发复刻 → 集群化装配。

### M2.1 部署工作流编排器（Orchestrator）

**文件**: `platform-backend/internal/services/deploy_orchestrator.go`

```
type DeployOrchestrator struct {
    pluginRegistry *plugins.Registry
    credentialVault *CredentialVault
    agentClient     *AgentClient
    deploySvc       *ClusterDeployService
    // 三段式工作流状态机
}

// Run 三段式部署入口
// Phase1_TemplateBuild  — 选定首节点，下发 kernel plugin，生成基础配置，初始化单点
// Phase2_Replicate     — 提取首节点配置模板，并发下发到其余节点，替换差异化参数
// Phase3_Assemble      — 触发架构插件，统一轮转密码，建立同步关系
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/deploy_orchestrator.go`
- [ ] `Phase1_TemplateBuild`:
  - [ ] 从 `CredentialVault` 获取/生成标准账号密码
  - [ ] 通过 `AgentClient` 向首节点下发 kernel plugin 安装任务
  - [ ] 等待 Agent 返回成功，提取首节点配置（basedir、datadir、端口、server-id）
  - [ ] 创建 `Instance` 记录，关联到集群
- [ ] `Phase2_Replicate`:
  - [ ] 从首节点提取配置模板（Go Template），生成参数化差异表（server-id 递增、IP、port、gtid 标识）
  - [ ] 并发（`sync.WaitGroup` + goroutine）向其余目标主机的 Agent 下发安装任务
  - [ ] 并发结果收集，任一失败则标记整个部署失败并触发回滚
- [ ] `Phase3_Assemble`:
  - [ ] 调用 `CredentialVault.RotateClusterCredentials` 统一密码
  - [ ] 调用架构插件（通过 Plugin Registry）的 `Execute` 方法
  - [ ] 架构插件内部执行：CHANGE MASTER TO / group_replication 组网 / MHA Manager 部署
  - [ ] 更新 `InstanceTopology` 关系图（master_id、slave_ids、replication_mode）
  - [ ] 创建 `ClusterDeployment` 完成记录

### M2.2 Arch Plugin — Replica Addon（主从复制插件）

**文件**: `platform-backend/internal/plugins/arch/replica_addon.go`

```
// Execute 流程：
// 1. 对所有从库执行 CHANGE MASTER TO (master_host, master_user, master_password, MASTER_AUTO_POSITION=1)
// 2. START SLAVE
// 3. 检查 Slave_IO_Running / Slave_SQL_Running = Yes
// 4. 如配置了 Keepalived，触发 keepalived-addon 安装
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/arch/replica_addon.go`
- [ ] Agent 侧：`agent/internal/executor/replication_setup.go` — CHANGE MASTER TO + START SLAVE + 检查复制状态

### M2.3 Arch Plugin — MHA Addon

**文件**: `platform-backend/internal/plugins/arch/mha_addon.go`

```
// Execute 流程：
// 1. 在所有节点安装 MHA Node
// 2. 在 Manager 节点安装 MHA Manager
// 3. 生成 mha.cnf（masterha_manager 配置）
// 4. 配置 SSH 互信（masterha_check_repl 依赖）
// 5. 启动 masterha_manager
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/arch/mha_addon.go`
- [ ] Agent 侧：`agent/internal/executor/mha_setup.go` — 安装 MHA Node/Manager + 配置

### M2.4 Arch Plugin — MGR Addon

**文件**: `platform-backend/internal/plugins/arch/mgr_addon.go`

```
// Execute 流程：
// 1. 所有节点配置 group_replication 变量（group_name, local_address, group_seeds）
// 2. 首节点 SET GLOBAL group_replication_bootstrap_group=ON + START GROUP_REPLICATION
// 3. 其余节点并发 START GROUP_REPLICATION
// 4. 检查 performance_schema.replication_group_members 状态为 ONLINE
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/arch/mgr_addon.go`
- [ ] Agent 侧：`agent/internal/executor/mgr_setup.go` — MGR 组网引导

### M2.5 Arch Plugin — PXC Galera Addon

**文件**: `platform-backend/internal/plugins/arch/pxc_galera_addon.go`

```
// Execute 流程：
// 1. 首节点 wsrep_cluster_address=gcomm:// 引导启动
// 2. 其余节点 wsrep_cluster_address=gcomm://<首节点IP> 加入
// 3. 检查 wsrep_cluster_size == 节点数
// 4. 检查 wsrep_local_state_comment = Synced
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/arch/pxc_galera_addon.go`
- [ ] Agent 侧：`agent/internal/executor/pxc_setup.go` — PXC SST 加入

### M2.6 版本差异抹平层（VersionAdapter）

> ⚠️ **依赖说明**：M2.6 必须优先于 M2.7 实现。原位于 M5.1，因 ConfigRenderer 强依赖版本差异逻辑，故前置到此。

**文件**: `platform-backend/internal/services/version_adapter.go`

```
// VersionAdapter 根据 flavor + major.minor 返回差异化的配置片段
// 差异清单（自动化，无需人工干预）：
//   MySQL 5.7 vs 8.0: default_authentication_plugin, mysql_native_password
//   MySQL 8.0.17+: 移除 mysql_upgrade 命令调用
//   MariaDB: gtid_domain_id 替代 gtid_mode, mariadbd 替代 mysqld
//   MariaDB 10.5+: innodb_buffer_pool_chunk_size 默认值变化
//   Percona: xtrabackup 组件路径差异
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/version_adapter.go`
- [ ] 扩展 `VersionCatalog` 的 `VersionEntry`：添加 `ConfigHints map[string]string` 字段，记录版本特有配置

### M2.7 配置模板渲染引擎（ConfigRenderer）

> 依赖 M2.6 VersionAdapter 获取差异参数后渲染最终配置。

**文件**: `platform-backend/internal/services/config_renderer.go`

```
// ConfigRenderer 使用 Go Template 渲染 my.cnf / mariadb.cnf
// 调用 VersionAdapter 获取版本差异参数，抹平 MySQL 5.7→8.3 差异
// 差异化处理：
//   MySQL 5.7: gtid_mode=ON, enforce_gtid_consistency=ON
//   MySQL 8.0: default_authentication_plugin=mysql_native_password (兼容性)
//   MariaDB: gtid_domain_id=<server_id>, 不支持 enforce_gtid_consistency
//   MariaDB 10.5+: 使用 mariadbd 命令而非 mysqld
//
// 模板存储：platform-backend/internal/templates/*.tmpl
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/config_renderer.go` — 内部调用 `VersionAdapter` 获取差异参数
- [ ] 创建模板文件：
  - [ ] `platform-backend/internal/templates/mysql57.cnf.tmpl`
  - [ ] `platform-backend/internal/templates/mysql80.cnf.tmpl`
  - [ ] `platform-backend/internal/templates/mariadb10.cnf.tmpl`
  - [ ] `platform-backend/internal/templates/percona80.cnf.tmpl`
- [ ] Agent 侧接收渲染后的配置文本，写入 `/etc/my.cnf.d/` 或 `/etc/mysql/`

### M2.MW 中间件插件

> 目标：定义并实现 keepalived-addon 和 proxysql-addon，在集群装配阶段可选挂载，实现 VIP 漂移和读写分离。

#### M2.MW.1 Keepalived Addon

**文件**: `platform-backend/internal/plugins/middleware/keepalived_addon.go`

```
// Execute 流程：
// 1. 在所有节点安装 keepalived
// 2. 生成 keepalived.conf（virtual_router_id、priority、interface、virtual_ipaddress）
// 3. 配置健康检查脚本（mysqlchk.sh：mysqladmin ping）
// 4. 优先级：Master 节点 priority=100，Backup 节点 priority=90
// 5. 启动 keepalived 服务
//
// Teardown 流程：
// 1. 停止 keepalived 服务
// 2. 删除 keepalived.conf 及健康检查脚本
// 3. 卸载 keepalived 包
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/middleware/keepalived_addon.go`
- [ ] Agent 侧：`agent/internal/executor/keepalived_setup.go` — 安装 keepalived + 配置 + 启动
- [ ] Keepalived 健康检查脚本：`agent/scripts/mysqlchk.sh`
- [ ] 在 `DeployOrchestrator.Phase3_Assemble` 中可选触发 keepalived-addon

#### M2.MW.2 ProxySQL Addon

**文件**: `platform-backend/internal/plugins/middleware/proxysql_addon.go`

```
// Execute 流程：
// 1. 在 ProxySQL 节点安装 proxysql
// 2. 配置 MySQL 后端组（writer_hostgroup=10, reader_hostgroup=20）
// 3. 注册所有数据节点到 ProxySQL（区分 writer/reader）
// 4. 配置监控账号（monitor）
// 5. 保存在线配置到磁盘
//
// Teardown 流程：
// 1. 停止 proxysql 服务
// 2. 卸载 proxysql 包
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/plugins/middleware/proxysql_addon.go`
- [ ] Agent 侧：`agent/internal/executor/proxysql_setup.go` — 安装 + 配置 + 注册后端
- [ ] 在 `DeployOrchestrator.Phase3_Assemble` 中可选触发 proxysql-addon

---

## M3：gRPC 实时通道与消息总线

> 目标：替代当前 HTTP 轮询，实现 Agent↔后端 gRPC 双向流、结构化日志推送、前端 WebSocket 实时展示。

### M3.1 gRPC Proto 定义

**文件**: `proto/agent_service.proto`

```protobuf
service AgentService {
    // 双向流：后端下发任务，Agent 实时回传日志
    rpc ExecuteTask (stream TaskRequest) returns (stream TaskProgress);
    // 单次调用：检查 Agent 健康状态
    rpc HealthCheck (HealthRequest) returns (HealthResponse);
}

message TaskProgress {
    string task_id = 1;
    int32 progress = 2;           // 0-100
    string stage = 3;             // "installing" | "configuring" | "starting"
    string log_line = 4;          // 实时 stdout 行
    string status = 5;            // "running" | "completed" | "failed"
    map<string, string> metadata = 6;
}
```

**任务清单**:
- [ ] 创建 `proto/agent_service.proto` — 定义 gRPC 服务、消息类型
- [ ] 运行 `protoc` 生成 Go 代码（`platform-backend/pkg/grpc/` 和 `agent/pkg/grpc/`）

### M3.2 Agent gRPC Server

**文件**: `agent/internal/grpc/server.go`

```
// gRPC Server 接收 TaskRequest，执行 Shell 脚本
// 通过 stream.Send(TaskProgress) 实时回传 stdout
// 解析 stdout 为结构化状态（正则匹配 [n/m] 开头的步骤标记）
```

**任务清单**:
- [ ] 创建 `agent/internal/grpc/server.go` — gRPC 服务实现
- [ ] 创建 `agent/internal/grpc/streamer.go` — Shell 执行器 + stdout 流式回传
- [ ] 修改 `agent/cmd/main.go` — 启动 gRPC Server（与现有 HTTP Server 并行）

### M3.3 后端 gRPC Client 与消息总线

**文件**: `platform-backend/internal/services/grpc_agent_client.go`

```
// GRPCAgentClient 通过 gRPC 连接目标 Agent
// ExecuteTaskStream 建立双向流，实时接收 TaskProgress
// 将 TaskProgress 发布到消息总线
```

**设计决策**：第一阶段使用**内存消息总线**（基于 Go channel 的发布/订阅模型），避免引入 Redis 依赖。消息总线接口（`MessageBus` interface）预留 `RedisPubSubBackend` 实现，未来可按需切换。

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/grpc_agent_client.go`
- [ ] 创建 `platform-backend/internal/services/message_bus.go` — 定义 `MessageBus` 接口，提供内存实现（发布/订阅/按 taskID 过滤）
- [ ] 预留 `RedisPubSubBackend` 占位，接口签名与内存实现一致
- [ ] 修改 `DeployOrchestrator` — 将 Agent 调用从 HTTP 切换到 gRPC 流式调用（HTTP 保留为 fallback）

### M3.4 前端 WebSocket 实时推送

**文件**: `platform-backend/internal/controllers/ws_controller.go`

```
// WebSocket 端点：/ws/tasks/:taskID
// 前端建立连接后，后端从 MessageBus 订阅该 taskID 的事件流
// 推送格式：{type: "progress"|"log"|"status", data: {...}}
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/controllers/ws_controller.go`
- [ ] 创建 `platform-backend/internal/services/ws_hub.go` — WebSocket 连接池管理
- [ ] 前端：`web-console/src/pages/DeployProgress.tsx` — 进度条+滚动日志+节点状态实时展示

---

## M4：端口避让与资源隔离

> 目标：Agent 在构建单点前扫描本机端口/目录，智能避让；通过 Cgroup 注入防止单机多实例资源抢占。

### M4.1 端口与目录扫描器

**文件**: `agent/internal/executor/port_scanner.go`

```
// ScanUsedPorts 执行 ss -tlnp，解析占用端口列表
// ScanMySQLDataDirs 扫描 /data/mysql_*/ 目录，提取已使用端口
// FindAvailablePort(startPort, excludePorts) 从 startPort 起找第一个空闲端口
// FindAvailableDataDir(basePath, port) 生成 /data/mysql_<port>/ 路径并校验目录不存在
```

**任务清单**:
- [ ] 创建 `agent/internal/executor/port_scanner.go`
- [ ] 创建 `agent/internal/executor/port_scanner_test.go`
- [ ] 识别 MySQL 专有端口：MGR 33061、PXC 4567/4568/4444、MHA manager 未使用

### M4.2 Cgroup 资源隔离

**文件**: `agent/internal/executor/cgroup_manager.go`

```
// GenerateSystemdOverride(port, memoryLimitMB, cpuQuotaPercent) 生成 systemd drop-in
// 输出：/etc/systemd/system/mysqld_<port>.service.d/limits.conf
// 内容：[Service]\nMemoryMax=<M>\nCPUQuota=<P>%
// ReloadSystemd daemon-reload
```

**任务清单**:
- [ ] 创建 `agent/internal/executor/cgroup_manager.go`
- [ ] 修改 Agent 部署流程：在 `GenerateSystemdService` 之后调用 `GenerateSystemdOverride`
- [ ] 默认值：MemoryMax=实例内存的 70%，CPUQuota=核心数×100%

---

## M5：多版本软件分发

> 目标：支持 APT 源和中继服务器双安装源，实现多分支 MySQL/Percona/MariaDB 的自动化软件分发。
> 注：配置渲染相关的版本差异抹平（VersionAdapter）已移至 M2.6 作为 ConfigRenderer 的前置依赖。

### M5.1 APT 源动态配置

**文件**: `agent/internal/executor/apt_source_manager.go`

```
// ConfigureAPTSource(flavor, version) 动态写入 /etc/apt/sources.list.d/mysql.list
// MySQL: deb http://repo.mysql.com/apt/{os_codename}/ mysql-{major.minor} main
// MariaDB: deb http://mirror.mariadb.org/repo/{version}/{os_codename}/ ...
// Percona: deb http://repo.percona.com/ps-80 {os_codename} main
// 执行 apt-get update
```

**任务清单**:
- [ ] 创建 `agent/internal/executor/apt_source_manager.go`
- [ ] 创建 `agent/internal/executor/apt_source_manager_test.go`

### M5.2 中继服务器下载

**文件**: `agent/internal/executor/relay_downloader.go`

```
// DownloadFromRelay(relayURL, branch, version) 从 Relay Server 下载二进制包
// URL 格式：{relayURL}/{branch}/{version}/{tarball_name}
// 下载后校验 SHA256，解压到 /usr/local/
// 处理动态链接库（ldconfig）
```

**任务清单**:
- [ ] 创建 `agent/internal/executor/relay_downloader.go`
- [ ] 复用现有 `InstallFromURL` 函数（`version_install.go`），添加 `ldconfig` 步骤

---

## M6：集群全生命周期管理

> 目标：实现版本升级、节点重建、扩缩容、角色切换、集群销毁/重建等全生命周期操作。

### M6.1 滚动升级增强

**文件**: `platform-backend/internal/services/rolling_upgrade_service.go`

```
// 当前 UpgradeExecutor.ExecuteRollingUpgrade 仅通过 SSH 逐节点操作，需增强为：
// 1. 先复刻并升级从库（复用 Phase2 逻辑）
// 2. 执行角色切换（调用 SwitchService）
// 3. 升级原主库
// 4. 跨大版本自动触发逻辑导出导入流（复用 ExecuteLogicalMigration）
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/rolling_upgrade_service.go` — 编排层
- [ ] 修改 `agent/internal/executor/upgrade_executor.go` — `ExecuteRollingUpgrade` 支持 Agent 侧 gRPC 流式回传
- [ ] 新增 `UpgradeModeRollingArch` 模式：先从后切主，再升级原主

### M6.2 扩容（Scale Out）

**文件**: `platform-backend/internal/services/scale_service.go`

```
// ScaleOut(clusterID, newNodes) 流程：
// 1. 复用 DeployOrchestrator 的 Phase2 逻辑，在目标机快速拉起独立从节点
// 2. 调用架构插件的 Join 方法将新节点加入现有集群
//    MGR: CLONE INSTANCE → START GROUP_REPLICATION
//    PXC: SST 自动触发
//    Replica: CHANGE MASTER TO → START SLAVE
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/scale_service.go`
- [ ] 在 Plugin 接口中添加 `Join(ctx, env, newNode) error` 方法
- [ ] 各 Arch Plugin 实现 `Join` 逻辑

### M6.3 缩容（Scale In）

**任务清单**:
- [ ] `ScaleIn(clusterID, removeNodeIDs)`:
  - [ ] 安全检查：如果目标是主节点，先触发角色切换（调用 SwitchService）
  - [ ] 调用架构插件的 `Leave` 方法踢出集群拓扑
  - [ ] 通过 Agent 停服、删除数据目录、删除 systemd service
  - [ ] 删除 Instance 记录、更新 Topology
- [ ] 在 Plugin 接口中添加 `Leave(ctx, env, node) error` 方法

### M6.4 节点重建（Rebuild）

**文件**: `platform-backend/internal/services/rebuild_service.go`

```
// RebuildNode(clusterID, nodeID) 流程：
// 1. 通过 Agent 停止受损节点
// 2. 清空数据目录
// 3. 重新调用 kernel plugin 初始化（Phase1 逻辑）
// 4. 调用架构插件 Join 以从库身份加入集群
// 5. 触发自动数据同步（Clone/SST/CHANGE MASTER TO）
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/rebuild_service.go`
- [ ] Agent 侧：`agent/internal/executor/node_rebuild.go` — 停服+清数据+重新初始化

### M6.5 角色切换增强

**文件**: `platform-backend/internal/services/switch_service.go`（修改现有）

```
// 当前 SwitchService 已有基础切换逻辑，需增强：
// MHA/主从：LOCK TABLES → 等待从库追平(Seconds_Behind_Master=0) → CHANGE MASTER TO → Keepalived VIP 抢占
// MGR：SELECT group_replication_set_as_primary_member(member_uuid) 平滑切主
// 切换成功后自动更新 InstanceTopology，并发布拓扑变更事件
```

**任务清单**:
- [ ] 修改 `SwitchService` — 添加 MGR 模式的 `group_replication_set_as_primary_member` 切主逻辑
- [ ] 添加 Keepalived 联动：切换成功后触发 `keepalived-addon` VIP 漂移
- [ ] 切换完成后调用 `TopologyService` 自动更新关系图
- [ ] 实现拓扑变更事件通知：每次拓扑更新时发布 `TopologyChangeEvent`（含 event_type、cluster_id、affected_nodes）到消息总线
- [ ] 创建 `platform-backend/internal/models/topology_event.go` — `TopologyChangeEvent` 模型
- [ ] 创建 `platform-backend/internal/repositories/topology_event_repository.go` — `topology_events` 表 CRUD

### M6.6 集群销毁与重建

**文件**: `platform-backend/internal/services/cluster_lifecycle_service.go`

```
// DestroyCluster(clusterID) 逆向执行：
// 1. 中间件插件 Teardown（卸载 Keepalived/ProxySQL）
// 2. 架构插件 Teardown（解组网、停止复制）
// 3. 内核插件 Teardown（停服、删数据、删 service）
// 4. 清理后端凭据（CredentialVault 删除）
// 5. 清理拓扑元数据（InstanceTopology 删除）
// 6. 标记 ClusterDeployment 为 destroyed
//
// RebuildCluster(clusterID) 正向重建：
// 1. 提取原 ClusterDeployment.RequestJSON 中的拓扑参数
// 2. 重新触发完整的 DeployOrchestrator 三段式工作流
```

**任务清单**:
- [ ] 创建 `platform-backend/internal/services/cluster_lifecycle_service.go`
- [ ] 在 Plugin 接口确保 `Teardown(ctx, env) error` 被所有插件正确实现
- [ ] 修改 `ClusterDeployService.DestroyCluster`（现有）整合到新 `ClusterLifecycleService`

---

## M7：前端联调与端到端验证

> 目标：前端适配新的 gRPC 实时通道，完成全链路集成测试。

### M7.1 前端部署进度页增强

**文件**: `web-console/src/pages/DeployProgress.tsx`

```
// 实时进度条（WebSocket 连接）
// 各节点独立状态展示：待执行 → 执行中 → 成功/失败
// 滚动日志面板（结构化 [n/m] 步骤标记）
// 三段式阶段可视化：模板构建 → 参数复刻 → 集群装配
```

**任务清单**:
- [ ] 创建/修改 `web-console/src/pages/DeployProgress.tsx`
- [ ] 创建 `web-console/src/services/useTaskWebSocket.ts` — WebSocket 连接 service（项目使用 React/TypeScript，非 Vue）
- [ ] 修改集群部署表单：增加插件选择（架构类型 + 可选中间件插件）

### M7.2 集群生命周期管理 UI

**任务清单**:
- [ ] 集群详情页：添加"扩容"、"缩容"、"重建"、"销毁"操作按钮
- [ ] 升级向导页：选择目标版本 → 兼容性预检 → 选择升级策略（原地/滚动/逻辑迁移）→ 执行
- [ ] 角色切换对话框：选择目标主节点 → 预检 → 确认执行

### M7.3 端到端集成测试

**任务清单**:
- [ ] `platform-backend/internal/services/integration_test.go` — 三段式部署全流程 mock 测试
- [ ] `agent/internal/executor/integration_test.go` — Agent 侧端口扫描+Cgroup+账号管理集成测试
- [ ] 手工验证 checklist：
  - [ ] MHA 集群部署 → 扩容 → 角色切换 → 滚动升级 → 销毁
  - [ ] MGR 集群部署 → 缩容 → 节点重建 → 销毁
  - [ ] PXC 集群部署 → 扩容 → 销毁

---

## 依赖关系图

```
M1 (插件+凭据)
 ├─→ M2 (模板复刻引擎 + 版本适配 + 中间件)
 │    ├─→ M5 (多版本软件分发)
 │    └─→ M6 (生命周期管理)
 │         └─→ M7 (前端联调)
 ├─→ M3 (gRPC 实时通道)
 │    └─→ M7
 └─→ M4 (端口避让+资源隔离)
      └─→ M2 (Phase1 使用)

注：M2.6 VersionAdapter 原属 M5.1，因 ConfigRenderer 强依赖前置到此。
```

**建议执行顺序**：M1 → M4（可并行） → M2（含 M2.6 VersionAdapter 内置，无需额外依赖 M5） → M3+M5（可并行） → M6 → M7

---

## OpenSpec 关键规范

### Agent ↔ 后端 API 契约（现有 HTTP，过渡期保留）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/deploy/init` | POST | 单点初始化（Phase1） |
| `/api/v1/deploy/replicate` | POST | 参数化复刻（Phase2） |
| `/api/v1/deploy/assemble` | POST | 集群装配（Phase3） |
| `/api/v1/accounts/setup` | POST | 账号初始化 |
| `/api/v1/accounts/rotate` | POST | 密码轮转 |
| `/api/v1/ports/scan` | GET | 端口扫描 |
| `/api/v1/env/check` | GET | 环境预检 |

### gRPC 新契约（M3 引入后替代以上部分端点）

| RPC | 说明 |
|-----|------|
| `AgentService.ExecuteTask` | 双向流，替代所有 deploy/upgrade 端点 |
| `AgentService.HealthCheck` | 健康检查 |

### 数据库新增表

| 表名 | 用途 | 里程碑 |
|------|------|--------|
| `cluster_credentials` | 集群凭据（AES-256 加密存储） | M1 |
| `plugin_registry` | 已注册插件清单 | M1 |
| `deploy_workflows` | 部署工作流状态机记录 | M2 |
| `topology_events` | 拓扑变更事件日志 | M6 |
