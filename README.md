# MySQL 运维平台

MySQL 运维平台是一个大厂级别的 MySQL 数据库全生命周期管理平台，提供从环境检测、实例部署、版本升级、数据迁移到运维监控的完整解决方案。

## 特性

- **多数据库支持**: MySQL (Oracle)、Percona Server、MariaDB
- **版本范围**: MySQL 5.6 至 8.4.x，MariaDB 10.x 至 11.x
- **集群架构**: MHA、MGR、PXC
- **完整流程**: 环境检测 → 实例部署 → 参数调优 → 备份恢复 → 高可用管理 → 版本升级 → 数据迁移 → 监控告警
- **企业级特性**: 参数模板、审批流程、审计日志、拓扑视图
- **高性能**: 支持 5000+ 实例、10000+ 告警规则、100+ 并发迁移任务

## 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gin
- **ORM**: GORM
- **数据库**: PostgreSQL 13+
- **缓存**: Redis 6+
- **时序数据**: ClickHouse 22+

### Agent
- **语言**: Go 1.21+
- **部署**: 静态二进制，无需额外依赖
- **工具**: mysqld, xtrabackup, mysql client

### 前端
- **语言**: TypeScript
- **框架**: React 18+
- **UI 库**: Ant Design 5+
- **状态管理**: Redux Toolkit
- **图表**: Recharts

## 项目结构

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

平台需要以下数据库服务才能完整运行（用于存储配置、监控数据等）：

1. **PostgreSQL** (>= 13.x)
   - 存储实例配置、用户信息、操作日志等
   - 端口: 5432

2. **Redis** (>= 6.x)
   - 缓存和会话管理
   - 端口: 6379

3. **ClickHouse** (>= 22.x)
   - 存储监控指标和时序数据
   - 端口: 8123

4. **MySQL** (>= 5.6 或 MariaDB >= 10.x)
   - 被管理的目标数据库
   - 支持：MySQL (Oracle)、Percona Server、MariaDB
   - 支持版本: 5.6 至最新 (MySQL 8.4.x, MariaDB 11.x)

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
- PostgreSQL (port 5432)
- Redis (port 6379)
- ClickHouse (port 8123)
- MySQL 测试实例 (port 3306)
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
| PostgreSQL | 13.x | 存储配置和日志 | 5432 |
| Redis | 6.x | 缓存和会话 | 6379 |
| ClickHouse | 22.x | 监控指标存储 | 8123 |
| MySQL | 5.6+ | 被管理数据库 | 3306 |

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

### Standalone 模式问题

如果 standalone 模式下遇到 500 错误：
- 确保已修复所有 Repository 的 nil pointer 问题
- 检查日志确认是否为数据库连接失败
- 这是正常行为，部分功能需要数据库支持

### 数据库服务启动失败

```bash
# 检查 Docker 服务
docker ps

# 查看特定服务日志
docker-compose -f docker-compose.dev.yml logs postgres
docker-compose -f docker-compose.dev.yml logs redis
docker-compose -f docker-compose.dev.yml logs clickhouse

# 重新启动服务
docker-compose -f docker-compose.dev.yml restart
```

## 已知问题和限制

### Standalone 模式限制

当数据库服务不可用时，平台会以 standalone 模式运行，此时有以下限制：

- 无法持久化配置数据（重启后丢失）
- 只能返回空列表或 mock 数据
- 部分功能不可用（创建实例、保存配置等）
- 这是预期行为，需要启动数据库服务才能完整使用

### Agent 工具依赖

Agent 需要以下工具才能正常工作：

- **mysqld**: 用于实例部署（如果未安装，部署任务会失败）
- **xtrabackup**: 用于物理备份（如果未安装，备份任务会失败）
- **mysql**: 用于执行 SQL 命令

这些工具需要在目标 MySQL 节点上安装，且版本要与目标 MySQL 匹配。

### API 路由限制

部分 API 端点返回 404 错误，可能原因：

- 需要具体的子路径（如 `/api/v1/upgrades/{id}` 而非 `/api/v1/upgrades`）
- 路由配置需要改进
- 需要提供必需的查询参数

### 参数验证

部分 API 返回 400 错误，可能原因：

- 请求参数格式不正确
- 缺少必需的参数
- 参数类型不匹配

建议：查看 API 文档或检查错误消息详情。

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