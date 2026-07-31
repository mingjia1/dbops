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

## Adding a Flavor Executor

1. Update the baseline in `backend/internal/services/flavor_release_catalog.go` using the vendor's current supported release information.
2. Prepare a local package bundle and manifest under `/opt/dbops/packages/<flavor>/<version>/`.
3. Validate the bundle before any installer command.
4. Add the dedicated agent handler and backend plugin contract.
5. Enable only the capabilities proven by tests in `flavor_capability.go`.
