# DBA 问题清单

扫描范围：failover / 复制 / my.cnf / 备份恢复 / HA 探活 / 参数模板 / 认证默认。  
状态：`open` | `fixed` | `wontfix`

| ID | 严重度 | 状态 | 问题 | 位置 | 修复/备注 |
|----|--------|------|------|------|-----------|
| Q1 | P0 | fixed | Failover 用 binlog 位点重建复制，GTID 拓扑下易追错 | `failover_service.go` RebuildReplication | `MASTER_AUTO_POSITION=1`；`f78116c` |
| Q2 | P0 | fixed | Promote 未关 `super_read_only` | `failover_service.go` PromoteToMaster | 先关 super_read_only 再关 read_only；`05fc1ef` |
| Q3 | P0 | fixed | 切主前只停 IO 线程；候选主不比 lag | StopReplication / SelectCandidate | `STOP SLAVE` + 最低 lag；`bb1a2c4` |
| Q4 | P0 | fixed | 主库判定 fallback 到集群第一台 | GetCurrentMaster | 无 primary role 直接失败；`5243ba3` |
| Q5 | P0 | fixed | MGR 模板 `group_name` 写成 seeds | `mysql80.cnf.tmpl` / config_renderer | 独立 `GroupName` UUID；`553aabe` |
| Q6 | P0 | fixed | full 备份立即 `--prepare` 打断增量链；restore 不关库不清空 datadir | `task_executor.go` | 备份不 prepare；restore 停库+wipe；`49ccd60` |
| Q7 | P1 | fixed | mysqlchk 只 ping，VIP 可挂只读从 | `mysqlchk.sh` | 要求 read_only/super_read_only=OFF；`99bfcb5` |
| Q8 | P1 | fixed | CHANGE MASTER 密码未转义 `'` | `replication_setup.go` | 单引号 escape；`99bfcb5` |
| Q9 | P1 | fixed | 默认 my.cnf 缺生产关键项；socket/pid 在 `/tmp` | templates + config_renderer inline | socket/pid/log-error 进 datadir；sync_binlog=1、flush=1、expire、relay_log_recovery、skip_name_resolve |
| Q10 | P1 | fixed | Preflight `Force=true` 直接 Pass，可跳过 lag/GTID | `failover_service.go` PreflightFailover | Force 只写 warning，不改 Pass |
| Q11 | P1 | fixed | xtrabackup 缺失时静默降级 mysqldump | `task_executor.go` | 显式 `xtrabackup` 失败；`auto` 选方法时无工具才选 mysqldump |
| Q12 | P2 | fixed | 参数模板 buffer pool 与规格描述不符 | `preset_templates.go` | small 1G / medium 4G / large 16G |
| Q13 | P2 | fixed | 8.0 默认 `mysql_native_password` | NewMySQLConfig + VersionAdapter | 默认空，沿用 server caching_sha2 |

## 进度

- [x] Q1–Q8（已推 origin/master）
- [x] Q9–Q13（本轮）
