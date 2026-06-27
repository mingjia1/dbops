# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: hosts.spec.ts >> Host Management >> should navigate to new host form
- Location: e2e\hosts.spec.ts:25:7

# Error details

```
Error: expect(received).toContain(expected) // indexOf

Expected substring: "/hosts/new"
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
  5  | test.describe('Host Management', () => {
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
  17 |   test('should display host list page', async ({ page }) => {
  18 |     await page.goto(`${BASE}/hosts`)
  19 |     await page.waitForTimeout(2000)
  20 |     expect(page.url()).toContain('/hosts')
  21 |     const tableOrList = page.locator('.ant-table, .ant-list, [class*="host"]')
  22 |     expect(await tableOrList.count()).toBeGreaterThan(0)
  23 |   })
  24 | 
  25 |   test('should navigate to new host form', async ({ page }) => {
  26 |     await page.goto(`${BASE}/hosts/new`)
  27 |     await page.waitForTimeout(2000)
> 28 |     expect(page.url()).toContain('/hosts/new')
     |                        ^ Error: expect(received).toContain(expected) // indexOf
  29 |     const form = page.locator('form, .ant-form')
  30 |     expect(await form.count()).toBeGreaterThan(0)
  31 |   })
  32 | 
  33 |   test('should have host action buttons', async ({ page }) => {
  34 |     await page.goto(`${BASE}/hosts`)
  35 |     await page.waitForTimeout(2000)
  36 |     const actionButtons = page.locator('button')
  37 |     const count = await actionButtons.count()
  38 |     expect(count).toBeGreaterThan(0)
  39 |   })
  40 | })
  41 | 
```