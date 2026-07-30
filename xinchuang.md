# 信创数据库分层纳管 — 剩余改动计划

本文档承接已完成的 8 次提交（`0250393` → `02cf270`），针对**按不同型号仍需大量改动**的部分制定后续计划。

---

## 一、已完成基线

| 提交 | 内容 |
|---|---|
| `0250393` | flavor 在纳管时持久化到 `instance_versions`（此前被完整丢弃） |
| `ce16456` | `FlavorCapability` 静态表 + 6 处服务入口 gate |
| `686ac99` | `DBConnector` 按 wire protocol 分派，引入 pgx |
| `71340fa` | GBase 8s（`oninit`）；修复握手复探覆盖 flavor 的隐患 |
| `dc3791d` | GBase 8a（`gclusterd` / `gbased` 双层） |
| `5fee209` | 达梦 DM（`dmserver` + dm.ini `PORT_NUM`） |
| `090201b` | 神舟通用（`oscarserver` / `osrvr`） |
| `02cf270` | tasklist.md 状态与能力矩阵 |

### 现有能力常量（`flavor_capability.go`）
`CapReplication` / `CapFailover` / `CapClusterDeploy` / `CapPhysicalBackup` / `CapInPlaceUpgrade` / `CapSQLHealthCheck`

### 现有 gate 位置（8 处）
`backup_service.go:221`、`failover_service.go:133`、`failover_service.go:290`、`switch_service.go:146`、`switch_service.go:219`、`switch_service.go:405`、`universal_cluster_deploy.go:411`、`upgrade_service.go:422`

---

## 二、通过代码图谱发现的 gate 缺口

对所有会对实例执行 MySQL 专属操作的服务入口做了完整枚举，**11 处仍未受 capability 约束**。这些是真实缺口：非 MySQL 实例走进去会执行 MySQL 语句或调度 MySQL 专属 agent 任务。

### 高危（会对非 MySQL 实例发起破坏性操作）

| # | 位置 | 问题 | 所需能力 |
|---|---|---|---|
| G1 | `backup_service.go:661` `RestoreBackup` | 物理恢复会覆盖目标实例 datadir。已 gate 备份但**未 gate 恢复** | `CapPhysicalBackup` |
| G2 | `scale_service.go:322` `ScaleIn` | 调 arch plugin `RunLeave` 后**删除实例记录** | `CapScale` (新增) |
| G3 | `scale_service.go:404` `RebuildNode` | 跑 `<flavor>-core` kernel plugin 重建节点数据 | `CapNodeRebuild` (新增) |
| G4 | `rebuild_service.go:43` `RebuildNode` | 先 `RunTeardown` 再 `RunExecute`，销毁性操作 | `CapNodeRebuild` (新增) |
| G5 | `cluster_lifecycle_service.go:102` `RebuildCluster` | 集群级重建 | `CapNodeRebuild` (新增) |

### 中危（会调度无意义的 MySQL agent 任务）

| # | 位置 | 问题 | 所需能力 |
|---|---|---|---|
| G6 | `scale_service.go:67` `ScaleOut` | 用 `<flavor>-core` plugin 装 MySQL 节点 | `CapScale` (新增) |
| G7 | `instance_service.go` `Deploy` | 单实例 MySQL 部署 | `CapInstanceDeploy` (新增) |
| G8 | `upgrade_service.go:489` `ExecuteLogicalMigration` | mysqldump/mysqlpump 逻辑迁移 | `CapLogicalUpgrade` (新增) |
| G9 | `upgrade_service.go:605` `ExecuteRollingUpgrade` | 集群滚动升级，硬编码 `"mysql"` flavor | `CapInPlaceUpgrade` |
| G10 | `instance_service.go` `ReplicationStatus` | 查 `SHOW SLAVE STATUS` 等 | `CapReplication` |
| G11 | `parameter_template_service.go:155` `Apply` | 下发 my.cnf 参数到实例 | `CapParameterTemplate` (新增) |

> G9 的 `normalizeRequestedTargetVersion(req.TargetVersion, "", "mysql")` 硬编码 flavor 为 `mysql`，说明该路径从设计上就假定 MySQL；补 gate 而非改逻辑。

---

## 三、其它按型号的差异化改动

### D1：`DetectVersion` 对 PG 兼容引擎不可用
`instance_service.go` 的 `DetectVersion` 走 agent `version-detect` 任务，执行 `SELECT @@version, @@version_comment`。这是 MySQL 语法：

- PG 兼容引擎（Kingbase / openGauss / HighGo / GBase 8a / 神舟）应改用已有的 `DBConnector.ServerVersion()`（执行 `SELECT version()`）
- 无驱动引擎（DM / GBase 8s）应明确返回"不支持自动版本检测"，而不是让 agent 任务报一个含混的 SQL 错误

`ServerVersion()` 在阶段 C 已实现但**目前没有任何调用点**——这是它的第一个真实用途。

### D2：前端未按 flavor 收敛操作入口
后端 gate 已就位，但前端仍对所有实例展示 failover / 切换 / 备份 / 升级按钮。用户点击后只会拿到后端错误。

需要一份与后端能力表对齐的前端能力映射，在 UI 层隐藏或禁用不适用的操作。**后端 gate 仍是权威**，前端只做体验优化。

### D3：能力矩阵测试缺少"新增能力"覆盖
新增 5 个能力常量后，`flavor_capability_test.go` 的矩阵断言需同步扩展，否则新能力对各 flavor 的取值无人验证。

---

## 四、实施步骤

### 步骤 1：扩展能力常量与矩阵
- [ ] 在 `flavor_capability.go` 新增 `CapScale`、`CapNodeRebuild`、`CapInstanceDeploy`、`CapLogicalUpgrade`、`CapParameterTemplate`
- [ ] 补 `capabilityLabels` 中文标签
- [ ] `mysqlProtocolCapabilities()` 全部置 true；`tieredOnboardingCapabilities()` 全部置 false
- [ ] 扩展 `flavor_capability_test.go` 矩阵断言
- 验证：`go test -run TestHasCapability|TestRequireCapability`

### 步骤 2：补齐 5 处高危 gate（G1–G5）
- [ ] `RestoreBackup`：在取到 `target` 后立即 gate
- [ ] `ScaleIn` / `ScaleService.RebuildNode`：从 `instRepo` 解析 flavor 后 gate
- [ ] `RebuildService.RebuildNode`：gate 在 `RunTeardown` **之前**
- [ ] `ClusterLifecycleService.RebuildCluster`：集群级 flavor 解析后 gate
- 验证：每处配套测试，断言返回错误且**未发起 plugin/agent 调用**

### 步骤 3：补齐 6 处中危 gate（G6–G11）
- [ ] `ScaleOut`、`Deploy`、`ExecuteLogicalMigration`、`ExecuteRollingUpgrade`、`ReplicationStatus`、`ParameterTemplateService.Apply`
- 验证：同上

### 步骤 4：`DetectVersion` 按 flavor 分派（D1）
- [ ] MySQL 协议引擎：保持现有 agent 路径**完全不变**
- [ ] PG 兼容引擎：改用 `DBConnector.ServerVersion()`
- [ ] 无驱动引擎：返回明确的不支持错误
- 验证：断言 MySQL 路径零行为变化；`gbase8s` 返回可读错误

### 步骤 5：前端能力收敛（D2）
- [ ] 新增与后端对齐的 flavor 能力映射
- [ ] 对不支持的操作按钮做禁用 + tooltip 说明
- 验证：`npm run build`

### 步骤 6：文档
- [ ] 更新 `tasklist.md` 能力矩阵，纳入新增 5 个能力

---

## 五、依赖顺序

```
步骤1 ──┬→ 步骤2 ──┐
        └→ 步骤3 ──┼→ 步骤5 → 步骤6
           步骤4 ──┘
```

步骤 1 是硬前置。步骤 2/3/4 可并行。

---

## 六、风险与缓解

| 风险 | 缓解 |
|---|---|
| 新增能力默认值写错导致 MySQL 实例被误拒 | 未登记/空 flavor 一律按 MySQL 放行；矩阵测试显式覆盖 `""` 与未知 flavor |
| gate 插入点位置不当（插在破坏性操作之后） | G4 明确要求 gate 在 `RunTeardown` 之前；测试断言"未发起调用"而非仅"返回错误" |
| 改 `DetectVersion` 影响既有 MySQL 版本检测 | MySQL 分支代码路径完全不动，仅新增 else 分支 |
| 前端禁用逻辑与后端能力表漂移 | 前端映射注释指向 `flavor_capability.go`；后端 gate 保持权威 |
| 集群级操作中实例 flavor 不一致 | 沿用 `clusterFlavorFromInstances` 取首个非空 flavor 的既有语义 |

---

## 七、验证命令

```powershell
cd D:\test_tmple\new_dbops\backend; go build ./...
cd D:\test_tmple\new_dbops\backend; go build -tags dm_driver ./...
cd D:\test_tmple\new_dbops\backend; go test ./...
cd D:\test_tmple\new_dbops\frontend; npm run build
```

基线：`go test ./internal/services/` 约 73–80s 且为 `ok`。

### 关键回归断言
1. 空 flavor 实例所有能力照旧可用
2. 每个新 gate 在拒绝时未发起 plugin / agent / SQL 调用
3. `RebuildService` 的 gate 早于 `RunTeardown`
4. MySQL 实例的 `DetectVersion` 行为不变
5. 已实现的 7 个 MySQL 兼容 flavor 识别结果不受影响
