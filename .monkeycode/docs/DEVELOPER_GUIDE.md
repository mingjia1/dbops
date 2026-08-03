# Developer Guide

## Prerequisites

- Go 1.25 for the backend module.
- Go 1.21 for the agent module.
- Node.js 18 or later for the frontend.

## Common Commands

```bash
# Build all components
make build

# Run backend and agent tests
make test

# Format Go source
make fmt

# Run backend service
make run-backend

# Run agent service
make run-agent

# Run web console
make run-web
```

## Local Package Bundle Tests

```bash
# Run the local package bundle unit tests
cd agent
go test ./internal/executor -run '^TestValidateLocalPackageBundle' -count=1

# Run static checks for the executor package
go vet ./internal/executor
```

## Xinchuang Core Contract Tests

```bash
# Run lifecycle contract tests
cd backend
go test ./internal/plugins/kernel -run '^TestXinchuangCoreBase' -count=1

# Run static checks for the kernel package
go vet ./internal/plugins/kernel
```

## Flavor Task Executor Tests

```bash
# Run flavor registration, local bundle, and version detection tests
cd agent
go test ./internal/executor -run '^TestFlavorTaskExecutor' -count=1

# Run the flavor task route tests
go test ./cmd -run '^TestFlavorTaskRoute' -count=1

# Run local environment fixture scenarios
go test ./internal/executor -run '^TestFlavorTaskExecutorFixture' -count=1

# Run OceanBase deployment, backup, restore, migration, upgrade, monitoring, and teardown tests
go test ./internal/executor -run '^TestOceanBase' -count=1

# Run TiDB deployment, BR/PITR, migration, TLS, monitoring, teardown, and configuration tests
go test ./internal/executor -run '^TestTiDB' -count=1

# Run Dameng DM9 deployment, TLS, backup, restore, migration, monitoring, and teardown tests
go test ./internal/executor -run '^TestDameng' -count=1
```

## Flavor Capability Tests

```bash
# Verify the per-flavor single-node capability matrix
cd backend
go test ./internal/services -run 'Test.*Capability' -count=1

# Run static checks for capability services
go vet ./internal/services
```

## Lifecycle Contract Fixtures

```bash
# Verify Agent success and failure result handling, including rollback dispatch
cd backend
go test ./internal/plugins/kernel -run '^TestXinchuangCoreBase' -count=1
```

## Adding a Flavor Executor

1. Update the baseline in `backend/internal/services/flavor_release_catalog.go` using the vendor's current supported release information.
2. Prepare a local package bundle and manifest under `/opt/dbops/packages/<flavor>/<version>/`.
3. Validate the bundle before any installer command.
4. Add the dedicated agent handler and backend plugin contract.
5. Enable only the capabilities proven by tests in `flavor_capability.go`.
