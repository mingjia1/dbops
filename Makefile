.PHONY: all build run test test-xinchuang-lifecycle clean docker-up docker-down docker-logs install-backend install-agent install-web fmt lint db-migrate upgrade rollback pre-upgrade-check upgrade-apply smoke-test test-backup-restore

all: install-backend install-agent install-web

install-backend:
	cd backend && go mod download && go mod tidy

install-agent:
	cd agent && go mod download && go mod tidy

install-web:
	cd frontend && npm install

build: build-backend build-agent build-web

build-backend:
	make -C backend build

build-agent:
	make -C agent build

build-web:
	make -C frontend build

run: run-backend run-agent run-web

run-backend:
	cd backend && go run ./cmd/main.go

run-agent:
	cd agent && go run ./cmd/main.go

run-web:
	cd frontend && npm run dev

test: test-backend test-agent

test-backend:
	make -C backend test

test-agent:
	make -C agent test

test-xinchuang-lifecycle:
	cd agent && go test ./internal/executor -run '^(TestOceanBase|TestTiDB|TestDameng|TestKingbase|TestOpenGauss|TestGBase8a|TestGBase8s)' -count=1
	cd backend && go test ./internal/plugins/kernel -run '^TestXinchuangCoreBase' -count=1
	cd backend && go test ./internal/services -run 'Test.*(Capability|Refused|Rejects)' -count=1
	cd frontend && npm test -- --run src/services/flavorCapability.test.ts

docker-up:
	@if [ ! -f deploy/docker/docker-compose.yml ]; then echo "deploy/docker/docker-compose.yml not found"; exit 1; fi
	docker compose -f deploy/docker/docker-compose.yml up -d --build

docker-down:
	@if [ ! -f deploy/docker/docker-compose.yml ]; then echo "deploy/docker/docker-compose.yml not found"; exit 1; fi
	docker compose -f deploy/docker/docker-compose.yml down

docker-logs:
	@if [ ! -f deploy/docker/docker-compose.yml ]; then echo "deploy/docker/docker-compose.yml not found"; exit 1; fi
	docker compose -f deploy/docker/docker-compose.yml logs -f

clean:
	make -C backend clean
	make -C agent clean
	make -C frontend clean

fmt:
	cd backend && go fmt ./...
	cd agent && go fmt ./...

lint:
	cd backend && golangci-lint run
	cd agent && golangci-lint run

db-migrate:
	cd backend && go run ./cmd/main.go migrate

dist: build-backend build-agent build-web
	rm -rf dist && mkdir -p dist/bin dist/config dist/scripts
	cp backend/bin/platform dist/bin/dbops-backend
	cp agent/bin/agent dist/bin/dbops-agent
	cp -r backend/config/*.yaml dist/config/ 2>/dev/null || true
	cp -r bin dist/scripts/bin 2>/dev/null || true
	cp frontend/build dist/web -r 2>/dev/null || cp -r frontend/dist dist/web 2>/dev/null || true
	tar -czf dbops-offline-$(shell date +%Y%m%d).tar.gz dist/
	@echo "Offline package: dbops-offline-$(shell date +%Y%m%d).tar.gz"

upgrade: pre-upgrade-check build-backend build-agent upgrade-apply smoke-test

pre-upgrade-check:
	@echo "=== Pre-Upgrade Checks ==="
	@echo "Checking backend health..."
	@curl -sf http://localhost:8080/health/ready > /dev/null || (echo "ERROR: Backend not ready at http://localhost:8080"; exit 1)
	@echo "Backing up platform metadata..."
	@mkdir -p /backup
	@if [ -f data/dbops.db ]; then tar -czf /backup/dbops-platform-$$(date +%Y%m%d-%H%M%S).tar.gz data/dbops.db data/*.json 2>/dev/null || true; fi
	@if [ -f deploy/systemd/dbops-platform.service ]; then systemctl is-active --quiet dbops-platform && cp /opt/dbops-platform/bin/dbops-backend /opt/dbops-platform/bin/dbops-backend.bak || true; fi
	@echo "Pre-upgrade checks passed."

upgrade-apply:
	@echo "=== Applying Upgrade ==="
	@echo "Stopping services..."
	@if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet dbops-platform; then systemctl stop dbops-platform dbops-agent; else echo "Services not managed by systemd, skipping stop"; fi
	@echo "Installing new binaries..."
	@if [ -f backend/bin/platform ]; then cp backend/bin/platform /usr/local/bin/dbops-backend; else echo "backend/bin/platform not found"; exit 1; fi
	@if [ -f agent/bin/agent ]; then cp agent/bin/agent /usr/local/bin/dbops-agent; else echo "agent/bin/agent not found"; exit 1; fi
	@echo "Running DB migrations..."
	@if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet dbops-platform; then systemctl start dbops-platform && sleep 5 && systemctl restart dbops-platform; else cd backend && go run ./cmd/main.go migrate; fi
	@echo "Verifying upgrade..."
	@curl -sf http://localhost:8080/health/ready > /dev/null || (echo "ERROR: Backend health check failed after upgrade"; echo "Rollback: cp /opt/dbops-platform/bin/dbops-backend.bak /opt/dbops-platform/bin/dbops-backend && systemctl restart dbops-platform"; exit 1)
	@echo "Starting services..."
	@if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet dbops-agent; then systemctl start dbops-agent; fi
	@echo "=== Upgrade Complete ==="
	@echo "Rollback: make rollback"

smoke-test:
	@if [ ! -f scripts/smoke-test.sh ]; then echo "scripts/smoke-test.sh not found"; exit 1; fi
	@chmod +x scripts/smoke-test.sh
	@scripts/smoke-test.sh

rollback:
	@echo "=== Rolling Back ==="
	@echo "Stopping services..."
	@if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet dbops-platform; then systemctl stop dbops-platform dbops-agent; fi
	@echo "Restoring previous binaries..."
	@if [ -f /opt/dbops-platform/bin/dbops-backend.bak ]; then cp /opt/dbops-platform/bin/dbops-backend.bak /opt/dbops-platform/bin/dbops-backend; echo "Backend binary restored"; else echo "No backup found"; exit 1; fi
	@if [ -f /backup/dbops-platform-*.tar.gz ]; then LATEST=$$(ls -t /backup/dbops-platform-*.tar.gz 2>/dev/null | head -1); if [ -n "$$LATEST" ]; then echo "Restoring metadata from $$LATEST..."; tar -xzf $$LATEST -C /opt/dbops-platform/; fi; fi
	@echo "Starting services..."
	@if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet dbops-platform; then systemctl start dbops-platform dbops-agent; fi
	@echo "Verifying..."
	@curl -sf http://localhost:8080/health/ready > /dev/null && echo "Rollback successful" || (echo "ERROR: Rollback verification failed"; exit 1)

help:
	@echo "MySQL Ops Platform Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make install-backend    Install backend dependencies"
	@echo "  make install-agent      Install agent dependencies"
	@echo "  make install-web        Install web console dependencies"
	@echo "  make build              Build all components"
	@echo "  make dist               Build offline install package"
	@echo "  make upgrade            One-click platform upgrade"
	@echo "  make run                Run all components"
	@echo "  make test               Run tests"
	@echo "  make docker-up          Start Docker services"
	@echo "  make docker-down        Stop Docker services"
	@echo "  make clean              Clean build artifacts"
	@echo "  make fmt                Format code"
	@echo "  make lint               Run linters"
	@echo "  make smoke-test         Run post-upgrade smoke tests"
	@echo "  make test-backup-restore Run backup/restore smoke test"

test-backup-restore:
	@if [ ! -f scripts/backup-platform.sh ]; then echo "scripts/backup-platform.sh not found"; exit 1; fi
	@chmod +x scripts/backup-platform.sh
	@echo "=== Backup/Restore Smoke Test ==="
	@echo "Step 1: Running backup..."
	@scripts/backup-platform.sh
	@echo "Step 2: Verifying backup files..."
	@ls -lh /backup/dbops-platform-* 2>/dev/null || fail "No backup files found"
	@echo "Step 3: Simulating restore..."
	@RESTORE_TMP=$$(mktemp -d) && \
	 LATEST=$$(ls -t /backup/dbops-platform-*.tar.gz 2>/dev/null | head -1) && \
	 if [ -n "$$LATEST" ]; then tar -xzf "$$LATEST" -C "$$RESTORE_TMP" && echo "Restored to $$RESTORE_TMP"; fi
	@echo "=== Backup/Restore Smoke Test Passed ==="
