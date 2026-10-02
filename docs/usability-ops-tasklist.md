# DBOps Platform — Usability & Ops Improvements Implementation Task List

## Completed
- [x] A1: Sidebar menu alignment — `frontend/src/services/dashboardMenu.tsx` now exposes all 37 routes under 8 groups; `findSelectedKey` does exact match before prefix fallback; tests in `dashboardMenu.test.tsx` (9 tests pass)
- [x] A2: Capability matrix page — `frontend/src/pages/CapabilityMatrix.tsx` created, route `capability-matrix` added to `App.tsx`, menu entry exists under 主机与实例; tests in `CapabilityMatrix.test.tsx` (4 tests pass)
- [x] A3: Documentation fixes — corrected stale names/paths in `docs/README.md`, `docs/OPS_MANUAL.md`, `readme_ZH.md`, `readme_US.md`, `.env.example`
- [x] A4: Scan confidence — added `Source` enum (`tcp-handshake`/`process-cmdline`/`process-ini`/`default-port`) and `PortConfident` to `ScannedInstance`; updated `probePort`/`discoverByProcess`/`runScan`; updated `HostDetail.tsx` to show confidence column and new source tags; frontend type updated in `api.ts`
- [x] A5: Agent parameter naming — added `Normalize()` and `UnmarshalJSON` to `DeployTaskRequest` in `agent/internal/executor/task_executor.go`; normalizes `mysql_host`/`target_host` → `host`, `mysql_pass`/`target_pass` → `pass`, `mysql_port`/`target_port` → `port` during JSON unmarshal; agent builds and `go vet` clean
- [x] B1: Deployment scripts — created `deploy/systemd/dbops-platform.service`, `deploy/systemd/dbops-agent.service`, `deploy/docker/docker-compose.yml`, `deploy/docker/Dockerfile.backend`, `deploy/docker/Dockerfile.agent`, `deploy/docker/nginx.conf`; updated `Makefile` docker-up/down/logs and `upgrade` targets; updated `bin/ubuntu/start-backend.sh` to prefer pre-built binaries
- [x] B2: Agent build skip — updated `backend/cmd/main.go` to check valid binary first (ELF/PE), only build when missing and `DBOPS_SKIP_AGENT_BINARY_BUILD` is not true; backend builds clean
- [x] B3: Agent version negotiation — added `MinAgentVersion`/`MaxAgentVersion` to backend `VersionEntry` (`backend/internal/services/version_catalog.go`); exported to frontend `VersionEntry` interface (`frontend/src/services/api.ts`); `ExecuteUpgradeModal` shows agent compat warning when selecting target version; backend `dispatchUpgrade` validates agent version via `validateAgentVersionForUpgrade` before executing upgrade
- [x] B4: Data dir normalization — `writeBootstrapAdminCredential` fallback changed from `./data` to `../db` to match config default; `DBOPS_DATA_DIR` already bound via `bindBackendEnv`
- [x] B5: Windows host onboarding — added section 3.3.1 in `docs/OPS_MANUAL.md` covering Windows agent deployment (NSSM/sc.exe), WinRM prerequisites, platform host addition steps, and known limitations (no `ps` discovery, TCP-only scanning)

## Pending
- [ ] Backend validation for B3 could be extended with structured error codes/messages for frontend localization

## Verification
- Frontend: `cd frontend && npm test -- --run` (307 tests pass) and `npm run build` (TypeScript + Vite build succeeds)
- Backend: `cd backend && go build ./...`, `go vet ./...`, and `go test ./...` (all packages pass)
- Agent: `cd agent && go build ./...`, `go vet ./...`, `go test ./cmd` (passes; executor flavor tests have pre-existing failures unrelated to A5)
- Docs: review `docs/README.md` and `docs/OPS_MANUAL.md` for stale references
