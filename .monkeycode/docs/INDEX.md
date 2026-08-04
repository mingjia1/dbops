# DBOps Platform Documentation

This documentation describes the repository's React console, Go backend, and Go host agent. It is based on the source tree and build files currently in this repository.

## Core Documents

- `ARCHITECTURE.md`: components, responsibilities, and execution flow.
- `INTERFACES.md`: internal contracts relevant to database lifecycle work.
- `DEVELOPER_GUIDE.md`: build, test, and contribution commands.
- `ACCEPTANCE_MATRIX.md`: traceable fixture and real-environment validation matrix for completed 信创 single-node flavors.

## Current Lifecycle Work

The 信创 executor work plan is recorded in `plan0731.md`. Local package bundles are validated by `agent/internal/executor/local_package_bundle.go` before flavor executors perform installation work. OceanBase provides the first dedicated single-node executor for deployment, configuration, backup and restore, upgrade and migration, monitoring, and teardown. TiDB provides an offline TiUP executor for single-host PD, TiKV, and TiDB deployment, configuration reload, TLS, BR backup and restore, PITR, Dumpling and Lightning migration, rolling upgrades, monitoring, and confirmed teardown. Dameng DM9 provides offline silent installation, explicit initialization, parameter configuration, account management, SSL enablement, online and offline backup, archive recovery, logical migration, monitoring, and confirmed uninstall. openGauss provides a constrained single-node executor for deployment, configuration, physical backup, logical restore and migration, monitoring snapshots, and confirmed final uninstall. GBase 8s provides fixed `onstat`, `ps`, and `df` monitoring snapshots; teardown validates confirmation and final backup inputs before returning a controlled refusal because public official documentation has no verified uninstall command.
