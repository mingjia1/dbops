# MySQL DBA 平台

一个基于 Python Django + Vue 3 的 MySQL 数据库智能运维平台，支持数据库部署、监控告警、版本升级、数据同步、AI 智能诊断和安全加固。

## 功能特性

### 核心模块

| 模块 | 功能 |
|------|------|
| **实例管理** | MySQL 实例全生命周期 CRUD，支持单机/主从/PXC/MGR/MHA 等多种架构，连接测试与元数据采集 |
| **数据库部署** | 通过 SSH + apt/yum 自动安装 MySQL 单机或集群，8 步标准流程（环境检查→安装→初始化→启动→密码设置→验证），集群模式额外 2 步（加入集群→集群配置） |
| **监控告警** | MySQL 性能指标采集与可视化（ECharts 趋势图），阈值告警规则配置与告警事件管理，慢查询分析 |
| **数据同步** | 全量/增量同步，单向/双向同步，自动冲突检测与解决 |
| **版本升级** | MySQL 版本升级全流程管理（原地/蓝绿），升级前检查清单与 SSH 远程执行引擎 |
| **参数管理** | MySQL 参数模板、实例参数对比、参数变更历史 |
| **AI 能力** | 集成 AI 大模型（OpenAI/DeepSeek），SQL 生成与优化、故障诊断、索引建议，内置多轮会话 |
| **安全加固** | 密码策略、访问控制、数据加密（Fernet）、安全审计日志、会话管理 |

### 技术栈

**后端:**
- Python 3.11 + Django 5.2.14
- Django REST Framework 3.17
- SimpleJWT (JWT 认证)
- Celery 5.3 (任务调度)
- MariaDB 10.11 / MySQL 8.0+ (生产), SQLite (开发)
- Redis 7.0 (缓存/Celery Broker)
- paramiko 5.0 (SSH 远程执行)
- cryptography (Fernet 加密)

**前端:**
- Vue 3 + Vite 5
- Element Plus 2.5+ (UI 组件库)
- Pinia 2 (状态管理)
- ECharts 5 (图表可视化)
- Axios (HTTP 客户端)

## 快速开始

### 环境要求

- Python 3.11+
- Node.js 18+
- MariaDB 10.11+ 或 MySQL 8.0+（可选，默认使用 SQLite 开发）
- Redis 7.0+（可选，默认使用内存 Broker）

### 安装依赖

```bash
# 安装 Python 依赖
pip install -r requirements.txt

# 安装前端依赖
cd frontend
npm install
```

### 初始化数据库

```bash
python manage.py migrate
```

### 创建管理员用户

```bash
python manage.py shell -c "
from accounts.models import User, Role
admin_role = Role.objects.get(code='admin')
admin = User.objects.create_user(username='superadmin', password='SuperAdmin@2026', email='admin@example.com')
admin.role = admin_role
admin.is_staff = True
admin.is_superuser = True
admin.save()
print('管理员用户创建成功')
"
```

### 启动服务

```bash
# 开发模式
python manage.py runserver 0.0.0.0:8000     # 后端 (端口 8000)
cd frontend && npm run dev                    # 前端 (端口 3000)

# 生产模式 (Linux)
bash start.sh                                 # 启动后端+前端
bash stop.sh                                  # 停止服务

# 生产模式 (Windows)
start.bat                                     # 启动后端+前端
stop.bat                                      # 停止服务
```

访问 http://localhost:3000 使用 `superadmin` / `SuperAdmin@2026` 登录。

## 项目结构

```
.
├── accounts/              # 用户认证（登录/注册/角色/会话）
├── instances/             # 核心模块
│   ├── models/            #   模型包（Instance / 同步 / 升级 / 部署 / 参数）
│   ├── deploy_engine.py   #   部署引擎（SSH + apt/yum 安装 MySQL）
│   ├── deploy_views.py    #   部署 API
│   ├── deploy_serializers.py
│   └── tests_deploy.py    #   部署模块测试（43 个）
├── monitor/               # 监控告警（指标/告警规则/慢查询）
├── ai_service/            # AI 能力（多厂商 AI 会话/建议/SQL 优化）
├── security/              # 安全加固（审计/加密/密码策略/访问控制）
├── frontend/              # Vue 3 前端
│   └── src/
│       ├── api/           #   后端 API 封装
│       ├── views/         #   页面组件
│       └── router/        #   前端路由
├── tests/                 # 全量集成测试（安全/权限/加密/配置）
├── upgrade_engine.py      # 版本升级执行引擎（SSH）
├── start.sh / stop.sh     # Linux 启停脚本
├── start.bat / stop.bat   # Windows 启停脚本
├── requirements.txt       # Python 依赖
├── .github/workflows/     # CI 配置
└── manage.py              # Django 管理脚本
```

## API 文档

### 用户认证
| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/accounts/login/` | 用户登录，返回 JWT Token |
| POST | `/api/accounts/logout/` | 用户注销 |
| POST | `/api/accounts/register/` | 用户注册 |
| GET | `/api/accounts/me/` | 当前用户信息 |
| POST | `/api/accounts/change-password/` | 修改密码 |

### 实例管理
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/instances/` | 实例列表/创建 |
| GET/PUT/DELETE | `/api/instances/{id}/` | 实例详情/更新/删除 |
| POST | `/api/instances/{id}/test_connection/` | 测试连接 |
| GET | `/api/dashboard/` | 实例仪表盘 |
| GET | `/api/metadata-tree/` | 元数据树 |

### 数据库部署
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/deploy-plans/` | 部署计划列表/创建 |
| POST | `/api/deploy-plans/{id}/approve/` | 批准计划 |
| POST | `/api/deploy-plans/{id}/start/` | 启动部署（创建 8/10 步） |
| POST | `/api/deploy-plans/{id}/cancel/` | 取消计划 |
| POST | `/api/deploy-plans/{id}/reopen/` | 重新打开失败计划 |
| GET | `/api/deploy-plans/statistics/` | 计划统计 |
| GET/POST | `/api/deploy-hosts/` | 主机节点管理 |
| GET | `/api/deploy-executions/` | 执行记录列表 |
| POST | `/api/deploy-executions/{id}/execute_step/` | 单步执行（含重试） |
| POST | `/api/deploy-executions/{id}/run/` | 批量执行全部步骤 |
| POST | `/api/deploy-executions/{id}/confirm_init_data/` | 确认初始化数据（覆盖/换目录） |
| GET | `/api/deploy-executions/{id}/steps/` | 查看步骤详情 |
| GET | `/api/deploy-dashboard/` | 部署仪表盘 |

部署流程（单机 8 步）：
`环境检查` → `安装 MySQL` → `创建目录` → `初始化配置` → **`初始化数据`** → `启动服务` → `设置密码` → `部署验证`

### 监控告警
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/monitor/metric-definitions/` | 指标定义管理 |
| GET/POST | `/api/monitor/metric-data/` | 指标数据 |
| GET/POST | `/api/monitor/alert-rules/` | 告警规则管理 |
| GET/POST | `/api/monitor/alerts/` | 告警事件列表 |
| POST | `/api/monitor/alerts/{id}/acknowledge/` | 确认告警 |
| POST | `/api/monitor/alerts/{id}/resolve/` | 解决告警 |
| GET | `/api/monitor/slow-queries/` | 慢查询列表 |
| POST | `/api/monitor/slow-queries/{id}/explain/` | SQL 执行计划分析 |
| GET | `/api/monitor/dashboard/` | 监控仪表盘 |

### 数据同步
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/sync-tasks/` | 同步任务管理 |
| POST | `/api/sync-tasks/{id}/start/` | 启动任务 |
| POST | `/api/sync-tasks/{id}/stop/` | 停止任务 |
| GET | `/api/sync-dashboard/` | 同步仪表盘 |

### 版本升级
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/upgrade-plans/` | 升级计划管理 |
| POST | `/api/upgrade-plans/{id}/approve/` | 批准升级 |
| POST | `/api/upgrade-plans/{id}/start/` | 开始执行 |
| POST | `/api/upgrade-plans/{id}/rollback/` | 回滚 |
| GET | `/api/upgrade-dashboard/` | 升级仪表盘 |

### 参数管理
| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/mysql-parameters/` | MySQL 系统参数列表 |
| GET/POST | `/api/parameter-templates/` | 参数模板 CRUD |
| POST | `/api/parameter-templates/{id}/apply/` | 应用参数模板 |
| GET/POST | `/api/instance-parameters/` | 实例参数管理 |
| POST | `/api/instance-parameters/sync/` | 同步实例参数 |
| POST | `/api/instance-parameters/{id}/apply/` | 应用参数变更 |
| GET | `/api/parameter-dashboard/` | 参数仪表盘 |

### AI 能力
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/ai/providers/` | AI 提供商配置管理 |
| POST | `/api/ai/providers/{id}/test_connection/` | 测试 API 连接 |
| GET/POST | `/api/ai/chats/` | AI 会话列表/创建 |
| POST | `/api/ai/chats/{id}/message/` | 发送消息（调 AI API） |
| GET/POST | `/api/ai/advices/` | AI 建议列表 |
| POST | `/api/ai/advices/{id}/acknowledge/` | 确认建议 |
| POST | `/api/ai/advices/{id}/resolve/` | 解决建议 |
| POST | `/api/ai/generate-sql/` | SQL 生成 |
| POST | `/api/ai/optimize-sql/` | SQL 优化 |
| POST | `/api/ai/diagnose/` | 故障诊断 |
| GET | `/api/ai/dashboard/` | AI 服务仪表盘 |

### 安全管理
| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/api/security/security-audits/` | 安全审计（含导出） |
| GET/POST | `/api/security/encryption-keys/` | 加密密钥管理（含轮换/吊销） |
| GET/POST | `/api/security/password-policies/` | 密码策略管理 |
| GET/POST | `/api/security/access-rules/` | 访问控制规则（含评估） |
| GET/POST | `/api/security/audit-logs/` | 审计日志 |
| GET/POST | `/api/security/session-policies/` | 会话策略 |
| GET | `/api/security/dashboard/` | 安全仪表盘 |

## 测试

```bash
# 运行全量测试（236 个）
python -m pytest accounts/ security/ instances/ ai_service/ monitor/ tests/ -v

# 仅部署模块测试（43 个）
python -m pytest instances/tests_deploy.py -v

# 仅 AI 模块测试（8 个）
python -m pytest ai_service/ -v

# 运行 Django 系统检查
python manage.py check
```

测试覆盖：6 个测试模块，236 个测试用例，涵盖部署引擎、权限边界、状态机、加密、SQL 安全、API 端点等。

## 开发指南

### 添加新模块

1. 创建 Django app: `python manage.py startapp xxx`
2. 在 `settings.py` 中注册 app
3. 创建模型并生成迁移：`python manage.py makemigrations`
4. 运行迁移：`python manage.py migrate`
5. 创建 API views 和 serializers
6. 配置 URL 路由

## 许可证

MIT License

## 联系方式

- 项目地址：https://github.com/mingjia1/dbops
- 问题反馈：https://github.com/mingjia1/dbops/issues
