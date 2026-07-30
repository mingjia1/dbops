# 未实现信创数据库任务清单

按预计代码修改量从小到大排序。

| 优先级 | 数据库 | 当前状态 | 预计改动量 | 主要改动范围 |
|---|---|---|---|---|
| 1 | OceanBase | 已最小实现 | 小 | 已完成 flavor 识别和基础标签展示；不含部署编排 |
| 2 | TiDB | 未实现 | 小 | 当前不提供 flavor 识别、版本目录或部署能力 |
| 3 | GaussDB(for MySQL) | 已最小实现 | 小到中 | 已完成 MySQL 握手 flavor 识别和基础标签展示；不含云托管部署编排 |
| 4 | PolarDB(for MySQL) | 已最小实现 | 小到中 | 已完成 MySQL 握手 flavor 识别和基础标签展示；不含云托管部署编排 |
| 5 | TDSQL(MySQL 兼容) | 已最小实现 | 中 | 已完成 MySQL 握手 flavor 识别和基础标签展示；不含腾讯云托管部署编排 |
| 6 | Kingbase ES | 已分层纳管 | 中 | SSH 进程扫描、pgx 只读连接健康检测、标签展示；部署/复制/切换/物理备份/升级显式不支持 |
| 7 | openGauss | 已分层纳管 | 中 | SSH 进程扫描、pgx 只读连接健康检测、标签展示；部署/复制/切换/物理备份/升级显式不支持 |
| 8 | HighGo | 已分层纳管 | 中 | SSH 进程扫描、pgx 只读连接健康检测、标签展示；部署/复制/切换/物理备份/升级显式不支持 |
| 9 | GBase 8a | 已分层纳管 | 中 | gclusterd/gbased 双层进程扫描、pgx 只读连接健康检测、标签展示；部署/复制/切换/物理备份/升级显式不支持 |
| 10 | 神舟通用 (OSCAR) | 已分层纳管 | 中 | oscarserver/osrvr 进程扫描、pgx 只读连接健康检测、标签展示；部署/复制/切换/物理备份/升级显式不支持 |
| 11 | 达梦 DM | 已分层纳管 | 中 | dmserver 进程扫描 + dm.ini PORT_NUM 解析、标签展示；无纯 Go 驱动, 健康检测降级为 TCP 探测；部署/复制/切换/物理备份/升级显式不支持 |
| 12 | GBase 8s | 已分层纳管 | 中 | oninit 进程扫描、标签展示；Informix 协议无纯 Go 驱动, 健康检测降级为 TCP 探测；部署/复制/切换/物理备份/升级显式不支持 |

## 分层纳管说明

「已分层纳管」指平台提供纳管、健康检测、拓扑与类型展示，但不提供 MySQL 专属运维能力。

能力边界由 `backend/internal/services/flavor_capability.go` 的静态表统一约束，
在 21 处服务入口拒绝并返回可读错误，不会对非 MySQL 引擎执行 MySQL 语句或调度 MySQL 专属 agent 任务。

前端 `frontend/src/services/flavorCapability.ts` 镜像同一张表，用于禁用注定会被拒绝的操作入口；
**后端 gate 始终是权威**，前端仅做体验优化。

### 能力矩阵

| 能力 | MySQL 兼容 | Kingbase / openGauss / HighGo / GBase 8a / 神舟 | 达梦 DM / GBase 8s |
|---|---|---|---|
| SQL 健康检测 `health_sql` | 是 | 是 (pgx) | 否 (仅 TCP) |
| 主从复制 `replication` | 是 | 否 | 否 |
| 故障切换 `failover` | 是 | 否 | 否 |
| 集群架构部署 `cluster_deploy` | 是 | 否 | 否 |
| 物理备份/恢复 `backup_physical` | 是 | 否 | 否 |
| 原地升级 `upgrade_inplace` | 是 | 否 | 否 |
| 逻辑迁移升级 `upgrade_logical` | 是 | 否 | 否 |
| 节点扩缩容 `scale` | 是 | 否 | 否 |
| 节点重建 `node_rebuild` | 是 | 否 | 否 |
| 单实例部署 `instance_deploy` | 是 | 否 | 否 |
| 参数模板下发 `parameter_template` | 是 | 否 | 否 |
| 实例管理操作 `instance_admin` | 是 | 否 | 否 |

flavor 为空或未登记的实例一律按 MySQL 兼容处理，保证既有纳管实例零回归。

### 已受 gate 约束的服务入口

`failover_service.go`（自动/手动切换、preflight）、`switch_service.go`（架构切换单/多实例、角色切换）、
`universal_cluster_deploy.go`（集群部署）、`backup_service.go`（物理备份、物理恢复）、
`upgrade_service.go`（原地升级、逻辑迁移、滚动升级）、`scale_service.go`（扩容、缩容、节点重建）、
`rebuild_service.go`（节点重建，gate 早于 teardown）、`cluster_lifecycle_service.go`（集群重建，gate 早于 destroy）、
`instance_service.go`（单实例部署、复制状态、实例管理操作、批量改密）、`parameter_template_service.go`（参数下发）

`instance_admin` 覆盖 agent 的 instance-admin 任务族：`create_user` / `grant_privileges` / `set_variable` /
`read_config` / `write_config` / `service_control` / `decommission` 等，均为 MySQL DDL/DCL 与 my.cnf 操作。

### 实例删除

分层纳管的实例并非由平台部署，不存在可取的 xtrabackup 镜像，也不应由平台删除远端 datadir。
`InstanceService.Delete` 对这些引擎走**仅注销**路径：删除平台侧记录并写审计
（`deregistered_only=true`），远端数据库不受影响。MySQL 实例仍保持「先全量备份 + 退役，再删除」的约束。

### 版本检测

`instance_service.go` 的 `DetectVersion` 按 flavor 分派：

- MySQL 协议引擎：走 agent `version-detect` 任务（`SELECT @@version, @@version_comment`），行为不变
- PG 兼容引擎：走 `DBConnector.ServerVersion()`（`SELECT version()`）
- 无驱动引擎（达梦 / GBase 8s）：返回「不支持自动版本检测」，需手动维护版本信息


## 连接层

`backend/internal/services/db_connector.go` 按 wire protocol 而非厂商分派：

- MySQL 协议：mysql / mariadb / percona / oceanbase / gaussdb-mysql / polardb-mysql / tdsql-mysql
- PostgreSQL 协议 (pgx)：kingbase / opengauss / highgo / gbase8a / shentong
- 无纯 Go 驱动：dm (私有协议)、gbase8s (Informix 协议) — 返回 `ErrNoSQLConnector`，健康检测降级为 TCP + SSH 进程发现

## 不在范围内

以下能力对分层纳管的数据库明确不提供，需要单独立项：

- 通过平台部署安装达梦 / GBase / 神舟（依赖厂商商业安装包与授权）
- 主从复制搭建、自动故障切换、MGR/PXC/MHA 架构部署
- xtrabackup 物理备份、mysql_upgrade 原地升级
- 将 backend 元数据库迁移到国产数据库

## 元数据库 schema

`instance_versions` 的版本列已放宽以容纳非 MySQL 引擎的版本串：

- `version` VARCHAR(64)、`full_version` VARCHAR(512)
- 原因：PG 兼容引擎的 `SELECT version()` 返回散文式 banner（openGauss 约 100 字符），
  Kingbase 返回 `KingbaseES V008R006C008B0014 ...`；原 VARCHAR(32)/VARCHAR(64) 在
  `STRICT_TRANS_TABLES` 下会直接报错
- 已有 MySQL 部署通过幂等的 `ALTER TABLE ... MODIFY COLUMN` 在启动时自动升级

## 备注

- OceanBase、GaussDB(for MySQL)、PolarDB(for MySQL)、TDSQL 走 MySQL 兼容协议，具备完整 MySQL 运维能力。
- 端口默认值仅作为进程命令行未携带端口时的兜底；优先解析命令行，达梦额外读取 dm.ini 的 PORT_NUM。
