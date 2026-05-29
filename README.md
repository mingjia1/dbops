# MySQL 运维平台

MySQL 运维平台是一个大厂级别的 MySQL 数据库全生命周期管理平台，提供从环境检测、实例部署、版本升级、数据迁移到运维监控的完整解决方案。

## 项目结构

```
mysql-ops-platform/
├── platform-backend/     # 后端平台服务
├── agent/                # Agent 执行器
├── web-console/          # 前端 Web Console
├── docker-compose.dev.yml # 开发环境 Docker 配置
├── Makefile              # 构建脚本
└── .monkeycode/          # 规范文档
    └── specs/
        └── mysql-ops-platform/
            ├── requirements.md   # 需求文档
            ├── design.md         # 技术设计
            └── tasklist.md       # 实施计划
```

## 快速开始

### 1. 启动开发环境

```bash
make docker-up
```

### 2. 安装依赖

```bash
make all
```

### 3. 运行服务

```bash
# 运行后端
make run-backend

# 运行 Agent
make run-agent

# 运行前端
make run-web
```

### 4. 访问服务

- 后端 API: http://localhost:8080
- 前端 Console: http://localhost:3000
- Agent: http://localhost:9090

## 开发指南

### 后端开发

后端使用 Go + Gin + GORM + PostgreSQL 开发：

```bash
cd platform-backend
go mod tidy
go run ./cmd
```

### Agent 开发

Agent 使用 Go 开发，部署在数据库节点上：

```bash
cd agent
go mod tidy
go run ./cmd
```

### 前端开发

前端使用 React + TypeScript + Ant Design 开发：

```bash
cd web-console
npm install
npm run dev
```

## 测试

```bash
make test
```

## 构建

```bash
make build
```

## 文档

详细文档位于 `.monkeycode/specs/mysql-ops-platform/` 目录：

- `requirements.md`: 完整需求文档
- `design.md`: 技术设计文档
- `tasklist.md`: 实施计划

## License

MIT