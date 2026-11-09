# 项目全面评审计划与问题清单

- 评审日期：2026-11-09
- 范围：backend、agent、frontend/src、CI 与部署配置；静态审阅为主。
- 基线：当前分支 `master` 落后 `origin/master` 4 个提交。工作区有未提交 AI 诊断相关改动（backend/cmd/main.go、ai_controller.go、ai_service.go、frontend/src/pages/AIDiagnosisPage.tsx、agent/internal/executor/flavor_task_executor.go，以及 aioptimizer、aiops executor/test 新文件）；这些改动未作归因或修改。
- 测试策略：本轮未运行全量测试或构建，避免在存在用户未提交代码的工作区中产生额外产物；最近找到的全量测试记录日期为 2026-09-28，不能证明当前代码仍通过。
- 证据说明：历史 `docs/ops-review-2026-09-28.md` 中已标注修复的项目均按当前源码复核；没有当前证据的旧问题不作为本轮缺陷结论。

## 当前确认的问题

### P0 - Compose 构建上下文与 Dockerfile COPY 路径不匹配

- 依据：`deploy/docker/docker-compose.yml:7-8,31-32` 的 `context: .` 相对于 Compose 文件目录 `deploy/docker` 解析；`deploy/docker/Dockerfile.backend:3-5` 与 `Dockerfile.agent:3-5` 则从 context 根目录读取 `backend/...`、`agent/...`。
- 影响：从 Compose 配置执行常规镜像构建时，context 不包含 Go 模块目录，构建会因 COPY 源不存在失败。
- 计划：将 context 指向仓库根（相对 compose 文件路径 `../..`），Dockerfile 路径保持相对 context；加入 compose config/build 验证。

### P1 - 已登录用户可按 taskID 订阅任意任务事件

- 依据：`backend/internal/controllers/ws_controller.go:22-30` 只检查 taskID 非空即调用 `messageBus.Subscribe(taskID)`；`RegisterRoutes` 的 `backend/internal/controllers/ws_controller.go:51-56` 仅挂认证中间件。handler 未验证任务归属或权限。
- 影响：已认证用户如果猜到或获知其他 taskID，可订阅任务 SSE 并读取任务事件内容。
- 计划：订阅前校验任务访问权限；不可访问时统一返回 404/403；增加跨用户订阅拒绝测试。

### P1 - LDAP TLS 未校验证书

- 依据：`backend/pkg/auth/ldap.go:48-50` 使用 `tls.Config{InsecureSkipVerify: true}`。
- 影响：LDAP TLS 连接不验证服务端身份，凭据及认证流量可能遭中间人读取或篡改。
- 计划：配置受信任 CA 与 ServerName，默认强制证书验证；支持显式 CA bundle；补齐错误主机名、不受信任 CA 测试。

### P1 - 前端长期 JWT 仍存于 localStorage

- 依据：`frontend/src/services/api.ts:92-98` 从 localStorage 取 token 并附加 Authorization；`frontend/src/pages/Login.tsx` 写入 token；`useTaskSSE.ts`、`useSettings.ts` 等也直接读取。API 同时启用 HttpOnly cookie。
- 影响：同源脚本漏洞可读取并外传可重放的 bearer token，cookie-only 保护被并行凭据路径削弱。
- 计划：统一 cookie-only 认证，清理 token 写入/读取和 Authorization 兼容路径；确认 SSE 使用短期一次性 ticket；补登录、退出和 SSE 集成测试。

### P1 - SSH known_hosts 解析失败会降级为首次信任

- 依据：`backend/internal/services/ssh_helpers.go:35-50` 和 `agent/internal/executor/ssh_hostkey.go:35-49` 在 known_hosts 存在但解析失败时继续首次记录；两端还允许 `DBOPS_SSH_INSECURE=1` 跳过校验（backend 行 68-83，agent 行 28-32）。
- 影响：known_hosts 损坏时可能将攻击者密钥当作首次密钥信任；不安全开关若误用于生产则完全关闭 SSH 主机身份验证。
- 计划：文件存在但解析失败时 fail-closed；生产启动时拒绝 insecure 开关；增加损坏文件、密钥变更和首次连接测试。

### P1 - SQLite 默认路径仍使用旧目录且依赖 cwd

- 依据：`backend/cmd/main.go:109-114` 在 `cfg.SQLitePath` 为空时设置 `../db/dbops.db`；`backend/pkg/config/config.go` 默认说明为 `<DataDir>/dbops.db`，二者不一致。
- 影响：更换启动目录可能读写另一份数据库；配置和备份目录容易指向错误文件。
- 计划：统一由配置解析出绝对路径；启动与 db-backup 共用该解析逻辑；补不同 cwd 路径测试与数据迁移说明。

### P1 - Makefile 升级目标与 systemd 二进制路径可能不一致

- 依据：`Makefile:105-106` 将二进制复制到 `/usr/local/bin/dbops-backend` 与 `/usr/local/bin/dbops-agent`；对应 unit 在 `deploy/systemd/`，需确认其 ExecStart 是否使用相同目标。
- 影响：若 systemd 使用 `/opt/dbops-platform/bin`，升级会覆盖未被服务运行的文件，造成升级假成功或重启失败。
- 计划：核对并统一 unit 的 ExecStart 与升级安装路径；增加脚本路径一致性检查、备份恢复验证。

### P2 - Compose 将 Redis/ClickHouse 端口映射到宿主机所有网卡

- 依据：`deploy/docker/docker-compose.yml:43-53` 映射 `6379:6379`、`8123:8123`、`9000:9000`，未绑定 loopback。
- 影响：宿主机可达网络中的其他用户可直接探测或访问内部数据服务，扩大攻击面。
- 计划：仅供 Compose 网络使用时移除端口映射；确需宿主访问则绑定 `127.0.0.1` 并设置访问控制。

### P2 - Prometheus build_info 指标重复

- 依据：`backend/pkg/middleware/metrics.go:47-48` 输出无标签 `dbops_build_info 1`，随后又输出带 version 标签的同名样本。
- 影响：数据冗余且版本指标语义不够清晰。
- 计划：只保留带版本标签的 gauge，并以 exposition parser 验证输出。

### 待授权策略确认 - RequireFeature 无业务路由调用

- 依据：`backend/pkg/middleware/license.go` 有 `RequireFeature` 定义，但全仓未发现调用点。DataMask 已在 `backend/cmd/main.go` protected 中间件链挂载，因此旧报告“DataMask 未接线”不再成立。
- 影响：如果商业授权要求在 API 层阻断 EE/UE 功能，当前许可闸门不会执行。
- 计划：按正式许可矩阵列出受限 API 后接线并补路由测试；确认 DataMask 是否应受 `FeatureDataMasking` 控制。策略未确认前不作为确定缺陷。

## 已复核的旧评审事项

- CI 文件、systemd unit 和 Compose 文件均存在；本轮不再沿用旧评审中“完全缺失”的结论。
- Dockerfile 版本已是 Go 1.25；CI 工作流覆盖 backend、agent 与 frontend。
- DataMask 已挂载，SSE 已使用一次性 ticket 机制。
- 旧数据库路径、LDAP TLS、Compose context、task stream 授权和升级路径经本轮复核后，分别列入上述问题或专项。

## 后续执行顺序

1. 修正 Compose context 与升级路径；在 Linux/CI 环境实际验证镜像构建及升级/回滚路径。
2. 修复任务订阅越权、LDAP 证书校验、SSH known_hosts fail-closed。
3. 统一 SQLite 数据路径和备份命令使用的配置。
4. 清理前端 localStorage token 路径，完成 cookie-only 与 SSE 回归。
5. 确认许可矩阵后接入 RequireFeature，并修正 build_info 输出。
6. 将 Compose 内部服务端口改为仅内部访问或 loopback 绑定。
7. 在隔离工作树或干净基线上运行 backend、agent、frontend 定向验证；必要时执行全量测试并记录基线、命令与结果。

## 本轮限制

- 未修改业务代码；仅创建本计划文件。
- 未运行测试或构建。
- 结论来自静态源码与仓库文档；生产配置、最近 CI 运行状态、容器构建和 SSH/备份端到端行为尚未验证。

## 执行结果（2026-11-09）

1. `6c00fa0`、`7312964`：Compose 构建上下文、挂载路径和 SQLite 环境变量修正；升级二进制路径与 systemd 对齐。
2. `e000042`：任务 SSE 限制为管理员；LDAP 默认校验证书并支持 CA 文件；后端和 Agent 对损坏 known_hosts 失败关闭。
3. `ad52db4`：配置加载统一生成绝对 SQLite 路径，启动、迁移和备份共用。
4. `3d4de42`：浏览器改为 HttpOnly cookie 认证，移除 token 存储/请求头及登录响应 token，退出时清除服务端 cookie。
5. `12edb14`：依据现有许可矩阵给 AI（UE）和数据脱敏（EE/UE）路由组接入 RequireFeature；build_info 仅输出一个带版本标签的样本。
6. `1337e46`：移除 Redis 和 ClickHouse 的宿主机端口映射，仅供 Compose 内部网络访问。

### 验证

- 后端：`go test ./... -count=1` 全部通过。
- Agent：全量并发运行被环境终止；随后 `./cmd`、`./internal/aioptimizer`、`./internal/collector`、`./internal/executor`、`./internal/grpc`、`./pkg/config`、`./pkg/logger` 分组运行全部通过。
- 前端：`npm run build`（TypeScript + Vite）通过。Vitest 受运行环境 `EPERM` 临时文件写入错误影响，路由用例未执行；sessionSecrets 的 3 个用例通过。
- Compose/Make：本机无 docker、make 命令，未执行实际镜像构建、升级或回滚。
- 未提交用户原有 AI 诊断相关改动，也未提交 `.workbuddy/` 工作区文件。
