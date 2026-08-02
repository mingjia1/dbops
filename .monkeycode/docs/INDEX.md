# DBOps Platform Documentation

This documentation describes the repository's React console, Go backend, and Go host agent. It is based on the source tree and build files currently in this repository.

## Core Documents

- `ARCHITECTURE.md`: components, responsibilities, and execution flow.
- `INTERFACES.md`: internal contracts relevant to database lifecycle work.
- `DEVELOPER_GUIDE.md`: build, test, and contribution commands.

## Current Lifecycle Work

The 信创 executor work plan is recorded in `plan0731.md`. Local package bundles are validated by `agent/internal/executor/local_package_bundle.go` before flavor executors perform installation work. OceanBase provides the first dedicated single-node executor for deployment, configuration, backup and restore, upgrade and migration, monitoring, and teardown. TiDB provides an offline TiUP executor for single-host PD, TiKV, and TiDB deployment, configuration reload, TLS, BR backup and restore, PITR, Dumpling and Lightning migration, and rolling upgrades.
