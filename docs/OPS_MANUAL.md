# DBOps 平台运维手册

> 版本: 1.0 | 最后更新: 2026-06-22

---

## 目录

1. [部署架构](#1-部署架构)
2. [环境准备](#2-环境准备)
3. [部署步骤](#3-部署步骤)
4. [启动与停止](#4-启动与停止)
5. [平台功能](#5-平台功能)
6. [Agent API 参考](#6-agent-api-参考)
7. [集群部署指南](#7-集群部署指南)
8. [故障排查](#8-故障排查)

---

## 1. 部署架构

```
┌──────────────┐     REST API      ┌──────────────────┐     HTTP+Token     ┌──────────────┐
│  Web Console │  ◄──────────────► │ Platform Backend │  ◄──────────────► │    Agent     │
│  :3000       │    /api/v1/*      │  :8080           │    /agent/tasks/*  │  :9090       │
│  React 18    │                   │  Go + Gin        │                    │  Go          │
└──────────────┘                   └────────┬─────────┘                    └──────────────┘
                                            │                                    │
                                            │ metadata storage                   │ mysqld, xtrabackup
                                            ▼                                    ▼
                                    ┌──────────────┐                    ┌──────────────┐
                                    │  MySQL/SQLite │                    │  MySQL 实例   │
                                    │  (平台元数据)  │                    │  (被管理)     │
                                    └──────────────┘                    └──────────────┘
```

**组件说明:**

| 组件 | 端口 | 语言 | 功能 |
|------|------|------|------|
| **platform-backend** | 8080 | Go 1.25+ | REST API 服务，管理主机、实例、用户、部署 |
| **web-console** | 3000 | React 18 | 网页管理界面 |
| **agent** | 9090 | Go 1.21+ | 部署在被管理主机上，通过 HTTP API 执行 MySQL 操作 |

**数据流:**
1. 用户操作 Web Console → REST API 请求到 Backend
2. Backend 验证权限 → 调用 Agent API 执行具体 MySQL 操作
3. Agent 在本机执行 `mysqld`、`mysql`、`xtrabackup` 等命令
4. 后端将操作记录和元数据持久化到 SQLite 或 MySQL

---

## 2. 环境准备

### 2.1 系统要求

| 组件 | 要求 |
|------|------|
| Go | 1.25+ (Backend), 1.21+ (Agent) |
| Node.js | 18+ |
| npm | ✓ |
| 操作系统 | Windows (Backend/Console), Linux (Agent/MySQL) |
| 存储 | SQLite 或 MySQL 5.7+/8.0 (平台元数据库) |

### 2.2 被管理主机要求

| 项目 | 要求 |
|------|------|
| OS | Ubuntu 20.04+/CentOS 7+ |
| 内存 | ≥ 4GB |
| 磁盘 | ≥ 20GB |
| 网络 | Agent 端口 (9090) 可从 Backend 访问 |
| Python | 可选，用于部分工具安装 |
| SSH | root 密码或密钥，用于部分自动化操作 |

### 2.3 网络拓扑

```
Backend/Console ── 8080/3000 ──► 开发机 (Windows)
Agent           ── 9090    ──► 被管理主机 (Linux, 如 10.1.81.21/22/32/41)
MySQL 元数据库   ── 3306    ──► 可选，用作元数据存储
```

---

## 3. 部署步骤

### 3.1 快速部署 (Windows)

```powershell
# 1. 克隆仓库
git clone https://github.com/mingjia1/dbops.git
cd dbops

# 2. 配置环境变量
copy .env.example .env
# 编辑 .env，至少设置：
#   DBOPS_JWT_SECRET=你的JWT密钥(≥32字符)
#   DBOPS_AGENT_TOKEN=你的Agent令牌(≥16字符)
#   DBOPS_ENCRYPTION_KEY=你的加密密钥(≥32字符)

# 3. 一键构建并启动 (脚本会自动编译后端/Agent/前端)
.\start.bat

# 4. 验证
Invoke-RestMethod http://localhost:8080/health

# 5. 停止服务
.\stop.bat

# 6. 仅启动已有产物 (跳过重新编译)
.\start.bat -SkipBuild
```

### 3.2 生产部署 (Linux)

```bash
# 1. 在构建机上交叉编译
GOOS=linux GOARCH=amd64 go build -o build/platform-backend-linux-amd64 ./platform-backend/cmd/main.go
GOOS=linux GOARCH=amd64 go build -o build/agent-linux-amd64 ./agent/cmd/main.go

# 2. 推送二进制到目标服务器
scp build/platform-backend-linux-amd64 root@10.1.81.41:/opt/dbops-platform/
scp build/agent-linux-amd64 root@10.1.81.21:/opt/dbops-agent/

# 3. 创建配置文件 (Backend: /opt/dbops-platform/config.yaml)
cat > /opt/dbops-platform/config.yaml << 'EOF'
server:
  port: 8080
storage:
  mode: sqlite  # 或 mysql
  dsn: /opt/dbops-platform/data/platform.db
auth:
  jwt_secret: "your-jwt-secret-key-at-least-32-chars-long"
  agent_token: "your-agent-token-at-least-16-chars"
encryption:
  key: "your-encryption-key-at-least-32-chars"
EOF

# 4. 创建 systemd 服务
cat > /etc/systemd/system/dbops-platform.service << 'EOF'
[Unit]
Description=DBOps Platform Backend
After=network.target

[Service]
Type=simple
ExecStart=/opt/dbops-platform/platform-backend-linux-amd64
WorkingDirectory=/opt/dbops-platform
Environment=DBOPS_DB_URL=root:password@tcp(localhost:3306)/dbops_platform
Environment=DBOPS_JWT_SECRET=your-jwt-secret
Environment=DBOPS_AGENT_TOKEN=dbops-agent-token-16
Environment=DBOPS_ENCRYPTION_KEY=your-encryption-key
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

# 5. 启动
systemctl daemon-reload
systemctl enable --now dbops-platform
```

### 3.3 Agent 部署 (被管理主机)

```bash
# Agent 的 systemd 服务
cat > /etc/systemd/system/dbops-agent.service << 'EOF'
[Unit]
Description=DBOps Agent
After=network.target

[Service]
Type=simple
ExecStart=/opt/dbops-agent/agent-linux-amd64
WorkingDirectory=/opt/dbops-agent
Environment=DBOPS_AGENT_TOKEN=dbops-agent-token-16
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now dbops-agent

# 验证 Agent 运行
curl -s -X POST http://localhost:9090/agent/tasks/health-check \
  -H "Authorization: Bearer dbops-agent-token-16" \
  -H "Content-Type: application/json" -d '{}'
```

### 3.4 Web Console 部署

```bash
# 构建静态文件 (自动编译)
bash bin/start-backend.sh  # 启动后端

# 或用脚本编译前端
cd web-console
npm install
npm run build  # 输出在 web-console/dist/

# 用 nginx 托管
cat > /etc/nginx/sites-available/dbops-web << 'EOF'
server {
    listen 80;
    server_name _;
    root /opt/dbops-web/dist;
    index index.html;
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    location / {
        try_files $uri $uri/ /index.html;
    }
}
EOF

# 开发模式 (通过脚本启动)
bash bin/start-web.sh
```

---

## 4. 启动与停止

### 4.1 一键启停

本项目提供 `bin/` 下便捷脚本统一管理服务启停。

#### Windows

```powershell
# 启动所有服务 (含构建)
.\start.bat

# 跳过构建步骤 (仅启动)
.\start.bat -SkipBuild

# 停止所有服务
.\stop.bat

# 重启所有服务
.\restart.bat
```

#### Linux (开发/调试模式)

```bash
# 启动所有服务 (前台运行，Ctrl+C 停止)
bash bin/start-all.sh

# 停止所有服务
bash bin/stop.sh
```

> 生产环境建议使用 systemd 管理 (参见 §4.3)。

### 4.2 单组件管理

#### Windows

```powershell
# 一键启动 (脚本会自动编译并启动全部服务)
.\start.bat

# 仅启动指定组件 (backend / agent / frontend)
.\start.bat -Component backend
.\start.bat -Component agent
.\start.bat -Component frontend

# 跳过编译，直接启动
.\start.bat -SkipBuild

# 停止所有服务
.\stop.bat

# 重启所有服务
.\restart.bat
```

#### Linux

```bash
# 使用 bin/ 下脚本 (自动编译并启动)
bash bin/start-backend.sh
bash bin/start-agent.sh
bash bin/start-web.sh

# 停止所有服务
bash bin/stop.sh

# 直接调试 (手动)
cd platform-backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd web-console && npm run dev -- --host 0.0.0.0 --port 3000
```

### 4.3 生产部署 (Linux systemd)

```bash
# 启动
systemctl start dbops-platform
systemctl start dbops-agent

# 停止
systemctl stop dbops-platform
systemctl stop dbops-agent

# 查看状态
systemctl status dbops-platform
journalctl -u dbops-platform -f
```

### 4.4 验证服务状态

```powershell
# Backend
Invoke-RestMethod http://localhost:8080/health

# Agent
Invoke-RestMethod http://localhost:9090/agent/tasks/health-check `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"}

# Web Console
Invoke-WebRequest http://localhost:3000 -UseBasicParsing
```

---

## 5. 平台功能

### 5.1 功能总览

| 模块 | 功能 | 说明 |
|------|------|------|
| **主机纳管** | 主机添加/删除/检查 | 管理被监控的 Linux 主机 |
| **实例管理** | MySQL 单实例部署/销毁/版本检测/用户管理/授权 | 生命周期管理 |
| **HA 复制** | 主从复制部署/切换 | 异步复制架构 |
| **MHA** | 高可用集群 | Perl 版 MHA 工具管理 |
| **MGR** | 组复制 (8.0+) | MySQL Group Replication |
| **PXC** | Percona XtraDB Cluster | 同步多主架构 |
| **备份恢复** | xtrabackup / mysqldump | 全量/增量备份和恢复 |
| **升级** | 原地/滚动/逻辑升级 | MySQL 版本升级 |
| **监控** | 指标采集 | QPS/连接数/复制延迟等 |
| **健康检查** | TCP/MySQL/复制 | 实例健康状态 |

### 5.2 主机纳管

通过 Backend API 注册和管理主机:

```powershell
# 注册主机
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/hosts" `
  -Method Post -Headers @{Authorization="Bearer <jwt>"} `
  -ContentType "application/json" `
  -Body '{"host":"10.1.81.21","port":22,"user":"root","password":"hcfc!2017","agent_port":9090}'

# 验证主机连通性 (agent 健康检查)
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/health-check" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"}
```

### 5.3 单实例部署

```powershell
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/deploy" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"deploy-01","config":{"deploy_mode":"single","host":"10.1.81.21","port":3307,"mysql_pass":"Test@123456","mysql_version":"5.7.44"}}'
```

### 5.4 版本检测

```powershell
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/version-detect" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"ver-01","config":{"target_host":"10.1.81.21","target_port":3307,"target_user":"root","target_pass":"Test@123456"}}'
```

### 5.5 HA 主从复制

```powershell
# Step 1: 部署 master
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/deploy" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"ha-master","config":{"deploy_mode":"ha-master","host":"10.1.81.21","port":3307,"mysql_pass":"Test@123456","replicate_user":"repl","replicate_pass":"Repl@123456"}}'

# Step 2: 部署 replica (需显式指定 server_id，否则从 slave 读取现有值)
Invoke-RestMethod -Uri "http://10.1.81.22:9090/agent/tasks/deploy" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"ha-replica","config":{"deploy_mode":"ha-replica","master_host":"10.1.81.21","master_port":3307,"slave_host":"10.1.81.22","slave_port":3307,"mysql_pass":"Test@123456","replicate_user":"repl","replicate_pass":"Repl@123456","server_id":2}}'
```

### 5.6 实例管理 (用户/授权)

```powershell
# 创建用户
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/instance-admin" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"create-user","config":{"action":"create_user","target_host":"10.1.81.21","target_port":3307,"target_user":"root","target_pass":"Test@123456","username":"app_user","password":"App@2024"}}'

# 用户列表
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/instance-admin" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"list-users","config":{"action":"list_users","target_host":"10.1.81.21","target_port":3307,"target_user":"root","target_pass":"Test@123456"}}'

# 授权
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/instance-admin" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"grant","config":{"action":"grant_privileges","target_host":"10.1.81.21","target_port":3307,"target_user":"root","target_pass":"Test@123456","username":"app_user","privileges":"SELECT,INSERT,UPDATE,DELETE","scope":"*.*"}}'
```

### 5.7 备份恢复

```powershell
# 全量备份 (自动选择 mysqldump 或 xtrabackup)
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/backup" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"backup-01","config":{"mysql_host":"10.1.81.21","mysql_port":3307,"mysql_user":"root","mysql_pass":"Test@123456"}}'

# 恢复 (自动检测备份类型: .sql→mysqldump, 含 xtrabackup_checkpoints→xtrabackup)
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/restore" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"restore-01","config":{"mysql_host":"10.1.81.21","mysql_port":3307,"backup_path":"/backup/mysql/full-xxx.sql"}}'
```

### 5.8 实例下线

```powershell
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/decommission" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"decom-01","config":{"target_host":"10.1.81.21","target_port":3307,"mysql_data_dir":"/data/mysql/3307"}}'
```

---

## 6. Agent API 参考

### 6.1 基础信息

- **Base URL**: `http://<host>:9090/agent/tasks/`
- **认证**: `Authorization: Bearer <token>`
- **Content-Type**: `application/json`
- **响应格式**:
  ```json
  {"code": 200, "message": "success", "data": {"task_id": "...", "status": "completed", ...}}
  ```

### 6.2 端点列表

| 端点 | 方法 | 功能 | 关键配置字段 |
|------|------|------|-------------|
| `/deploy` | POST | 部署 MySQL (单实例/HA/MHA/MGR/PXC) | `deploy_mode`, `host`, `port`, `mysql_pass` |
| `/backup` | POST | 执行备份 | `mysql_host`, `mysql_port`, `mysql_user`, `mysql_pass` |
| `/restore` | POST | 恢复备份 | `backup_path`, `mysql_host`, `mysql_port` |
| `/version-detect` | POST | 检测 MySQL 版本 | `target_host`, `target_port`, `target_user`, `target_pass` |
| `/health-check` | POST | Agent/实例健康检查 | `instance_id` (query), `config` 可选 |
| `/instance-admin` | POST | 实例管理操作 | `action`, `target_host`, `target_port` |
| `/decommission` | POST | 下线实例 | `target_host`, `target_port`, `mysql_data_dir` |
| `/upgrade` | POST | MySQL 升级 | `upgrade_type`, `target_version`, `current_version` |
| `/cluster-switch` | POST | 集群切换/故障转移 | `switch_type`, `cluster_type`, `nodes` |
| `/check-environment` | POST | 检查环境 | `target_host`, `target_user`, `target_pass` |
| `/install-tools` | POST | 安装工具 | `target_host`, `tools` |
| `/blank-host-init` | POST | 空白主机初始化 | `host`, `mysql_version`, `root_password` |
| `/general-cluster-init` | POST | 通用集群初始化 | 同上 + `cluster_type` |
| `/relay/fetch` | POST | 中继下载文件 | `url`, `name` |
| `/relay/packages` | GET | 列出缓存包 | - |
| `/relay/status` | GET | 中继状态 | - |
| `/metrics` | POST | MySQL 指标 | `instance_id` (query) |

### 6.3 deploy_mode 取值

| 值 | 说明 | 额外要求 |
|------|------|---------|
| `single` | 单实例部署 | `host`, `port` |
| `ha-master` | HA 主库 | 同上 + `replicate_user` |
| `ha-replica` | HA 从库 | `master_host`, `master_port`, `slave_host`, `slave_port`, `replicate_user`, `replicate_pass`, `server_id` |
| `mha` | MHA 集群 | `manager_host`, `slave_hosts`, `ssh_passwords` |
| `mgr-single-primary` | MGR (需 8.0+) | `group_name`, `local_address` |
| `pxc` | PXC 集群 | `cluster_name`, `nodes` |
| `blank-host-init` | 空白主机初始化 | 本地执行，非远程 |

### 6.4 参数命名说明

部分模块使用不同的参数名表示相同的含义:

| 含义 | deploy 模块 | backup/restore | version-detect | 说明 |
|------|-----------|---------------|----------------|------|
| 主机 | `host` / `target_host` | `mysql_host` | `target_host` | 正在统一中 |
| 端口 | `port` / `target_port` | `mysql_port` | `target_port` | 同上 |
| 密码 | `mysql_pass` | `mysql_pass` | `target_pass` | MHA/MGR/PXC 也接受 `mysql_password` |

---

## 7. 集群部署指南

### 7.1 HA 主从架构

```
10.1.81.21:3307 (master)  ←── 异步复制 ──→  10.1.81.22:3307 (slave)
                                                        10.1.81.32:3307 (slave)
```

**部署步骤:**
1. 在 master 主机上部署单实例
2. 配置 master（创建复制用户）
3. 在 slave 主机上部署单实例（server_id 不可与 master 相同）
4. 配置 slave（CHANGE MASTER TO + START SLAVE）

**注意:** server_id 必须唯一。slave 的 server_id 默认为 slave_port，也可通过配置显式指定。

### 7.2 MHA (Master High Availability)

```
Manager (10.1.81.32)
    │
    ├── Master (10.1.81.21:3307)
    └── Slave  (10.1.81.22:3307)
```

**前置条件:**
- 所有节点之间 SSH 免密登录
- Perl 环境 (自动通过 `mha4mysql-node` 和 `mha4mysql-manager` 安装)
- 已建立主从复制关系

### 7.3 MGR (MySQL Group Replication)

**要求:** MySQL 8.0+ (5.7 不支持 Group Replication)

```
Primary (10.1.81.21:3306)
    │
    ├── Secondary (10.1.81.22:3306)
    └── Secondary (10.1.81.32:3306)
```

### 7.4 PXC (Percona XtraDB Cluster)

```
Node 1 (10.1.81.21:3306)  ←── 同步复制 ──→  Node 2 (10.1.81.22:3306)
      ↑                                             ↑
      └──────────────── Node 3 (10.1.81.32:3306) ───┘
```

---

## 8. 故障排查

### 8.1 常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| Agent 连接失败 | Agent 未运行或端口不可达 | `systemctl status dbops-agent`，检查 9090 端口 |
| health-check 返回 404 | Backend 用 GET 调用 Agent POST 路由 | 已修复: AgentClient 改为 POST (commit df682a6) |
| 主从复制 IO 线程报 1593 | server_id 冲突 | 确保 master/slave 的 server_id 不同 |
| 备份失败 "Can't connect to socket" | 使用 mysql CLI 默认连接 socket | 改用 `mysql_host` + `mysql_port` 参数指定 TCP 连接 |
| 恢复失败 | 备份类型检测失败 | 显式指定 `backup_type` 为 `mysqldump` 或 `xtrabackup` |
| MGR 部署失败 | MySQL 版本 < 8.0 | Group Replication 需要 8.0+ |
| restore 一直失败 | xtrabackup 备份用错恢复命令 | 确保 backup_type 正确，SQL 备份用 `mysqldump` |

### 8.2 Agent 日志

```bash
# Agent 日志 (默认 stdout)
journalctl -u dbops-agent -f

# Backend 日志
journalctl -u dbops-platform -f
```

### 8.3 调试命令

```powershell
# 直接调 Agent API (绕过 Backend)
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/health-check" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"}

# 检查 Agent 路由
Invoke-RestMethod -Uri "http://10.1.81.21:9090/agent/tasks/version-detect" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -Body '{"task_id":"debug","config":{"target_host":"10.1.81.21","target_port":3307,"target_user":"root","target_pass":"Test@123456"}}'

# 检查 MySQL 连通性
mysql -h 10.1.81.21 -P 3307 -u root -pTest@123456 -e "SELECT @@version, @@server_id"
```

### 8.4 已知限制

1. Agent 长任务无进度持久化（不可查询历史任务状态）
2. 无 ClickHouse 时监控面板不可用
3. 无 Redis 时部分缓存和队列功能不可用
4. MGR 不兼容 MySQL 5.7
5. restore 功能仅支持 xtrabackup 和 mysqldump 格式
6. upgrade 在 5.7→5.7 同级升级会失败（需要跨版本验证）

---

## 附录 A: 配置文件参考

### 启停脚本指南

```powershell
# Windows (根目录或 bin/ 下均可执行)
.\start.bat                    # 构建并启动全部服务
.\start.bat -SkipBuild         # 仅启动已有产物
.\start.bat -Component backend # 仅启动后端
.\stop.bat                     # 停止全部服务
.\restart.bat                  # 重启全部服务

# Linux
bash bin/start-backend.sh       # 启动后端
bash bin/start-agent.sh         # 启动 Agent
bash bin/start-web.sh           # 启动前端
bash bin/start-all.sh           # 启动全部
bash bin/stop.sh                # 停止全部
```

### .env 文件

```env
DBOPS_DB_URL=root:password@tcp(10.1.81.41:3306)/dbops_platform
DBOPS_JWT_SECRET=your-jwt-secret-key-at-least-32-characters-long
DBOPS_AGENT_TOKEN=dbops-agent-token-16
DBOPS_ENCRYPTION_KEY=your-encryption-key-at-least-32-characters-long
```

### Backend config.yaml

```yaml
server:
  port: 8080
storage:
  mode: sqlite       # 或 mysql
  dsn: /data/dbops-platform/platform.db
auth:
  jwt_secret: "${DBOPS_JWT_SECRET}"
  agent_token: "${DBOPS_AGENT_TOKEN}"
encryption:
  key: "${DBOPS_ENCRYPTION_KEY}"
redis:
  addr: "localhost:6379"
clickhouse:
  addr: "localhost:8123"
```

### Agent config.yaml

```yaml
server:
  port: 9090
platform:
  url: "http://10.1.81.41:8080"
auth:
  token: "${DBOPS_AGENT_TOKEN}"
```

---

## 附录 B: 测试环境 (当前)

| 主机 | IP | 角色 | MySQL 端口 | Agent 端口 |
|------|-----|------|-----------|-----------|
| tvy-dbtest-05 | 10.1.81.21 | Master / Agent | 3307 | 9090 |
| tvy-dbtest-06 | 10.1.81.22 | Slave / Agent | 3307 | 9090 |
| tvy-dbtest-07 | 10.1.81.32 | Slave / Agent | 3307 | 9090 |
| tvy-dbtest-08 | 10.1.81.41 | Platform DB | 3306 | - |

**统一密码:** `hcfc!2017` (SSH), `Test@123456` (MySQL root)  
**Agent Token:** `dbops-agent-token-16`
