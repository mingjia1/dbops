# DBOps Platform Documentation

This documentation describes the repository's React console, Go backend, and Go host agent. It is based on the source tree and build files currently in this repository.

## Core Documents

- `ARCHITECTURE.md`: components, responsibilities, and execution flow.
- `INTERFACES.md`: internal contracts relevant to database lifecycle work.
- `DEVELOPER_GUIDE.md`: build, test, and contribution commands.
- `ACCEPTANCE_MATRIX.md`: traceable fixture and real-environment validation matrix for completed 信创 single-node flavors.

## Current Lifecycle Work

### Kingbase V9R1C10 Monitoring And Teardown Boundary

Kingbase monitoring uses fixed `sys_ctl status`, `sys_stat_activity`, `sys_stat_archiver`, `ps`, and `df` snapshots. Teardown validates explicit confirmation and a controlled final-backup destination, then returns a zero-command refusal until a vendor-verifiable uninstall executable and parameter set is available. Replication, failover, and replica rebuild remain unavailable.

The 信创 executor work plan is recorded in `plan0731.md`. Local package bundles are validated by `agent/internal/executor/local_package_bundle.go` before flavor executors perform installation work. OceanBase provides the first dedicated single-node executor for deployment, configuration, backup and restore, upgrade and migration, monitoring, and teardown. TiDB provides an offline TiUP executor for single-host PD, TiKV, and TiDB deployment, configuration reload, TLS, BR backup and restore, PITR, Dumpling and Lightning migration, rolling upgrades, monitoring, and confirmed teardown. Dameng DM9 provides offline silent installation, explicit initialization, parameter configuration, account management, SSL enablement, online and offline backup, archive recovery, logical migration, monitoring, and confirmed uninstall. Kingbase KES V9R1C10 provides constrained deployment, configuration, physical `sys_basebackup`, and logical `sys_dump`/`sys_restore` migration workflows under `/opt/dbops/backups/kingbase/`; PITR and upgrades require verified official repository, archive, and upgrade prerequisites before execution. openGauss provides a constrained single-node executor for deployment, configuration, physical backup, logical restore and migration, monitoring snapshots, and confirmed final uninstall. GBase 8s provides fixed `onstat`, `ps`, and `df` monitoring snapshots; teardown validates confirmation and final backup inputs before returning a controlled refusal because public official documentation has no verified uninstall command.
