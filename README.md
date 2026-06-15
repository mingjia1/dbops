# MySQL Ops Platform

Current supported system:

- `platform-backend`: Go + Gin backend API.
- `web-console`: React + TypeScript web console.
- `agent`: Go host-side execution agent.

Legacy Django modules and the old Vue frontend have been removed. New backend or frontend work should target only the three components above.

## Architecture

```text
web-console (:3000)
        |
        | REST API /api/v1
        v
platform-backend (:8080)  ---- HTTP + Bearer token ---->  agent (:9090)
        |
        | metadata storage
        v
SQLite or MySQL, depending on storage_mode
```

The platform manages MySQL hosts and instances through the Agent. Target hosts must provide the required operating system access and MySQL tooling for the requested operations.

## Repository Layout

```text
platform-backend/   Go backend API and storage layer
web-console/        React web console
agent/              Go execution agent deployed on managed hosts
bin/                Linux helper scripts for current components
scripts/            Operational helper scripts
docs/               Supplemental reports and guides
start.bat/.ps1      Windows all-in-one startup
stop.bat/.ps1       Windows all-in-one shutdown
Makefile            Build/test helpers for current components
```

## Requirements

- Go 1.21+
- Node.js 18+
- npm
- PowerShell 5.1+ on Windows, or bash on Linux
- Optional: Redis and ClickHouse for cache/monitoring paths
- Target MySQL hosts: `mysqld`, `mysql` client, and backup/deploy tools required by the selected operation

## Configuration

Copy `.env.example` to `.env` and set at least:

```env
DBOPS_DB_URL=root:password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local
DBOPS_JWT_SECRET=replace-with-at-least-32-chars
DBOPS_ENCRYPTION_KEY=replace-with-at-least-32-chars
DBOPS_AGENT_TOKEN=replace-with-at-least-16-chars
```

Backend config is read from `platform-backend/config/config.yaml` plus environment variables.

## Start On Windows

```powershell
.\start.bat
```

This builds and starts:

- Backend: `http://localhost:8080`
- Web console: `http://localhost:3000`
- Local agent: `http://localhost:9090`

Stop everything:

```powershell
.\stop.bat
```

## Start Manually

```bash
make install-web
make build

cd platform-backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

## Build

```bash
make build
```

Equivalent component commands:

```bash
cd platform-backend && go build -o bin/platform ./cmd/main.go
cd agent && go build -o bin/agent ./cmd/main.go
cd web-console && npm run build
```

## Test

```bash
cd platform-backend && go test ./...
cd agent && go test ./...
cd web-console && npx tsc --noEmit && npm run build
```

## Notes

- Do not add new Django or Vue modules. The active frontend is `web-console`.
- Do not use root-level Python/Django entry points; they have been removed.
- Long-running operational flows should go through backend APIs and Agent task execution, not direct UI-only scripts.
