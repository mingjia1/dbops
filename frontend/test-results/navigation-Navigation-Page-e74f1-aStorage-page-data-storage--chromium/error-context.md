# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: navigation.spec.ts >> Navigation & Page Loading >> should load DataStorage page (data-storage)
- Location: e2e\navigation.spec.ts:41:9

# Error details

```
Error: expect(received).toContain(expected) // indexOf

Expected substring: "data-storage"
Received string:    "http://127.0.0.1:3000/login"
```

# Page snapshot

```yaml
- generic [ref=e4]:
  - generic [ref=e6]:
    - heading "MySQL 运维平台" [level=1] [ref=e7]
    - paragraph [ref=e8]: 企业级数据库全生命周期管理
    - generic [ref=e9]:
      - generic [ref=e10]:
        - img "cloud" [ref=e12]:
          - img [ref=e13]
        - generic [ref=e15]:
          - generic [ref=e16]: 主机与实例
          - generic [ref=e17]: 资产盘点 · 自动发现
      - generic [ref=e18]:
        - img "safety-certificate" [ref=e20]:
          - img [ref=e21]
        - generic [ref=e23]:
          - generic [ref=e24]: 备份与恢复
          - generic [ref=e25]: 策略化 · 一键回滚
      - generic [ref=e26]:
        - img "thunderbolt" [ref=e28]:
          - img [ref=e29]
        - generic [ref=e31]:
          - generic [ref=e32]: 监控告警
          - generic [ref=e33]: 指标聚合 · 阈值告警
      - generic [ref=e34]:
        - img "cluster" [ref=e36]:
          - img [ref=e37]
        - generic [ref=e39]:
          - generic [ref=e40]: 高可用集群
          - generic [ref=e41]: MHA · MGR · PXC
    - generic [ref=e42]: v1.0 · 统一管理 · 可观测 · 可控制
  - generic [ref=e44]:
    - generic [ref=e45]:
      - generic [ref=e46]: 欢迎回来
      - generic [ref=e47]: 登录以继续管理工作台
    - generic [ref=e48]:
      - tablist [ref=e49]:
        - generic [ref=e51]:
          - tab "登录" [selected] [ref=e53] [cursor=pointer]
          - tab "注册" [ref=e55] [cursor=pointer]
      - tabpanel "登录" [ref=e58]:
        - generic [ref=e59]:
          - generic [ref=e65]:
            - img "user" [ref=e67]:
              - img [ref=e68]
            - textbox "用户名" [active] [ref=e70]
          - generic [ref=e76]:
            - img "lock" [ref=e78]:
              - img [ref=e79]
            - textbox "密码" [ref=e81]
            - img "eye-invisible" [ref=e83] [cursor=pointer]:
              - img [ref=e84]
          - button "登 录" [ref=e92] [cursor=pointer]:
            - generic [ref=e93]: 登 录
    - generic [ref=e94]: 首次使用?请查看后端启动日志中的默认 admin 密码
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test'
  2  | 
  3  | const BASE = '/dashboard'
  4  | 
  5  | test.describe('Navigation & Page Loading', () => {
  6  |   test.beforeEach(async ({ page }) => {
  7  |     await page.goto('/login')
  8  |     const usernameInput = page.locator('input[id*="username"], input[name*="username"], input[placeholder*="用户名"], input[placeholder*="Username"]').first()
  9  |     const passwordInput = page.locator('input[type="password"]').first()
  10 |     await usernameInput.fill('admin')
  11 |     await passwordInput.fill('admin123')
  12 |     const loginButton = page.locator('button:has-text("登录"), button:has-text("Login"), button[type="submit"]').first()
  13 |     await loginButton.click()
  14 |     await page.waitForTimeout(3000)
  15 |   })
  16 | 
  17 |   const pages = [
  18 |     { path: 'home', name: 'Home' },
  19 |     { path: 'hosts', name: 'HostList' },
  20 |     { path: 'instances', name: 'InstanceList' },
  21 |     { path: 'cluster-deploy', name: 'ClusterDeploy' },
  22 |     { path: 'ha', name: 'HAManage' },
  23 |     { path: 'env-check', name: 'EnvironmentCheck' },
  24 |     { path: 'backup', name: 'BackupManage' },
  25 |     { path: 'monitor', name: 'MonitorDashboard' },
  26 |     { path: 'parameter-templates', name: 'ParameterTemplateList' },
  27 |     { path: 'approvals', name: 'ApprovalManage' },
  28 |     { path: 'audit-logs', name: 'AuditLog' },
  29 |     { path: 'upgrade', name: 'UpgradeManage' },
  30 |     { path: 'alert-rules', name: 'AlertRuleList' },
  31 |     { path: 'topology', name: 'TopologyView' },
  32 |     { path: 'migration', name: 'MigrationManage' },
  33 |     { path: 'role-switch', name: 'RoleSwitch' },
  34 |     { path: 'data-storage', name: 'DataStorage' },
  35 |     { path: 'agent-manage', name: 'AgentManage' },
  36 |     { path: 'plugins', name: 'PluginManage' },
  37 |     { path: 'security-settings', name: 'SecuritySettings' },
  38 |   ]
  39 | 
  40 |   for (const p of pages) {
  41 |     test(`should load ${p.name} page (${p.path})`, async ({ page }) => {
  42 |       const response = await page.goto(`${BASE}/${p.path}`)
  43 |       await page.waitForTimeout(2000)
  44 |       const url = page.url()
> 45 |       expect(url).toContain(p.path)
     |                   ^ Error: expect(received).toContain(expected) // indexOf
  46 |       const bodyText = await page.locator('body').innerText()
  47 |       expect(bodyText.length).toBeGreaterThan(0)
  48 |     })
  49 |   }
  50 | })
  51 | 
```