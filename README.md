# MySQL Ops Platform

Database operations platform for host management, instance management, backup and restore, monitoring, cluster deployment, upgrade workflows, and related automation.

## Components

- `backend`: Go API server
- `frontend`: React + TypeScript web console
- `agent`: Go agent for host-side execution

## Quick Start

```bash
make build
make test
make install-web
```

## Default Local URLs

- Backend API: `http://localhost:8080`
- Frontend: `http://localhost:3000`
- Agent: `http://localhost:9090`

## Documentation

- Chinese: [readme_ZH.md](readme_ZH.md)
- English: [readme_US.md](readme_US.md)

## Screenshots

Screenshots are available in `docs/screenshots/`.
