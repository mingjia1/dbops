# MySQL Ops Platform / MySQL 操作平台 

> **商业化数据库架构级生命周期管理平台
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
DBOPS_DB_URL=root:password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local

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
  dsn: root:password@tcp(localhost:3306)/mysql_ops

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

<!-- 语言支持 -->

## 🤝 社区参与 / Community Participation

### Issue 提交 / Issue Submission

欢迎向本项目提交Issue！我们鼓励社区开发者发现并报告问题。

**如何提交Issue：**

1. **检查现有Issue** - 先在issue列表中搜索您的问题是否已被报告或解决
2. **复制Issue模板** - 使用标准Issue模板，确保提供足够的信息
3. **填写必要信息** - 包括问题描述、再现步骤、预期结果和实际结果
4. **附加详细信息** - 如截图、日志文件和系统环境信息
5. **选择合适的标签** - 根据问题类型选择合适的分类标签

**Issue模板：**

```yaml
title: [问题类型] 简洁的问题描述

## 问题描述

## 重现步骤

## 预期结果

## 实际结果

## 系统环境信息

## 附加文件
```

**Issue相关资源：**

- **Issue跟踪系统** - 使用GitHub Issues进行管理
- **社区讨论** - 加入技术讨论频道参与交流
- **贡献指南** - 查看`CONTRIBUTING.md`了解贡献规范

### 🏢 企业咨询 / Enterprise Consultation

如果您是企业用户，正在考虑定制开发或商业化解决方案，我们的专业团队将为您提供专业的技术咨询服务。

**企业咨询服务：**

- **技术咨询** - 系统架构设计和优化方案
- **定制开发** - 根据业务需求开发专属功能模块
- **升级迁移** - 从现有的解决方案平稳过渡
- **安全审计** - 评估和增强系统安全性
- **培训支持** - 技术人员培训和文档制作

**联系方式：**

- **邮箱咨询** - enterprise@dbops.io
- **电话咨询** - +86-400-123-4567
- **在线咨询** - 通过GitHub提交企业工单

**企业服务流程：**

1. **需求咨询** - 初步技术需求沟通和方案评估
2. **方案设计** - 定制开发方案和技术架构设计
3. **项目启动** - 正式启动定制开发项目
4. **项目交付** - 迭代开发和质量保证
5. **交付验收** - 项目验收和使用培训
6. **后续服务** - 维护支持和技术升级

**商业版本优势：**

- **EE版** - 企业版包含所有社区功能 + 高可用/升级/迁移/审计功能
- **UE版** - 旗舰版包含EE功能 + AI智能化功能
- **专属技术支持** - 7x24小时技术支持服务
- **安全保障** - 企业级安全解决方案

### 📧 联系我们 / Contact Us

如果您有任何问题或需求，欢迎随时联系我们：

- **GitHub** - 提交Issue或Pull Request
- **邮箱** - support@dbops.io
- **网站** - https://dbops.io
- **社交媒体** - 关注我们的官方账号获取最新动态

## 🚀 快速入门 / Quick Start

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
DBOPS_DB_URL=root:password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local

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
  dsn: root:password@tcp(localhost:3306)/mysql_ops

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

<!-- 语言支持 -->