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
│   ├── cmd/              # 主程序入口
│   │   ├── main.go       # 后端主程序
│   │   ├── destroy_cluster/  # 集群销毁工具
│   │   └── list_clusters/    # 集群列表工具
│   ├── internal/         # 内部实现
│   │   ├── handlers/     # HTTP 处理器
│   │   ├── services/     # 业务逻辑
│   │   ├── models/       # 数据模型
│   │   └── repositories/ # 数据访问层
│   ├── config/           # 配置文件
│   └── data/             # SQLite 数据文件
├── agent/                # Agent 执行器
│   ├── cmd/              # Agent 入口
│   ├── internal/         # Agent 内部实现
│   │   └── executor/     # 任务执行器
│   └── config/           # Agent 配置
├── web-console/          # 前端 Web Console
│   ├── src/              # 源代码
│   │   ├── pages/        # 页面组件
│   │   ├── components/   # 公共组件
│   │   └── services/     # API 服务
│   ├── public/           # 静态资源
│   └── dist/             # 构建产物
├── bin/                  # Linux/Unix 启动脚本
│   ├── init-ubuntu.sh    # Ubuntu 22.04 一键初始化
│   ├── start-all.sh      # 启动所有服务
│   ├── start-backend.sh  # 启动后端
│   ├── start-agent.sh    # 启动 Agent
│   ├── start-web.sh      # 启动前端
│   └── stop-services.sh  # 停止所有服务
├── scripts/              # 工具脚本
│   ├── deploy_mysql.sh   # MySQL 部署脚本
│   ├── init_57.sh        # MySQL 5.7 初始化
│   ├── secure_mysql.sh   # MySQL 安全加固
│   ├── grant_devbox.sh   # 开发环境权限
│   ├── smoke_test.sh     # 冒烟测试
│   └── write_units.sh    # systemd 单元生成
├── docs/                 # 文档
│   ├── MHA_CLUSTER_DESTROY_TEST_REPORT.md  # 集群销毁测试报告
│   ├── cluster_destroy_test_guide.md       # 集群销毁测试指南
│   ├── topology_optimization_summary.md    # 拓扑优化总结
│   └── SCRIPT_FINAL_REPORT.md              # 脚本检查报告
├── start.ps1 / start.bat         # Windows 启动所有服务
├── start-server.ps1 / start-server.bat  # Windows 启动服务器（不含Agent）
├── stop.ps1 / stop.bat           # Windows 停止服务
├── restart.ps1 / restart.bat     # Windows 重启服务
├── .env.example          # 环境变量模板
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

### 前置准备

#### 基础环境要求

| 组件 | 最低版本 | 推荐版本 | 说明 |
|------|---------|---------|------|
| **操作系统** | Ubuntu 20.04 / Windows 10 | Ubuntu 22.04 / Windows 11 | CentOS 7+ / macOS 也支持 |
| **Go** | 1.21 | 1.22+ | 用于编译后端和Agent |
| **Node.js** | 18.x | 20.x+ | 用于编译前端 |
| **MySQL** | 5.7 | 8.0+ | 平台配置数据库 |

#### 必需的系统工具

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install -y curl wget git vim net-tools lsof build-essential
```

**CentOS/RHEL:**
```bash
sudo yum install -y curl wget git vim net-tools lsof gcc make
```

**Windows:**
- Git Bash 或 PowerShell 5.1+
- Visual C++ Redistributable（通常已内置）

#### 必需的数据库服务

平台运行需要以下数据库：

**1. MySQL（必需）**
- **用途**: 存储平台配置、实例信息、用户数据、操作日志
- **版本**: MySQL 5.7+ / 8.0+ 或 MariaDB 10.x+
- **端口**: 3306（默认）
- **最低配置**: 
  - 内存: 512MB+
  - 磁盘: 10GB+
  - 连接数: 100+

**2. Redis（可选，推荐）**
- **用途**: 会话缓存、任务队列
- **版本**: Redis 6.x+
- **端口**: 6379（默认）
- **最低配置**: 内存 256MB+

**3. ClickHouse（可选）**
- **用途**: 监控指标时序数据存储
- **版本**: ClickHouse 22.x+
- **端口**: 8123（HTTP）、9000（Native）
- **最低配置**: 
  - 内存: 2GB+
  - 磁盘: 50GB+

#### 必需的MySQL管理工具（Agent节点）

Agent需要在目标MySQL节点上安装以下工具：

**1. mysqld（MySQL Server）**
```bash
# Ubuntu/Debian
sudo apt install -y mysql-server-8.0

# CentOS/RHEL
sudo yum install -y mysql-server
```

**2. xtrabackup（Percona XtraBackup）**
```bash
# Ubuntu/Debian
wget https://repo.percona.com/apt/percona-release_latest.$(lsb_release -sc)_all.deb
sudo dpkg -i percona-release_latest.$(lsb_release -sc)_all.deb
sudo apt update
sudo apt install -y percona-xtrabackup-80

# CentOS/RHEL
sudo yum install -y https://repo.percona.com/yum/percona-release-latest.noarch.rpm
sudo percona-release enable-only tools release
sudo yum install -y percona-xtrabackup-80
```

**3. mysql（MySQL Client）**
```bash
# Ubuntu/Debian
sudo apt install -y mysql-client-8.0

# CentOS/RHEL
sudo yum install -y mysql
```

#### 环境变量配置

复制模板文件并填写必需配置：

```bash
cp .env.example .env
```

**必需的环境变量：**

```bash
# 数据库连接（必需）
DBOPS_DB_URL=root:password@tcp(localhost:3306)/dbops?charset=utf8mb4&parseTime=true

# JWT密钥（必需，至少32字符）
DBOPS_JWT_SECRET=your-secret-key-at-least-32-characters-long

# 数据加密密钥（必需，至少32字符）
DBOPS_ENCRYPTION_KEY=your-encryption-key-32-chars-min

# Agent认证Token（必需，至少32字符）
DBOPS_AGENT_TOKEN=your-agent-token-at-least-32-chars
```

**可选的环境变量：**

```bash
# Redis连接（可选）
DBOPS_REDIS_URL=redis://localhost:6379/0

# ClickHouse连接（可选）
DBOPS_CLICKHOUSE_URL=http://localhost:8123

# 日志级别（可选，默认info）
DBOPS_LOG_LEVEL=info

# 服务端口（可选）
DBOPS_BACKEND_PORT=8080
DBOPS_AGENT_PORT=9090
DBOPS_WEB_PORT=3000
```

#### 防火墙配置

**Ubuntu (ufw):**
```bash
sudo ufw allow 8080/tcp   # Backend API
sudo ufw allow 9090/tcp   # Agent
sudo ufw allow 3000/tcp   # Web Console
sudo ufw reload
```

**CentOS (firewalld):**
```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=9090/tcp
sudo firewall-cmd --permanent --add-port=3000/tcp
sudo firewall-cmd --reload
```

**Windows Firewall:**
```powershell
# PowerShell (以管理员身份运行)
New-NetFirewallRule -DisplayName "MySQL Ops Backend" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
New-NetFirewallRule -DisplayName "MySQL Ops Agent" -Direction Inbound -LocalPort 9090 -Protocol TCP -Action Allow
New-NetFirewallRule -DisplayName "MySQL Ops Web" -Direction Inbound -LocalPort 3000 -Protocol TCP -Action Allow
```

#### 验证环境准备

运行以下命令验证环境是否就绪：

```bash
# 验证Go
go version
# 预期输出: go version go1.21+ ...

# 验证Node.js
node --version
# 预期输出: v18.x.x 或更高

npm --version
# 预期输出: 9.x.x 或更高

# 验证MySQL（如果本地安装）
mysql --version
# 预期输出: mysql  Ver 8.0.x ...

# 验证XtraBackup（Agent节点）
xtrabackup --version
# 预期输出: xtrabackup version 8.0.x ...

# 验证mysqld（Agent节点）
mysqld --version
# 预期输出: mysqld  Ver 8.0.x ...
```

### Ubuntu 22.04 部署（推荐）

#### 1. 一键环境初始化

```bash
# 克隆项目
git clone <repo-url>
cd dbops

# 运行一键初始化脚本（需要 root 权限）
sudo bash bin/init-ubuntu.sh
```

该脚本会自动安装：
- Go 1.21+
- Node.js 18.x
- MySQL 8.0 Server + Client
- Percona XtraBackup 8.0
- Docker + Docker Compose（可选）

#### 2. 配置环境变量

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑配置文件，填写必需的环境变量
vim .env
```

必需配置项：
```bash
DBOPS_DB_URL=root:password@tcp(localhost:3306)/dbops?charset=utf8mb4&parseTime=true
DBOPS_JWT_SECRET=your-secret-key-min-32-chars
DBOPS_ENCRYPTION_KEY=your-encryption-key-32-chars
DBOPS_AGENT_TOKEN=your-agent-token-min-32-chars
```

#### 3. 启动服务

```bash
# 启动所有服务（后端 + Agent + 前端）
bash bin/start-all.sh

# 或分别启动
bash bin/start-backend.sh  # 后端 (端口 8080)
bash bin/start-agent.sh    # Agent (端口 9090)
bash bin/start-web.sh      # 前端 (端口 3000)
```

#### 4. 访问服务

- 前端控制台: http://localhost:3000
- 后端 API: http://localhost:8080
- Agent: http://localhost:9090

#### 5. 停止服务

```bash
bash bin/stop-services.sh
```

### Windows 部署

#### 1. 安装前置软件

1. **安装 Go 1.21+**
   - 下载: https://golang.org/dl/
   - 安装到: `D:\Program Files\go`（或修改脚本中的路径）

2. **安装 Node.js 18.x+**
   - 下载: https://nodejs.org/
   - 确保 npm 在 PATH 中

3. **（可选）安装 Git**
   - 下载: https://git-scm.com/download/win

#### 2. 克隆项目

```powershell
git clone <repo-url>
cd dbops
```

#### 3. 配置环境变量

复制 `.env.example` 为 `.env` 并填写必需配置：

```powershell
copy .env.example .env
notepad .env
```

必需配置项：
```
DBOPS_DB_URL=root:password@tcp(localhost:3306)/dbops?charset=utf8mb4&parseTime=true
DBOPS_JWT_SECRET=your-secret-key-min-32-chars
DBOPS_ENCRYPTION_KEY=your-encryption-key-32-chars
DBOPS_AGENT_TOKEN=your-agent-token-min-32-chars
```

#### 4. 启动服务

**方式一：双击启动（推荐）**

直接双击以下文件：
- `start.bat` - 启动所有服务（后端 + Agent + 前端）
- `start-server.bat` - 只启动服务器（后端 + 前端，不含Agent）

**方式二：PowerShell 启动**

```powershell
# 启动所有服务（自动编译）
powershell -ExecutionPolicy Bypass -File .\start.ps1

# 跳过编译，直接启动
powershell -ExecutionPolicy Bypass -File .\start.ps1 -SkipBuild

# 只启动服务器（不含Agent）
powershell -ExecutionPolicy Bypass -File .\start-server.ps1
```

启动脚本会自动：
- 检测 Go 和 Node.js 环境
- 编译后端和 Agent（如果源码有更新）
- 安装前端依赖（如果 node_modules 缺失）
- 构建前端（如果源码有更新）
- 后台启动所有服务
- 等待服务就绪并进行健康检查

#### 5. 访问服务

- 前端控制台: http://localhost:3000
- 后端 API: http://localhost:8080
- Agent: http://localhost:9090

日志目录: `logs/`
- `backend.log` / `backend.err` - 后端日志
- `agent.log` / `agent.err` - Agent 日志
- `web.log` / `web.err` - 前端日志

#### 6. 停止服务

**方式一：双击**
- `stop.bat` - 停止所有服务

**方式二：PowerShell**
```powershell
powershell -ExecutionPolicy Bypass -File .\stop.ps1
```

#### 7. 重启服务

**方式一：双击**
- `restart.bat` - 重启所有服务

**方式二：PowerShell**
```powershell
powershell -ExecutionPolicy Bypass -File .\restart.ps1
```

### 开发环境快速启动（使用Docker）

如果您已经安装了Docker和Docker Compose，可以使用以下命令快速启动包含所有依赖的开发环境：

```bash
# 启动所有服务（MySQL + Redis + ClickHouse + Backend + Agent + Frontend）
make docker-up

# 查看服务状态
docker-compose -f docker-compose.dev.yml ps

# 查看日志
docker-compose -f docker-compose.dev.yml logs -f

# 停止服务
make docker-down
```

这将启动以下服务：
- MySQL (port 3306) - 平台配置数据库
- Redis (port 6379) - 缓存服务
- ClickHouse (port 8123) - 监控数据存储
- Backend (port 8080) - 后端API
- Agent (port 9090) - Agent服务
- Web Console (port 3000) - 前端控制台

### 手动安装依赖

如果不使用Docker，需要手动安装依赖：

```bash
# 安装后端依赖
cd platform-backend && go mod tidy

# 安装Agent依赖
cd agent && go mod tidy

# 安装前端依赖
cd web-console && npm install
```

或使用Makefile：

```bash
make all
```

### 手动运行服务

如果不使用Ubuntu/Windows脚本，可以手动运行各个服务：

#### 使用 Makefile

```bash
# 运行后端
make run-backend

# 运行 Agent
make run-agent

# 运行前端
make run-web
```

#### 直接运行

```bash
# 后端
cd platform-backend && go run ./cmd

# Agent
cd agent && go run ./cmd

# 前端
cd web-console && npm run dev
```

### 访问服务

服务启动后，可通过以下地址访问：

- **前端控制台**: http://localhost:3000
  - 登录页面: http://localhost:3000/login
  - 仪表板: http://localhost:3000/dashboard

- **后端 API**: http://localhost:8080
  - 健康检查: http://localhost:8080/health
  - API文档: http://localhost:8080/api/docs（如果启用）

- **Agent**: http://localhost:9090
  - 健康检查: http://localhost:9090/health

### Standalone 模式（仅开发调试）

如果没有运行数据库服务，平台可以以 Standalone 模式运行用于开发调试：

```bash
# 后端会自动检测数据库并跳过认证
make run-backend
```

**Standalone 模式限制**:
- ⚠️ 无法持久化数据（实例配置、用户信息等）
- ⚠️ 只能查看空列表或 mock 数据
- ⚠️ 无法执行需要数据库的操作（创建实例、保存配置等）
- ⚠️ Agent 部署/备份任务仍需要本地 MySQL 工具支持

**不推荐在生产环境使用 Standalone 模式。**

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
| 拓扑视图 | 🟢 已实现 | 集群拓扑、实例关系展示、健康状态可视化 |
| 集群销毁 | 🟢 已实现 | 备份验证 → 数据目录删除 → 平台元数据清理 |
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

### 功能文档

- **[MHA集群销毁测试报告](docs/MHA_CLUSTER_DESTROY_TEST_REPORT.md)** - MHA集群完整销毁功能验证报告
- **[集群销毁测试指南](docs/cluster_destroy_test_guide.md)** - 集群销毁功能的完整测试流程
- **[拓扑优化总结](docs/topology_optimization_summary.md)** - 拓扑视图优化技术文档
- **[脚本检查报告](docs/SCRIPT_FINAL_REPORT.md)** - 项目脚本组织和语法检查报告

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