.PHONY: all build run test clean docker-up docker-down install-backend install-agent install-web

all: install-backend install-agent install-web

install-backend:
	cd platform-backend && go mod download && go mod tidy

install-agent:
	cd agent && go mod download && go mod tidy

install-web:
	cd web-console && npm install

build: build-backend build-agent build-web

build-backend:
	cd platform-backend && go build -o bin/platform ./cmd

build-agent:
	cd agent && go build -o bin/agent ./cmd

build-web:
	cd web-console && npm run build

run: run-backend run-agent run-web

run-backend:
	cd platform-backend && go run ./cmd

run-agent:
	cd agent && go run ./cmd

run-web:
	cd web-console && npm run dev

test: test-backend test-agent

test-backend:
	cd platform-backend && go test ./... -v

test-agent:
	cd agent && go test ./... -v

docker-up:
	docker-compose -f docker-compose.dev.yml up -d

docker-down:
	docker-compose -f docker-compose.dev.yml down

docker-logs:
	docker-compose -f docker-compose.dev.yml logs -f

clean:
	rm -rf platform-backend/bin
	rm -rf agent/bin
	rm -rf web-console/dist
	rm -rf web-console/node_modules

fmt:
	cd platform-backend && go fmt ./...
	cd agent && go fmt ./...

lint:
	cd platform-backend && golangci-lint run
	cd agent && golangci-lint run

db-migrate:
	cd platform-backend && go run ./cmd migrate

help:
	@echo "MySQL Ops Platform Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make install-backend    Install backend dependencies"
	@echo "  make install-agent      Install agent dependencies"
	@echo "  make install-web        Install web console dependencies"
	@echo "  make build              Build all components"
	@echo "  make run                Run all components"
	@echo "  make test               Run tests"
	@echo "  make docker-up          Start Docker services"
	@echo "  make docker-down        Stop Docker services"
	@echo "  make clean              Clean build artifacts"
	@echo "  make fmt                Format code"
	@echo "  make lint               Run linters"