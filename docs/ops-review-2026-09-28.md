# 平台运维巡检报告（评审稿）

- 巡检日期：2026-09-28
- 巡检视角：系统运维（可用性 / 安全 / 数据可靠性 / 部署交付 / 可观测性 / 可维护性）
- 巡检对象：智能 MySQL 运维平台（backend Go / agent Go / frontend React）
- 现场基线：`go vet ./...` 通过、`go build ./...` 通过、`go test ./pkg/...` 全绿；工作区存在未提交变更（package.json / package-lock.json）

---

## 一、结论摘要

平台功能面很广（纳管、部署、备份、迁移、告警、AI、审计、信创分层纳管），代码里已有大量安全加固痕迹（P0/P1/B 系列编号的加固点、密钥启动强校验、审计 HMAC 链、登录限流、CORS 白名单）。但从系统运维视角看，**"能跑"与"能运维"之间仍有明显差距**，集中在：

1. **部署交付链断裂**：生产启动方式是 `go run`，无 systemd/compose/CI，Dockerfile 与 go.mod 版本冲突根本构建不出来；
2. **安全边界残留后门**：standalone 模式认证旁路、SSH 主机密钥不校验、relay 端点 SSRF + 上传路径穿越、SSE token 走 URL；
3. **自身数据可靠性缺失**：平台元数据库无备份策略、迁移失败仅告警不阻断、数据目录散落在源码树；
4. **已实现未接线**：DataMask 中间件、license 功能闸门写好了但没挂到路由，属于"功能空转"。

共识别 **8 项高风险、9 项中风险、若干低风险**。建议优先处理 P0-1 ~ P0-4。

---

## 二、高风险（P0，建议尽快处理）

### P0-1 生产部署形态缺失，`go run` 当启动方式
- `bin/ubuntu/start-backend.sh` 最后一行是 `go run cmd/main.go`（前台进程、依赖源码与 Go 工具链、无开机自启、无崩溃拉起）。
- 根 `Makefile` 的 `docker-up/down/logs` 直接 `exit 1`（注释承认 compose 文件不在仓库）；仓库内无任何 systemd unit / supervisord / compose / k8s manifest。
- `.gitignore` 中**显式忽略 `.github/`**，意味着 CI 配置被禁止入库 —— 全部 129 个测试文件没有任何自动执行入口，回归完全靠自觉。
- `make upgrade` 依赖 `scripts/stop.sh`、`scripts/start.sh`，但 `scripts/` 目录只有 `scan-local-secrets.ps1`，且失败被 `|| true` 吞掉，升级会"静默假装完成"。
- **建议**：提供 systemd unit（含 `WorkingDirectory=`，见 P1-6 相对路径问题）；恢复 CI（允许 `.github/` 入库）；补齐 compose 或离线包脚本；`make upgrade` 对缺失脚本 fail-fast。

### P0-2 Dockerfile 与 go.mod 版本冲突，镜像构建必败
- `backend/Dockerfile` 使用 `FROM golang:1.22-alpine`，而 `backend/go.mod` 声明 `go 1.25.0` —— 多阶段构建在 `go build` 一步直接失败。
- `backend/go.mod` 与 `agent/go.mod` 依赖版本严重分叉：`golang.org/x/crypto` v0.53.0 vs **v0.16.0**（agent 端低于 Terrapin 修复版 v0.17.0，`x/net` 也停在 v0.19.0）。
- **建议**：Dockerfile 升到 `golang:1.25-alpine`；两个模块的 x/crypto、gin（1.9.1 → 1.10.x）、viper 统一升级；引入 dependabot/renovate。

### P0-3 relay 端点组：SSRF + 上传路径穿越，且普通登录用户可调用
位置：`backend/cmd/main.go` `/api/v1/relay/*`（protected 组内，**未挂 RequirePermission("admin")**）：
- `POST /relay/download-to-relay`、`POST /relay/scan-url`、`POST /relay/test-source`：接受任意 URL 由服务端发起请求（30 分钟超时大下载、目录递归抓取），可被用于内网探测/打点（SSRF），无目标地址白名单、无私网段校验。
- `POST /relay/upload`：`destDir = filepath.Join(dataDir/packages, subPath)`，`subPath` 与 `header.Filename` **均未做 `..` 清洗**（对比 `delete-package` 反而做了）。`path=../../..` 可把任意文件写到平台进程权限可及的任意位置。
- `scan-remote-ssh` / `pull-from-relay`：把 host 的 SSH 明文密码解出后直接使用，`services.NewSSHClient` 内部是 `ssh.InsecureIgnoreHostKey()`（见 P0-4）。
- **建议**：relay 组整体加 admin 权限；上传/下载路径统一走 `filepath.Clean` + 根目录包含性校验；对外部 URL 增加协议（仅 http/https）与私网 IP 黑名单。

### P0-4 SSH 主机密钥不校验（MITM 风险）
- `backend/internal/services/ssh_helpers.go:24`：`HostKeyCallback: ssh.InsecureIgnoreHostKey()`。该构造器被 relay 端点、环境检查、实例扫描等多处使用。
- 虽然 `host_service.go:746`、`instance_service.go:2022-2038` 有 known_hosts 记录逻辑（TOFU），但 `NewSSHClient` 这条主路径完全不校验。
- 平台是"拿 root SSH 密码批量操作数据库主机"的系统，主机密钥不校验意味着网络内劫持可批量收割所有被管主机凭据。
- **建议**：所有 SSH 出口统一走 known_hosts（首次记录 + 后续严格比对），并提供人工重置指纹的运维命令；中长期支持 SSH key 认证替代密码。

### P0-5 standalone 认证旁路可达性需要收紧
- `backend/internal/services/auth_service.go:105`：`IsStandalone()` 时**任意用户名密码登录并授予 admin+\* 权限**，仅打一行 log。
- 该模式在 `db=nil`（JSON store 降级）时激活。当前 `storage_mode=auto` 下 DB 初始化失败是 Fatal，正常情况走不到；但 JSONStore/standalone 分支代码仍然保留，一旦某次改动让 `db` 可为 nil（历史上显然走过这条路），管理面就是裸的。
- **建议**：给 standalone 模式加显式开关（如 `DBOPS_STANDALONE=1`）而非由 db 状态隐式推导；standalone 下拒绝 `/relay`、备份恢复、failover 等高危写接口；日志升级为启动横幅。

### P0-6 SSE 认证 token 走 URL 查询串
- `backend/internal/controllers/auth_controller.go:131`：`/tasks/stream/:id?token=xxx`。URL 中的 JWT 会进入反向代理访问日志、浏览器历史、Referer。
- 同时 `ValidateToken` 中 header 与 cookie 都存在且 header 校验失败时**回退到 cookie 再验一次**（144-150 行）——双轨容忍让"哪条是权威凭据"变模糊，也放大了 token 混用排障成本。
- **建议**：EventSource 无法带 header 的标准解法是短时一次性 ticket（用 JWT 换 30s 有效的一次性 stream ticket），而不是裸 token 进 URL；逐步废弃 localStorage token 双轨（见 P1-5）。

### P0-7 平台自身数据无备份/无恢复演练，元数据库落在源码树
- `db/dbops.db`（含 -wal/-shm/.bak）就躺在工作区；`main.go` 强制 `sqlitePath = "../db/dbops.db"`（注释 "Force use old database file"），与 `.env.example` 宣称的 `./data/dbops.db` 不一致 —— 文档、代码、示例三方各说各话。
- 平台元库里存的是**全部被管实例的加密凭据、主机信息、审计链**，却没有对 dbops.db 的定时备份、备份校验、恢复演练机制（平台自己给 MySQL 做备份管理，自身却裸奔）。
- **建议**：统一数据目录到 `data/`；增加元数据库定时快照（SQLite 用 `.backup` API，MySQL 模式用 mysqldump）与恢复演练任务；`db/` 从仓库工作区清出。

### P0-8 schema 迁移失败不阻断启动
- `main.go:104`：`runMigrations` 失败仅 `logInstance.Warn("Migrations skipped: ...")` 后继续对外服务。schema 与代码版本漂移后，后续写操作会产生难排查的脏数据。
- **建议**：迁移失败按策略处理 —— 至少 `storage_mode=mysql` 时 Fatal；sqlite 时提供 `--migrate-only` 维护模式入口（`migrate` 子命令在 Makefile 有提及，但 main.go 并未实现 os.Args 分支，`make db-migrate` 实际不会生效）。

---

## 三、中风险（P1，规划处理）

### P1-1 已实现未接线：DataMask 与 license 功能闸门空转
- `pkg/middleware/datamask.go` 的 `DataMask` 完整实现了按角色脱敏 JSON 响应，但 `main.go` 中**没有任何路由挂载它** —— 数据脱敏功能（EE 卖点之一）实际不生效。
- `pkg/middleware/license.go` 的 `RequireFeature` 同样零调用；EE/UE 功能差异目前只有 flavor 能力闸门（这部分做得不错），商业版功能门禁在 API 层缺位。
- **建议**：确定脱敏应用范围（至少 instances 列表、credentials、审计日志），挂载中间件；对 EE 特性路由补 `RequireFeature`。

### P1-2 审批流权限倒挂
- `/approvals`：`POST /approvals`（创建审批）要求 admin，而 `POST /:id/approve`、`/:id/reject` **任何登录用户可调**。审批应是"申请人任意、审批人受限"。
- **建议**：approve/reject 加独立权限（如 `approval:approve`），创建放开给普通用户。

### P1-3 JWT 无吊销机制，改密/禁用后旧 token 仍有效
- Claims 无版本号/jti，`ValidateToken` 只验签名 + 用户是否禁用（后者依赖查库，需确认是否每次请求查）。用户改密、角色变更后，已签发的 24h token 依然全权有效。
- **建议**：Claims 增加 `pw_version`（密码版本号），改密后旧 token 自然失效；登出增加服务端黑名单（Redis 已在栈内）。

### P1-4 弱口令策略：后端仅 `min=6`
- `ChangePasswordRequest`/`ResetAllPasswordsRequest` 仅 `min=6`；前端 SecuritySettings 有 password_policy，但后端不消费。运维平台存着全网 root 密码，口令策略应后端强制。
- `backend/config.yaml.example` 仍带 `Repl#2024!ChangeMe` 类示例密码（虽然 config.go 默认值已改为空，值得肯定）。
- **建议**：后端统一实现复杂度校验并消费 settings 中的 password_policy；bcrypt cost 显式化（当前 DefaultCost=10，可接受但建议配置化）。

### P1-5 前端 token 存 localStorage，与 HttpOnly cookie 双轨并存
- `frontend/src/services/api.ts:62`、`Login.tsx:28`：token 同时进 localStorage 与 HttpOnly cookie。localStorage 中的长效 JWT 在任何 XSS 下直接丢权。
- vite dev server `host: 0.0.0.0` + `allowedHosts: 'all'`，开发模式对局域网全开。
- **建议**：以 HttpOnly cookie 为唯一凭据源，localStorage 仅存非敏感 user 展示信息；dev server 绑回 localhost。

### P1-6 全仓相对路径依赖启动目录
- `./frontend/dist`、`../db/dbops.db`、`./data`、`./data/backups` 等均为相对 cwd 的路径。systemd/容器/手工启动的 WorkingDirectory 稍有不同，数据与备份就写到不同位置甚至静默新建目录。
- **建议**：启动时解析并打印绝对路径；配置项统一转绝对路径；systemd unit 显式 `WorkingDirectory=`。

### P1-7 内存态限流/会话/任务状态，单副本绑定
- `RateLimiter`（进程内 map）、WSHub、MessageBus、任务进度均为内存态。重启丢限流与推送连接；扩第二个副本会出现"任务在 A 副本跑、SSE 订在 B 副本"的割裂。Redis 已引入但仅用于 pubsub，未用于限流/会话共享。
- **建议**：至少把登录限流迁到 Redis（代码注释自己也承认"大规模部署应替换为 Redis 限流"）；文档明确"平台当前仅支持单副本"。

### P1-8 优雅停机不彻底
- HTTP 有 `srv.Shutdown`，但部署编排、备份、升级等后台 goroutine 无等待与取消传播；SQLite 无关闭前 checkpoint（WAL 文件已出现在 `db/`）。
- **建议**：引入全局 context + `errgroup` 等待后台任务落库完成；停机钩子里对 SQLite 做 WAL checkpoint。

### P1-9 平台无自身可观测性出口
- 无 `/metrics`（Prometheus）、无 pprof、无 trace 导出；日志 JSON 到 stdout 但 `go run` 前台跑时无轮转与落盘策略；`backend/` 目录已经堆积 `backend.log/backend_err.log/stdout.log/...` 8+ 个遗留日志和 `*.exe` 残留（虽被 gitignore，但说明缺日志目录约定）。
- 平台是给 MySQL 做监控告警的，自身却是黑盒。
- **建议**：暴露 Prometheus 指标（连接池、任务队列、agent 在线数）；pprof 挂 internal 鉴权组；约定 logs/ 目录 + 轮转（lumberjack 或 logrotate）。

---

## 四、低风险 / 观察项（P2）

1. **agent 侧鉴权细节**：`agent/cmd/main.go` 中 `tok != agentToken` 为非常量时间比较（backend 已用 `subtle.ConstantTimeCompare`，agent 未对齐）；token 未配置时返回 500 而非 503。
2. **脱敏算法双实现不一致**：`masking_service.go` 的 `maskMD5` 是真 MD5，`pkg/middleware/datamask.go` 的 `MaskingMD5` 分支却是星号掩码。接线前先统一（见 P1-1）。
3. **权限模型双轨**：`middleware.RequirePermission` 内置一份 rolePermissions 硬编码 map，与 DB 角色/权限表并存，语义易漂移。建议以后者为准，middleware 只做查表。
4. **backend 启动即编译 agent**（`EnsureAgentBinaries`）：要求生产机装 Go 工具链且 agent 源码在位，否则 Fatal。建议改为发布产物内置 + `DBOPS_SKIP_AGENT_BINARY_BUILD` 文档化，运行机器不应有编译步骤。
5. **agent 版本管理**：`AgentVersion = "1.0.0"` 常量，平台与 agent 之间无版本协商/兼容性上报，升级 agent 全靠手工。
6. **Makefile `dist` 目标**：`cp backend/bin/platform dist/bin/dbops-backend` 与 Makefile 实际产物名 `bin/platform` 一致性 OK，但 frontend 产物路径 `frontend/build` fallback `frontend/dist` 的写法掩盖了真实产物位置。
7. **`/health/ready` 中 ClickHouse 未参与**：监控链路依赖 ClickHouse，就绪检查只看 db/redis，CH 故障时 ready 仍绿。
8. **`LoginRateLimit` 开关语义**：代码只认 `DBOPS_LOGIN_RATELIMIT_DISABLE=1`，`.env.example` 写 `false`，易误解为支持 true/false。
9. **仓库卫生**：根 `package.json` 只有一个 `freebuff ^0.1.2` 依赖且工作区有未提交变更，与项目主体无关，建议明确其用途或移除；`backend/cmd.exe`、`main.exe` 等残留文件建议清理。
10. **README 端点描述**：`http://localhost:8080` 标注为 "Backend Admin"，实际是纯 API；避免误导部署验收。

---

## 五、做得好的地方（保持）

- 启动时密钥强校验（`validateSecrets`，占位符/弱值拒绝启动 + 测试覆盖）；
- 审计日志 HMAC 链 + `verify-chain` 接口；
- 备份/恢复、failover、升级、删除等高危操作全部 `RequirePermission("admin")`，且信创引擎能力闸门有静态表 + 21 处服务入口约束（`tasklist.md` 文档清晰）；
- CORS 白名单、登录端点专用限流、body 上限可配置；
- 凭据 AES-GCM 加密落库 + 密钥轮换服务；DSN 转义有专项测试（特殊字符密码）；
- 探活/就绪分离（`/health/live`、`/health/ready`）。

---

## 六、建议行动清单（按优先级）

| 优先级 | 事项 | 对应问题 |
|---|---|---|
| 本周 | 修复 Dockerfile Go 版本；relay 上传路径穿越 + admin 权限；恢复 CI 入库 | P0-1/2/3 |
| 两周 | SSH host key 校验统一；standalone 显式开关；SSE ticket 机制 | P0-4/5/6 |
| 一个月 | 元数据库备份策略 + 数据目录归一；迁移失败阻断 + migrate 子命令；Prometheus 指标 | P0-7/8、P1-9 |
| 规划 | JWT 吊销、脱敏/许可证接线、审批权限修正、限流迁 Redis、systemd 交付 | P1-1~7 |

---

## 附：巡检方法说明

- 静态走读：backend 路由全景（main.go 1317 行）、config/中间件/auth、agent 执行器（115 处 os/exec 调用点抽查）、前端鉴权与构建配置、启动/初始化脚本、两套 Dockerfile、全部 Makefile；
- 动态验证：`go vet`、`go build`、`go test ./pkg/...` 实测通过；
- 未覆盖：`go test ./internal/...` 全量回归（耗时长，建议 CI 承接）、前端 vitest、真实部署链路演练（受限于本机 Windows 环境）。此部分建议列入下一轮巡检。
