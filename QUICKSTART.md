# MySQL 运维平台 - 快速入门指南

本指南帮助您在 10 分钟内快速启动 MySQL 运维平台并进行基本操作。

## 前提条件

在开始之前，请确保您的系统已安装：

- Docker (>= 20.10)
- Docker Compose (>= 2.0)
- Git

### 检查安装

```bash
docker --version
docker compose version
git --version
```

如果未安装，请参考 [详细安装指南](INSTALL.md)。

---

## 1. 克隆并启动项目

```bash
# 克隆代码仓库
git clone <repository-url>
cd mysql-ops-platform

# 启动所有服务（包括数据库）
make docker-up

# 等待服务启动（约 1-2 分钟）
# 看到 "All services are healthy!" 表示启动成功
```

这将启动以下服务：

| 服务 | 端口 | 说明 |
|------|------|------|
| Backend | 8080 | 后端 API 服务 |
| Frontend | 3000 | 前端 Web Console |
| Agent | 9090 | Agent 执行器 |
| PostgreSQL | 5432 | 数据库 |
| Redis | 6379 | 缓存 |
| ClickHouse | 8123 | 时序数据库 |
| MySQL | 3306 | 测试用 MySQL |

---

## 2. 访问系统

### 方法 1: 通过预览 URL（推荐）

如果您在 MonkeyCode 平台上运行，系统会自动生成预览 URL：

- **前端**: https://3000-xxxxx.monkeycode-ai.online
- **后端**: http://8080-xxxxx.monkeycode-ai.online
- **Agent**: http://9090-xxxxx.monkeycode-ai.online

### 方法 2: 本地访问

```bash
# 前端
open http://localhost:3000

# 或使用浏览器访问
# http://localhost:3000
```

---

## 3. 创建第一个 MySQL 实例

### 3.1 通过 Web 界面

1. 打开前端页面，默认会跳转到登录页面
2. 输入任意用户名和密码（Standalone 模式下不验证）
3. 登录后进入仪表板
4. 点击左侧菜单 "实例管理"
5. 点击 "添加实例" 按钮
6. 填写实例信息：
   - **名称**: my-test-db
   - **主机**: localhost
   - **端口**: 3306
   - **用户名**: root
   - **密码**: (留空或输入测试密码)
   - **版本**: MySQL 8.0
   - **类型**: 单机
7. 点击 "保存"

### 3.2 通过 API

```bash
curl -X POST http://localhost:8080/api/v1/instances \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-test-db",
    "host": "localhost",
    "port": 3306,
    "username": "root",
    "password": "",
    "version": "8.0",
    "type": "standalone"
  }'
```

---

## 4. 使用参数模板

### 4.1 查看预设模板

```bash
curl http://localhost:8080/api/v1/parameter-templates/presets
```

返回示例：
```json
{
  "templates": [
    {
      "id": "small-instance",
      "name": "小实例模板",
      "description": "适用于小规模实例",
      "version": "8.0"
    },
    {
      "id": "large-instance",
      "name": "大实例模板",
      "description": "适用于大规模实例",
      "version": "8.0"
    }
  ]
}
```

### 4.2 应用参数模板

1. 进入 "参数模板" 页面
2. 选择 "大实例模板"
3. 点击 "应用"
4. 选择目标实例
5. 点击 "确认"

---

## 5. 执行备份

### 5.1 创建备份任务

```bash
curl -X POST http://localhost:8080/api/v1/backups \
  -H "Content-Type: application/json" \
  -d '{
    "instance_id": "1",
    "backup_type": "full",
    "method": "physical"
  }'
```

### 5.2 查看备份列表

```bash
curl http://localhost:8080/api/v1/backups
```

---

## 6. 设置告警规则

### 6.1 创建告警规则

1. 进入 "告警规则" 页面
2. 点击 "添加规则"
3. 配置规则：
   - **规则名称**: CPU 使用率告警
   - **指标类型**: CPU
   - **阈值**: 80
   - **持续时间**: 5 分钟
   - **通知方式**: 邮件
4. 保存规则

### 6.2 查看告警

```bash
curl http://localhost:8080/api/v1/alerts
```

---

## 7. 查看监控指标

### 7.1 通过 Web 界面

1. 点击左侧菜单 "监控仪表板"
2. 选择要监控的实例
3. 查看实时指标：
   - QPS（每秒查询数）
   - TPS（每秒事务数）
   - 连接数
   - CPU 使用率
   - 内存使用率
   - 磁盘 I/O

### 7.2 通过 API

```bash
# 获取实例监控指标
curl http://localhost:8080/api/v1/monitoring/metrics?instance_id=1

# 返回示例
{
  "qps": 150,
  "tps": 30,
  "connections": 50,
  "cpu_usage": 45.5,
  "memory_usage": 62.3,
  "disk_io": {
    "read": 1024000,
    "write": 512000
  }
}
```

---

## 8. 查看 Agent 状态

```bash
# Agent 健康检查
curl http://localhost:9090/health

# 返回示例
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 3600
}

# 获取 Agent 上报的指标
curl -X POST http://localhost:9090/agent/tasks/metrics \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

## 9. 常用 API 快速参考

### 实例管理

```bash
# 获取实例列表
GET /api/v1/instances

# 获取实例详情
GET /api/v1/instances/{id}

# 创建实例
POST /api/v1/instances

# 更新实例
PUT /api/v1/instances/{id}

# 删除实例
DELETE /api/v1/instances/{id}
```

### 参数模板

```bash
# 获取参数模板列表
GET /api/v1/parameter-templates

# 获取预设模板
GET /api/v1/parameter-templates/presets

# 创建自定义模板
POST /api/v1/parameter-templates

# 应用模板到实例
POST /api/v1/parameter-templates/{id}/apply
```

### 备份管理

```bash
# 获取备份列表
GET /api/v1/backups

# 创建备份
POST /api/v1/backups

# 恢复备份
POST /api/v1/backups/{id}/restore
```

### 告警管理

```bash
# 获取告警规则
GET /api/v1/alerts/rules

# 创建告警规则
POST /api/v1/alerts/rules

# 获取告警列表
GET /api/v1/alerts

# 确认告警
PUT /api/v1/alerts/{id}/acknowledge
```

### 监控指标

```bash
# 获取监控指标
GET /api/v1/monitoring/metrics

# 获取实例健康状态
GET /api/v1/monitoring/health/{instance_id}
```

---

## 10. 停止服务

```bash
# 停止所有服务
make docker-down

# 或者
docker-compose -f docker-compose.dev.yml down
```

---

## 下一步

现在您已经熟悉了基本操作，可以：

1. **深入学习**: 阅读 [详细安装指南](INSTALL.md) 了解更多配置选项
2. **生产部署**: 参考 [README.md](README.md) 的生产环境部署部分
3. **功能探索**: 尝试更多功能如集群部署、版本升级、数据迁移等
4. **贡献代码**: 查看项目规范，提交 Pull Request

---

## 常见问题

### Q: 服务启动后无法访问？

**A**: 检查服务是否正常启动：
```bash
docker-compose -f docker-compose.dev.yml ps
```

### Q: 提示数据库连接失败？

**A**: 等待数据库服务启动完成，约需 30 秒：
```bash
docker-compose -f docker-compose.dev.yml logs postgres
```

### Q: Agent 任务执行失败？

**A**: 检查 Agent 日志和工具安装：
```bash
docker-compose -f docker-compose.dev.yml logs agent
```

### Q: 如何查看详细日志？

**A**:
```bash
# 查看所有日志
docker-compose -f docker-compose.dev.yml logs -f

# 查看特定服务
docker-compose -f docker-compose.dev.yml logs -f backend
docker-compose -f docker-compose.dev.yml logs -f agent
docker-compose -f docker-compose.dev.yml logs -f frontend
```

---

## 获取更多帮助

- 📖 **详细文档**: [INSTALL.md](INSTALL.md)
- 📋 **需求文档**: `.monkeycode/specs/mysql-ops-platform/requirements.md`
- 🏗️ **技术设计**: `.monkeycode/specs/mysql-ops-platform/design.md`
- ✅ **验收标准**: `.monkeycode/specs/mysql-ops-platform/tasklist.md`

---

**快速入门指南** | **版本**: 1.0 | **最后更新**: 2026-05-30