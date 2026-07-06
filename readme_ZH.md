# MySQL Ops Platform / 智能 MySQL 运维平台

> 面向数据库架构级生命周期管理的 MySQL 运维平台。
>
> 平台通过后端 API、React 控制台和主机侧 Agent，统一管理主机、MySQL 实例、集群、中间件、监控、备份、升级和角色切换。

[![Go Version][go-image]][go-url] [![Node.js][node-image]][node-url] [![License][license-image]][license-url] [![Language][lang-image]][lang-url] [![Status][status-image]][status-url] [![Build][build-image]][build-url]

## 平台概览

当前仓库采用三组件架构：

- `backend`：Go 后端 API，负责部署编排、元数据、审计、监控接入和任务协调。
- `frontend`：React + TypeScript Web 控制台，负责集群部署、主机管理、监控、备份、升级和角色切换页面。
- `agent`：部署在被管主机上的 Go Agent，通过带认证的 HTTP API 执行 MySQL、中间件、备份、健康检查和升级任务。

支持的数据库架构：

- 单实例 MySQL 管理
- HA 主从架构
- MHA
- MGR
- PXC

## 最新功能

- 新增基于 React Flow 的流程图式集群部署编排。
- 集群部署详情页支持实时步骤进度、状态颜色、计划预览和状态轮询。
- 表单部署和流程编排共用后端部署计划模型。
- 支持通过 Agent 任务接口部署 Keepalived 和 ProxySQL 中间件。
- 支持环境预检、部署后健康检查、部署后基线备份等工具节点。
- 数据库部署完成后执行元数据同步，自动登记集群和实例。
- 部署状态支持 failed、partial、interrupted、in_progress 等韧性状态。
- Agent 管理支持批量安装、更新、删除、状态查看、心跳、最近操作和版本展示。
- 升级管理支持多种架构，并增强角色感知的滚动升级计划。
- 改进 HA/MHA 复制状态识别，以及 MGR/PXC 部署和角色切换处理。
- 打通 Agent 指标采集、后端接入和前端监控无数据/未配置状态展示。
- 增加 `.env.example`、后端示例配置、凭据加密和本地密钥扫描脚本，减少敏感信息误提交风险。

## 流程图式部署编排

集群部署页面现在提供两种部署模式：

- **流程编排**：默认模式。用户选择架构、配置数据库节点、添加兼容的中间件或工具节点、预览部署计划，然后提交部署。
- **表单部署**：保留原有表单部署入口，作为备用部署方式。

首版流程图节点包括：

- 架构节点：HA、MHA、MGR、PXC
- 数据库节点：master/replica、manager、primary/secondary、bootstrap/secondary
- 中间件节点：Keepalived、ProxySQL
- 工具节点：环境预检、健康检查、基线备份

兼容矩阵：

| 架构 | Keepalived | ProxySQL | 环境预检 | 健康检查 | 基线备份 |
|------|------------|----------|----------|----------|----------|
| HA | 支持 | 支持 | 支持 | 支持 | 支持 |
| MHA | 支持 | 支持 | 支持 | 支持 | 支持 |
| MGR | 首版禁用 | 支持 | 支持 | 支持 | 支持 |
| PXC | 首版禁用 | 支持 | 支持 | 支持 | 支持 |

流程编排继续复用现有部署接口：

- `POST /deployments/validate`
- `POST /deployments`
- `GET /deployments/:id/status`
- `GET /deployments/:id/plan`

流程图 JSON 保存在部署请求的 `custom.flow_spec` 中，首版不额外增加流程模板持久化表。

## Agent 能力

Agent 是平台执行主机侧操作的边界。当前支持：

- 主机和 Agent 生命周期：安装、更新、删除、状态查看、心跳、版本上报。
- MySQL 实例操作：部署、启动、停止、重启、删除、状态查看。
- 集群任务：HA/MHA 复制配置和检查，MGR 配置和角色切换支持，PXC 配置和状态检查。
- 中间件任务：
  - `POST /agent/tasks/keepalived-setup`
  - `POST /agent/tasks/proxysql-setup`
- 升级任务：原地升级、逻辑迁移升级、角色感知滚动升级。
- 指标采集：CPU、内存、磁盘、MySQL、复制状态和服务健康数据。

ProxySQL 部署使用目标主机的 Agent 端口，不使用 ProxySQL admin 端口。Keepalived 当前只在 HA/MHA 部署流程中启用。

## 系统架构

```text
frontend (:3000 或 Vite 开发端口)
        |
        | REST API /api/v1
        v
backend (:8080)  ---- HTTP + Bearer token ---->  agent (:9090)
        |
        | 元数据、审计、任务、监控
        v
SQLite 或 MySQL，取决于存储模式配置
```

可选集成：

- Redis：缓存或队列场景。
- ClickHouse：监控数据存储。
- Keepalived 和 ProxySQL：用于 HA/MHA/MGR/PXC 的流量管理模式。

## 仓库布局

```text
backend/            Go 后端 API、服务、仓储、配置和迁移
frontend/           React + TypeScript Web 控制台
agent/              部署在被管主机上的 Go 执行 Agent
bin/                辅助二进制和脚本
scripts/            运维脚本，包括本地密钥扫描
docs/               补充文档和截图
data/               本地开发数据
logs/               本地运行日志
Makefile            构建、测试、安装、打包和升级辅助命令
start.bat/.ps1      Windows 一键启动
stop.bat/.ps1       Windows 一键停止
```

## 系统要求

| 组件 | 版本 | 说明 |
|------|------|------|
| Go | Backend 1.25+，Agent 1.21+ | 后端和 Agent 构建/运行 |
| Node.js | 18+ | 前端构建/运行 |
| npm | 必需 | 前端依赖管理 |
| PowerShell | 5.1+ | Windows 脚本 |
| bash | Linux/macOS 必需 | Shell 脚本 |
| Redis | 可选 | 缓存/队列场景 |
| ClickHouse | 可选 | 监控数据存储 |

目标 MySQL 主机需要具备选定操作所需的系统权限和 MySQL 工具。根据部署计划不同，还可能需要 MySQL 安装包、备份工具、Keepalived、ProxySQL 或包下载访问能力。

## 配置

复制 `.env.example` 为 `.env`，并在非本地环境中设置强密钥：

```env
DBOPS_DB_URL=dbops_user:replace-with-strong-password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local
DBOPS_JWT_SECRET=replace-with-at-least-32-chars
DBOPS_AGENT_TOKEN=replace-with-at-least-16-chars
DBOPS_ENCRYPTION_KEY=replace-with-at-least-32-chars
```

后端示例配置文件：

```text
backend/config/config.example.yaml
```

安全注意事项：

- 不要提交真实密钥、Token、数据库密码、SSH 私钥或 License 材料。
- 敏感凭据使用 AES-GCM 加密。
- 发布改动前建议执行本地密钥扫描：

```powershell
.\scripts\scan-local-secrets.ps1
```

## 构建和运行

安装依赖：

```bash
make install-backend
make install-agent
make install-web
```

构建所有组件：

```bash
make build
```

手动启动组件：

```bash
cd backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd frontend && npm run dev -- --host 0.0.0.0 --port 3000
```

Windows 一键启动：

```powershell
.\start.bat
```

默认本地访问地址：

- 后端 API：`http://localhost:8080`
- Web 控制台：`http://localhost:3000`
- Agent 服务：`http://localhost:9090`

## 测试

```bash
cd backend && go test ./...
cd agent && go test ./...
cd frontend && npm test -- --run
cd frontend && npm run build
```

近期验证覆盖后端部署计划、仓储、认证、密码加密、Agent 中间件任务路由、Agent 指标采集、前端部署辅助函数、角色展示和流程图转换逻辑。

## 运维说明

- 长周期操作应通过后端 API 和 Agent 任务执行。
- 部署进度应从部署状态和计划接口读取，不应只依赖前端本地状态。
- 核心数据库部署完成后，如果中间件、健康检查或基线备份失败，部署可进入 `partial` 状态并保留已创建资源。
- 后端启动时会识别中断的部署并标记为 interrupted。
- MGR 和 PXC 重新部署前，建议确认主机安装包布局、MySQL 数据目录、插件路径和 Agent 版本。

## 文档

- 中文文档：[readme_ZH.md](readme_ZH.md)
- 英文文档：[readme_US.md](readme_US.md)
- 截图目录：[docs/screenshots](docs/screenshots)
- 密钥扫描脚本：[scripts/scan-local-secrets.ps1](scripts/scan-local-secrets.ps1)

## 商业版本

- **CE**：社区版，提供平台核心功能。
- **EE**：企业版，包含 CE 功能以及高可用、升级、迁移、审计能力。
- **UE**：旗舰版，包含 EE 功能以及 AI 辅助运维和商业功能。

## 联系方式

- GitHub：提交 Issue 或 Pull Request。
- 支持邮箱：`ice_out@sina.com`
- 企业咨询：`ice_out@sina.com`

[go-image]: https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go
[go-url]: https://go.dev/
[node-image]: https://img.shields.io/badge/Node.js-18+-339933?style=flat&logo=node.js
[node-url]: https://nodejs.org/
[license-image]: https://img.shields.io/badge/License-MIT-blue.svg
[license-url]: https://opensource.org/licenses/MIT
[lang-image]: https://img.shields.io/badge/Language-Go%20%7C%20TypeScript-blue
[lang-url]: https://github.com/mingjia1/dbops
[status-image]: https://img.shields.io/badge/Status-active-brightgreen.svg
[status-url]: https://github.com/mingjia1/dbops
[build-image]: https://img.shields.io/badge/Build-manual-lightgrey.svg
[build-url]: https://github.com/mingjia1/dbops
