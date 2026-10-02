# MySQL Ops Platform / 智能 MySQL 运维平台

> 面向数据库架构级生命周期管理的运维平台。
>
> 通过后端 API、React 控制台和主机侧 Agent，统一管理主机、MySQL 实例、集群、中间件、监控、备份、升级和角色切换。

[![Go Version][go-image]][go-url] [![Node.js][node-image]][node-url] [![License][license-image]][license-url] [![Language][lang-image]][lang-url]

- **中文文档**：[readme_ZH.md](../readme_ZH.md)
- **英文文档**：[readme_US.md](../readme_US.md)
- **运维手册**：[OPS_MANUAL.md](OPS_MANUAL.md)
- **巡检报告**：[ops-review-2026-09-28.md](ops-review-2026-09-28.md)

---

## 技术栈

- **后端**：Go 1.25+ + Gin + SQLite/MySQL + Redis（可选）+ ClickHouse（可选）
- **前端**：React 18 + TypeScript + Ant Design 5
- **Agent**：Go 1.25+ + HTTP + Bearer Token 认证

## 商业版本

- **CE**（社区版）：基础功能，MIT 协议
- **EE**（企业版）：CE + 高可用/升级/迁移/审计功能
- **UE**（旗舰版）：EE + AI 智能化，商业授权

---

## 仓库布局

```text
backend/            Go 后端 API、服务、仓储、配置和迁移
frontend/           React + TypeScript Web 控制台
agent/              部署在被管主机上的 Go 执行 Agent
bin/windows/        Windows 一键启停脚本 (start/stop/restart .bat/.ps1)
bin/ubuntu/         Ubuntu 启停脚本
bin/centos/         CentOS 启停脚本
deploy/             systemd unit 与 docker-compose 生产交付形态
scripts/            运维脚本，包括本地密钥扫描
docs/               补充文档、运维手册和截图
data/               本地开发数据（gitignore）
logs/               本地运行日志（gitignore）
Makefile            构建、测试、安装、打包和升级辅助命令
.env.example        环境变量示例
```

> 注意：一键启停脚本位于 `bin/windows/`、`bin/ubuntu/`、`bin/centos/`，**不在仓库根目录**。

---

## 快速开始

### 环境要求

| 组件 | 版本 | 说明 |
|------|------|------|
| Go | 1.25+ | backend 与 agent 共用（见 `backend/go.mod`、`agent/go.mod`） |
| Node.js | 18+ | 前端构建/运行 |
| npm | 必需 | 前端依赖管理 |
| PowerShell | 5.1+ | Windows 脚本 |
| bash | Linux/macOS 必需 | Shell 脚本 |
| Redis | 可选 | 缓存/队列场景 |
| ClickHouse | 可选 | 监控数据存储 |

### Windows 一键启动

```powershell
# 1. 配置环境变量
copy .env.example .env
# 编辑 .env，至少设置（启动时会强校验，缺省/占位/弱值直接拒启动）：
#   DBOPS_DB_URL=dbops_user:strong-password@tcp(127.0.0.1:3306)/dbops_platform?parseTime=true&loc=Local
#   DBOPS_JWT_SECRET=<>=32 chars>
#   DBOPS_ENCRYPTION_KEY=<>=32 chars>
#   DBOPS_AGENT_TOKEN=<>=16 chars>

# 2. 构建并启动全部服务（脚本会自动编译 backend/agent/frontend）
.\bin\windows\start.bat

# 3. 验证
Invoke-RestMethod http://localhost:8080/health

# 4. 停止 / 重启
.\bin\windows\stop.bat
.\bin\windows\restart.bat

# 5. 跳过编译，仅启动已有产物
.\bin\windows\start.bat -SkipBuild
```

### Linux 启动（开发/调试）

```bash
bash bin/ubuntu/start-all.sh   # 前台运行，Ctrl+C 停止
bash bin/ubuntu/stop.sh
```

### 生产部署

生产环境应使用 systemd 或 docker-compose，见 `deploy/systemd/` 与 `deploy/docker/`，详见 [OPS_MANUAL.md](OPS_MANUAL.md) §3.2。

### 手动启动

```bash
cd backend && go run ./cmd/main.go
cd agent && go run ./cmd/main.go
cd frontend && npm run dev -- --host 0.0.0.0 --port 3000
```

---

## 构建与测试

```bash
# 安装依赖
make install-backend
make install-agent
make install-web

# 构建所有组件
make build

# 运行后端与 Agent 测试
make test

# 信创单机生命周期测试（Agent 夹具 + 后端能力闸门 + 前端镜像）
make test-xinchuang-lifecycle

# 打离线安装包
make dist
```

等价的手动命令：

```bash
cd backend && go build -o bin/platform ./cmd/main.go
cd agent && go build -o bin/agent ./cmd/main.go
cd frontend && npm run build
```

---

## API 访问地址

| 服务 | 地址 | 协议 | 认证 |
|------|------|------|------|
| 后端 API | `http://localhost:8080` | HTTP/REST（`/api/v1/*`） | JWT |
| Web 控制台 | `http://localhost:3000` | HTTP/WS | 会话 Cookie |
| Agent 服务 | `http://localhost:9090` | HTTP（`/agent/tasks/*`） | Agent Token |

> 8080 是**纯 REST API**，没有管理界面；管理界面在 3000。

---

## 配置

复制 `.env.example` 为 `.env` 并设置强值。后端示例配置见 `backend/config.yaml.example`（注意是仓库根下的 `backend/config.yaml.example`，不是 `backend/config/config.example.yaml`）。

后端同时支持环境变量覆盖（`DBOPS_*` 前缀），优先级高于 yaml。

```yaml
# backend/config.yaml 示例（扁平键名）
server_port: "8080"
storage_mode: "auto"          # auto | mysql | sqlite
data_dir: "./data"
database_url: "..."
jwt_secret: "..."
encryption_key: "..."
agent_token: "..."
```

安全注意事项：

- 不要提交真实密钥、Token、数据库密码、SSH 私钥或 License 材料。
- 敏感凭据使用 AES-GCM 加密落库。
- 发布改动前建议执行本地密钥扫描：`.\scripts\scan-local-secrets.ps1`。

---

## 支持的数据库架构

- 单实例 MySQL 管理
- HA 主从架构
- MHA
- MGR（需 MySQL 8.0+）
- PXC

信创/国产引擎（OceanBase、TiDB、Kingbase、openGauss、HighGo、GBase 8a/8s、达梦、神舟通用、GaussDB/PolarDB/TDSQL for MySQL）通过分层纳管接入，能力边界由 `backend/internal/services/flavor_capability.go` 的静态表统一约束。详见根目录 `tasklist.md` 与 `plan0731.md`。

---

## 文档资源

- 中文文档：[readme_ZH.md](../readme_ZH.md)
- 英文文档：[readme_US.md](../readme_US.md)
- 运维手册：[OPS_MANUAL.md](OPS_MANUAL.md)
- API 参考：[API.md](API.md)
- 截图：[screenshots](screenshots)
- 密钥扫描脚本：[scan-local-secrets.ps1](../scripts/scan-local-secrets.ps1)

> 项目暂无 `CONTRIBUTING.md`、`SECURITY.md`、`specs/` 与 Swagger 文档；以上链接仅为占位说明，请勿参照执行。

---

## 联系方式

- GitHub：提交 Issue 或 Pull Request
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
