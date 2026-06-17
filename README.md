<div align="center">

# 智能 MySQL 运维平台

[![Go Version][go-image]][go-url] [![Node.js][node-image]][node-url] [![License][license-image]][license-url] [![Language][lang-image]][lang-url]

---

**[📖 中文文档 / Chinese Documentation](readme_ZH.md)** | **[📘 English Documentation / 英文文档](readme_US.md)**

</div>

---

## Tech Stack / 技术栈

| Component | Technology |
|-----------|------------|
| **Backend / 后端** | Go 1.25+ + Gin + SQLite/MySQL + Redis |
| **Frontend / 前端** | React 18 + TypeScript + Ant Design 5 |
| **Agent** | Go 1.21+ + HTTP + Bearer Token |

## Quick Start / 快速入门

```bash
# Build all components / 构建所有组件
make build

# Run tests / 运行测试
make test

# Start development environment / 启动开发环境
make install-web
```

## API Access / 访问地址

- **Backend Admin / 后台管理**: `http://localhost:8080`
- **Web Console / 控制台**: `http://localhost:3000`
- **Agent Service / Agent服务**: `http://localhost:9090`

## Commercial Editions / 商业版本

| Edition | Features |
|---------|----------|
| **CE** (Community / 社区版) | Core features, MIT license |
| **EE** (Enterprise / 企业版) | CE + HA/Upgrade/Migration/Audit |
| **UE** (Ultimate / 旗舰版) | EE + AI-powered intelligence |

---

<div align="center">

**[📖 中文 / Chinese](readme_ZH.md)** | **[📘 English / 英文](readme_US.md)**

</div>

[go-image]: https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go
[go-url]: https://go.dev/
[node-image]: https://img.shields.io/badge/Node.js-18+-339933?style=flat&logo=node.js
[node-url]: https://nodejs.org/
[license-image]: https://img.shields.io/badge/License-MIT-blue.svg
[license-url]: https://opensource.org/licenses/MIT
[lang-image]: https://img.shields.io/badge/Language-Go%20%7C%20TypeScript-blue
[lang-url]: https://github.com/mingjia1/dbops
