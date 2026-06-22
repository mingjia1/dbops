# API Overview

The active API is served by `platform-backend` under `/api/v1`.

## Health

```http
GET /health
GET /health/live
GET /health/ready
```

## Auth

```http
POST /api/v1/auth/login
POST /api/v1/auth/register
POST /api/v1/auth/logout
POST /api/v1/auth/change-password
```

## Main Resources

```http
GET    /api/v1/hosts
POST   /api/v1/hosts
GET    /api/v1/hosts/:id
PUT    /api/v1/hosts/:id
DELETE /api/v1/hosts/:id

GET    /api/v1/instances
POST   /api/v1/instances
GET    /api/v1/instances/:id
PUT    /api/v1/instances/:id
DELETE /api/v1/instances/:id

GET    /api/v1/tasks
GET    /api/v1/tasks/:id
```

## Operational Areas

- `/api/v1/deployments`
- `/api/v1/backups`
- `/api/v1/monitoring`
- `/api/v1/parameter-templates`
- `/api/v1/switch`
- `/api/v1/topology`
- `/api/v1/ha`
- `/api/v1/upgrades`
- `/api/v1/migrations`
- `/api/v1/alerts`
- `/api/v1/approvals`
- `/api/v1/audit-logs`
- `/api/v1/data-migration`

Use `web-console/src/services/api.ts` as the frontend contract source for currently wired endpoints.
