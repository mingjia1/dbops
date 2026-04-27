# MySQL DBA 平台

一个基于 Python Django + Vue.js 的 MySQL 数据库智能运维平台。

## 功能特性

### 核心功能

- **实例管理**: MySQL 实例全生命周期管理，支持单机/主从/PXC/MGR/MHA 等多种架构
- **监控告警**: 实时监控 MySQL 性能指标，支持阈值告警和慢查询分析
- **数据同步**: 支持全量/增量同步，单向/双向同步，自动冲突解决
- **版本升级**: MySQL 版本升级管理，支持原地/蓝绿等多种升级策略
- **AI 能力**: 集成 AI 大模型，提供 SQL 优化、故障诊断、索引建议等智能功能
- **安全加固**: 密码策略、访问控制、数据加密、安全审计

### 技术栈

**后端:**
- Python 3.11 + Django 5.2
- Django REST Framework 3.15
- Celery 5.3 (任务调度)
- MariaDB 10.11 (元数据存储)
- Redis 7.0 (缓存/Celery Broker)

**前端:**
- Vue 3 + Vite 5
- Element Plus 2.5 (UI 组件库)
- Pinia 2 (状态管理)
- ECharts 5 (图表可视化)

## 快速开始

### 环境要求

- Python 3.11+
- Node.js 18+
- MariaDB 10.11+ 或 MySQL 8.0+
- Redis 7.0+

### 安装依赖

```bash
# 安装系统依赖
apt-get install -y mariadb-server redis-server

# 启动数据库和 Redis
service mariadb start
service redis-server start

# 创建数据库
mysql -u root -e "CREATE DATABASE mysql_dba_platform CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -u root -e "CREATE USER 'django'@'localhost' IDENTIFIED BY 'django_password_2026';"
mysql -u root -e "GRANT ALL PRIVILEGES ON mysql_dba_platform.* TO 'django'@'localhost';"
mysql -u root -e "FLUSH PRIVILEGES;"

# 安装 Python 依赖
pip3 install --break-system-packages -r requirements.txt

# 安装前端依赖
cd frontend
npm install
```

### 初始化数据库

```bash
python3 manage.py migrate
```

### 创建管理员用户

```bash
python3 manage.py shell -c "
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
# 启动后端（端口 8000）
python3 manage.py runserver 0.0.0.0:8000

# 启动前端（新终端，端口 3000）
cd frontend
npm run dev
```

访问 http://localhost:3000 使用 admin / SuperAdmin@2026 登录。

## 项目结构

```
.
├── accounts/           # 用户认证模块
├── instances/          # 实例管理 + 数据同步 + 版本升级
├── monitor/            # 监控告警模块
├── ai_service/         # AI 能力模块
├── security/           # 安全加固模块
├── frontend/           # Vue 3 前端
├── requirements.txt    # Python 依赖
└── manage.py          # Django 管理脚本
```

## API 文档

### 用户认证
- `POST /api/accounts/login/` - 用户登录
- `POST /api/accounts/logout/` - 用户注销
- `POST /api/accounts/register/` - 用户注册
- `GET /api/accounts/me/` - 当前用户信息

### 实例管理
- `GET/POST /api/instances/` - 实例列表/创建
- `GET/PUT/DELETE /api/instances/<id>/` - 实例详情/更新/删除
- `POST /api/instances/<id>/test_connection/` - 测试连接

### 监控告警
- `GET /api/monitor/metric-data/` - 指标数据
- `GET /api/monitor/alerts/` - 告警列表
- `POST /api/monitor/alerts/<id>/acknowledge/` - 确认告警
- `GET /api/monitor/slow-queries/` - 慢查询

### 数据同步
- `GET/POST /api/sync-tasks/` - 同步任务管理
- `POST /api/sync-tasks/<id>/start/` - 启动任务
- `POST /api/sync-tasks/<id>/stop/` - 停止任务

### 版本升级
- `GET/POST /api/upgrade-plans/` - 升级计划管理
- `POST /api/upgrade-plans/<id>/start/` - 开始执行

### AI 能力
- `GET/POST /api/ai/chats/` - AI 会话
- `POST /api/ai/chats/<id>/message/` - 发送消息
- `GET/POST /api/ai/advices/` - AI 建议

### 安全管理
- `GET /api/security/security-audits/` - 安全审计
- `GET/POST /api/security/encryption-keys/` - 加密密钥
- `GET /api/security/dashboard/` - 安全仪表盘

## 开发指南

### 添加新模块

1. 创建 Django app: `python3 manage.py startapp xxx`
2. 在 `settings.py` 中注册 app
3. 创建模型并生成迁移：`python3 manage.py makemigrations`
4. 运行迁移：`python3 manage.py migrate`
5. 创建 API views 和 serializers
6. 配置 URL 路由

### 前端组件开发

```bash
cd frontend
npm run dev
```

组件模板位于 `src/views/`, 复用组件位于 `src/components/`。

## 部署

### 生产环境部署

```bash
# 后端 (使用 gunicorn)
pip3 install gunicorn
gunicorn mysql_dba_platform.wsgi:application --bind 0.0.0.0:8000 --workers 4

# 前端 (构建静态文件)
cd frontend
npm run build
# 部署 dist/ 目录到 Nginx
```

### Docker 部署

```dockerfile
# TODO: 添加 Dockerfile
```

## 许可证

MIT License

## 联系方式

- 项目地址：https://github.com/mingjia1/dbops
- 问题反馈：https://github.com/mingjia1/dbops/issues
