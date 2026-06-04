# MySQL 运维平台

MySQL 运维平台是一个大厂级别的 MySQL 数据库全生命周期管理平台，提供从环境检测、实例部署、版本升级、数据迁移到运维监控的完整解决方案。

> **前置条件**: 平台负责 MySQL 层面的运维管理，**操作系统 (OS) 由用户自行提供**。部署节点上需预先安装 mysqld、xtrabackup、mysql client 等 MySQL 管理工具。

## 架构

```
┌───────────────┐     HTTP API      ┌──────────────────┐
│   Backend     │ ────────────────→ │  Agent (host:9090)│
│  (Gin :8080)  │ ←──────────────── │  mysqld/xtrabackup│
│               │   Task Result     │  本地命令执行     │
└───────┬───────┘                   └──────────────────┘
        │
        ▼
┌───────────────┐
│   MySQL DB    │
│  存储配置/日志  │
└───────────────┘
```

- **Backend**: 核心管理平台，提供 REST API，管理者通过这里操作
- **Agent**: 部署在每个目标节点上的执行器，接收 Backend 指令，在本地执行 MySQL 管理操作
- **MySQL**: 平台自身的配置存储数据库（非被管理实例）

## 特性

- **多数据库支持**: MySQL (Oracle)、Percona Server、MariaDB
- **版本范围**: MySQL 5.6 至 8.4.x，MariaDB 10.x 至 11.x
- **集群架构**: MHA、MGR、PXC
- **完整流程**: 环境检测 → 实例部署 → 参数调优 → 备份恢复 → 高可用管理 → 版本升级 → 数据迁移 → 监控告警
- **企业级特性**: 参数模板、审批流程、审计日志、拓扑视图
- **高性能**: 支持 5000+ 实例、10000+ 告警规则、100+ 并发迁移任务

## 技术栈

### 后端
- **语言**: Go 1.24+
- **框架**: Gin
- **数据库**: MySQL 5.6+
- **缓存**: Redis 6+
- **时序数据**: ClickHouse 22+

### Agent
- **语言**: Go 1.21+
- **部署**: 静态二进制，部署在目标 MySQL 节点上
- **工具**: mysqld, xtrabackup, mysql client
- **通信**: HTTP REST API，由 Backend 主动调用

### 前端
- **语言**: TypeScript
- **框架**: React 18+
- **UI 库**: Ant Design 5+
- **状态管理**: Redux Toolkit
- **图表**: Recharts

## 项目结构

```
mysql-ops-platform/
├── platform-backend/     # 后端平台服务
├── agent/                # Agent 执行器
├── web-console/          # 前端 Web Console
├── docker-compose.dev.yml # 开发环境 Docker 配置
├── Makefile              # 构建脚本
└── .monkeycode/          # 规范文档
    └── specs/
        └── mysql-ops-platform/
            ├── requirements.md   # 需求文档
            ├── design.md         # 技术设计
            └── tasklist.md       # 实施计划
```

## 快速开始

### 前置要求

#### 必需的系统工具

1. **Go** (>= 1.21)
   - 用于编译后端和 Agent
   - 安装: https://go.dev/doc/install

2. **Node.js** (>= 18.x) + npm
   - 用于编译前端
   - 安装: https://nodejs.org/

3. **Docker** (>= 20.10) + Docker Compose (>= 2.0)
   - 用于运行开发环境
   - 安装: https://docs.docker.com/get-docker/

#### 必需的数据库服务

平台需要以下数据库服务才能完整运行：

1. **MySQL** (>= 5.6 或 MariaDB >= 10.x)
   - 存储平台配置、实例信息、用户数据、操作日志等
   - 端口: 3306
   - 可以是自建 MySQL，也可复用被管理集群中的一个实例

2. **Redis** (>= 6.x)（可选）
   - 缓存和会话管理
   - 端口: 6379

3. **ClickHouse** (>= 22.x)（可选）
   - 存储监控指标和时序数据
   - 端口: 8123

#### 必需的 MySQL 管理工具

Agent 在目标 MySQL 节点上需要以下工具才能执行管理任务：

1. **mysqld** (MySQL Server)
   - 用于实例部署和启动
   - 必须与目标 MySQL 版本匹配

2. **xtrabackup** (Percona XtraBackup)
   - 用于物理备份和恢复
   - 推荐: XtraBackup 8.0 for MySQL 8.0
   - 安装: https://docs.percona.com/percona-xtrabackup/

3. **mysql** (MySQL Client)
   - 用于执行 SQL 命令和检查
   - 通常与 mysql-server 同包

#### 可选工具

1. **git** - 用于版本控制
2. **make** - 用于构建脚本
3. **curl/wget** - 用于下载依赖

### 1. 启动开发环境（包含所有数据库服务）

```bash
make docker-up
```

这将启动以下服务：
- MySQL (port 3306) - 平台配置数据库
- Redis (port 6379)（可选）
- ClickHouse (port 8123)（可选）
- MySQL 测试实例 (port 3307)
- Backend (port 8080)
- Agent (port 9090)
- Frontend (port 3000)

### 2. 安装依赖

```bash
make all
```

或分别安装：

```bash
cd platform-backend && go mod tidy
cd agent && go mod tidy
cd web-console && npm install
```

### 3. 运行服务

#### 使用 Makefile（推荐）

```bash
# 运行后端
make run-backend

# 运行 Agent
make run-agent

# 运行前端
make run-web
```

#### 手动运行

```bash
# 后端
cd platform-backend && go run ./cmd

# Agent
cd agent && go run ./cmd

# 前端
cd web-console && npm run dev
```

### 4. 访问服务

- **后端 API**: http://localhost:8080
  - 健康检查: http://localhost:8080/api/v1/health
  - API 文档: http://localhost:8080/api/v1/docs (如果有)

- **前端 Console**: http://localhost:3000
  - 登录页面: http://localhost:3000/login
  - 仪表板: http://localhost:3000/dashboard

- **Agent**: http://localhost:9090
  - 健康检查: http://localhost:9090/health

### 5. Standalone 模式（无数据库）

如果没有运行数据库服务，平台可以以 standalone 模式运行：

```bash
# 后端会自动检测数据库并跳过认证
make run-backend
```

**Standalone 模式限制**:
- 无法持久化数据（实例配置、用户信息等）
- 只能查看空列表或 mock 数据
- 无法执行需要数据库的操作（创建实例、保存配置等）
- Agent 部署/备份任务需要本地 MySQL 工具支持

## 系统依赖清单

### 开发环境

| 工具 | 最低版本 | 用途 | 安装链接 |
|------|---------|------|---------|
| Go | 1.21+ | 编译后端和 Agent | https://go.dev/doc/install |
| Node.js | 18.x+ | 编译前端 | https://nodejs.org/ |
| Docker | 20.10+ | 运行开发环境 | https://docs.docker.com/get-docker/ |
| Docker Compose | 2.0+ | 编排开发环境 | https://docs.docker.com/compose/install/ |

### 生产数据库服务

| 服务 | 最低版本 | 用途 | 端口 |
|------|---------|------|------|
| MySQL | 5.6+ | 平台配置和日志存储 | 3306 |
| Redis | 6.x | 缓存和会话（可选） | 6379 |
| ClickHouse | 22.x | 监控指标存储（可选） | 8123 |

### MySQL 管理工具（Agent 节点必需）

| 工具 | 用途 | 安装方式 |
|------|------|---------|
| mysqld | MySQL 服务端 | apt/yum install mysql-server |
| xtrabackup | 物理备份 | https://docs.percona.com/percona-xtrabackup/ |
| mysql | MySQL 客户端 | apt/yum install mysql-client |

## 安装 MySQL 管理工具示例

### Ubuntu/Debian

```bash
# MySQL Server (根据需要选择版本)
sudo apt update
sudo apt install -y mysql-server-8.0 mysql-client-8.0

# Percona XtraBackup 8.0 for MySQL 8.0
wget https://repo.percona.com/apt/percona-release_latest.$(lsb_release -sc)_all.deb
sudo dpkg -i percona-release_latest.$(lsb_release -sc)_all.deb
sudo apt update
sudo apt install -y percona-xtrabackup-80

# 验证安装
mysql --version
xtrabackup --version
```

### CentOS/RHEL

```bash
# MySQL Server
sudo yum install -y mysql-server mysql

# Percona XtraBackup
sudo yum install -y https://repo.percona.com/yum/percona-release-latest.noarch.rpm
sudo percona-release enable-only tools release
sudo yum install -y percona-xtrabackup-80

# 验证安装
mysql --version
xtrabackup --version
```

### 验证工具安装

```bash
# 检查 mysqld
which mysqld
mysqld --version

# 检查 xtrabackup
which xtrabackup
xtrabackup --version

# 检查 mysql client
which mysql
mysql --version
```

## 开发指南

### 后端开发

后端使用 Go + Gin + GORM + PostgreSQL 开发：

```bash
cd platform-backend
go mod tidy
go run ./cmd
```

### Agent 开发

Agent 使用 Go 开发，部署在数据库节点上：

```bash
cd agent
go mod tidy
go run ./cmd
```

### 前端开发

前端使用 React + TypeScript + Ant Design 开发：

```bash
cd web-console
npm install
npm run dev
```

## 测试

```bash
make test
```

### 运行特定测试

```bash
# 后端单元测试
cd platform-backend && go test ./internal/services/... -v

# Agent 单元测试
cd agent && go test ./internal/executor/... -v

# E2E 测试
cd platform-backend && go test ./tests/e2e/... -v

# 性能测试
cd platform-backend && go test ./tests/benchmark/... -bench=. -benchmem
```

## 构建

```bash
make build
```

构建产物：
- `platform-backend/mysql-ops-platform`: 后端可执行文件（~20MB）
- `agent/mysql-ops-agent`: Agent 可执行文件（~15MB）
- `web-console/dist/`: 前端静态文件（~1.7MB）

## 实施状态

| 功能 | 状态 | 说明 |
|------|------|------|
| 环境检测 | 🟢 已实现 | Backend 编排，Agent 执行远程检测 |
| 实例部署（从无到有） | 🟢 已实现 | Agent 执行 mysqld 初始化+启动+验证 |
| 主机管理 | 🟢 已实现 | SSH 连接测试、凭据加密存储 |
| 实例管理 | 🟢 已实现 | 全生命周期 CRUD、版本检测 |
| 参数模板 | 🟢 已实现 | 模板创建/推荐/校验/应用 |
| 集群部署 MHA | 🟢 已实现 | Manager + Node 自动化部署 |
| 集群部署 MGR | 🟢 已实现 | Group Replication 自动化搭建 |
| 集群部署 PXC | 🟢 已实现 | Percona XtraDB Cluster 自动化部署 |
| 高可用/故障切换 | 🟢 已实现 | 自动故障检测 + VIP 漂移 + 主从切换 |
| 版本升级 | 🟢 已实现 | In-Place / 逻辑迁移 / Rolling 三种策略 |
| 数据迁移 | 🟢 已实现 | 物理迁移 / 复制迁移 / GTID 迁移 / 在线切换 |
| 备份恢复 | 🟢 已实现 | Xtrabackup 物理备份、全量/增量 |
| 监控指标 | 🟢 已实现 | 系统指标 + MySQL 指标采集 |
| 告警管理 | 🟢 已实现 | 规则配置、通道管理（邮件/钉钉/企业微信） |
| 审批流程 | 🟢 已实现 | 申请/审批/驳回 |
| 审计日志 | 🟢 已实现 | 操作记录全追踪 |
| 拓扑视图 | 🟢 已实现 | 集群拓扑、实例关系展示 |
| 单点→集群切换 | 🔵 规划中 | 将独立实例接入 MHA/MGR/PXC |
| 集群内角色切换 | 🔵 规划中 | 同一集群内主/从、Primary/Secondary 角色互转，副本拓扑重搭 |

## 部署

### 使用 Docker

```bash
# 构建镜像
docker-compose -f docker-compose.prod.yml build

# 启动服务
docker-compose -f docker-compose.prod.yml up -d

# 查看日志
docker-compose -f docker-compose.prod.yml logs -f
```

### 手动部署

1. 准备服务器（Ubuntu 20.04+ / CentOS 8+）
2. 安装系统依赖（见上文）
3. 启动数据库服务（PostgreSQL、Redis、ClickHouse）
4. 配置环境变量（见 `platform-backend/config/config.yaml`）
5. 启动后端服务
6. 在目标 MySQL 节点启动 Agent
7. 使用 Nginx 反向代理前端服务

## 故障排查

### 后端启动失败

```bash
# 检查端口占用
lsof -i :8080

# 检查数据库连接
psql -h localhost -U postgres -d mysql_ops_platform

# 查看日志
tail -f /var/log/mysql-ops-platform/backend.log
```

### Agent 无法连接

```bash
# 检查 Agent 健康状态
curl http://localhost:9090/health

# 检查工具安装
which mysqld
which xtrabackup
which mysql

# 查看日志
tail -f /var/log/mysql-ops-platform/agent.log
```

### 前端页面空白

```bash
# 检查 Node.js 版本
node --version

# 重新安装依赖
cd web-console && rm -rf node_modules && npm install

# 检查构建
npm run build
```

### API 返回 400 错误

常见原因：
- 请求参数格式不正确
- 缺少必需的参数
- 参数验证失败

解决方法：
- 检查 API 文档
- 查看错误消息详情
- 提供正确的参数格式

### 数据库服务启动失败

```bash
# 检查 Docker 服务
docker ps

# 查看特定服务日志
docker-compose -f docker-compose.dev.yml logs mysql
docker-compose -f docker-compose.dev.yml logs redis
docker-compose -f docker-compose.dev.yml logs clickhouse

# 重新启动服务
docker-compose -f docker-compose.dev.yml restart
```

## 已知问题和限制

### Agent 工具依赖

Agent 需要以下工具才能正常工作：

- **mysqld**: 用于实例部署（如果未安装，部署任务会失败）
- **xtrabackup**: 用于物理备份（如果未安装，备份任务会失败）
- **mysql**: 用于执行 SQL 命令

这些工具需要在目标 MySQL 节点上安装，且版本要与目标 MySQL 匹配。

### IPv6 支持

平台支持 IPv6 地址，但需要：

- 确保 MySQL 实例绑定 IPv6 地址
- 使用方括号格式：`[::1]:3306`
- 检查防火墙规则允许 IPv6 连接

## 文档

### 快速开始

- **[快速入门指南](QUICKSTART.md)** - 10 分钟快速上手
- **[详细安装指南](INSTALL.md)** - 完整的安装和部署文档

### 项目文档

详细文档位于 `.monkeycode/specs/mysql-ops-platform/` 目录：

- `requirements.md`: 完整需求文档
- `design.md`: 技术设计文档
- `tasklist.md`: 实施计划

### 测试报告

- `final_test_report.md`: 最终功能测试报告（系统生成的测试报告）

## 快速链接

- [安装依赖](#1-安装依赖)
- [运行服务](#3-运行服务)
- [访问服务](#4-访问服务)
- [故障排查](#故障排查)

## License

MIT