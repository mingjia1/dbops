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
| **backend** | 8080 | Go 1.25+ | REST API 服务，管理主机、实例、用户、部署 |
| **frontend** | 3000 | React 18 | 网页管理界面 |
| **agent** | 9090 | Go 1.25+ | 部署在被管理主机上，通过 HTTP API 执行 MySQL 操作 |

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
| Go | 1.25+ (backend/agent 共用，见各自 go.mod) |
| Node.js | 18+ |
| npm | ✓ |
| 操作系统 | Windows (开发/Console 推荐), Linux (Backend/Agent/MySQL 生产环境) |
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
Backend/Console ── 8080/3000 ──► Windows 开发机 (仅开发/Console)
Agent           ── 9090    ──► 被管理主机 (Linux, 如 192.0.2.21/22/32/41)
MySQL 元数据库   ── 3306    ──► 可选，用作元数据存储
```

> **注意**：生产环境 Backend 必须部署在 Linux 上（systemd/docker）。Windows 仅用于开发或运行 Web Console 前端。

---

## 3. 部署步骤

### 3.1 快速部署 (Windows - 仅开发环境)

> **生产环境请使用 Linux 部署（参见 3.2 和 3.3）**

```powershell
# 1. 克隆仓库
git clone https://github.com/mingjia1/dbops.git
cd dbops

# 2. 配置环境变量
copy .env.example .env
# 编辑 .env，至少设置：
#   DBOPS_JWT_SECRET=replace-with-strong-random-jwt-secret-at-least-32-chars
#   DBOPS_AGENT_TOKEN=replace-with-strong-random-agent-token
#   DBOPS_ENCRYPTION_KEY=replace-with-32-byte-random-encryption-key

# 3. 一键构建并启动 (脚本会自动编译后端/Agent/前端)
.\bin\windows\start.bat

# 4. 验证
Invoke-RestMethod http://localhost:8080/health

# 5. 停止服务
.\bin\windows\stop.bat

# 6. 仅启动已有产物 (跳过重新编译)
.\bin\windows\start.bat -SkipBuild
```

### 3.2 生产部署 (Linux)

```bash
# 1. 在构建机上交叉编译
GOOS=linux GOARCH=amd64 go build -o build/platform-linux-amd64 ./backend/cmd/main.go
GOOS=linux GOARCH=amd64 go build -o build/agent-linux-amd64 ./agent/cmd/main.go

# 2. 推送二进制到目标服务器
scp build/platform-linux-amd64 root@192.0.2.41:/opt/dbops-platform/
scp build/agent-linux-amd64 root@192.0.2.21:/opt/dbops-agent/

# 3. 创建配置文件 (Backend: /opt/dbops-platform/config.yaml)
#    注意：后端配置是扁平键名（见 backend/config.yaml.example），不是嵌套 yaml。
cat > /opt/dbops-platform/config.yaml << 'EOF'
server_port: "8080"
storage_mode: "auto"   # auto | mysql | sqlite
data_dir: "/opt/dbops-platform/data"
database_url: "dbops_user:replace-with-strong-password@tcp(localhost:3306)/dbops_platform?parseTime=true&loc=Local"
jwt_secret: "your-jwt-secret-key-at-least-32-chars-long"
encryption_key: "your-encryption-key-at-least-32-chars"
agent_token: "your-agent-token-at-least-16-chars"
EOF

# 4. 创建 systemd 服务
cat > /etc/systemd/system/dbops-platform.service << 'EOF'
[Unit]
Description=DBOps Platform Backend
After=network.target

[Service]
Type=simple
ExecStart=/opt/dbops-platform/dbops-backend
WorkingDirectory=/opt/dbops-platform
Environment=DBOPS_DB_URL=dbops_user:replace-with-strong-password@tcp(localhost:3306)/dbops_platform
Environment=DBOPS_JWT_SECRET=replace-with-strong-random-jwt-secret-at-least-32-chars
Environment=DBOPS_AGENT_TOKEN=replace-with-strong-random-agent-token
Environment=DBOPS_ENCRYPTION_KEY=replace-with-32-byte-random-encryption-key
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
Environment=DBOPS_AGENT_TOKEN=replace-with-strong-random-agent-token
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

### 3.3.1 Windows 主机纳管

平台支持将 Windows 主机纳管为被管理主机，但 Windows 上的自动化能力有限：
- **SSH 进程发现**：`discoverByProcess` 仅支持 Linux `ps -eo pid,user,rss,args`，Windows 上不会自动发现 mysqld 进程。
- **端口探测**：TCP 端口探测（`probePort`）在 Windows 上仍然可用。
- **Agent 任务**：Agent 二进制 `agent-windows-amd64.exe` 已包含在平台构建产物中，但需手动部署。

**前提条件**
1. Windows 主机开启 **WinRM**（PowerShell 远程管理），便于平台通过 HTTP API 调用 Agent。
2. 防火墙开放 Agent 端口（默认 `9090`）和 MySQL 端口。
3. 将 `agent-windows-amd64.exe` 放到 Windows 主机上（可从 `agent/bin/` 或 `make dist` 产物中获取）。

**部署步骤**

```powershell
# 1. 将 agent-windows-amd64.exe 放到 C:\dbops-agent\
New-Item -ItemType Directory -Path C:\dbops-agent -Force
Copy-Item .\agent\bin\agent-windows-amd64.exe C:\dbops-agent\agent.exe

# 2. 创建配置文件
@"
agent_token: replace-with-strong-random-agent-token
backend_url: http://<platform-backend>:8080
"@ | Out-File -FilePath C:\dbops-agent\config.yaml -Encoding utf8

# 3. 安装为 Windows 服务（使用 NSSM 或 sc.exe）
# 方式 A: NSSM（推荐）
nssm install DBOpsAgent "C:\dbops-agent\agent.exe"
nssm set DBOpsAgent ApplicationDirectory "C:\dbops-agent"
nssm set DBOpsAgent Start SERVICE_AUTO_START
nssm start DBOpsAgent

# 方式 B: sc.exe
sc.exe create DBOpsAgent binPath= "C:\dbops-agent\agent.exe" start= auto
sc.exe start DBOpsAgent

# 4. 验证 Agent 运行
Invoke-RestMethod -Uri http://localhost:9090/health -Method Get
```

**在平台中添加 Windows 主机**
1. 进入「主机管理」→「添加主机」。
2. 填写主机地址、SSH 端口（如 `22` 或 WinRM 端口 `5985`）、SSH 用户。
3. 在「Agent 管理」中执行「安装 Agent」或「更新 Agent」时，选择 Windows 主机。
4. 由于 Windows 上进程发现不可用，建议在添加主机后手动执行「检查状态」以确认 Agent 可达。

**已知限制**
- Windows 主机上的实例扫描不会通过 `ps` 发现进程，只能依赖 TCP 端口探测。
- 达梦、GBase 等非 MySQL 协议引擎的端口推断在 Windows 上未经验证。
- 集群部署、高可用等 Linux 专属操作不适用于 Windows 主机。

### 3.4 Web Console 部署

```bash
# 构建静态文件 (自动编译)
bash bin/ubuntu/start-backend.sh  # 启动后端

# 或用脚本编译前端
cd frontend
npm install
npm run build  # 输出在 frontend/dist/

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
bash bin/ubuntu/start-web.sh
```

---

## 4. 启动与停止

### 4.1 一键启停

本项目提供 `bin/` 下便捷脚本统一管理服务启停。

#### Windows

```powershell
# 启动所有服务 (含构建)
.\bin\windows\start.bat

# 跳过构建步骤 (仅启动)
.\bin\windows\start.bat -SkipBuild

# 停止所有服务
.\bin\windows\stop.bat

# 重启所有服务
.\bin\windows\restart.bat
```

#### Linux (开发/调试模式)

```bash
# 启动所有服务 (前台运行，Ctrl+C 停止)
bash bin/ubuntu/start-all.sh

# 停止所有服务
bash bin/ubuntu/stop.sh
```

> 生产环境建议使用 systemd 管理 (参见 §4.3)。

### 4.2 单组件管理

#### Windows

```powershell
# 一键启动 (脚本会自动编译并启动全部服务)
.\bin\windows\start.bat

# 仅启动指定组件 (backend / agent / frontend)
.\bin\windows\start.bat -Component backend
.\bin\windows\start.bat -Component agent
.\bin\windows\start.bat -Component frontend

# 跳过编译，直接启动
.\bin\windows\start.bat -SkipBuild

# 停止所有服务
.\bin\windows\stop.bat

# 重启所有服务
.\bin\windows\restart.bat
```

#### Linux

```bash
# 使用 bin/ 下脚本 (自动编译并启动)
bash bin/ubuntu/start-backend.sh
bash bin/ubuntu/start-agent.sh
bash bin/ubuntu/start-web.sh

# 停止所有服务
bash bin/ubuntu/stop.sh

# 直接调试 (手动)
cd backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd frontend && npm run dev -- --host 0.0.0.0 --port 3000
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
  -Body '{"host":"192.0.2.21","port":22,"user":"root","password":"example_password","agent_port":9090}'

# 验证主机连通性 (agent 健康检查)
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/health-check" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"}
```

### 5.3 单实例部署

```powershell
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/deploy" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"deploy-01","config":{"deploy_mode":"single","host":"192.0.2.21","port":3307,"mysql_pass":"example_password","mysql_version":"5.7.44"}}'
```

### 5.4 版本检测

```powershell
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/version-detect" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"ver-01","config":{"target_host":"192.0.2.21","target_port":3307,"target_user":"root","target_pass":"example_password"}}'
```

### 5.5 HA 主从复制

```powershell
# Step 1: 部署 master
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/deploy" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"ha-master","config":{"deploy_mode":"ha-master","host":"192.0.2.21","port":3307,"mysql_pass":"example_password","replicate_user":"repl","replicate_pass":"example_password"}}'

# Step 2: 部署 replica (需显式指定 server_id，否则从 slave 读取现有值)
Invoke-RestMethod -Uri "http://192.0.2.22:9090/agent/tasks/deploy" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"ha-replica","config":{"deploy_mode":"ha-replica","master_host":"192.0.2.21","master_port":3307,"slave_host":"192.0.2.22","slave_port":3307,"mysql_pass":"example_password","replicate_user":"repl","replicate_pass":"example_password","server_id":2}}'
```

### 5.6 实例管理 (用户/授权)

```powershell
# 创建用户
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/instance-admin" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"create-user","config":{"action":"create_user","target_host":"192.0.2.21","target_port":3307,"target_user":"root","target_pass":"example_password","username":"app_user","password":"example_password"}}'

# 用户列表
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/instance-admin" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"list-users","config":{"action":"list_users","target_host":"192.0.2.21","target_port":3307,"target_user":"root","target_pass":"example_password"}}'

# 授权
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/instance-admin" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"grant","config":{"action":"grant_privileges","target_host":"192.0.2.21","target_port":3307,"target_user":"root","target_pass":"example_password","username":"app_user","privileges":"SELECT,INSERT,UPDATE,DELETE","scope":"*.*"}}'
```

### 5.7 备份恢复

```powershell
# 全量备份 (自动选择 mysqldump 或 xtrabackup)
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/backup" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"backup-01","config":{"mysql_host":"192.0.2.21","mysql_port":3307,"mysql_user":"root","mysql_pass":"example_password"}}'

# 恢复 (自动检测备份类型: .sql→mysqldump, 含 xtrabackup_checkpoints→xtrabackup)
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/restore" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"restore-01","config":{"mysql_host":"192.0.2.21","mysql_port":3307,"backup_path":"/backup/mysql/full-xxx.sql"}}'
```

### 5.8 实例下线

```powershell
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/decommission" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -ContentType "application/json" `
  -Body '{"task_id":"decom-01","config":{"target_host":"192.0.2.21","target_port":3307,"mysql_data_dir":"/data/mysql/3307"}}'
```

### 5.9 平台备份与恢复

平台自身的元数据（hosts、instances、users、audit logs、upgrade history）存储在 `data_dir` 或 MySQL 中，需要定期备份。

#### 5.9.1 SQLite 模式

```bash
# 停止 backend 服务以确保数据一致性
systemctl stop dbops-platform

# 备份 SQLite 数据库和 JSON 数据
tar -czf /backup/dbops-platform-$(date +%Y%m%d).tar.gz \
  /opt/dbops-platform/data/

# 验证备份
tar -tzf /backup/dbops-platform-$(date +%Y%m%d).tar.gz
```

恢复：

```bash
systemctl stop dbops-platform
tar -xzf /backup/dbops-platform-$(date +%Y%m%d).tar.gz -C /opt/dbops-platform/
systemctl start dbops-platform
```

#### 5.9.2 MySQL 模式

```bash
# 使用 mysqldump 备份平台元数据库
mysqldump -h localhost -u dbops_user -p \
  --single-transaction \
  --routines --triggers \
  dbops_platform > /backup/dbops-platform-$(date +%Y%m%d).sql

# 验证备份
head -20 /backup/dbops-platform-$(date +%Y%m%d).sql
```

恢复：

```bash
systemctl stop dbops-platform
mysql -h localhost -u dbops_user -p dbops_platform < /backup/dbops-platform-$(date +%Y%m%d).sql
systemctl start dbops-platform
```

#### 5.9.3 备份检查清单

- [ ] `data/dbops.db`（SQLite）或 MySQL `dbops_platform` 数据库已备份
- [ ] `data/*.json`（如有遗留 JSON 数据）已备份
- [ ] `config.yaml` 和 `.env` 已备份（不含真实密钥时可只备份结构）
- [ ] 备份文件已上传到异地存储
- [ ] 恢复流程已在测试环境验证

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
192.0.2.21:3307 (master)  ←── 异步复制 ──→  192.0.2.22:3307 (slave)
                                                        192.0.2.32:3307 (slave)
```

**部署步骤:**
1. 在 master 主机上部署单实例
2. 配置 master（创建复制用户）
3. 在 slave 主机上部署单实例（server_id 不可与 master 相同）
4. 配置 slave（CHANGE MASTER TO + START SLAVE）

**注意:** server_id 必须唯一。slave 的 server_id 默认为 slave_port，也可通过配置显式指定。

### 7.2 MHA (Master High Availability)

```
Manager (192.0.2.32)
    │
    ├── Master (192.0.2.21:3307)
    └── Slave  (192.0.2.22:3307)
```

**前置条件:**
- 所有节点之间 SSH 免密登录
- Perl 环境 (自动通过 `mha4mysql-node` 和 `mha4mysql-manager` 安装)
- 已建立主从复制关系

### 7.3 MGR (MySQL Group Replication)

**要求:** MySQL 8.0+ (5.7 不支持 Group Replication)

```
Primary (192.0.2.21:3306)
    │
    ├── Secondary (192.0.2.22:3306)
    └── Secondary (192.0.2.32:3306)
```

### 7.4 PXC (Percona XtraDB Cluster)

```
Node 1 (192.0.2.21:3306)  ←── 同步复制 ──→  Node 2 (192.0.2.22:3306)
      ↑                                             ↑
      └──────────────── Node 3 (192.0.2.32:3306) ───┘
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
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/health-check" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"}

# 检查 Agent 路由
Invoke-RestMethod -Uri "http://192.0.2.21:9090/agent/tasks/version-detect" `
  -Method Post -Headers @{Authorization="Bearer dbops-agent-token-16"} `
  -Body '{"task_id":"debug","config":{"target_host":"192.0.2.21","target_port":3307,"target_user":"root","target_pass":"example_password"}}'

# 检查 MySQL 连通性
mysql -h 192.0.2.21 -P 3307 -u root -pexample_password -e "SELECT @@version, @@server_id"
```

### 8.4 已知限制

1. Agent 长任务无进度持久化（不可查询历史任务状态）
2. 无 ClickHouse 时监控面板不可用
3. 无 Redis 时部分缓存和队列功能不可用
4. MGR 不兼容 MySQL 5.7
5. restore 功能仅支持 xtrabackup 和 mysqldump 格式
6. upgrade 在 5.7→5.7 同级升级会失败（需要跨版本验证）

---

## 9. 灾难恢复

### 9.1 恢复目标

- RPO（恢复点目标）≤ 24 小时（每日备份）
- RTO（恢复时间目标）≤ 2 小时（含二进制 redeploy + 元数据恢复）

### 9.2 恢复步骤

#### Step 1: 准备新主机

```bash
# 1. 安装基础依赖
apt-get update && apt-get install -y curl mysql-client

# 2. 创建目录
mkdir -p /opt/dbops-platform/{bin,config,data}
mkdir -p /opt/dbops-agent

# 3. 部署二进制（从离线包或 CI 产物）
tar -xzf dbops-offline-20261001.tar.gz -C /opt/dbops-platform/
cp /opt/dbops-platform/dist/bin/dbops-backend /opt/dbops-platform/bin/
cp /opt/dbops-platform/dist/bin/dbops-agent /opt/dbops-agent/
```

#### Step 2: 恢复平台元数据

```bash
# SQLite 模式
tar -xzf /backup/dbops-platform-20261001.tar.gz -C /opt/dbops-platform/

# 或 MySQL 模式
mysql -h <db-host> -u dbops_user -p dbops_platform < /backup/dbops-platform-20261001.sql
```

#### Step 3: 恢复配置

```bash
# 恢复 config.yaml 和 .env（注意替换密钥）
cp config.yaml /opt/dbops-platform/config/
cp .env /opt/dbops-platform/.env

# 验证关键配置
grep -E 'DBOPS_JWT_SECRET|DBOPS_ENCRYPTION_KEY|DBOPS_AGENT_TOKEN' /opt/dbops-platform/.env
```

#### Step 4: 启动并验证

```bash
# 启动 platform
systemctl daemon-reload
systemctl enable --now dbops-platform

# 验证健康
curl -s http://localhost:8080/health/ready
curl -s http://localhost:8080/metrics | head -5

# 验证前端可访问
curl -s http://localhost/ | grep "dbops"
```

#### Step 5: 验证 Agent 连通性

```bash
# 对每个被管理主机执行
curl -s -X POST http://<host>:9090/agent/tasks/health-check \
  -H "Authorization: Bearer <agent-token>" \
  -H "Content-Type: application/json" -d '{}'

# 在平台 UI 中进入「主机管理」→ 选择主机 →「检查状态」
```

#### Step 6: 验证数据完整性

```bash
# 检查 hosts/instances 数量是否与备份前一致
curl -s -H "Authorization: Bearer <jwt>" http://localhost:8080/api/v1/hosts | jq '.data | length'
curl -s -H "Authorization: Bearer <jwt>" http://localhost:8080/api/v1/instances | jq '.data | length'
```

### 9.3 回滚预案

如果新版本启动失败：

```bash
# 1. 停止新版本
systemctl stop dbops-platform

# 2. 恢复旧二进制
cp /opt/dbops-platform/bin/dbops-backend.bak /opt/dbops-platform/bin/dbops-backend

# 3. 如需回滚 DB 迁移，从备份恢复
# SQLite: 直接覆盖 dbops.db
# MySQL: 从 mysqldump 恢复

# 4. 重启
systemctl start dbops-platform

# 5. 验证
curl -s http://localhost:8080/health/ready
```

### 9.4 恢复检查清单

- [ ] 新主机 OS 版本与之前一致（或兼容）
- [ ] 平台二进制已部署且版本正确
- [ ] 配置文件已恢复且密钥有效
- [ ] 元数据 DB 已恢复且可连接
- [ ] `/health/ready` 返回 200
- [ ] 所有 Agent 健康检查通过
- [ ] 前端可访问且数据完整
- [ ] 审计日志时间范围连续（无断层）

---

## 10. 监控与可观测性

### 10.1 Prometheus 指标

平台在 `/metrics` 暴露 Prometheus 文本格式指标，包含：

- `dbops_process_uptime_seconds` — 进程运行时间
- `dbops_build_info{version="..."}` — 构建版本
- `dbops_goroutines` — 当前 goroutine 数
- `dbops_memory_alloc_bytes` — 堆已分配字节
- `dbops_memory_sys_bytes` — 系统获取内存字节
- `dbops_gc_runs_total` — GC 次数
- `dbops_threads` — OS 线程数
- `dbops_db_open_connections` — 数据库打开连接数
- `dbops_db_in_use_connections` — 正在使用连接数
- `dbops_db_idle_connections` — 空闲连接数
- `dbops_db_max_open_connections` — 最大打开连接数
- `dbops_db_wait_count_total` — 等待连接总次数
- `dbops_db_wait_duration_seconds_total` — 等待连接总耗时

### 10.2 Prometheus 配置示例

```yaml
scrape_configs:
  - job_name: 'dbops-platform'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
    scrape_timeout: 10s
```

### 10.3 关键告警规则

```yaml
groups:
  - name: dbops-platform
    rules:
      - alert: DBOpsDown
        expr: up{job="dbops-platform"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "DBOps platform is down"
          description: "Platform backend has been down for more than 1 minute."

      - alert: DBOpsHighDBConnections
        expr: dbops_db_open_connections / dbops_db_max_open_connections > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "DBOps DB connection pool nearing exhaustion"
          description: "DB connection utilization is above 80% for 5 minutes."

      - alert: DBOpsAgentUnreachable
        expr: dbops_agent_up == 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "DBOps agent unreachable"
          description: "Agent on {{ $labels.host }} has not reported for 2 minutes."
```

### 10.4 Grafana 快速开始

1. 添加 Prometheus 数据源指向 `http://<prometheus>:9090`
2. 导入 Dashboard ID `1860`（Prometheus 官方仪表盘）作为基础
3. 自定义添加以下面板：
   - 平台 uptime：`dbops_process_uptime_seconds`
   - Goroutine 数量：`dbops_goroutines`
   - DB 连接池使用率：`dbops_db_open_connections / dbops_db_max_open_connections * 100`
   - 内存趋势：`dbops_memory_alloc_bytes` / `dbops_memory_sys_bytes`

### 10.5 可选依赖降级行为

平台在缺少可选依赖时仍可运行，但部分功能会降级：

| 依赖 | 缺失影响 | 降级行为 |
|------|---------|---------|
| **Redis** | 缓存和队列功能不可用 | 任务进度轮询退化为前端短轮询；部分缓存热点（如版本目录）每次请求都查数据库 |
| **ClickHouse** | 监控历史不可用 | `/monitoring/metrics` 返回当前瞬时值，无历史趋势；Grafana 监控面板无数据 |
| **MySQL (平台库)** | 元数据库不可用 | 自动回退 SQLite；重启后仍可用，但多进程并发写入性能受限 |
| **AI Provider** | AI 诊断不可用 | 智能诊断页面显示“未配置 AI 提供商”；不影响其他功能 |

启动时日志会明确提示每个依赖的可用状态，运维人员可通过日志确认当前运行模式。

---

## 附录 A: 配置文件参考

### 启停脚本指南

```powershell
# Windows (根目录或 bin/ 下均可执行)
.\bin\windows\start.bat                    # 构建并启动全部服务
.\bin\windows\start.bat -SkipBuild         # 仅启动已有产物
.\bin\windows\start.bat -Component backend # 仅启动后端
.\bin\windows\stop.bat                     # 停止全部服务
.\bin\windows\restart.bat                  # 重启全部服务

# Linux
bash bin/ubuntu/start-backend.sh       # 启动后端
bash bin/ubuntu/start-agent.sh         # 启动 Agent
bash bin/ubuntu/start-web.sh           # 启动前端
bash bin/ubuntu/start-all.sh           # 启动全部
bash bin/ubuntu/stop.sh                # 停止全部
```

### .env 文件

```env
DBOPS_DB_URL=dbops_user:replace-with-strong-password@tcp(192.0.2.41:3306)/dbops_platform
DBOPS_JWT_SECRET=replace-with-strong-random-jwt-secret-at-least-32-chars
DBOPS_AGENT_TOKEN=replace-with-strong-random-agent-token
DBOPS_ENCRYPTION_KEY=replace-with-32-byte-random-encryption-key
```

### Backend config.yaml

```yaml
# Backend 使用扁平键名（viper flat keys），不是嵌套 yaml。
# 只有 cluster_defaults 允许嵌套。
server_port: "8080"
log_level: "info"
storage_mode: "auto"   # auto | mysql | sqlite
sqlite_path: ""
data_dir: "./data"
database_url: "root:password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local"
jwt_secret: "YOUR_32_CHAR_JWT_SECRET_HERE_CHANGE_ME_NOW"
encryption_key: "YOUR_32_CHAR_ENCRYPTION_KEY_HERE_CHANGE_ME"
agent_token: "YOUR_16_CHAR_AGENT_TOKEN_HERE"
redis_url: "localhost:6379"
redis_password: ""
redis_db: 0
clickhouse_url: "clickhouse://default@localhost:9000/default"
allowed_origins: "http://localhost:3000,http://127.0.0.1:3000"
ai_base_url: "https://api.openai.com"
ai_api_key: ""
ai_model: "gpt-4"
ai_max_tokens: 2048
cluster_defaults:
  replication_user: "repl"
  replication_pass: "Repl#2024!ChangeMe"
  sst_user: "sstuser"
  sst_pass: "Sst#2024!ChangeMe"
  ssh_user: "root"
```

### Agent config.yaml

```yaml
# Agent 配置也使用扁平键名；relay 相关配置允许嵌套。
agent_port: "9090"
platform_url: "http://192.0.2.41:8080"
agent_token: "YOUR_16_CHAR_AGENT_TOKEN_HERE"
log_level: "warning"
relay:
  enabled: false
  cache_dir: "/data/relay/packages"
  relay_port: 9091
  max_cache_size_gb: 50
  cache_expire_hours: 168
```

---

## 附录 B: 测试环境 (当前)

| 主机 | IP | 角色 | MySQL 端口 | Agent 端口 |
|------|-----|------|-----------|-----------|
| tvy-dbtest-05 | 192.0.2.21 | Master / Agent | 3307 | 9090 |
| tvy-dbtest-06 | 192.0.2.22 | Slave / Agent | 3307 | 9090 |
| tvy-dbtest-07 | 192.0.2.32 | Slave / Agent | 3307 | 9090 |
| tvy-dbtest-08 | 192.0.2.41 | Platform DB | 3306 | - |

**统一密码:** `example_password` (SSH), `example_password` (MySQL root)  
**Agent Token:** `dbops-agent-token-16`

---

## 附录 C: 从旧文档迁移说明

本文档早期版本使用 `platform-backend/`、`web-console/` 等目录名，实际仓库目录为 `backend/`、`frontend/`。一键启停脚本位于 `bin/windows/`、`bin/ubuntu/`、`bin/centos/`，不在仓库根目录。Agent 与 Backend 均要求 Go 1.25+（见各自 `go.mod`）。

后端配置为**扁平键名**（见 `backend/config.yaml.example`），不是嵌套 yaml；`server:`、`storage:`、`auth:`、`encryption:` 分段写法不生效。

---

## 附录 D: 日志轮转

### systemd journald 配置

创建 `/etc/systemd/journald.conf.d/dbops.conf`：

```ini
[Journal]
SystemMaxUse=500M
SystemMaxFileSize=50M
MaxRetentionSec=7day
```

### logrotate 配置（非 systemd 环境）

创建 `/etc/logrotate.d/dbops-platform`：

```bash
/var/log/dbops/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    maxsize 100M
}
```

### 后端日志级别

生产环境建议配置：

```env
DBOPS_LOG_LEVEL=info
```

调试时可临时改为 `debug`，但生产环境长期运行建议使用 `info` 或 `warning` 以避免日志量过大。

---

## 附录 E: 容量规划参考

以下数据基于单实例部署（2vCPU / 4GB RAM / SSD）：

| 指标 | 建议上限 | 备注 |
|------|---------|------|
| 被管理主机数 | 50 | 超过 50 建议拆分 backend 或启用 Redis 缓存 |
| 每后端实例数 | 200 | 含历史任务记录 |
| 并发长任务数 | 10 | 受 Agent 连接数和后端 goroutine 限制 |
| 监控指标保留 | 30 天 | ClickHouse 单节点，建议分区按月 |
| Redis 内存 | 4GB | 用于任务状态缓存和队列 |
| ClickHouse 磁盘 | 100GB+ | 按每秒 100 条指标估算 |

扩容建议：
- 主机数 > 50：增加 Redis 缓存层，降低 DB 查询压力
- 实例数 > 200：考虑按环境拆分 backend 或启用 MySQL 存储模式
- 监控数据 > 30 天：ClickHouse 配置 TTL 或启用分区清理

---

## 附录 F: 安全加固清单

### F.1 TLS 证书配置

生产环境建议为 Backend API 启用 HTTPS：

```bash
# 1. 准备证书
mkdir -p /opt/dbops-platform/tls
cp server.crt /opt/dbops-platform/tls/
cp server.key /opt/dbops-platform/tls/
chmod 600 /opt/dbops-platform/tls/server.key

# 2. 在 config.yaml 中配置
tls_cert_path: "/opt/dbops-platform/tls/server.crt"
tls_key_path: "/opt/dbops-platform/tls/server.key"

# 3. 重启服务
systemctl restart dbops-platform
```

### F.2 防火墙规则

```bash
# Backend 主机
ufw allow 22/tcp      # SSH
ufw allow 8080/tcp    # Backend API
ufw allow 9090/tcp    # Agent 通信（如 backend 与 agent 同机）

# 被管理主机
ufw allow 22/tcp      # SSH
ufw allow 9090/tcp    # Agent 端口
ufw deny 3306/tcp     # MySQL 只允许 platform backend 访问，或使用 VPN
```

### F.3 最小权限运行

```ini
# dbops-platform.service
[Service]
User=dbops
Group=dbops
# 限制可访问的主机/端口
ProtectSystem=strict
ProtectHome=true
NoNewPrivileges=true
PrivateTmp=true
```

```bash
# 创建专用用户
useradd -r -s /bin/false dbops
chown -R dbops:dbops /opt/dbops-platform
```

### F.4 密钥轮换

定期轮换以下密钥：
- `DBOPS_JWT_SECRET`：建议 90 天
- `DBOPS_ENCRYPTION_KEY`：建议 180 天，轮换需重新加密现有数据
- `DBOPS_AGENT_TOKEN`：建议 180 天，支持多 token 并行过渡

轮换步骤：
1. 生成新密钥
2. 更新配置并重启 backend
3. 验证新密钥生效
4. 废弃旧密钥

### F.5 审计日志

启用审计日志并定期审查：
- 登录失败
- 密码重置
- 高危操作（部署、升级、删除）
- Agent 注册/删除

