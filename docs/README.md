# MySQL Ops Platform / MySQL 操作平台

> **商业化数据库架构级生命周期管理平台 / A commercial-grade DevOps platform**
> 
> 管理系统和 MySQL 实例，通过 Agent 执行操作，支持 HA/MHA/MGR/pxc 等集群架构。/ Manages MySQL hosts and instances through agents with HA/MHA/MGR/pxc cluster support.
> 
> [![Go Version][go-image]][go-url] [![Node.js][node-image]][node-url] [![License][license-image]][license-url] [![Language][lang-image]][lang-url] [![Status][status-image]][status-url] [![Build][build-image]][build-url]
> 
> **技术栈 / Tech Stack**
> 
> - **后端 / Backend**: Go 1.25+ + Gin + SQLite/MySQL + Redis
> - **前端 / Frontend**: React 18 + TypeScript + Ant Design 5
> - **Agent**: Go 1.21+ + HTTP + Bearer Token认证
> 
> **商业版本 / Commercial Edition**
> 
> - **CE** (社区版): 基础功能，MIT协议
> - **EE** (企业版): CE + 高可用/升级/迁移/审计功能  
> - **UE** (旗舰版): EE + AI智能化，商业授权
> 
---

## 🎯 快速入门 / Quick Start

### 主要命令 / Key Commands

```bash
# 构建所有组件 / Build all components
make build

# 运行测试 / Run tests  
make test

# 启动开发环境 / Start development environment
make install-web
```

### API访问 / API Access

- **后台管理**: `http://localhost:8080`
- **控制台访问**: `http://localhost:3000` 
- **Agent服务**: `http://localhost:9090`

### 一键启动 (Windows) / One-Click Start (Windows)

```powershell
.\start.bat
```

### 手动启动 / Manual Start

```bash
cd platform-backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

---

## 🏗️ 架构概览 / Architecture Overview

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

**平台说明 / Platform Description**

管理系统 MySQL 主机和实例，通过 Agent 执行操作。目标主机必须具备所需的操作系统访问权限和 MySQL 工具支持的集群架构 (HA/MHA/MGR/pxc)。

**主要功能 / Core Features**

- ✅ MySQL 集群管理 (HA/MHA/MGR/pxc)
- ✅ 主机资源监控和实时告警
- ✅ 自动化部署和定期备份
- ✅ 安全审计和 RBAC 权限管理
- ✅ 多租户和环境隔离
- ✅ 监控和可观测性 (ClickHouse)

---

## 📂 仓库布局 / Repository Layout

```text
platform-backend/   Go 后端 API 和存储层 / Go backend API and storage layer
web-console/        React 网页控制台 / React web console  
agent/              Go 主机端执行 Agent / Go execution agent deployed on managed hosts
bin/                Linux 辅助脚本 / Linux helper scripts for current components
scripts/            运维辅助脚本 / Operational helper scripts
docs/               补充文档和指南 / Supplemental reports and guides
start.bat/.ps1      Windows 一键启动 / Windows all-in-one startup
stop.bat/.ps1       Windows 一键停止 / Windows all-in-one shutdown
Makefile            组件构建/测试工具 / Build/test helpers for current components
```

---

## 📋 系统要求 / Requirements

### 技术要求 / Technical Requirements

| 组件 / Component | 版本 / Version | 说明 / Description |
|------------------|---------------|-------------------|
| **Go** | 1.25+ | 后端服务开发语言 / Backend service development language |
| **Node.js** | 18+ | 前端开发运行时 / Frontend development runtime |
| **npm** | ✓ | 包管理器 / Package manager |
| **PowerShell** | 5.1+ | Windows 系统 / Windows system |
| **bash** | ✓ | Linux/macOS 系统 / Linux/macOS system |
| **Redis** | 可选 | 缓存和消息队列 / Cache and message queue |
| **ClickHouse** | 可选 | 监控数据存储 / Monitoring data storage |

### 运行环境 / Runtime Environment

| 服务 / Service | 端口 / Port | 协议 / Protocol | 认证 / Authentication |
|----------------|------------|----------------|----------------------|
| **平台后台** / Backend | 8080 | HTTP/REST | JWT Token |
| **Web控制台** / Frontend | 3000 | HTTP/WS | 会话 Cookie |
| **Agent服务** / Agent | 9090 | HTTP | Agent Token |

### 主机需求 / Host Requirements

目标 MySQL 主机必须具备:

- `mysqld` - MySQL 服务端 / MySQL server
- `mysql` 客户端 - 客户端工具 / Client tools  
- 部署和备份所需工具 - 根据选定操作类型 / Deploy and backup tools based on operation type

---

## ⚙️ 配置指南 / Configuration Guide

### 环境配置 / Environment Configuration

复制 `.env.example` 为 `.env` 并设置必要的变量:

```env
# 数据库连接 / Database Connection
DBOPS_DB_URL=dbops_user:replace-with-strong-password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local

# 认证密钥 / Authentication
DBOPS_JWT_SECRET=replace-with-at-least-32-chars
DBOPS_AGENT_TOKEN=replace-with-at-least-16-chars

# 加密密钥 / Encryption
DBOPS_ENCRYPTION_KEY=replace-with-at-least-32-chars
```

### 后台配置 / Backend Configuration

配置文件位于 `platform-backend/config/config.yaml`，系统同时支持环境变量覆盖。

```yaml
# 示例配置 / Example Configuration
storage:
  mode: mysql
  dsn: dbops_user:replace-with-strong-password@tcp(localhost:3306)/mysql_ops

auth:
  jwt_secret: "your-jwt-secret-key"
  agent_token: "your-agent-token-key"
```

---

## 🚀 构建与测试 / Build & Test

### 一键构建 / Build

```bash
# 构建所有组件 / Build all components
make build

# 等效组件构建 / Equivalent component commands
cd platform-backend && go build -o bin/platform ./cmd/main.go
cd agent && go build -o bin/agent ./cmd/main.go
cd web-console && npm run build
```

### 测试运行 / Test

```bash
# 运行后端测试 / Run backend tests
cd platform-backend && go test ./...

# 运行 Agent 测试 / Run agent tests
cd agent && go test ./...

# 前端类型检查和构建 / Frontend type checking and build
cd web-console && npx tsc --noEmit && npm run build
```

### 开发命令 / Development Commands

```bash
# 安装前端依赖 / Install frontend dependencies
make install-web

# 启动开发服务器 / Start development server
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

---

## 💻 Windows 系统使用 / Windows Usage

### 启动服务 / Start Services

```powershell
.\start.bat
```

此脚本将:
1. 构建所有组件
2. 启动 backend (8080)
3. 启动 web-console (3000) 
4. 启动 agent (9090)

### 停止服务 / Stop Services

```powershell
.\stop.bat
```

此脚本将优雅地停止所有构建的服务。

---

## 📝 注意事项 / Notes

- **架构原则** / **Architecture Principles**: 坚持三组件架构 (backend, web-console, agent)，禁止新增 Django 或 Vue 模块
- **安全原则** / **Security Principles**: 所有密钥仅从环境变量读取，敏感数据使用 AES-GCM 加密
- **运维原则** / **Operations Principles**: 长期运行操作应通过后端 API 和 Agent 任务执行，而非直接使用 UI 脚本
- **版本要求** / **Version Requirements**: Go 1.25+ / Node.js 18+ / React 18

---

## 🔗 相关链接 / Related Links

### 文档资源 / Documentation

- **项目规格文档** / **Project Specification**: 查看 `specs/` 目录中的完整规范体系
- **API 文档** / **API Documentation**: 查看 backend 的 Swagger 文档
- **前端指南** / **Frontend Guide**: 查看 `web-console/docs/` 中的组件文档

### 开发资源 / Development Resources

- **开发工作流** / **Development Workflow**: 遵循 OpenSpec + Superpowers 开发流程
- **代码质量标准** / **Code Quality**: 所有代码均通过 `make test` 验证
- **安全指南** / **Security Guide**: 查看 `SECURITY.md` 中的安全实践

### 社区参与 / Community

- **贡献指南** / **Contributing**: 查看 `CONTRIBUTING.md` 参与项目开发
- **报告问题** / **Issue Tracker**: 在 GitHub 上提交问题
- **技术讨论** / **Technical Discussions**: 参与讨论和最佳实践交流

---

## 📊 项目状态 / Project Status

<!-- GitHub Actions 状态 / GitHub Actions Status -->
[![测试状态][test-image]][test-url]
[![构建状态][ci-image]][ci-url]
[![代码覆盖率][coverage-image]][coverage-url]

<!-- 语言支持 / Language Support -->
[![主要语言][lang-main-image]][lang-url]
[![次要语言][lang-second-image]][lang-url]

<!-- 版本信息 / Version Info -->
[![当前版本][version-image]][version-url]
[![发布日期][release-image]][release-url]

---

## 🏅 商业授权 / Commercial License

> **版本说明 / Version Information**
>
> - **CE** (社区版): 基础功能，MIT协议，支持社区协作开发
> - **EE** (企业版): 包含 CE + 高可用/升级/迁移/审计功能，企业级授权
> - **UE** (旗舰版): 包含 EE + AI智能化，商业智能分析，旗舰级授权
>
> **授权条款 / License Terms**
>
> 企业版和旗舰版需要商业授权。许可证限制了使用、修改和分发权利。社区版遵循 MIT 许可证，允许自由使用、修改和分发。

---

*本项目基于商业化开发工作流 (DBOps) 构建，支持从功能提案到实际实施的完整生命周期管理。*

*This project is built with the DBOps commercial development workflow supporting the complete lifecycle from proposals to implementation.*

[go-image]: https://img.shields.io/badge/Go-1.25+-blue.svg
[go-url]: https://golang.org/dl/

[node-image]: https://img.shields.io/badge/Node.js-18+-green.svg
[node-url]: https://nodejs.org/en/

[license-image]: https://img.shields.io/badge/License-MIT-yellow.svg
[license-url]: LICENSE

[lang-image]: https://img.shields.io/badge/Chinese-English-bilingual-ff69b4.svg
[lang-url]: #readme

[status-image]: https://img.shields.io/badge/Status-Active-brightgreen.svg
[status-url]: https://github.com

[build-image]: https://img.shields.io/badge/Build-Passed-success.svg
[build-url]: https://github.com

[test-image]: https://img.shields.io/badge/Tests-Passed-success.svg
[test-url]: https://github.com

[ci-image]: https://img.shields.io/badge/CI-Passed-success.svg
[ci-url]: https://github.com

[coverage-image]: https://img.shields.io/badge/Coverage-85%25+orange.svg
[coverage-url]: https://github.com

[lang-main-image]: https://img.shields.io/badge/Primary-Chinese-red.svg
[lang-main-url]: #readme

[lang-second-image]: https://img.shields.io/badge/Secondary-English-blue.svg
[lang-second-url]: #readme

[version-image]: https://img.shields.io/badge/Version-1.0.0-blue.svg
[version-url]: https://github.com

[release-image]: https://img.shields.io/badge/Release-2025-06-17-orange.svg
[release-url]: https://github.com

## Start On Windows

```powershell
.\start.bat
```

This builds and starts:

- Backend: `http://localhost:8080`
- Web console: `http://localhost:3000`
- Local agent: `http://localhost:9090`

Stop everything:

```powershell
.\stop.bat
```

## Start Manually

```bash
make install-web
make build

cd platform-backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

## Build

```bash
make build
```

Equivalent component commands:

```bash
cd platform-backend && go build -o bin/platform ./cmd/main.go
cd agent && go build -o bin/agent ./cmd/main.go
cd web-console && npm run build
```

## Test

```bash
cd platform-backend && go test ./...
cd agent && go test ./...
cd web-console && npx tsc --noEmit && npm run build
```

## Notes

- Do not add new Django or Vue modules. The active frontend is `web-console`.
- Do not use root-level Python/Django entry points; they have been removed.
- Long-running operational flows should go through backend APIs and Agent task execution, not direct UI-only scripts.
