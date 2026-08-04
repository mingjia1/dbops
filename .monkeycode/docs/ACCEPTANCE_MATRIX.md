# 信创单机生命周期验收矩阵

此矩阵将自动化夹具覆盖与真实环境验收分开记录。自动化夹具使用临时本地介质、校验 manifest 和注入命令执行器，不会安装、卸载或修改真实数据库实例。

| Flavor | 基线版本 | 架构 | 阶段 | 自动化场景 | 真实环境状态 |
| --- | --- | --- | --- | --- | --- |
| OceanBase | `v4.4.2_CE_BP2` | RPM 单机 | 部署与配置 | `TestOceanBaseDeployInstallsStartsAndCreatesTenant`、`TestOceanBaseDeployConfiguresTLS` | 待提供已校验 RPM、证书和目标主机 |
| OceanBase | `v4.4.2_CE_BP2` | RPM 单机 | 备份、恢复、迁移与升级 | `TestOceanBaseBackupConfiguresArchiveAndVerifiesJob`、`TestOceanBaseRestoreWaitsAndValidatesTenantData`、`TestOceanBaseUpgradeCreatesBackupBeforeObshellUpgrade` | 待提供备份存储和源数据 |
| OceanBase | `v4.4.2_CE_BP2` | RPM 单机 | 监控、卸载与多节点边界 | `TestOceanBaseMonitoringCollectsVersionServerAndTenantMetrics`、`TestOceanBaseTeardownBacksUpStopsAndRemovesApprovedPaths`、`TestOceanBaseRejectsMultiNodeOperations` | 待提供受控最终备份目录 |
| TiDB | `v8.5.7` | amd64 单机 PD/TiKV/TiDB | 部署、配置与 TLS | `TestTiDBDeploysOfflineHybridTopology`、`TestTiDBConfigureReloadsComponentConfiguration`、`TestTiDBDeployIncludesTLSConfiguration` | 待提供官方 server/toolkit 归档、证书和目标主机 |
| TiDB | `v8.5.7` | amd64 单机 PD/TiKV/TiDB | 备份、恢复、迁移与升级 | `TestTiDBBackupStartsBRAndLogBackup`、`TestTiDBRestoreUsesPITRAndValidatesData`、`TestTiDBMigrationAndUpgradeUseApprovedPaths` | 待提供 BR 存储和迁移数据集 |
| TiDB | `v8.5.7` | amd64 单机 PD/TiKV/TiDB | 监控、卸载与多节点边界 | `TestTiDBMonitorTeardownAndSingleHostBoundaries` | 待提供单机集群和受控最终备份目录 |
| DM | `DM9` | 单机 | 部署、配置与 TLS | `TestDamengDeploysOfflineInstanceAndVerifiesSQL`、`TestDamengTLSUsesServerEncryptionSetting` | 待提供 DM9 安装包、证书、目标主机与 flavor Agent 任务入口 |
| DM | `DM9` | 单机 | 备份、恢复与迁移 | `TestDamengBackupRestoreAndMigrationUseControlledPaths` | 待提供备份集、归档集和迁移数据 |
| DM | `DM9` | 单机 | 监控、卸载与多节点边界 | `TestDamengMonitorTeardownAndSingleInstanceBoundaries` | 待提供可访问实例和受控最终备份目录 |
| Kingbase | `KES V9R1C10` | 单机 | 部署、配置、角色与 TLS | `TestKingbaseDeploysWithSilentSetupAndInputOnlyCredentials`、`TestKingbaseConfigureAppliesTLSAndExactCIDR`、`TestKingbaseRejectsUnsafeBundlesTLSCIDRsAndOperations` | 待提供授权安装包、许可证、普通证书文件和目标主机 |
| openGauss | `6.0.5` | 单机 | 部署、配置、备份、恢复与迁移 | `TestOpenGaussDeploysSingleNodeAndCreatesApplicationUser`、`TestOpenGaussTLSUsesOfficialGUCSettings`、`TestOpenGaussBackupRestoreAndMigrationUseControlledPaths` | 待提供已校验安装包、证书、目标主机和备份目录 |
| openGauss | `6.0.5` | 单机 | 监控、卸载与多节点边界 | `TestOpenGaussMonitorCollectsControlledSingleNodeSnapshot`、`TestOpenGaussTeardownBacksUpBeforeFixedUninstall`、`TestOpenGaussTeardownDoesNotUninstallWhenFinalBackupFails`、`TestOpenGaussRejectsUnsafeLifecycleRequests` | 待提供可访问单机实例和受控最终备份目录 |
| GBase 8s | `vendor-supported` | 单机 | 部署、配置、备份、恢复与迁移 | `TestGBase8sDeploysAndVerifiesDocumentedSingleNodeWorkflow`、`TestGBase8sConfigureUsesPersistentAndMemoryModes`、`TestGBase8sBackupRestoreAndMigrationUseControlledDirectories`、`TestGBase8sRejectsBackupMigrationBoundaryAndExternalUpgrade` | 待提供厂商介质、目标主机和备份目录 |
| GBase 8s | `vendor-supported` | 单机 | 监控、teardown 拒绝与多节点边界 | `TestGBase8sMonitorsWithFixedOnstatProcessAndDiskCommands`、`TestGBase8sTeardownRequiresConfirmationBackupAndVerifiedUninstallPath`、`TestGBase8sRejectsUnsupportedLifecycleOperations` | 公开官方入口缺少可核验的卸载命令，等待厂商书面流程 |

运行 `make test-xinchuang-lifecycle` 可执行已完成 flavor 的 Agent 生命周期夹具、内核 Agent 路由契约、后端 capability 门禁和前端 capability 镜像测试。openGauss 与 GBase 8s 的已实现操作仍通过专属 flavor Agent 路由执行，通用 MySQL API 与前端操作保持禁用直到完成后端路由接入。真实环境验收必须记录实际安装包 SHA-256、运行日期、目标架构、任务标识和每个阶段的 Agent 结果。

## 自动化验证记录

- 日期：2026-08-03
- 后端：`cd backend && go test ./...` 通过。
- Agent：`cd agent && go test ./...` 通过。
- 前端：`cd frontend && npm test` 通过，17 个测试文件与 292 个测试通过。
- 前端构建：`cd frontend && npm run build` 通过。
- 静态检查：`cd backend && go vet ./internal/services` 通过。
- 真实环境：OceanBase、TiDB 和 DM 的介质、目标主机、备份存储与阶段性 Agent 结果尚未提供。
