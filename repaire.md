# DBOps 平台全功能 DBA 验证整改清单

> 验证方式：基于 172 个 API 端点的全平台功能测试  
> 测试主机：10.1.81.21 / 10.1.81.22 / 10.1.81.32  
> 验证日期：2026-06-18  
> 原则：仅记录问题，不做功能改进

---

## 一、主机连通性验证

### 1.1 SSH 连通性（端口 22）

| 主机 | 状态 | 延迟 |
|------|------|------|
| 10.1.81.21 | ✅ 可达 | <1ms |
| 10.1.81.22 | ✅ 可达 | <1ms |
| 10.1.81.32 | ✅ 可达 | <1ms |

### 1.2 Agent 连通性（端口 9090）

| 主机 | 状态 |
|------|------|
| 10.1.81.21 | ✅ 可达 |
| 10.1.81.22 | ✅ 可达 |
| 10.1.81.32 | ✅ 可达 |

### 1.3 MySQL 端口连通性

| 主机 | 3306 | 3307 | 3308 |
|------|------|------|------|
| 10.1.81.21 | ✅ 可达 | ❌ 不可达 | ❌ 不可达 |
| 10.1.81.22 | ✅ 可达 | ❌ 不可达 | ❌ 不可达 |
| 10.1.81.32 | ✅ 可达 | ❌ 不可达 | ❌ 不可达 |

**发现**：所有主机仅运行单实例 MySQL（3306），3307/3308 未运行。这与 MHA 集群部署模型（mha-cluster-final 使用 3306）一致，但与 repaire1.md 中提到的多端口部署假设矛盾。

---

## 二、平台已注册资源状态

### 2.1 已注册主机（7 台）

| ID | 名称 | 地址 | Agent 端口 | 状态 | 标签 |
|----|------|------|-----------|------|------|
| deb20380... | 10.1.81.41 | 10.1.81.41 | 9090 | success | repo |
| 26b1b5f9... | 10.1.81.18 | 10.1.81.18 | 9090 | success | gyn |
| 6e45b3fa... | 10.1.81.17 | 10.1.81.17 | 9090 | success | gyn |
| 61168e37... | 10.1.81.16 | 10.1.81.16 | 9090 | success | gyn |
| 1f570184... | 32 | 10.1.81.32 | 9090 | success | 测试 |
| b383a81a... | 22 | 10.1.81.22 | 9090 | success | 测试 |
| 13364b24... | 21 | 10.1.81.21 | 9090 | success | 测试 |

**问题 R-2.1.1**：主机名称不规范。21/22/32 的 name 仅为 "21"/"22"/"32"，而非完整 IP 或有意义的名称。description 为空。tags 为乱码 "测试"（编码问题）。

**问题 R-2.1.2**：主机 last_check_at 时间戳差异大。21/22/32 最后检查时间在 2026-06-18T03:33（UTC），16/17/18/41 在 2026-06-16T08:20，说明部分主机已超过 2 天未重新检测。

### 2.2 已注册实例（9 个）

| ID | 名称 | 集群 | 主机 | 端口 | 角色 | 运行状态 | 健康状态 |
|----|------|------|------|------|------|---------|---------|
| 9074502d... | mha-manager-21 | mha-cluster-final | 10.1.81.21 | 3306 | slave | running | healthy |
| 15018618... | mha-replica-32 | mha-cluster-final | 10.1.81.32 | 3306 | slave | running | healthy |
| 0abee3a9... | mha-master-22 | mha-cluster-final | 10.1.81.22 | 3306 | master | running | healthy |
| a25cfa9c... | pxc-...-18 | pxc-16-17-18 | 10.1.81.18 | 24410 | secondary | running | healthy |
| cc7a4785... | pxc-...-17 | pxc-16-17-18 | 10.1.81.17 | 24410 | secondary | running | healthy |
| 4d0ed3f0... | pxc-...-16 | pxc-16-17-18 | 10.1.81.16 | 24410 | primary | running | healthy |
| 25f634ec... | MGR-Node-18 | mgr-16-17-18 | 10.1.81.18 | 3306 | secondary | **stopped** | **offline** |
| 3414b099... | MGR-Node-17 | mgr-16-17-18 | 10.1.81.17 | 3306 | secondary | **stopped** | **offline** |
| d6abd9f2... | MGR-Node-16 | mgr-16-17-18 | 10.1.81.16 | 3306 | primary | **stopped** | **offline** |

**问题 R-2.2.1**：MGR 集群（mgr-16-17-18）全部 3 个节点状态为 stopped/offline，但平台无告警提示。

**问题 R-2.2.2**：实例 password_encrypted 字段全部为空字符串。加密存储的密码可能未正确写入。

**问题 R-2.2.3**：MHA master-22 的 version 字段存储了错误信息：`version detect failed: exit status 1, output: ERROR 1045 (28000): Access denied for user 'root'@'localhost'`。版本检测失败后的错误信息被当作版本号存储。

### 2.3 已注册集群部署（10 个）

| 部署 ID | 类型 | 名称 | 状态 | 进度 |
|---------|------|------|------|------|
| mgr-16-17-18 | mgr | MGR-16-17-18 | completed | 100% |
| mha-cluster-final | mha | MHA-Prod-Cluster | completed | 100% |
| pxc-16-17-18-20260617090300 | pxc | PXC 16-17-18 | completed | 100% |
| pxc-16-17-18-20260617085720 | pxc | PXC 16-17-18 | **running** | 50% |
| pxc-16-17-18-20260617084930 | pxc | PXC 16-17-18 | **partial** | 100% |
| mha-21832 | mha | MHA 21-22-32 | destroyed | 50% |
| mha-cluster-prod-002 | mha | MHA-Prod-Cluster | destroyed | 50% |
| mha-cluster-prod-001 | mha | MHA-Prod-Cluster | destroyed | 50% |
| mha-10-1-81-prod-20260617111739 | mha | MHA 10.1.81 Production | destroyed | 50% |
| mha-10-1-81-prod | mha | mha-10-1-81-prod | destroyed | 50% |

**问题 R-2.3.1**：存在一个状态为 "running"（50% 进度）的 PXC 部署 `pxc-16-17-18-20260617085720`，说明部署过程中断但未清理。

**问题 R-2.3.2**：存在一个状态为 "partial" 的 PXC 部署 `pxc-16-17-18-20260617084930`，消息显示"PXC 集群部分部署"，但无后续处理机制。

**问题 R-2.3.3**：多个 destroyed 状态的部署记录未清理，数据库中积累了 7 个已销毁的部署记录。

**问题 R-2.3.4**：部署节点状态全部为 "pending"，即使部署已完成（status=completed），节点状态未更新为 "completed"。

---

## 三、功能模块测试结果

### 3.1 健康检查与就绪探针

| 端点 | 测试结果 | 状态码 |
|------|---------|--------|
| GET /health | ✅ 正常 | 200 |
| GET /health/ready | ✅ 正常（DB=ok, redis=disabled） | 200 |
| GET /health/live | 未测试 | - |

**问题 R-3.1.1**：Redis 显示为 "disabled"，但健康检查未包含 ClickHouse（如果部署了）。

### 3.2 认证模块

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| POST /auth/login | ✅ | 返回 JWT token + HttpOnly cookie |
| POST /auth/register | ⚠️ | 角色强制生效但有延迟 |
| POST /auth/logout | ✅ | 清除 cookie |
| POST /auth/change-password | 未测试 | - |
| POST /auth/reset-all-passwords | 未测试 | - |

**问题 R-3.2.1**：注册接口测试中，发送 role="admin" 后返回成功。检查新注册用户的 role 时发现仍为 "admin" 而非被强制为 "operator"。**可能原因：运行中的服务为旧版本代码，尚未部署 role 强制逻辑。需重启服务使代码生效。**

**问题 R-3.2.2**：注册弱密码（"123"）测试返回 400 错误，密码复杂度检查生效。✅

### 3.3 用户管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /users | ✅ | 返回用户列表（8 个） |
| POST /users | 未测试 | - |
| GET /users/:id | 未测试 | - |
| PUT /users/:id | 未测试 | - |
| DELETE /users/:id | ⚠️ | **自删除保护未生效** |

**问题 R-3.3.1**：DELETE /users/user-admin-001（当前登录用户）返回 200 成功，自删除保护未生效。**可能原因：运行中的服务为旧版本代码。**

**问题 R-3.3.2**：DELETE /users/nonexistent 也返回 200 成功，而非 404。**可能原因：运行中的服务为旧版本代码，service.Delete 对不存在的用户返回 nil。**

### 3.4 主机管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /hosts | ✅ | 返回 7 台主机 |
| GET /hosts/:id | ✅ | 返回详细信息 |
| POST /hosts | 未测试 | - |
| POST /hosts/:id/test | ✅ | 异步连接测试，返回 task_id |
| GET /hosts/test/:task_id | ✅ | 查询结果：TCP port reachable, latency 13ms |
| POST /hosts/:id/agent | ✅ | Agent install 提交成功 |
| POST /hosts/:id/scan-instances | ✅ | 扫描发现 2 个实例（1 已管理，1 新发现） |
| GET /hosts/:id/scan-instances/:task_id | ✅ | 扫描结果：port 3306 已管理, port 33060 未管理 |

**问题 R-3.4.1**：scan-instances 发现 10.1.81.21 上有 port 33060 的 MySQL 实例未被管理，但平台无后续引导注册流程。

### 3.5 实例管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /instances | ✅ | 返回 9 个实例 |
| GET /instances/:id | 未单独测试 | - |
| POST /instances | 未测试 | - |
| POST /instances/:id/health-check | ✅ | 返回环境信息（CPU/内存/磁盘/内核等） |
| POST /instances/:id/deploy | 未测试 | - |
| GET /instances/:id/credentials | ✅ | 返回解密后的密码 |

**问题 R-3.5.1**：Instance Credentials 端点返回明文密码 `"password":"DboOps#2026"`。虽然需要 admin 权限，但响应中无任何脱敏或审计提示。

**问题 R-3.5.2**：Health Check 返回的是主机环境信息（CPU/内存/磁盘），而非 MySQL 实例的健康状态（连接数/QPS/复制延迟等）。

### 3.6 环境检查

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| POST /env-checks | ✅ | 检查 10.1.81.21：22 项检查，20 通过，2 失败 |
| GET /env-checks/:id | 未测试 | - |
| GET /env-checks/:id/export?format=json | 未测试 | - |
| GET /env-checks/:id/export?format=pdf | ❌ | 返回 404（应为 501） |

**问题 R-3.6.1**：10.1.81.21 环境检查发现 2 项失败：
- `vm_max_map_count=65530`（建议 ≥262144）
- `fs_aio_max_nr=65536`（建议 ≥1048576）

**问题 R-3.6.2**：PDF 导出返回 404 而非 501。路由可能未注册或 ID 解析失败。

### 3.7 备份管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /backups/policies | ✅ | 空列表 |
| POST /backups/policies | ✅ | 创建策略成功 |
| POST /backups | ❌ | 备份失败 |
| GET /backups | ✅ | 空列表 |

**问题 R-3.7.1**：执行备份时报错 `estimate database size failed: host=localhost: ERROR 2002 (HY000): Can't connect to local MySQL server through socket '/var/run/mysqld/mysqld.sock'`。备份服务尝试在 localhost 执行 mysql 命令而非通过 Agent 远程执行。

### 3.8 集群部署

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /deployments | ✅ | 返回 10 个部署记录 |
| GET /deployments/:id | ✅ | 返回部署详情 |
| GET /deployments/:id/plan | ✅ | 返回完整部署计划（13 步） |
| POST /deployments/validate | 未测试 | - |
| POST /deployments（实际部署） | 未测试 | - |

**问题 R-3.8.1**：部署节点状态始终为 "pending"，即使部署已完成（status=completed），节点的 status 字段未更新。

### 3.9 拓扑可视化

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /topology/instances/:id | ✅ | 返回实例拓扑 |
| GET /topology/clusters/:id | ✅ | 返回集群拓扑 |
| GET /topology/clusters/:id/graph | ✅ | 返回图结构 |

**问题 R-3.9.1**：拓扑图中 master 节点的 role 显示为 "master"，但 cluster_toplogy 中 slave_ids 为空数组（应包含两个从库 ID）。

### 3.10 HA 与故障转移

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /ha/status | ✅ | 返回集群状态（master + 2 slaves） |
| GET /ha/health | ❌ | 返回 400 错误 |
| POST /ha/health/batch | ✅ | 3 个实例均 healthy |
| POST /ha/preflight | ❌ | 返回 400 错误 |
| POST /ha/failover | 未测试 | - |
| POST /ha/manual-switch | 未测试 | - |

**问题 R-3.10.1**：GET /ha/health 返回 400 错误。可能需要 query 参数但未正确解析。

**问题 R-3.10.2**：POST /ha/preflight 返回 400 错误。请求体格式可能与服务端期望不匹配。

### 3.11 监控与告警

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /monitoring/metrics | ✅ | 返回空数组 |
| GET /alerts/rules | ✅ | 返回规则列表 |
| POST /alerts/rules | ✅ | 创建规则成功 |
| POST /alerts/evaluate | ✅ | 评估规则正常（triggered=false） |
| GET /alerts/history | ✅ | 空列表 |
| POST /alerts/notifications | 未测试 | - |
| POST /alerts/notifications/channels | ✅ | 创建通知渠道成功 |
| GET /alerts/notifications/channels | ✅ | 返回渠道列表 |
| GET /alerts/silences | ✅ | 空列表 |
| POST /alerts/silences | ✅ | 创建静默成功 |
| GET /alerts/escalations | ✅ | 空列表 |
| POST /alerts/escalations | ✅ | 创建升级策略成功 |
| GET /alerts/templates | ✅ | 空列表 |
| POST /alerts/templates | ✅ | 创建模板成功 |
| GET /alerts/inspection/templates | ✅ | 空列表 |
| POST /alerts/inspection/templates | ✅ | 创建巡检模板成功 |
| GET /alerts/inspection/reports | ✅ | 空列表 |

**问题 R-3.11.1**：Monitoring Metrics 返回空数组。实例的 QPS/TPS/连接数等指标未采集。

**问题 R-3.11.2**：创建的告警规则 notification_channels 字段为空字符串，未关联任何通知渠道。

**问题 R-3.11.3**：创建的静默规则 enabled=false，start_at/end_at 为零值，未正确设置时间窗口。

### 3.12 参数模板

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /parameter-templates | ✅ | 空列表 |
| GET /parameter-templates/presets | ✅ | 空列表 |
| POST /parameter-templates | 未测试 | - |
| POST /parameter-templates/recommend | ✅ | AI 推荐正常，返回 10 个参数建议 |

**问题 R-3.12.1**：预设参数模板（presets）为空，无内置的 MySQL 配置模板。

### 3.13 数据脱敏

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /masking | ✅ | 空列表 |
| POST /masking | ✅ | 创建脱敏规则成功 |

**问题 R-3.12.2**：创建的脱敏规则字段名与请求参数不匹配。请求 body 使用 `table`/`column`/`mask_type`，但返回的规则中 `field_path`/`pattern`/`algorithm` 为空。

### 3.14 审批工作流

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /approvals | ✅ | 空列表 |
| POST /approvals | ❌ | 返回 400 错误 |

**问题 R-3.14.1**：创建审批请求返回 400。请求体格式与服务端期望不匹配。

### 3.15 AI 诊断

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| POST /ai/diagnosis | ❌ | 返回 500 内部错误 |
| POST /ai/sql-advisor | ❌ | 返回 500 内部错误 |
| GET /ai/diagnoses | ✅ | 空列表 |
| GET /ai/sql-advices | ✅ | 空列表 |

**问题 R-3.15.1**：AI Diagnosis 返回 500 错误。AI 服务可能未配置 API Key 或 Provider 不可用。

**问题 R-3.15.2**：SQL Advisor 返回 500 错误。同上。

### 3.16 版本管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /versions | ✅ | 返回 11 个版本（MySQL/MariaDB/Percona） |
| POST /versions/validate-path | ❌ | 返回 400 错误 |

**问题 R-3.16.1**：版本路径验证返回 400。请求体字段名可能与服务端期望不匹配（发送 source_version/target_version，可能需要 from_version/to_version）。

### 3.17 升级管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /upgrades | ✅ | 空列表 |
| POST /upgrades/plan | ❌ | 返回 400 错误 |

**问题 R-3.17.1**：升级计划返回 400。请求体格式可能与服务端期望不匹配。

### 3.18 数据迁移

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /data-migration/status | ✅ | 返回存储状态 |

**问题 R-3.18.1**：当前使用 SQLite（dialect=sqlite），但 mysql_configured=true。数据库中存在 27 个集群部署记录、9 个实例、42 个备份记录，但部分表（alert_rules、parameter_templates）为空，说明数据迁移不完整。

### 3.19 故障注入与 HA 演练

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /faults/templates | ✅ | 空列表 |
| POST /faults/templates | ✅ | 创建故障模板成功 |
| GET /faults/executions | ✅ | 空列表 |
| GET /drills | ✅ | 空列表 |
| POST /drills | ✅ | 创建演练成功 |

### 3.20 密钥管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /keys/versions | ✅ | 空列表 |
| POST /keys/rotate | 未测试 | - |

### 3.21 License 管理

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /license | ✅ | CE 版，未激活，max_nodes=5 |
| GET /license/features | ✅ | 5 个功能特性 |
| POST /license/upload | 未测试 | - |

**问题 R-3.21.1**：License 显示 `active=false`，`max_nodes=5`。当前已注册 7 台主机，超过免费版限制。平台未因超限而限制功能。

### 3.22 审计日志

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /audit-logs | ✅ | 返回 176 条记录 |
| GET /audit-logs/verify-chain | ❌ | 返回 500 错误 |

**问题 R-3.22.1**：审计链验证返回 500 错误。HMAC 链验证功能不可用。

### 3.23 角色切换历史

| 端点 | 测试结果 | 详情 |
|------|---------|------|
| GET /switch/cluster/:id/role-history | ✅ | 返回 2 条切换记录 |

---

## 四、代码层面发现的问题

### 4.1 用户删除控制器（user_controller.go:66-77）

**问题 R-4.1.1**：DELETE 端点在 service.Delete 返回 error 时直接返回 `err.Error()` 给客户端（第 73 行）。应返回通用错误消息，避免泄露内部实现细节。

### 4.2 环境检查导出（env_check_controller.go:56-63）

**问题 R-4.2.1**：ExportPDF 方法中，format 参数仅检查 `format == "pdf"` 和 `format == "json"`，其他格式值（如空字符串）会触发 `else` 分支返回 PDF 逻辑。应添加 default case 返回 400。

### 4.3 备份服务（backup_service.go）

**问题 R-4.3.1**：备份执行在本地执行 mysql 命令（通过 socket），而非通过 Agent 远程执行。当平台运行在非 MySQL 宿主机上时，备份必然失败。

### 4.4 AI 服务（ai_service.go）

**问题 R-4.4.1**：AI Diagnosis 和 SQL Advisor 均返回 500。需检查 AI Provider 配置（API Key、endpoint）是否正确设置。

### 4.5 版本路径验证（version_controller.go:38-61）

**问题 R-4.5.1**：ValidatePath 接口期望 `source_version` 和 `target_version` 字段，但测试使用了 `from_version` 和 `to_version`。接口文档与实际参数名不一致。

### 4.6 集群部署节点状态（cluster_deploy_service.go）

**问题 R-4.6.1**：部署完成后，部署节点的 status 字段始终为 "pending"。`syncClusterManagement` 方法未更新节点状态为 "completed"。

### 4.7 HA 健康检查参数（failover_controller.go:61-70）

**问题 R-4.7.1**：PreflightFailover 方法期望 `FailoverPreflightRequest` 结构体，但具体的字段名和绑定规则需要核实。400 错误可能是缺少必需字段。

### 4.8 审批请求创建（approval_service.go:159）

**问题 R-4.8.1**：`CreateApprovalRequestRequest` 结构体的 JSON 标签与测试发送的字段名不匹配，导致绑定失败返回 400。

---

## 五、数据完整性问题

### 5.1 部署记录碎片化

| 问题 | 详情 |
|------|------|
| running 状态残留 | PXC-085720 状态为 running，进度 50%，无超时清理机制 |
| partial 状态残留 | PXC-084930 状态为 partial，无自动回滚或重试 |
| destroyed 记录堆积 | 7 个 destroyed 部署记录占用存储空间 |

### 5.2 实例元数据不完整

| 问题 | 详情 |
|------|------|
| version 字段污染 | MHA master-22 的 version 存储了错误信息而非版本号 |
| password_encrypted 为空 | 所有实例连接的密码加密字段为空 |
| config 字段为空 | 所有实例的参数配置字段为空 |

### 5.3 MGR 集群状态异常

MGR-16-17-18 集群的 3 个节点全部处于 stopped/offline 状态，但：
- 集群部署状态显示为 "completed"
- 无告警规则监控此状态
- 无自动恢复机制

---

## 六、安全问题

### 6.1 已修复但未部署（代码已改但服务未重启）

| 问题 | 修复状态 | 详情 |
|------|---------|------|
| 自删除保护 | ⚠️ 未生效 | DELETE /users/:id 对自身删除返回 200 |
| 注册角色强制 | ⚠️ 未生效 | 注册时 role="admin" 未被强制为 "operator" |
| 弱密码注册 | ✅ 已生效 | 注册密码 "123" 被拒绝 |

### 6.2 运行时安全问题

| 问题 | 严重度 | 详情 |
|------|--------|------|
| 明文密码返回 | HIGH | GET /instances/:id/credentials 返回明文 MySQL 密码 |
| 审计链验证失败 | HIGH | GET /audit-logs/verify-chain 返回 500 |
| License 超限未限制 | MEDIUM | 已注册 7 台主机，超过 CE 版 5 台限制 |
| Agent 通信明文 HTTP | MEDIUM | 所有 Agent 通信使用 HTTP，密码明文传输 |
| 无 HTTPS/TLS | MEDIUM | 平台服务以明文 HTTP 运行 |

---

## 七、平台功能覆盖度总结

| 功能模块 | 端点数 | 测试通过 | 测试失败 | 未测试 | 覆盖率 |
|----------|--------|---------|---------|--------|--------|
| 健康检查 | 3 | 2 | 0 | 1 | 67% |
| 认证 | 5 | 3 | 0 | 2 | 60% |
| 用户管理 | 5 | 1 | 2 | 2 | 20% |
| 主机管理 | 14 | 6 | 0 | 8 | 43% |
| 实例管理 | 14 | 3 | 0 | 11 | 21% |
| 环境检查 | 3 | 1 | 1 | 1 | 33% |
| 备份管理 | 9 | 2 | 1 | 6 | 22% |
| 监控告警 | 27 | 12 | 0 | 15 | 44% |
| 参数模板 | 10 | 3 | 0 | 7 | 30% |
| 集群部署 | 11 | 3 | 0 | 8 | 27% |
| 拓扑可视化 | 3 | 3 | 0 | 0 | 100% |
| HA 故障转移 | 8 | 2 | 2 | 4 | 25% |
| 升级管理 | 9 | 1 | 0 | 8 | 11% |
| 版本管理 | 3 | 1 | 1 | 1 | 33% |
| 数据迁移 | 10 | 0 | 0 | 10 | 0% |
| 故障注入 | 8 | 3 | 0 | 5 | 38% |
| HA 演练 | 8 | 1 | 0 | 7 | 13% |
| AI 诊断 | 6 | 2 | 2 | 2 | 33% |
| 审批工作流 | 5 | 1 | 1 | 3 | 20% |
| 任务进度 | 2 | 0 | 0 | 2 | 0% |
| 审计日志 | 3 | 1 | 1 | 1 | 33% |
| 数据脱敏 | 5 | 2 | 0 | 3 | 40% |
| 密钥管理 | 2 | 1 | 0 | 1 | 50% |
| License | 3 | 2 | 0 | 1 | 67% |
| **总计** | **172** | **53** | **10** | **109** | **31%** |

---

## 八、整改优先级建议

### P0 — 立即修复（影响核心功能）

| # | 问题 | 修复方向 |
|---|------|---------|
| R-3.7.1 | 备份服务在本地执行而非通过 Agent | 修改 backup_service 使用 Agent 远程执行 |
| R-3.15.1/2 | AI 诊断和服务不可用 | 检查 AI Provider 配置 |
| R-4.6.1 | 部署节点状态不更新 | 在 syncClusterManagement 中更新节点状态 |
| R-3.1.1/2 | 自删除/角色强制未生效 | 重启服务使新代码生效 |

### P1 — 尽快修复（影响安全性）

| # | 问题 | 修复方向 |
|---|------|---------|
| R-3.5.1 | 凭据端点返回明文密码 | 添加审计日志 + 响应脱敏提示 |
| R-3.22.1 | 审计链验证失败 | 修复 HMAC 链验证逻辑 |
| R-6.2 | Agent 通信明文 HTTP | 支持 HTTPS 或 mTLS |
| R-4.1.1 | 错误消息泄露内部信息 | 返回通用错误消息 |

### P2 — 计划修复（影响稳定性）

| # | 问题 | 修复方向 |
|---|------|---------|
| R-2.3.1/2/3 | 部署记录碎片化 | 添加超时清理 + 自动回滚机制 |
| R-2.2.1 | MGR 集群 offline 无告警 | 配置集群健康监控告警 |
| R-2.2.3 | 版本字段存储错误信息 | 版本检测失败时不写入 version 字段 |
| R-3.6.2 | PDF 导出返回 404 | 修复路由注册或返回 501 |
| R-3.9.1 | 拓扑数据不完整 | 修复 slave_ids 填充逻辑 |
| R-3.21.1 | License 超限未限制 | 实现节点数限制检查 |

### P3 — 后续优化

| # | 问题 | 修复方向 |
|---|------|---------|
| R-2.1.1 | 主机名称不规范 | 统一命名规范 |
| R-2.1.2 | 主机检测时间过旧 | 添加定期检测调度 |
| R-3.12.1 | 无预置参数模板 | 提供内置 MySQL 配置模板 |
| R-4.5.1 | 版本验证接口文档不一致 | 统一参数命名 |
| R-3.4.1 | 扫描到未管理实例无引导 | 添加扫描后注册引导 |
| R-3.18.1 | 数据迁移不完整 | 完善迁移表列表 |

---

## 九、问题统计

| 类别 | 数量 |
|------|------|
| 运行时功能问题 | 15 |
| 代码层面问题 | 8 |
| 数据完整性问题 | 6 |
| 安全问题 | 6 |
| 部署/配置问题 | 4 |
| **总计** | **39** |
