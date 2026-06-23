.PHONY: all build run test clean docker-up docker-down docker-logs install-backend install-agent install-web fmt lint db-migrate

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

docker-up:
	docker-compose -f docker-compose.dev.yml up -d

docker-down:
	docker-compose -f docker-compose.dev.yml down

docker-logs:
	docker-compose -f docker-compose.dev.yml logs -f

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
	cp backend/build/dbops-backend dist/bin/
	cp agent/build/dbops-agent dist/bin/
	cp -r backend/config/*.yaml dist/config/ 2>/dev/null || true
	cp scripts/upgrade-platform.sh scripts/start.sh scripts/stop.sh dist/scripts/
	cp docker-compose.dev.yml dist/
	cp frontend/build dist/web -r 2>/dev/null || cp -r frontend/dist dist/web 2>/dev/null || true
	tar -czf dbops-offline-$(shell date +%Y%m%d).tar.gz dist/
	@echo "Offline package: dbops-offline-$(shell date +%Y%m%d).tar.gz"

upgrade: build-backend build-agent
	@echo "=== DBOps Platform Upgrade ==="
	@echo "Stopping services..."
	scripts/stop.sh || true
	@echo "Installing new binaries..."
	cp backend/build/dbops-backend /usr/local/bin/dbops-backend 2>/dev/null || true
	cp agent/build/dbops-agent /usr/local/bin/dbops-agent 2>/dev/null || true
	@echo "Running DB migrations..."
	cd backend && go run ./cmd/main.go migrate 2>/dev/null || true
	@echo "Starting services..."
	scripts/start.sh || true
	@echo "=== Upgrade complete ==="

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
