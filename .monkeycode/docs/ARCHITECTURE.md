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

`backend/internal/services/flavor_capability.go` is the capability gate for database flavors. `completedSingleNodeCapabilities` records only lifecycle operations backed by a completed flavor-specific executor. The single-node capability constructor permits deployment, configuration, backup and restore, upgrade and migration, monitoring, and teardown-related instance administration. Replication, failover, scale, node rebuild, and cluster deployment remain disabled until multi-node integration tests prove those workflows.

`agent/internal/executor/local_package_bundle.go` validates pre-positioned package bundles. It reads `<root>/<flavor>/<version>/manifest.json`, verifies the requested flavor and version, each regular package file, its SHA-256, and a required license file. The default root is `/opt/dbops/packages`.

`backend/internal/plugins/kernel/xinchuang_core.go` provides the common lifecycle base for dedicated flavor executors. It validates the target Agent, dispatches each lifecycle phase through the flavor's Agent route, and accepts only a terminal successful Agent task status.

`agent/internal/executor/flavor_task_executor_fixture_test.go` provides an isolated integration fixture using a temporary local bundle and injected command runner. It validates success, missing-tool, checksum-failure, and permission-failure outcomes without requiring a database host. `agent/internal/executor/oceanbase_task_executor.go` implements OceanBase single-node deployment, configuration, physical backup, restore-based tenant migration, in-place upgrade, monitoring snapshots, and teardown. `agent/internal/executor/tidb_task_executor.go` validates official offline TiUP server and toolkit archives, writes a single-host PD, TiKV, and TiDB topology, then deploys, safely starts, verifies, and reloads the topology through TiUP. Teardown requires explicit request confirmation, a successful backup, a fixed cluster stop command, and protected removal targets. Backup and restore status queries use bounded polling, and restore completes only after structured schema, row-count, and checksum checks. HA, replication, and failover return an explicit multi-node capability error. The kernel contract suite verifies the deploy-failure and rollback task sequence.

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
