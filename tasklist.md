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
在服务入口 (failover / 角色切换 / 架构切换 / 集群部署 / 物理备份 / 原地升级) 处拒绝并返回可读错误，
不会对非 MySQL 引擎执行 MySQL 语句。

| flavor | SQL 健康检测 | 进程发现 | 复制 | 故障切换 | 集群部署 | 物理备份 | 原地升级 |
|---|---|---|---|---|---|---|---|
| Kingbase / openGauss / HighGo / GBase 8a / 神舟通用 | 是 (pgx) | 是 | 否 | 否 | 否 | 否 | 否 |
| 达梦 DM / GBase 8s | 否 (仅 TCP) | 是 | 否 | 否 | 否 | 否 | 否 |

flavor 为空或未登记的实例一律按 MySQL 兼容处理，保证既有纳管实例零回归。

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

## 备注

- OceanBase、GaussDB(for MySQL)、PolarDB(for MySQL)、TDSQL 走 MySQL 兼容协议，具备完整 MySQL 运维能力。
- 端口默认值仅作为进程命令行未携带端口时的兜底；优先解析命令行，达梦额外读取 dm.ini 的 PORT_NUM。
