# MySQL Ops Platform

> **A commercial-grade DevOps platform for database architecture-level lifecycle management**
> 
> Manages MySQL hosts and instances through agents, supporting HA/MHA/MGR/PXC cluster architectures.
> 
> [![Go Version][go-image]][go-url] [![Node.js][node-image]][node-url] [![License][license-image]][license-url] [![Language][lang-image]][lang-url] [![Status][status-image]][status-url] [![Build][build-image]][build-url]
> 
> **Tech Stack**
> 
> - **Backend**: Go 1.25+ + Gin + SQLite/MySQL + Redis
> - **Frontend**: React 18 + TypeScript + Ant Design 5
> - **Agent**: Go 1.21+ + HTTP + Bearer Token authentication
> 
> **Commercial Editions**
> 
> - **CE** (Community Edition): Core features, MIT license
> - **EE** (Enterprise Edition): CE + HA/Upgrade/Migration/Audit features
> - **UE** (Ultimate Edition): EE + AI-powered intelligence, commercial license

---

## Quick Start

### Key Commands

```bash
# Build all components
make build

# Run tests
make test

# Install frontend dependencies
make install-web
```

### API Access

- **Backend Admin**: `http://localhost:8080`
- **Web Console**: `http://localhost:3000`
- **Agent Service**: `http://localhost:9090`

### One-Click Start (Windows)

```powershell
.\start.bat
```

### Manual Start

```bash
cd platform-backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

---

## Architecture Overview

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

**Platform Description**

Manages MySQL hosts and instances, executing operations through agents. Target hosts must have the required OS access and MySQL tools for the selected cluster architecture (HA/MHA/MGR/PXC).

**Core Features**

- MySQL cluster management (HA/MHA/MGR/PXC)
- Host resource monitoring and real-time alerting
- Automated deployment and scheduled backups
- Security auditing and RBAC access control
- Multi-tenancy and environment isolation
- Monitoring and observability (ClickHouse)

---

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

---

## Requirements

### Technical Requirements

| Component | Version | Description |
|-----------|---------|-------------|
| **Go** | 1.25+ | Backend service development language |
| **Node.js** | 18+ | Frontend development runtime |
| **npm** | Yes | Package manager |
| **PowerShell** | 5.1+ | Windows system |
| **bash** | Yes | Linux/macOS system |
| **Redis** | Optional | Cache and message queue |
| **ClickHouse** | Optional | Monitoring data storage |

### Runtime Environment

| Service | Port | Protocol | Authentication |
|---------|------|----------|----------------|
| **Backend** | 8080 | HTTP/REST | JWT Token |
| **Frontend** | 3000 | HTTP/WS | Session Cookie |
| **Agent** | 9090 | HTTP | Agent Token |

### Host Requirements

Target MySQL hosts must have:

- `mysqld` - MySQL server
- `mysql` client - Client tools
- Deploy and backup tools based on operation type

---

## Configuration Guide

### Environment Configuration

Copy `.env.example` to `.env` and set the required variables:

```env
# Database Connection
DBOPS_DB_URL=root:password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local

# Authentication
DBOPS_JWT_SECRET=replace-with-at-least-32-chars
DBOPS_AGENT_TOKEN=replace-with-at-least-16-chars

# Encryption
DBOPS_ENCRYPTION_KEY=replace-with-at-least-32-chars
```

### Backend Configuration

Configuration file is located at `platform-backend/config/config.yaml`. The system also supports environment variable overrides.

```yaml
# Example Configuration
storage:
  mode: mysql
  dsn: root:password@tcp(localhost:3306)/mysql_ops

auth:
  jwt_secret: "your-jwt-secret-key"
  agent_token: "your-agent-token-key"
```

---

## Build & Test

### Build

```bash
# Build all components
make build

# Equivalent component commands
cd platform-backend && go build -o bin/platform ./cmd/main.go
cd agent && go build -o bin/agent ./cmd/main.go
cd web-console && npm run build
```

### Test

```bash
# Run backend tests
cd platform-backend && go test ./...

# Run agent tests
cd agent && go test ./...

# Frontend type checking and build
cd web-console && npx tsc --noEmit && npm run build
```

### Development Commands

```bash
# Install frontend dependencies
make install-web

# Start development server
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

---

## Windows Usage

### Start Services

```powershell
.\start.bat
```

This script will:
1. Build all components
2. Start backend (8080)
3. Start web-console (3000)
4. Start agent (9090)

### Stop Services

```powershell
.\stop.bat
```

This script will gracefully stop all running services.

---

## Notes

- **Architecture Principles**: Maintain the three-component architecture (backend, web-console, agent). No Django or Vue modules allowed.
- **Security Principles**: All secrets are read from environment variables only. Sensitive data is encrypted with AES-GCM.
- **Operations Principles**: Long-running operations should be executed via backend API and agent tasks, not directly through UI scripts.
- **Version Requirements**: Go 1.25+ / Node.js 18+ / React 18

---

## Related Links

### Documentation

- **Project Specification**: See the full specification system in the `specs/` directory
- **API Documentation**: See the backend Swagger documentation
- **Frontend Guide**: See component documentation in `web-console/docs/`

### Development Resources

- **Development Workflow**: Follow the OpenSpec + Superpowers development process
- **Code Quality**: All code is verified through `make test`
- **Security Guide**: See security practices in `SECURITY.md`

### Community

- **Contributing**: See `CONTRIBUTING.md` to participate in project development
- **Issue Tracker**: Submit issues on GitHub
- **Technical Discussions**: Join discussions and share best practices

---

## Project Status

[![Test Status][test-image]][test-url]
[![Build Status][ci-image]][ci-url]
[![Code Coverage][coverage-image]][coverage-url]

---

## Community Participation

### Issue Submission

We welcome issue submissions to this project! We encourage community developers to discover and report problems.

**How to Submit an Issue:**

1. **Check Existing Issues** - Search the issue list to see if your problem has been reported or resolved
2. **Use Issue Template** - Use the standard issue template to ensure sufficient information is provided
3. **Fill in Required Information** - Include problem description, reproduction steps, expected results, and actual results
4. **Attach Detailed Information** - Such as screenshots, log files, and system environment information
5. **Select Appropriate Labels** - Choose appropriate category labels based on the issue type

**Issue Template:**

```yaml
title: [Issue Type] Concise problem description

## Problem Description

## Reproduction Steps

## Expected Results

## Actual Results

## System Environment Information

## Additional Files
```

**Issue Resources:**

- **Issue Tracking System** - Managed using GitHub Issues
- **Community Discussions** - Join the technical discussion channel
- **Contributing Guide** - See `CONTRIBUTING.md` for contribution guidelines

### Enterprise Consultation

If you are an enterprise user considering custom development or commercial solutions, our professional team will provide expert technical consulting services.

**Enterprise Consulting Services:**

- **Technical Consulting** - System architecture design and optimization solutions
- **Custom Development** - Exclusive feature modules based on business requirements
- **Upgrade & Migration** - Smooth transition from existing solutions
- **Security Audit** - Assessment and enhancement of system security
- **Training & Support** - Technical staff training and documentation

**Contact Information:**

- **Email** - enterprise@dbops.io
- **Phone** - +86-400-123-4567
- **Online** - Submit enterprise tickets through GitHub

**Enterprise Service Process:**

1. **Requirement Consultation** - Initial technical requirements communication and solution evaluation
2. **Solution Design** - Custom development plan and technical architecture design
3. **Project Kickoff** - Formal launch of custom development project
4. **Project Delivery** - Iterative development and quality assurance
5. **Acceptance** - Project acceptance and usage training
6. **Ongoing Service** - Maintenance support and technical upgrades

**Commercial Edition Advantages:**

- **EE Edition** - Enterprise Edition includes all community features + HA/Upgrade/Migration/Audit
- **UE Edition** - Ultimate Edition includes EE features + AI-powered intelligence
- **Dedicated Technical Support** - 7x24 technical support services
- **Security Assurance** - Enterprise-grade security solutions

### Contact Us

If you have any questions or needs, feel free to contact us:

- **GitHub** - Submit Issues or Pull Requests
- **Email** - support@dbops.io
- **Website** - https://dbops.io
- **Social Media** - Follow our official accounts for the latest updates
