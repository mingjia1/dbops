# Architecture

## Overview

DBOps Platform is a database operations system with a React web console, a Go backend, and a Go agent that executes host-side tasks. The root README identifies backend administration on port 8080, the web console on port 3000, and the agent service on port 9090.

## Components

| Component | Location | Responsibility |
| --- | --- | --- |
| Web console | `frontend/` | React, TypeScript, Ant Design user interface. |
| Backend | `backend/` | Gin HTTP service, persistence integrations, task orchestration, capability gates, and plugins. |
| Agent | `agent/` | Go host-side task execution, installation helpers, migrations, upgrades, and local package verification. |

## Lifecycle Safety Boundaries

Kingbase monitoring returns a fixed snapshot from `sys_ctl status -D`, `sys_stat_activity`, `sys_stat_archiver`, `ps -C kingbase`, and `df -P <data_dir>`. Teardown accepts only `confirm_uninstall` with a final backup destination under `/opt/dbops/backups/kingbase/`; because the public KES V9R1C10 documentation has no verified uninstall executable and parameter set, it returns a controlled failure without backup, shutdown, or cleanup calls. Replication, failover, scale, and rebuild return explicit single-node capability errors.

`backend/internal/services/flavor_capability.go` is the capability gate for database flavors. `completedSingleNodeCapabilities` records only lifecycle operations backed by a completed flavor-specific executor. The single-node capability constructor permits deployment, configuration, backup and restore, upgrade and migration, monitoring, and teardown-related instance administration. Replication, failover, scale, node rebuild, and cluster deployment remain disabled until multi-node integration tests prove those workflows.

`frontend/src/services/flavorCapability.ts` mirrors this gate for action visibility. OceanBase and TiDB expose only their completed single-node operations in the console. DM uses a proprietary protocol, so its executor remains available only through the dedicated flavor Agent route while generic MySQL service actions retain their capability boundary.

`agent/internal/executor/local_package_bundle.go` validates pre-positioned package bundles. It reads `<root>/<flavor>/<version>/manifest.json`, verifies the requested flavor and version, each regular package file, its SHA-256, and a required license file. The default root is `/opt/dbops/packages`.

`backend/internal/plugins/kernel/xinchuang_core.go` provides the common lifecycle base for dedicated flavor executors. It validates the target Agent, dispatches each lifecycle phase through the flavor's Agent route, and accepts only a terminal successful Agent task status.

`agent/internal/executor/flavor_task_executor_fixture_test.go` provides an isolated integration fixture using a temporary local bundle and injected command runner. It validates success, missing-tool, checksum-failure, and permission-failure outcomes without requiring a database host. `agent/internal/executor/gbase8a_task_executor.go` accepts GBase 8a 10.1 deployment and configuration only through the dedicated flavor Agent. It requires a licensed manifest containing `SetSysEnv.py`, `gcinstall.py`, `demo.options`, and `gcChangeInfo.xml`; runs the approved environment, silent install, and single-node distribution commands; and permits only `gcluster_rebalancing_concurrent_count` values from 1 through 64 via `gccli SET GLOBAL`. TLS checks are restricted to ordinary absolute files under `/opt/dbops/gbase8a/certs/`; execution remains refused while a vendor restart command is unverified. `agent/internal/executor/oceanbase_task_executor.go` implements OceanBase single-node deployment, configuration, physical backup, restore-based tenant migration, in-place upgrade, monitoring snapshots, and teardown. `agent/internal/executor/tidb_task_executor.go` validates official offline TiUP server and toolkit archives, writes a single-host PD, TiKV, and TiDB topology, then deploys, safely starts, verifies, and reloads the topology through TiUP. Teardown requires explicit request confirmation, a successful backup, a fixed cluster stop command, and protected removal targets. Backup and restore status queries use bounded polling, and restore completes only after structured schema, row-count, and checksum checks. HA, replication, and failover return an explicit multi-node capability error. `agent/internal/executor/gbase8s_task_executor.go` emits only fixed `onstat -c`, `onstat -g cfg`, `onstat -V`, `ps -C oninit`, and `df -P <install>` monitoring commands. Its teardown validates `confirm_uninstall` and final `ontape` backup directories, then refuses without issuing a command until an official uninstall route is verified. The kernel contract suite verifies the deploy-failure and rollback task sequence.

```mermaid
flowchart LR
    Console[Web Console] --> Backend[Backend Service]
    Backend --> Capability[Flavor Capability Gate]
    Capability --> Agent[Host Agent]
    Agent --> Bundle[Local Package Bundle]
    Bundle --> Executor[Flavor Executor]
    Executor --> Result[Agent Task Result]
```

The local bundle validator only performs reads and hashes. Installation, extraction, downloads, and service commands are outside this validator.

`agent/internal/executor/kingbase_task_executor.go` restricts KES V9R1C10 installation and data paths to `/opt/dbops/kingbase/`, and backup assets to `/opt/dbops/backups/kingbase/`. It requires a licensed bundle containing `setup.sh` and `silent.conf`, initializes SCRAM credentials through the input runner, and permits controlled deploy, configure, physical backup, logical restore, and logical migration. Physical backup runs fixed `sys_basebackup -D <destination> -Fp -Pv -Xf`; migration uses `sys_dump -Fc -f <dump> <database>` so no shell redirect is required, followed by fixed `sys_restore -d <newdb> <dump>`. TLS checks regular absolute certificate files and a canonical non-global CIDR before it writes a `hostssl` rule. PITR requires verified `sys_rman` repository and archive configuration, while upgrades require a verified official `sys_upgrade` procedure with new and old controlled paths, an independent backup, and a target version; both currently stop before issuing a host command.
