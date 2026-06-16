<!-- DBOps 商业化开发工作流 -->

## DBOps 商业化开发指南

### 项目定位

数据库架构级/系统级生命周期管理的商业化 DevOps 平台。
技术栈: Go (Gin) + React (Ant Design) + SQLite/MySQL + Redis + ClickHouse

### 目录结构

- `specs/` - OpenSpec 规范体系
  - `SPEC.md` - 主规范文档
  - `proposals/` - 功能提案
  - `designs/` - 设计文档
  - `tasks/` - 任务拆解
  - `archive/` - 已完成归档
- `.opencode/skills/` - OpenCode 项目技能
- `platform-backend/` - Go 后端
- `web-console/` - React 前端
- `agent/` - Go Agent

### 开发工作流 (OpenSpec + Superpowers)

1. **提案**: 在 `specs/proposals/` 创建功能提案
2. **规范**: 更新 `specs/SPEC.md` 补充数据模型/API
3. **任务**: 在 `specs/tasks/` 拆解为 2-5 分钟粒度的子任务
4. **实现**: TDD + 后端 `make test` + 前端 `make build`
5. **验证**: API 符合 `{ code, message, data/error }` 规范
6. **归档**: 完成后的规范移至 `specs/archive/`

### 代码质量

- 后端: Go 1.25+, Gin, Clean Architecture
- 前端: React 18, TypeScript, Ant Design 5
- Agent: Go 1.21+, HTTP + Bearer Token
- 测试: `make test` (后端), `make build` (前端编译检查)
- 安全: 密钥仅从环境变量读取, 密码 AES-GCM 加密, JWT 鉴权

### 构建命令

```bash
make build          # 构建所有组件
make test           # 运行后端测试
make docker-up      # Docker Compose 启动
```

### 商业版本管理

- CE (社区版): 基础功能, MIT 协议
- EE (企业版): CE + 高可用/升级/迁移/审计, 商业授权
- UE (旗舰版): EE + AI 智能化, 商业授权
