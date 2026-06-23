# check4.md — MySQL Ops Platform 工程审计整改清单

> 审计方法：Andrej Karpathy 软件工程最佳实践  
> 审计范围：Architecture / Security / Error Handling / Testing / Performance / Go Best Practices  
> 生成时间：2026-06-23

---

## 一、审计总览

| 严重性 | 数量 | 说明 |
|--------|------|------|
| 🔴 Critical | 4 | 必须立即修复，存在安全漏洞或数据损坏风险 |
| 🟠 High | 6 | 需尽快修复，安全/架构风险 |
| 🟡 Medium | 7 | 计划内修复，影响可靠性/可维护性 |
| 🟢 Low | 3 | 改善类，不阻塞发布 |

---

## 二、Critical（4 项）

### C1. SQL 注入 — escapeSQL 仅转义单引号

| 属性 | 值 |
|------|-----|
| 文件 | `agent/internal/executor/task_executor.go:4143`, `platform-backend/internal/services/instance_service.go:1385` |
| 问题 | `escapeSQL` 仅做 `strings.ReplaceAll(s, "'", "''")`，未处理反斜杠、NUL 字节、多字节字符攻击 |
| 使用场景 | `CREATE USER`, `ALTER USER`, `GRANT` 等 18 处动态 SQL 拼接 |
| **修复方案** | ① Agent 侧：用 `--defaults-extra-file` 临时文件传递密码，避免 SQL 拼接；② 至少增加反斜杠转义：`ReplaceAll(ReplaceAll(s, "'", "''"), "\\", "\\\\")` ；③ 长期：改用参数化 `db.Exec` |
| 工作量 | 2h |

### C2. 硬编码默认密码

| 属性 | 值 |
|------|-----|
| 文件 | `agent/internal/executor/tool_installer.go:1351` (`Root@123`), `task_executor.go:1056` (`repl123`), `platform-backend/pkg/config/config.go:76` (`Repl#2024!ChangeMe`) |
| 问题 | 缺省密码未清零，生产环境可能使用弱凭据部署集群 |
| **修复方案** | ① 删除所有默认密码字符串；② 若调用方未传入凭据，函数应 `return error` 而非静默填充；③ `validateSecrets()` 已有启动时校验——扩展到部署凭据 |
| 工作量 | 1h |

### C3. 加密失败静默忽略

| 属性 | 值 |
|------|-----|
| 文件 | `platform-backend/internal/services/instance_service.go:914`, `1054`, `1284` |
| 问题 | `conn.PasswordEncrypted, _ = utils.Encrypt(...)` — 错误被丢弃，存入空密文，导致后续解密失败、凭据永久丢失 |
| **修复方案** | 检查 `Encrypt` 返回的 `error`，若非 nil 则中止操作并向上返回 |
| 工作量 | 0.5h |

### C4. HTTP 响应体 defer 泄漏

| 属性 | 值 |
|------|-----|
| 文件 | `platform-backend/internal/services/agent_client.go:217` |
| 问题 | `for` 循环内 `defer resp.Body.Close()` — defer 在函数返回时才执行，3 次重试累积 3 个未关闭的 Body（各最大 10MB） |
| **修复方案** | 将 `defer` 改为循环末尾显式 `resp.Body.Close()`，或用闭包包装循环体 |
| 工作量 | 0.5h |

---

## 三、High（6 项）

### H1. AES-GCM 密钥派生仅用单次 SHA-256

| 属性 | 值 |
|------|-----|
| 文件 | `platform-backend/pkg/utils/crypto.go:13-16` |
| 问题 | `sha256.Sum256([]byte(passphrase))` 无盐无迭代，低熵口令可被暴力破解 |
| **修复方案** | 改用 `golang.org/x/crypto/argon2` + 随机盐（盐随密文存储）。当前 `EncryptionKey` 要求 32+ 字符缓解了直接风险，但模式脆弱 |
| 工作量 | 2h |

### H2. Agent Token 以明文写入远程磁盘

| 属性 | 值 |
|------|-----|
| 文件 | `platform-backend/internal/services/host_service.go:706` |
| 问题 | `cat > /opt/dbops-agent/config.yaml` 包含明文 `agent_token` |
| **修复方案** | 写入后 `chmod 600`，或用文件路径引用 + 独立 secrets 目录 |
| 工作量 | 1h |

### H3. MYSQL_PWD 环境变量暴露凭据

| 属性 | 值 |
|------|-----|
| 文件 | `task_executor.go`, `mha_executor.go`, `pxc_executor.go` — 共 18 处 |
| 问题 | `cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)` — 可通过 `/proc/<pid>/environ` 读取；`mha_executor.go:482` 更严重——密码注入 shell 命令，`ps aux` 可见 |
| **修复方案** | 改用 `--defaults-extra-file=<tmpfile>`（0600 权限）；shell 命令用 `--defaults-group-suffix` 或 expect 脚本 |
| 工作量 | 3h |

### H4. 权限映射重复定义，存在漂移风险

| 属性 | 值 |
|------|-----|
| 文件 | `platform-backend/pkg/middleware/middleware.go:123` vs `internal/services/auth_service.go:192` |
| 问题 | 两处各自定义 `rolePermissions` map，新增角色/权限时易遗漏其一 |
| **修复方案** | 抽取到 `pkg/authz/` 包，提供 `HasPermission(role, perm) bool` 单一入口 |
| 工作量 | 1h |

### H5. Controller 忽略 JSON 绑定错误

| 属性 | 值 |
|------|-----|
| 文件 | `platform-backend/internal/controllers/host_controller.go:175` |
| 问题 | `_ = ctx.ShouldBindJSON(&req)` — 错误被丢弃，malformed 请求体静默使用零值 |
| **修复方案** | `if err := ctx.ShouldBindJSON(&req); err != nil { utils.BadRequestResponse(ctx, "..."); return }` |
| 工作量 | 0.5h |

### H6. 文档中包含真实密码

| 属性 | 值 |
|------|-----|
| 文件 | `docs/OPS_MANUAL.md:363,411` |
| 问题 | 示例含 `hcfc!2017`, `Test@123456`, `App@2024` 等看起来真实的密码 |
| **修复方案** | 替换为 `example_password_123` 等虚构值；若曾提交过真实凭据，用 BFG 清理 git 历史 |
| 工作量 | 0.5h |

---

## 四、Medium（7 项）

### M1. Fire-and-Forget Goroutine 无优雅关闭

| 属性 | 值 |
|------|-----|
| 文件 | `host_service.go:181,255`, `upgrade_service.go:830` |
| 问题 | goroutine 无 `sync.WaitGroup` / 无 cancel 传播，SIGTERM 时任务永久卡在 "running" |
| **修复方案** | 传入 server shutdown context；用 `errgroup` 或 `WaitGroup` 追踪活跃操作 |
| 工作量 | 2h |

### M2. 固定 `time.Sleep` 替代轮询

| 属性 | 值 |
|------|-----|
| 文件 | `task_executor.go` 15 处 + 服务层 13 处 |
| 问题 | `time.Sleep(5 * time.Second)` 等待 MySQL 启动——慢系统不够、快系统浪费；不响应 context 取消 |
| **修复方案** | 实现 `waitForCondition(ctx, checkFn, interval, timeout)` 轮询函数替代所有 sleep |
| 工作量 | 3h |

### M3. Service 层使用 `context.Background()`

| 属性 | 值 |
|------|-----|
| 文件 | `backup_service.go:486`, `upgrade_service.go:837`, `host_service.go:186` 等 8 处 |
| 问题 | 丢弃上游 context，链路追踪和中间件超时失效 |
| **修复方案** | 透传调用方 context；fire-and-forget 场景用 `context.WithTimeout(ctx, ...)` 派生 |
| 工作量 | 2h |

### M4. Plugin 接口使用 `map[string]interface{}`

| 属性 | 值 |
|------|-----|
| 文件 | `plugins/plugin.go:18` 及 100+ 下游使用点 |
| 问题 | 无编译期类型安全，消费者必须做不安全类型断言 |
| **修复方案** | 定义具体参数结构体（`KernelPluginParams`, `ArchPluginParams` 等）替代 map |
| 工作量 | 3h |

### M5. 生产代码中 `panic()`

| 属性 | 值 |
|------|-----|
| 文件 | `agent/pkg/logger/logger.go:24`, `platform-backend/pkg/logger/logger.go:24`, `pkg/license/license.go:88` |
| 问题 | logger 初始化失败 / license 校验失败时 panic，无法优雅关闭 |
| **修复方案** | 返回 `error`；logger fallback 到 `log.Default()` |
| 工作量 | 1h |

### M6. 部署时 `kill -9` 杀死所有 mysqld 进程

| 属性 | 值 |
|------|-----|
| 文件 | `task_executor.go:681-693` |
| 问题 | `ps aux \| grep '[m]ysqld'` 杀死主机上**所有** MySQL 进程，多实例环境灾难性；错误被静默忽略 |
| **修复方案** | 仅按目标端口/PID 文件定位并杀死特定实例 |
| 工作量 | 1h |

### M7. 无路径穿越校验

| 属性 | 值 |
|------|-----|
| 文件 | `config_writer.go:32`, `jsonstore.go:39`, `config_renderer.go:61` |
| 问题 | 若输入含 `../`，可在预期目录外读写文件 |
| **修复方案** | `filepath.Join` 后校验 `strings.HasPrefix(filepath.Clean(full), filepath.Clean(base))` |
| 工作量 | 1h |

---

## 五、Low（3 项）

### L1. 无 `t.Parallel()` — 88 个测试文件全部串行

**修复方案**：为无共享可变状态的测试添加 `t.Parallel()`，预计 CI 时间减少 40-60%。  
工作量：2h

### L2. 99% 白盒测试 — 87/88 个测试文件使用同包名

**修复方案**：新增测试优先使用 `_test` 后缀包名，通过公共 API 测试。  
工作量：持续

### L3. 大文件 / 高圈复杂度

| 文件 | 行数 |
|------|------|
| `task_executor.go` | 4,273 |
| `universal_cluster_deploy.go` | 1,712 |
| `cluster_deploy_service.go` | 1,664 |
| `switch_service.go` | 1,151 |

**修复方案**：按职责拆分 `task_executor.go`（deploy / backup / restore / migration / upgrade / health）。  
工作量：4h

---

## 六、正面观察 ✅

| 项目 | 评价 |
|------|------|
| 启动时密钥校验 (`validateSecrets`) | 正确拒绝弱 JWT/加密密钥/Agent Token |
| CORS 白名单 | 使用显式允许列表而非通配符 |
| 登录限流 | 5 req/min per IP，防止暴力破解 |
| 测试隔离 | 原子计数器 SQLite 模式确保每个测试独立数据库 |
| JWT 签名方法验证 | 正确拒绝非 HMAC 签名方法 |
| AES-256-GCM 加密 | 正确使用随机 nonce，提供机密性+完整性 |
| 连接池调优 | MaxOpenConns/MaxIdle/MaxLifetime 均已配置 |
| 请求体限制 | 10MB multipart 限制防止大上传 DoS |

---

## 七、优先修复路线

```
Week 1 — 🔴 Critical 安全修复
  C1 escapeSQL 强化          → 2h
  C2 删除硬编码默认密码       → 1h
  C3 加密错误不忽略           → 0.5h
  C4 HTTP Body defer 泄漏    → 0.5h
  H5 JSON 绑定错误检查       → 0.5h
  H6 文档密码替换             → 0.5h
                              ─────
                              5h

Week 2 — 🟠 High 安全加固
  H1 密钥派生改 argon2        → 2h
  H2 Agent Token 文件权限      → 1h
  H3 消除 MYSQL_PWD 环境变量   → 3h
  H4 权限映射去重              → 1h
  M7 路径穿越校验              → 1h
                              ─────
                              8h

Week 3 — 🟡 Medium 可靠性
  M2 time.Sleep → waitForCondition  → 3h
  M3 context.Background 透传         → 2h
  M6 kill -9 精确化                  → 1h
  M5 panic → error                   → 1h
                                     ─────
                                     7h

Week 4 — 🟡 Medium 架构改进
  M1 goroutine 优雅关闭      → 2h
  M4 Plugin 参数类型化        → 3h
  L3 task_executor.go 拆分    → 4h
  L1 t.Parallel 化            → 2h
                               ─────
                               11h
```

**总计预估工时：~31h（约 4 个工作日）**

---

## 八、测试覆盖率参考

| 包 | 覆盖率 | 状态 |
|----|--------|------|
| `agent/internal/executor` | 32.4% | ⚠️ 1 个预存测试失败 |
| `platform-backend/internal/plugins` | 64.3% | ✅ |
| `platform-backend/internal/plugins/arch` | 73.4% | ✅ |
| `platform-backend/internal/plugins/kernel` | 74.3% | ✅ |
| `platform-backend/internal/plugins/middleware` | 68.0% | ✅ |
| `platform-backend/internal/repositories` | 30.4% | ✅ 编译通过 |
| `platform-backend/internal/controllers` | 1.5% | 🔴 极低 |
| `platform-backend/internal/services` | ~6% | ⚠️ 5 个预存端口冲突测试 |
