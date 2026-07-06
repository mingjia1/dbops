# MySQL Ops Platform

> A database operations platform for architecture-level MySQL lifecycle management.
>
> It manages hosts, MySQL instances, clusters, middleware, monitoring, backup, upgrade, and role switching through a backend API, React console, and host-side Agent.

[![Go Version][go-image]][go-url] [![Node.js][node-image]][node-url] [![License][license-image]][license-url] [![Language][lang-image]][lang-url] [![Status][status-image]][status-url] [![Build][build-image]][build-url]

## Overview

MySQL Ops Platform is built around three components:

- `backend`: Go API server, deployment orchestration, metadata, audit, monitoring ingestion, and task coordination.
- `frontend`: React + TypeScript web console for cluster deployment, host management, monitoring, backup, upgrade, and role switching.
- `agent`: Go host-side service that executes MySQL, middleware, backup, health check, and upgrade tasks through authenticated HTTP APIs.

Supported database architectures:

- Standalone MySQL instance management
- HA master/replica
- MHA
- MGR
- PXC

## Latest Features

- Flow-based cluster deployment orchestration powered by React Flow.
- Cluster deployment detail pages with live step progress, status colors, plan preview, and status polling.
- Shared deployment plan model for form deployment and flow orchestration.
- Middleware add-ons for Keepalived and ProxySQL through Agent task APIs.
- Deployment tools for environment precheck, post-deploy health check, and baseline backup.
- Metadata synchronization after deployment, including cluster and instance registration.
- Resilient deployment state handling for failed, partial, interrupted, and in-progress deployments.
- Agent management with batch install, update, delete, status check, heartbeat, last action, and version display.
- Upgrade management for multiple architectures, including role-aware rolling upgrade planning.
- Improved HA/MHA replication status detection and MGR/PXC role/deployment handling.
- Monitoring collection loop from Agent metrics to backend ingestion and frontend no-data/config states.
- Secret hygiene through `.env.example`, example backend config, credential encryption, and local secret scanning script.

## Deployment Orchestration

The cluster deployment page now supports two deployment modes:

- **Flow Orchestration**: the default mode. Users choose an architecture, configure database nodes, add compatible middleware or tools, preview the generated plan, then submit deployment.
- **Form Deployment**: the existing form-based deployment path remains available as a fallback.

The flow editor supports these first-version nodes:

- Architecture nodes: HA, MHA, MGR, PXC
- Database nodes: master/replica, manager, primary/secondary, bootstrap/secondary
- Middleware nodes: Keepalived, ProxySQL
- Tool nodes: environment precheck, health check, baseline backup

Compatibility rules:

| Architecture | Keepalived | ProxySQL | Precheck | Health Check | Baseline Backup |
|--------------|------------|----------|----------|--------------|-----------------|
| HA | Supported | Supported | Supported | Supported | Supported |
| MHA | Supported | Supported | Supported | Supported | Supported |
| MGR | Disabled in v1 | Supported | Supported | Supported | Supported |
| PXC | Disabled in v1 | Supported | Supported | Supported | Supported |

Flow orchestration uses the existing deployment APIs:

- `POST /deployments/validate`
- `POST /deployments`
- `GET /deployments/:id/status`
- `GET /deployments/:id/plan`

The flow JSON is stored in the deployment request custom payload under `custom.flow_spec`, so no separate template table is required for the first version.

## Agent Capabilities

The Agent is the execution boundary for host-side operations. Current supported capabilities include:

- Host and Agent lifecycle: install, update, delete, status check, heartbeat, version reporting.
- MySQL instance operations: deploy, start, stop, restart, remove, status check.
- Cluster tasks: HA/MHA replication setup and checks, MGR setup and role switching support, PXC setup and status checks.
- Middleware tasks:
  - `POST /agent/tasks/keepalived-setup`
  - `POST /agent/tasks/proxysql-setup`
- Upgrade tasks: in-place, logical migration, and role-aware rolling upgrade execution support.
- Metrics collection: CPU, memory, disk, MySQL, replication, and service health data.

ProxySQL setup uses the target host Agent port, not the ProxySQL admin port. Keepalived is currently enabled only for HA/MHA deployment flows.

## Architecture

```text
frontend (:3000 or Vite dev port)
        |
        | REST API /api/v1
        v
backend (:8080)  ---- HTTP + Bearer token ---->  agent (:9090)
        |
        | metadata, audit, tasks, monitoring
        v
SQLite or MySQL, depending on storage mode
```

Optional integrations:

- Redis for cache and queue scenarios.
- ClickHouse for monitoring data storage.
- Keepalived and ProxySQL for HA/MHA/MGR/PXC traffic management patterns.

## Repository Layout

```text
backend/            Go backend API, services, repositories, config, and migrations
frontend/           React + TypeScript web console
agent/              Go execution agent deployed on managed hosts
bin/                Helper binaries and scripts
scripts/            Operational helper scripts, including local secret scanning
docs/               Supplemental documents and screenshots
data/               Local development data
logs/               Local runtime logs
Makefile            Build, test, install, dist, and upgrade helpers
start.bat/.ps1      Windows all-in-one startup
stop.bat/.ps1       Windows all-in-one shutdown
```

## Requirements

| Component | Version | Notes |
|-----------|---------|-------|
| Go | Backend 1.25+, Agent 1.21+ | Backend and Agent build/runtime |
| Node.js | 18+ | Frontend build/runtime |
| npm | Required | Frontend dependency management |
| PowerShell | 5.1+ | Windows scripts |
| bash | Required on Linux/macOS | Shell scripts |
| Redis | Optional | Cache/queue scenarios |
| ClickHouse | Optional | Monitoring storage |

Target MySQL hosts should have the required OS permissions and MySQL tools for the selected operation. Deployment flows may also require MySQL packages, backup tools, Keepalived, ProxySQL, or package download access depending on the selected plan.

## Configuration

Copy `.env.example` to `.env` and set strong values before running a non-local environment:

```env
DBOPS_DB_URL=dbops_user:replace-with-strong-password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local
DBOPS_JWT_SECRET=replace-with-at-least-32-chars
DBOPS_AGENT_TOKEN=replace-with-at-least-16-chars
DBOPS_ENCRYPTION_KEY=replace-with-at-least-32-chars
```

Example backend configuration is available at:

```text
backend/config/config.example.yaml
```

Security notes:

- Do not commit real secrets, tokens, database passwords, SSH keys, or license material.
- Sensitive credentials are encrypted with AES-GCM.
- Run the local secret scanner before publishing changes:

```powershell
.\scripts\scan-local-secrets.ps1
```

## Build And Run

Install dependencies:

```bash
make install-backend
make install-agent
make install-web
```

Build all components:

```bash
make build
```

Run components manually:

```bash
cd backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd frontend && npm run dev -- --host 0.0.0.0 --port 3000
```

Windows one-click startup:

```powershell
.\start.bat
```

Default local URLs:

- Backend API: `http://localhost:8080`
- Web console: `http://localhost:3000`
- Agent service: `http://localhost:9090`

## Test

```bash
cd backend && go test ./...
cd agent && go test ./...
cd frontend && npm test -- --run
cd frontend && npm run build
```

The recent validation suite covers backend deployment planning, repositories, authentication, password encryption, Agent middleware task routes, Agent metrics collection, frontend deployment helpers, role display, and flow-spec conversion.

## Operations Notes

- Long-running operations should be executed through backend APIs and Agent tasks.
- Deployment progress should be read from deployment status and plan endpoints rather than inferred from frontend state only.
- Middleware and tool failures after core database deployment can produce a `partial` deployment status while retaining created cluster resources.
- Interrupted deployments are detected on backend startup and marked accordingly.
- For MGR and PXC deployments, verify host package layout, MySQL data directories, plugin availability, and Agent version before redeploying.

## Documentation

- Chinese documentation: [readme_ZH.md](readme_ZH.md)
- English documentation: [readme_US.md](readme_US.md)
- Screenshots: [docs/screenshots](docs/screenshots)
- Secret scanning script: [scripts/scan-local-secrets.ps1](scripts/scan-local-secrets.ps1)

## Commercial Editions

- **CE**: Community Edition with core platform functionality.
- **EE**: CE plus high availability, upgrade, migration, and audit features.
- **UE**: EE plus AI-assisted operations and commercial features.

## Contact

- GitHub: submit issues or pull requests.
- Support email: `ice_out@sina.com`
- Enterprise consultation: `ice_out@sina.com`

[go-image]: https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go
[go-url]: https://go.dev/
[node-image]: https://img.shields.io/badge/Node.js-18+-339933?style=flat&logo=node.js
[node-url]: https://nodejs.org/
[license-image]: https://img.shields.io/badge/License-MIT-blue.svg
[license-url]: https://opensource.org/licenses/MIT
[lang-image]: https://img.shields.io/badge/Language-Go%20%7C%20TypeScript-blue
[lang-url]: https://github.com/mingjia1/dbops
[status-image]: https://img.shields.io/badge/Status-active-brightgreen.svg
[status-url]: https://github.com/mingjia1/dbops
[build-image]: https://img.shields.io/badge/Build-manual-lightgrey.svg
[build-url]: https://github.com/mingjia1/dbops
