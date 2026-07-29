# 未实现信创数据库任务清单

按预计代码修改量从小到大排序。

| 优先级 | 数据库 | 当前状态 | 预计改动量 | 主要改动范围 |
|---|---|---|---|---|
| 1 | OceanBase | 已最小实现 | 小 | 已完成 flavor 识别和基础标签展示；不含部署编排 |
| 2 | TiDB | 未实现 | 小 | 当前不提供 flavor 识别、版本目录或部署能力 |
| 3 | GaussDB(for MySQL) | 已最小实现 | 小到中 | 已完成 MySQL 握手 flavor 识别和基础标签展示；不含云托管部署编排 |
| 4 | PolarDB(for MySQL) | 已最小实现 | 小到中 | 已完成 MySQL 握手 flavor 识别和基础标签展示；不含云托管部署编排 |
| 5 | TDSQL(MySQL 兼容) | 已最小实现 | 中 | 已完成 MySQL 握手 flavor 识别和基础标签展示；不含腾讯云托管部署编排 |
| 6 | Kingbase ES | 已最小实现 | 中 | 已完成 SSH 进程扫描和基础标签展示；不含 PostgreSQL 协议连接、部署或元数据库方言适配 |
| 7 | openGauss | 已最小实现 | 中 | 已完成 SSH 进程扫描和基础标签展示；不含 PostgreSQL 协议连接、部署或元数据库方言适配 |
| 8 | HighGo | 未实现 | 中 | PostgreSQL 兼容驱动接入、backend 元数据库方言验证、连接/迁移适配 |
| 9 | GBase 8s/8a | 未实现 | 大 | 专用驱动/协议适配、SQL 方言、元数据存储或目标库管理逻辑 |
| 10 | 达梦 DM | 未实现 | 大 | 专用驱动、SQL 方言、迁移脚本、backend 元数据库兼容、部署/运维能力 |
| 11 | 神舟通用 | 未实现 | 大 | 专用驱动/协议适配、SQL 方言、部署和管理能力从零补齐 |

## 备注

- 当前代码库已对 TiDB 做了少量 flavor 识别，但未形成完整管理能力。
- OceanBase、GaussDB(for MySQL)、PolarDB(for MySQL)、TDSQL 可优先按 MySQL 兼容协议做最小接入。
- 达梦、GBase、神舟通用属于大改动，不建议和 MySQL 兼容数据库混在同一批实现。
