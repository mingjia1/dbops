# DBOps 平台遗留问题执行计划（按优先级）

- 生成日期：2026-10-29
- 来源：`docs/ops-review-2026-09-28.md` 文末「遗留待办」+ 第四节低风险观察项 + 第六节行动清单尾款
- 执行方式：严格按 **P0 → P1 → P2** 顺序逐步执行；每完成一项立即回写本文件（状态 + 执行记录）
- 状态图例：⬜ 待处理 / 🔄 进行中 / ✅ 完成 / ⏸ 暂缓（须注明原因）
- **执行状态（2026-10-30）：17/17 全部完成 ✅**（含交付演练发现的回归修复），全量回归见第五节「全量回归」

---

## 一、执行顺序总览

| 序号 | 优先级 | 任务 | 来源 | 状态 |
|---|---|---|---|---|
| 1 | P0 | P0-A 数据目录归一 + 全仓相对路径转绝对路径 | P0-7 尾款、P1-6 | ✅ |
| 2 | P0 | P0-B 元数据库定时备份调度 | P0-7 尾款 | ✅ |
| 3 | P0 | P0-C `make upgrade`/`pre-upgrade-check` 失败不吞错 | P0-1 尾款 | ✅ |
| 4 | P1 | P1-A license `RequireFeature` 接线 | P1-1 尾款 | ✅ |
| 5 | P1 | P1-B 前端 token 单轨化（HttpOnly cookie 唯一凭据源） | P1-5 | ✅ |
| 6 | P1 | P1-C 登录限流迁移 Redis（不可用时回退内存） | P1-7 | ✅ |
| 7 | P1 | P1-D 优雅停机：后台任务等待 + SQLite WAL checkpoint | P1-8 | ✅ |
| 8 | P1 | P1-E pprof 挂载 + `logs/` 目录与轮转约定 | P1-9 尾款 | ✅ |
| 9 | P2 | P2-A agent token 未配置返回 503 | P2-1 | ✅ |
| 10 | P2 | P2-B `LoginRateLimit` 开关语义与 `.env.example` 对齐 | P2-8 | ✅ |
| 11 | P2 | P2-C `/health/ready` 纳入 ClickHouse 就绪 | P2-7 | ✅ |
| 12 | P2 | P2-D Makefile `dist` 前端产物路径修正 | P2-6 | ✅ |
| 13 | P2 | P2-E README 端点描述修正 | P2-10 | ✅ |
| 14 | P2 | P2-F 仓库卫生核对（残留二进制/无用配置） | P2-9 | ✅ |
| 15 | P2 | P2-G 权限模型双轨（middleware 硬编码 vs DB）收敛评估 | P2-3 | ✅ |
| 16 | P2 | P2-H `urlguard` 路径穿越校验跨平台修复（Linux 下反斜杠/盘符未拦截，单测红） | 本轮验证发现 | ✅ |
| 17 | 交付 | P3-A 交付链路实跑：`make dist` 离线包 + backend 单文件构建回归修复 | 本轮交付演练发现 | ✅ |

**前置已核实（无需执行，直接标记）**

- P1-3 JWT 吊销：`Claims.PasswordVersion` + `password_changed_at` 已落地，改密即失效 ✅（本计划不重复执行）
- P0-1 主体：CI（`.github/workflows/ci.yml`）、systemd unit、docker compose 均已入库 ✅
- P2-9 中「根 package.json」：仓库根目录已无 `package.json`，`backend/*.exe` 残留也不存在 ✅

---

## 二、P0 — 最高优先级（数据可靠性 / 部署交付）

### P0-A 数据目录归一 + 全仓相对路径转绝对路径（含 P1-6）

- **现状**：`backend/cmd/main.go:112/1451/1495/1546` 硬编码回退 `../db/dbops.db`；`pkg/config/config.go:76` 默认 `data_dir: "../db"`；`.env.example` 却写 `./data`。全仓相对 cwd 的路径（sqlite、data、backups、前端 dist）导致 systemd/容器/手工启动结果不一致。
- **任务**：
  1. `pkg/config` 增加统一的绝对路径解析：`data_dir` / `sqlite_path` / `backup_dir` 启动时转绝对路径，空值按 `data_dir` 推导；
  2. `main.go` 移除「Force use old database file」隐式硬编码，改为：显式配置 > 环境变量 > 既有 `../db/dbops.db` 存在时兼容沿用 > 否则 `data/`；
  3. 启动日志打印所有关键路径的**绝对路径**；
  4. `.env.example`、`config.yaml.example`、`docs/` 三方口径统一。
- **涉及**：`backend/pkg/config/config.go`、`backend/cmd/main.go`、`.env.example`、`backend/config.yaml.example`
- **验证**：`go build ./... && go vet ./... && go test ./pkg/config/... ./cmd/...`；启动日志路径为绝对路径。

### P0-B 元数据库定时备份调度

- **现状**：`main.go db-backup` 子命令已可手动/由 `make` 触发，但平台进程内无定时快照；元数据库存着全部被管实例加密凭据，却无自动备份。
- **任务**：
  1. 新增后台调度：按 `db_backup_interval`（默认 24h，0 关闭）调用既有 `backupSQLite` 快照 + `keep` 轮转；
  2. 备份失败记录 Error 日志并计数（暴露到 `/metrics`）；
  3. 调度器随优雅停机（P1-D）退出，停机前不中断进行中的备份。
- **涉及**：`backend/cmd/main.go`（或 `internal/services/db_backup_scheduler.go`）、`pkg/config/config.go`
- **验证**：单测覆盖调度触发/轮转/失败记录；`go test ./...`。

### P0-C `make upgrade` 失败不吞错

- **现状**：`Makefile` `pre-upgrade-check` 中备份步骤 `tar ... || true`、二进制备份 `cp ... || true`，升级关键前置失败会被静默跳过。
- **任务**：备份失败改为 fail-fast（给出可读错误与恢复指引）；核对 `smoke-test.sh`、`backup-platform.sh` 存在性校验。
- **涉及**：`Makefile`
- **验证**：`make -n upgrade`、`make pre-upgrade-check`（无服务时应明确报错而非静默通过）。

---

## 三、P1 — 中风险（安全边界 / 可观测）

### P1-A license `RequireFeature` 接线

- **现状**：`pkg/middleware/license.go` 的 `RequireFeature` 全仓零调用，EE/UE 商业功能门禁在 API 层空转。
- **任务**：对 EE 特性路由挂载（至少：数据脱敏、审计链、密钥轮换、LDAP/SSO、自动 failover、AI/Text2SQL），CE 请求返回结构化 403 错误码（`feature_not_licensed`）；接线处与 flavor 能力闸门不冲突。
- **涉及**：`backend/cmd/main.go`、`pkg/middleware/license.go`
- **验证**：新增单测：CE tier 拒绝 / EE tier 放行；`go test ./...`。

### P1-B 前端 token 单轨化

- **现状**：`frontend/src/services/api.ts:94`、`Login.tsx:28` 等处 token 同时写 localStorage 与 HttpOnly cookie，`useTaskSSE`/`useSettings`/`SecuritySettings` 直接读 `localStorage.getItem('token')` 拼 Authorization；localStorage 中长效 JWT 在 XSS 下直接丢权。
- **任务**：
  1. 登录后不再写 `localStorage.token`（保留 `user` 展示信息）；
  2. 请求统一依赖 HttpOnly cookie（`withCredentials`），移除手工 `Authorization: Bearer <localStorage token>`；
  3. SSE 继续走一次性 ticket；
  4. 清理旧 token（登录时 `removeItem` 兼容历史残留）；同步更新受影响测试。
- **涉及**：`frontend/src/services/api.ts`、`Login.tsx`、`ProtectedRoute.tsx`、`Dashboard.tsx`、`useTaskSSE.ts`、`useSettings.ts`、`SecuritySettings.tsx` 及测试
- **验证**：`cd frontend && npx tsc --noEmit && npm test -- --run && npm run build`。

### P1-C 登录限流迁移 Redis

- **现状**：`pkg/middleware/ratelimit.go` 进程内 map，重启丢限流、多副本失效；Redis 已在技术栈内但仅用于 pubsub。
- **任务**：登录限流改 Redis `INCR + EXPIRE` 滑动窗口；Redis 不可用时回退现有内存限流并告警；文档明确「平台当前仅支持单副本，限流已具备跨副本能力」。
- **涉及**：`backend/pkg/middleware/ratelimit.go`、`cmd/main.go`、`docs/OPS_MANUAL.md`
- **验证**：单测（Redis mock/fake + 回退路径）；`go test ./pkg/middleware/...`。

### P1-D 优雅停机

- **现状**：只有 HTTP `srv.Shutdown`；后台 goroutine（健康检查 ticker、任务编排、备份调度、限流 cleanup）无取消传播；SQLite 停机前无 WAL checkpoint。
- **任务**：引入全局 `context.Context` + 等待组，停机钩子按序：停止接收新请求 → 等待后台任务落库（带超时）→ SQLite WAL checkpoint → 关闭 DB。
- **涉及**：`backend/cmd/main.go`、`internal/services/*`（后台 ticker）
- **验证**：单测覆盖停机等待与 checkpoint 调用；`go test ./...`。

### P1-E pprof + 日志目录/轮转约定

- **现状**：无 pprof；日志直出 stdout，`backend/` 历史上堆积 8+ 个遗留日志文件，缺目录约定。
- **任务**：
  1. `/debug/pprof/*` 挂到 internal/admin 鉴权组（非公开）；
  2. 仓库约定 `logs/` 目录并加入 `.gitignore`；文档说明用 logrotate/journald 管理轮转；
  3. 清理/说明 `backend/` 下遗留日志文件（若仍存在）。
- **涉及**：`backend/cmd/main.go`、`.gitignore`、`docs/OPS_MANUAL.md`
- **验证**：pprof 路由未鉴权 401/404；`go build/vet`。

---

## 四、P2 — 低风险 / 观察项

### P2-A agent token 未配置返回 503
- `agent/cmd/main.go:36` 当前 `500`；改为 `503 Service Unavailable`，同步更新 `main_test.go` 断言。

### P2-B `LoginRateLimit` 开关语义
- 代码只认 `DBOPS_LOGIN_RATELIMIT_DISABLE=1`，`.env.example` 写 `false`；改为兼容 `1/0/true/false/yes/no`，`.env.example` 注释说明。

### P2-C `/health/ready` 纳入 ClickHouse
- 当前 ready 只查 db/redis；ClickHouse 已配置时纳入 ping（未配置跳过），避免监控链路故障时就绪仍绿。

### P2-D Makefile `dist` 前端产物路径
- `cp frontend/build ... || cp frontend/dist` 掩盖真实产物位置；按实际产物（`frontend/dist`）修正并失败即报错。

### P2-E README 端点描述
- `http://localhost:8080` 标注 "Backend Admin" 实为纯 API；中英文 README 一并修正。

### P2-F 仓库卫生核对
- 核对残留二进制/日志/无用配置（根 `package.json`、`backend/*.exe` 已确认不存在，做一次全仓复核并记录）。

### P2-G 权限模型双轨收敛评估
- `middleware.RequirePermission` 内置硬编码 rolePermissions 与 DB 角色权限表并存。本轮**只做评估与文档化**（改动面涉及全部鉴权语义，需独立变更窗口），结论写入 `docs/OPS_MANUAL.md`。

---

## 五、执行记录（随执行回写）

> 每完成一项，在此追加：序号、改动摘要、验证结果。

### ✅ P0-A 数据目录归一 + 绝对路径（2026-10-29）

- 新增 `backend/pkg/config/paths.go`：`resolveDataPaths` / `resolveBackupDir` 统一把 `data_dir`、`sqlite_path`、`backup_dir` 解析为**绝对路径**；未配置 `data_dir` 时先兼容旧部署的 `../db/dbops.db`（存在则沿用并打弃用告警），否则回落 `./data`；`sqlite_path` 空值固定 `<data_dir>/dbops.db`，`backup_dir` 空值固定 `<data_dir>/backups`。
- `config.go`：移除 `data_dir="../db"` 硬编码默认；新增 `BackupDir` / `UsedLegacyDataDir` 字段与 `DBOPS_BACKUP_DIR` 绑定。
- `cmd/main.go`：删除 4 处 `"../db/dbops.db"` 隐式回退（`sqlitePathOf` 统一）、删除 "Force use old database file" 日志；启动时打印 DataDir / SQLite / BackupDir / frontend dist 的绝对路径；前端静态产物路径改 `filepath.Abs`。
- `internal/services/ssh_helpers.go`：known_hosts 目录改为可注入（`SetKnownHostsDir`），main 启动时传入绝对 DataDir，避免不同 cwd 各存一份 known_hosts。
- 文档口径统一：`.env.example`、`backend/config.yaml.example` 说明解析规则。
- **验证**：`go build ./...`、`go vet ./...`、`go test ./pkg/config/...` 通过（新增 5 个路径解析用例，含 legacy 兼容与默认回落）。

### ✅ P0-B 元数据库定时备份调度（2026-10-29）

- 新增 `backend/cmd/db_backup_scheduler.go`：进程内调度器，按 `db_backup_interval`（默认 `24h`，`0`/`off` 关闭）复用既有 `backupSQLite`/`backupMySQL` 快照 + `keep` 轮转；失败只记 Error + 计数不阻断；`Start/Stop` 幂等，`Stop` 等待进行中的备份收尾。
- 配置：新增 `db_backup_interval`/`db_backup_keep`（env `DBOPS_DB_BACKUP_INTERVAL`/`DBOPS_DB_BACKUP_KEEP`），非法值在 `config.Load` 直接报错（拼写错误不会静默变成关闭备份）。
- 可观测：`/metrics` 新增 `dbops_db_backup_success_total`、`dbops_db_backup_failure_total`、`dbops_db_backup_last_success_timestamp_seconds`。
- **验证**：`go build/vet` 通过；新增 3 个调度器用例（禁用返回 nil、成功/失败计数、Start/Stop 幂等）通过；`TestParseBackupInterval` 覆盖关闭值/合法值/非法值。

### ✅ P0-C 升级链路 fail-fast（2026-10-29）

- `Makefile pre-upgrade-check`：元数据备份不再 `|| true` —— 自动定位实际元数据库（`$DBOPS_DATA_DIR`、`data/`、`db/`、`backend/data/`）与 JSON 持久化文件，备份失败即中止升级；系统服务二进制备份失败也中止；全新安装无元数据时明确提示跳过。
- `rollback`：修复 `[ -f /backup/*.tar.gz ]` glob 判断失效的 bug，改为 `ls` 定位最新归档，解包失败报错退出。
- `test-backup-restore`：修复不存在的 `fail` 命令。
- **验证**：`make -n upgrade`/`make -n rollback` 语法通过；实跑 `make pre-upgrade-check`（后端未启动）→ 立即报错并退出码 2；备份文件收集逻辑单独实测（命中 `data/dbops.db` + `data/hosts.json`，空目录时为空）。

### ✅ P1-A license `RequireFeature` 接线（2026-10-29）

- `pkg/middleware/license.go`：`FeatureChecker` 签名改为 `func(ctx *gin.Context, feature) bool`（用请求 context 查 license，可取消/带超时）；403 返回结构化错误 `error=feature_not_licensed` + `feature` 字段，前端可本地化。
- `cmd/main.go` 新增 `requireFeature()` 帮助函数并挂载到 EE/UE 特性路由：
  - `data_masking` → `/masking` 整组；`key_rotation` → `/keys` 整组；`audit_chain` → `/audit-logs/verify-chain`；
  - `auto_failover` → `/ha/failover`、`/ha/manual-switch`；`chaos_test` → `/faults` 整组；
  - `notify_channel` → 通知渠道写操作与 `/alerts/notifications` 发送；`alert_upgrade` → `/alerts/escalations` 写操作；
  - `ai_assistant` → `/ai/diagnosis*`；`sql_optimize` → `/ai/sql-advisor*`（均为 UE 特性）。
- 未接线项（API 面尚不存在，记录待后续）：`ldap`/`sso`（仅有 `auth.ldap_enabled` 设置项，无专用路由）、`capacity_predict`（无对应路由）。
- **验证**：`go build/vet` 通过；新增 `pkg/middleware/license_test.go`（放行 + 结构化 403）通过。

### ✅ P1-B 前端 token 单轨化（2026-10-29）

- `services/api.ts`：移除从 localStorage 读 token 拼 `Authorization` 的请求拦截器；`withCredentials` + 同源 HttpOnly cookie 成为唯一凭据源；401 时仍清理 `user`（`token` 的 removeItem 保留作历史清理）。
- `pages/Login.tsx`：不再写 `localStorage.token`，改为清理历史遗留；`user`（非敏感展示信息）继续保留。
- `components/ProtectedRoute.tsx`：删除本地 token 格式校验逻辑，会话与否完全由后端 `/auth/me`（cookie）判定，同时清掉遗留 token。
- `services/useTaskSSE.ts`（换票 fetch）、`services/useSettings.ts`、`pages/SecuritySettings.tsx`（relay 系列 fetch）：去掉手工 `Authorization` 头，统一 `credentials: 'include'`。
- **验证**：`npx tsc --noEmit` 通过；`npm test -- --run` 19 文件 307 用例全绿（含更新后的 ProtectedRoute 用例）。

### ✅ P1-C 登录限流迁移 Redis（2026-10-29）

- `pkg/middleware/redis_ratelimit.go`（新增）：`RedisRateLimiter` 用 Redis `INCR + EXPIRE` 固定窗口计数，跨副本共享；单次 Redis 调用带超时（避免 Redis 挂起拖死登录请求）；**Redis 不可达/超时自动回退进程内 `RateLimiter`** 并只告警一次（degraded atomic 标志），限流存储故障绝不打挂登录接口。
- `pkg/middleware/ratelimit.go`：抽出 `KeyLimiter` 接口（`Allow(key) bool`），内存版与 Redis 版共同实现，中间件只依赖抽象；`RateLimiter.Stop()` 改为幂等（`sync.Once` + 等待 cleanup 协程退出，见 P1-D）。
- `cmd/main.go`：构造 Redis 客户端成功则用 `RedisRateLimiter(内存 fallback)`，否则保持纯内存；登录限流与 IP 全局限流都走 `KeyLimiter`。
- 文档：`docs/OPS_MANUAL.md` 8.4 已知限制新增第 7 条 —— 「仅支持单副本」及限流已具备跨副本能力的说明。
- **验证**：新增 `pkg/middleware/redis_ratelimit_test.go` 4 个用例：`AllowAndLimit`（超限拒绝 + 仅首击设过期 + 未降级）、`FallsBackWhenRedisDown`（Redis 故障回退内存限流且 `Degraded()=true`）、`NilSafe`（nil 放行）、`LoginRateLimitDisabledSemantics`（P2-B 开关语义）；`go build/vet`、`go test ./pkg/middleware/...` 通过。

### ✅ P1-D 优雅停机（2026-10-29）

- `internal/services/background.go`（新增）：进程级后台任务登记 —— `GoBackground(fn)` 启动纳入停机等待的任务（panic recover 不拖垮进程），`WaitForBackground(timeout)` **有界**等待（超时放行并告警，绝不因单个任务卡死无限期挂住停机）。
- 改造裸 `go func()`：`host_service`（主机扫描/连接测试）、`instance_service`（实例探测）、`universal_cluster_deploy`（部署编排）、`upgrade_service`（升级派发）改经 `GoBackground` 启动。
- `RateLimiter` 清理协程：`Stop()` 幂等并等待协程退出（`doneCh`），避免停机后仍有 goroutine 持锁。
- `cmd/main.go` 停机顺序：`srv.Shutdown`（排空在途 HTTP/SSE）→ `WaitForBackground(10s)` 等任务落库 → `dbBackupScheduler.Stop()`（等待进行中备份收尾）→ 限流器 `Stop()` → `db.Close()`（SQLite WAL checkpoint + 关连接池）。
- **验证**：新增 `internal/services/background_test.go`（登记任务被等待、超时返回 false、panic 任务不阻塞）；`go build/vet`、`go test ./internal/services/... ./cmd/... ./pkg/middleware/...` 通过。

### ✅ P1-E pprof + 日志目录约定（2026-10-30）

- `cmd/main.go`：挂载标准 Go 剖析端点 `/debug/pprof/*`（index/heap/goroutine/allocs/block/mutex/threadcreate/cmdline/profile/symbol/trace），整组走 `authController.ValidateToken` + `RequirePermission("admin")` —— 堆/协程转储可能含内存凭据，绝不公开；路径与标准一致，可直接 `go tool pprof http://admin:pass@host:8080/debug/pprof/heap` 取数。
- 日志约定：根 `logs/` 已在 `.gitignore`（本地落盘用）；轮转约定文档已存在（`docs/OPS_MANUAL.md` 附录 D：journald + logrotate）；`backend/` 下无遗留日志/`*.exe`（实测 `ls backend/*.log backend/*.exe` 为空）。
- 文档：`docs/OPS_MANUAL.md` 新增 10.6 节（pprof 用法 + 鉴权要求 + 日志目录约定）。
- **验证**：`go build/vet` 通过；**实跑后端**（sqlite + 临时数据目录 + 随机密钥，端口 18080）：
  - 无 token / 伪造 token 访问 `/debug/pprof/heap` 与 `/debug/pprof/` → **401**；`must_change_password` token → 403；
  - admin 登录改密后 → `heap`/`profile?seconds=1`/`goroutine`/`index`/`cmdline` 全部 **200** 且返回真实剖析数据；`/health` 200；
  - 顺带端到端验证 P1-D 停机链：SIGTERM → `Shutting down server...` → `Waiting for background tasks to flush` → `Stopping background jobs...` → `Shutdown complete`，端口释放、进程退出。
  - 测试产物已清理（`bin/platform-test`、`data/pproftest/`、`logs/`）。

### ✅ P2-A agent token 未配置返回 503（2026-10-30）

- `agent/cmd/main.go` `agentAuthMiddleware`：`agentToken == ""` 时 `500` → `503 Service Unavailable`（配置缺失是服务不可用，不应报成内部错误）；凭据缺失/错误仍返 401。
- **验证**：`agent/cmd/main_test.go` 新增 `TestAgentAuthMiddlewareStatusCodes`（未配置→503、缺 bearer→401、错 token→401）；`agent go build/vet/test ./cmd` 全绿。

### ✅ P2-B `LoginRateLimit` 开关语义对齐（2026-10-29，随 P1-C 落地）

- `pkg/middleware/ratelimit.go` 新增 `loginRateLimitDisabled()`：兼容 `1/true/yes/on`（关闭）与 `0/false/no/off/空/其他`（保持开启），不再只认 `=1`；`.env.example` 注释同步说明。
- **验证**：`TestLoginRateLimitDisabledSemantics` 覆盖 9 组取值；随 `go test ./pkg/middleware/...` 通过。

### ✅ P2-C `/health/ready` 纳入 ClickHouse 就绪（2026-10-30）

- `cmd/main.go` 就绪探针在 db/redis 之后新增 `clickhouse` 检查，分三种情形：
  1. 客户端存活 → 实际 `Ping`，失败 → `allOK=false`（ready 返 503，checks 给出错误）—— 运行期 CH 故障不再绿灯；
  2. 客户端为 nil 但**显式配置了**（`DBOPS_CLICKHOUSE_URL` 非空或配置文件含 `clickhouse_url`）→ 503 `configured but unavailable at startup (monitoring standalone; fix CH and restart)` —— 监控降级且不会自愈，必须修复后重启；
  3. 未显式配置 → `"disabled"` 跳过（CH 是可选依赖，与 OPS_MANUAL 10.5 降级表一致，不把无 CH 的合法部署打成不就绪）。
- `pkg/config/config.go`：新增 `Config.ClickHouseConfigured` 字段（`os.Getenv("DBOPS_CLICKHOUSE_URL") != "" || viper.InConfig("clickhouse_url")`）—— 因为 `viper.SetDefault("clickhouse_url", "clickhouse://default@localhost:9000/default")` 恒非空，仅凭 URL 无法区分“运维显式要求 CH”与“代码默认值”。
- **验证**：`go build/vet` + `go test ./pkg/config/... ./cmd/...` 通过；**本地实跑三情形**：未配置→200 `disabled`；配置但 URL 非法→503；配置但 CH 宕机→503（均含 db/redis 交叉核对）。

### ✅ P2-D Makefile `dist` 前端产物路径（2026-10-30）

- `Makefile dist`：`cp frontend/build ... || cp frontend/dist ... || true` 改为先断言 `frontend/dist` 存在（缺失即报错 `run 'make build-web'`），存在则 `cp -r frontend/dist dist/web` —— 不再用 `|| true` 掩盖产物缺失打出空 web 目录。
- 依据：`vite build` 默认 outDir 即 `frontend/dist`，`frontend/Makefile clean` 也只删 `dist`；`frontend/build` 路径并不存在。
- **验证**：`make -n dist` 语法通过；无 `frontend/dist` 时实测片段退出码 1 且报错信息可读；`npm run build` 后 `frontend/dist` 存在。

### ✅ P2-E README 端点描述修正（2026-10-30）

- `README.md`：`Backend Admin / 后台管理` → `Backend API / 后端接口（纯 REST API，无管理界面）` 并补 `/health`、`/health/ready`、`/metrics`；`readme_ZH.md`/`readme_US.md` 原本已写“后端 API / Backend API”，无需改动。

### ✅ P2-F 仓库卫生核对（2026-10-30，无改动）

- 实测：根 `package.json` 不存在；全仓无被 git 跟踪的 `*.exe`/`*.log`/`*.db`/`node_modules`；工作区仅剩 `agent/bin/*.exe` 等 **已被 `.gitignore` 覆盖的构建产物**；`git status` 仅含计划内改动。
- 结论：无需清理，记录为已核对。

### ✅ P2-G 权限模型双轨收敛评估（2026-10-30，按计划仅评估+文档）

- 产出：`docs/OPS_MANUAL.md` 附录 G —— 双轨现状（DB 轨 vs 硬编码轨）、4 项漂移风险（DB 空授权被兑底反向放行、两份硬编码 map 内容不一致、admin 恒放行、自定义角色静默无权限）、4 条收敛建议与当前可依赖的安全性质。
- 未改任何鉴权代码（涉及 98 处 `RequirePermission`，留独立变更窗口）。

### ✅ P2-H `urlguard` 路径穿越校验跨平台修复（2026-10-30）

- **发现**：全量测试在 Linux 下 `pkg/utils` 的 `TestSafeJoinUnderRejectsTraversal` 失败（`..\..\windows`、`C:\Windows` 未被拒绝）。该包在 HEAD 上即失败，属跨平台缺陷：`filepath.Separator` 在 Linux 是 `/`，`filepath.VolumeName` 非 Windows 恒为空，原注释宣称“按平台无关规则拦截”但实现只拦了一半。
- **修复**：`pkg/utils/urlguard.go` `SafeJoinUnder` 把 `"\"` 归一为 `"/"` 后再判：绝对路径 / 盘符（字母+`:`）/ 任意位置的 `..` 段一律拒绝；末尾仍保留 `filepath.Rel` 兜底校验。合法 POSIX 子路径行为不变。
- **验证**：`go test ./pkg/utils/...` 由红转绿；全仓 `go test ./...` 无失败。

### 全量回归（2026-10-30，计划执行完毕后的总验证）

- **Backend**：`go build ./...`、`go vet ./...`、`go test ./...` 全绿（含此前在 Linux 上失败的 `pkg/utils`，由 P2-H 修复）；P2-C 含 config 改动后已重跑。
- **Agent**：`go build ./...`、`go vet ./...`、`go test ./cmd` 全绿（含新增 503 用例）。
- **Frontend**：`npx tsc --noEmit` 通过；`npm test -- --run` 19 文件 307 用例全绿；`npm run build` 成功。
- **运行时（本地实跑验证）**：
  - pprof 鉴权链：无 token/伪造 token → 401，改密前 token → 403，admin token → `heap`/`profile`/`goroutine`/`index`/`cmdline` 全部 200 且返回真实剖析数据；
  - P1-D 优雅停机：SIGTERM → `Shutting down server...` → `Waiting for background tasks to flush` → `Stopping background jobs...` → `Shutdown complete`，端口释放、进程退出；
  - P2-C 就绪三情形：未配置 CH → 200 `disabled`；配置但 URL 非法 → 503；配置但 CH 宕机 → 503；
  - 交付链路（P3-A）：`make dist` 端到端成功，离线包含 bin/config/scripts/web 全套产物；`go run ./cmd migrate` 实跑 OK；
  - 测试产物已全部清理（`bin/platform-test`、`data/`、`logs/` 下临时文件、`dist/` 与离线包）。

### ✅ P3-A 交付链路实跑 + 单文件构建回归修复（2026-10-30，计划外发现）

- **发现**：实跑 `make dist` 时 `make -C backend build` 报 `undefined: newDBBackupScheduler` —— P0-B 把 `db_backup_scheduler.go` 加入 `cmd` 包后，全仓所有 **backend 单文件构建**（`go build/run ./cmd/main.go`）都会漏编译新文件。之前只跑了 `go build ./...`（按包）所以没暴露。
- **修复**：backend 全部改为按包构建 `./cmd`，共 11 处：根 `Makefile`（`run-backend`/`db-migrate`/`upgrade` 的 migrate 回退）、`backend/Makefile`、`backend/Dockerfile`、`deploy/docker/Dockerfile.backend`、`bin/ubuntu/start-backend.sh`、`bin/centos/start-backend.sh`、`bin/windows/start.ps1`、`docs/README.md`（run+build 两处）、`docs/OPS_MANUAL.md`（交叉编译+调试两处）、`readme_ZH.md`、`readme_US.md`。
- **agent 侧不动**：`agent/cmd/` 仍只有 `main.go`（+测试），单文件构建有效。
- **验证**：`make -C backend build` 通过；`make dist` 端到端成功 —— `dist/bin/{dbops-backend,dbops-agent}`、`dist/config`、`dist/scripts`、`dist/web/index.html` 齐全，产出 29MB 离线包 `dbops-offline-20261030.tar.gz`；`go run ./cmd migrate` 实跑输出 `migrate: OK`（exit 0）；`make -n run-backend db-migrate` 语法通过；演练产物（`dist/`、离线包、临时数据目录）已清理。

### 工作区卫生（2026-10-29）

- 发现一次误触发的全仓 `gofmt -w` 对 **56 个无关文件**产生了纯格式化改动（models/plugins/repositories/controllers 等对齐噪音）。已逐一用 `git diff -w` 甄别「纯空白/格式」与「实质改动」，将 56 个无关文件全部 `git checkout` 回滚，仅保留计划内改动集。

## 六、验证基线

```bash
# Backend
cd backend && go build ./... && go vet ./... && go test ./...
# Agent
cd agent && go build ./... && go vet ./... && go test ./cmd
# Frontend
cd frontend && npx tsc --noEmit && npm test -- --run && npm run build
```

### 遗留（本轮未闭环）

- **ldap/sso/capacity_predict 三个特性无路由可接线** —— API 面本身不存在（只有 auth.ldap_enabled／oauth2_enabled 等开关字段），本轮记录为"待后续"，非遗漏而是没有挂载点。
- **权限模型双轨收敛（P2-3）** —— 只完成评估与文档化（OPS_MANUAL 附录 G），代码未改。按计划刻意延后到独立变更窗口（涉及 98 处 RequirePermission）。
- **真实部署链路演练**：systemd/容器实际启动、SSE ticket 端到端、TOFU 首连/密钥变更实测 —— 本 VM 无 systemd/docker/浏览器；已做 `make dist`、migrate、pprof、优雅停机、ready 三情形等可执行部分，**Docker 镜像构建未实跑**。
- 观察项 4（启动即编译 agent）：默认行为仍在，但已文档化（DBOPS_SKIP_AGENT_BINARY_BUILD + 发布产物内置）—— 属"缓解+文档化"而非根除。

