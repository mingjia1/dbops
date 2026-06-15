# Quickstart

This guide starts the current supported stack: `platform-backend + web-console + agent`.

## 1. Prepare Environment

Install:

- Go 1.21+
- Node.js 18+
- npm

Create `.env` from `.env.example` and set the required `DBOPS_*` values.

## 2. Install Dependencies

```bash
cd web-console
npm install
cd ..
```

Go dependencies are downloaded automatically by `go build` or `go test`.

## 3. Start Everything On Windows

```powershell
.\start.bat
```

Open:

- Web console: `http://localhost:3000`
- Backend health: `http://localhost:8080/health`
- Agent health: `http://localhost:9090/health`

Stop:

```powershell
.\stop.bat
```

## 4. Start Components Manually

Backend:

```bash
cd platform-backend
go run ./cmd/main.go
```

Agent:

```bash
cd agent
go run ./cmd/main.go
```

Web console:

```bash
cd web-console
npm run dev -- --host 0.0.0.0 --port 3000
```

## 5. First Login

On first startup with an empty user table, the backend seeds an `admin` user and prints the generated password in the backend log. Change it after login.

## 6. Basic Flow

1. Add a host in Web Console.
2. Install or verify the Agent for that host.
3. Register or scan MySQL instances.
4. Run health checks, backups, deployments, migrations, upgrades, or HA operations from the corresponding pages.

## 7. Verification

```bash
curl http://localhost:8080/health
curl http://localhost:9090/health
```

For frontend validation:

```bash
cd web-console
npx tsc --noEmit
npm run build
```
