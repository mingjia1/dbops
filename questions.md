# 问题清单

状态：`open` | `fixed` | `wontfix`

## DBA（已关闭）

| ID | 严重度 | 状态 | 问题 | 位置 | 修复/备注 |
|----|--------|------|------|------|-----------|
| Q1 | P0 | fixed | Failover 用 binlog 位点重建复制 | `failover_service.go` | `MASTER_AUTO_POSITION=1` |
| Q2 | P0 | fixed | Promote 未关 `super_read_only` | `failover_service.go` | 关 super_read_only+read_only |
| Q3 | P0 | fixed | 切主只停 IO；候选主不比 lag | StopReplication / SelectCandidate | `STOP SLAVE` + 最低 lag |
| Q4 | P0 | fixed | 主库 fallback 到集群第一台 | GetCurrentMaster | 无 primary 则失败 |
| Q5 | P0 | fixed | MGR `group_name` 写成 seeds | templates / config_renderer | 独立 GroupName UUID |
| Q6 | P0 | fixed | full 备份 prepare 断增量；restore 不关库 | `task_executor.go` | 备份不 prepare；restore 停库+wipe |
| Q7 | P1 | fixed | mysqlchk 只 ping | `mysqlchk.sh` | 要求可写 |
| Q8 | P1 | fixed | CHANGE MASTER 密码未转义 | `replication_setup.go` | escape `'` |
| Q9 | P1 | fixed | my.cnf 缺生产项；socket 在 /tmp | templates | datadir 路径 + sync/flush/expire |
| Q10 | P1 | fixed | Preflight Force 直接 Pass | `failover_service.go` | Force 只 warning |
| Q11 | P1 | fixed | xtrabackup 静默降级 | `task_executor.go` | 显式 method 不降级 |
| Q12 | P2 | fixed | 参数模板 buffer 与规格不符 | `preset_templates.go` | 1G/4G/16G |
| Q13 | P2 | fixed | 强制 mysql_native_password | NewMySQLConfig / VersionAdapter | 默认空 |

## 运维 / 测试（codegraph 扫库）

扫描工具：`codegraph explore/impact/affected/callers`  
重点符号：`callAgent`、`callAgentWithTimeout`、`ExecuteBackup`、`ExecuteRestore`、`PreflightFailover`、`SelectCandidateMaster`、`assertSafeRestoreDatadir`

| ID | 严重度 | 状态 | 问题 | 位置 | 修复/备注 |
|----|--------|------|------|------|-----------|
| O1 | P0 | fixed | 备份/恢复走 `callAgent`（2min），大库必超时 | `backup_service.go` / `ExecuteBackup` | 改 `callAgentLong`（5min） |
| O2 | P0 | fixed | 所有 POST 最多重试 3 次；restore/backup 非幂等 | `agent_client.go` | 任务路径 `maxRetries=0` |
| O3 | P1 | fixed | 前端 restore 无长 timeout | `api.ts` | restore `timeout: 300000` |
| O4 | P1 | fixed | 关键路径无覆盖测试 | tests | agent 非幂等/preflight Force/restore 安全/选主 lag |
| O5 | P1 | fixed | 启动脚本 Go 版本不一致 | `bin/ubuntu/start-agent.sh` | agent 对齐 1.25 |
| O6 | P2 | fixed | Makefile 引用不存在 compose/scripts | 根 `Makefile` | 坏目标明确 fail；dist 用 bin/ |
| O7 | P1 | fixed | Windows 下 `assertSafeRestoreDatadir("/var")` 漏拦 | `task_executor.go` | `filepath.ToSlash` 统一路径 |

## 进度

- [x] Q1–Q13
- [x] O1–O7
