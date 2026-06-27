# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: auth.spec.ts >> Authentication >> should login successfully with valid credentials
- Location: e2e\auth.spec.ts:29:7

# Error details

```
Error: expect(received).toContain(expected) // indexOf

Expected substring: "/dashboard"
Received string:    "http://127.0.0.1:3000/login"
```

# Page snapshot

```yaml
- generic [ref=e1]:
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
              - textbox "用户名" [ref=e70]: admin
            - generic [ref=e76]:
              - img "lock" [ref=e78]:
                - img [ref=e79]
              - textbox "密码" [ref=e81]: admin123
              - img "eye-invisible" [ref=e83] [cursor=pointer]:
                - img [ref=e84]
            - button "loading 登 录" [active] [ref=e92] [cursor=pointer]:
              - generic:
                - img "loading"
              - generic [ref=e93]: 登 录
      - generic [ref=e94]: 首次使用?请查看后端启动日志中的默认 admin 密码
  - generic [ref=e96]:
    - img "exclamation-circle" [ref=e97]:
      - img [ref=e98]
    - generic [ref=e100]: 登录尝试过于频繁，请稍后再试
  - generic [ref=e102]:
    - img "close-circle" [ref=e103]:
      - img [ref=e104]
    - generic [ref=e106]: too many login attempts, please try again later
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test'
  2  | 
  3  | test.describe('Authentication', () => {
  4  |   test('should redirect unauthenticated user to login page', async ({ page }) => {
  5  |     await page.goto('/dashboard')
  6  |     await expect(page).toHaveURL(/\/login/)
  7  |   })
  8  | 
  9  |   test('should show login form with username and password fields', async ({ page }) => {
  10 |     await page.goto('/login')
  11 |     await expect(page.locator('input[id*="username"], input[name*="username"], input[placeholder*="用户名"], input[placeholder*="Username"]')).toBeVisible()
  12 |     await expect(page.locator('input[type="password"]')).toBeVisible()
  13 |   })
  14 | 
  15 |   test('should show error on invalid credentials', async ({ page }) => {
  16 |     await page.goto('/login')
  17 |     const usernameInput = page.locator('input[id*="username"], input[name*="username"], input[placeholder*="用户名"], input[placeholder*="Username"]').first()
  18 |     const passwordInput = page.locator('input[type="password"]').first()
  19 |     await usernameInput.fill('wronguser')
  20 |     await passwordInput.fill('wrongpass')
  21 |     const loginButton = page.locator('button:has-text("登录"), button:has-text("Login"), button[type="submit"]').first()
  22 |     await loginButton.click()
  23 |     await page.waitForTimeout(2000)
  24 |     const errorMsg = page.locator('.ant-message-error, .ant-alert-error, [class*="error"]')
  25 |     const stillOnLogin = await page.url().includes('/login')
  26 |     expect(stillOnLogin || (await errorMsg.count()) > 0).toBeTruthy()
  27 |   })
  28 | 
  29 |   test('should login successfully with valid credentials', async ({ page }) => {
  30 |     await page.goto('/login')
  31 |     const usernameInput = page.locator('input[id*="username"], input[name*="username"], input[placeholder*="用户名"], input[placeholder*="Username"]').first()
  32 |     const passwordInput = page.locator('input[type="password"]').first()
  33 |     await usernameInput.fill('admin')
  34 |     await passwordInput.fill('admin123')
  35 |     const loginButton = page.locator('button:has-text("登录"), button:has-text("Login"), button[type="submit"]').first()
  36 |     await loginButton.click()
  37 |     await page.waitForTimeout(3000)
  38 |     const url = page.url()
> 39 |     expect(url).toContain('/dashboard')
     |                 ^ Error: expect(received).toContain(expected) // indexOf
  40 |   })
  41 | 
  42 |   test('should be able to logout', async ({ page }) => {
  43 |     await page.goto('/login')
  44 |     const usernameInput = page.locator('input[id*="username"], input[name*="username"], input[placeholder*="用户名"], input[placeholder*="Username"]').first()
  45 |     const passwordInput = page.locator('input[type="password"]').first()
  46 |     await usernameInput.fill('admin')
  47 |     await passwordInput.fill('admin123')
  48 |     const loginButton = page.locator('button:has-text("登录"), button:has-text("Login"), button[type="submit"]').first()
  49 |     await loginButton.click()
  50 |     await page.waitForTimeout(3000)
  51 |     const userMenu = page.locator('[class*="user"], [class*="avatar"], .ant-dropdown-trigger').first()
  52 |     if (await userMenu.count() > 0) {
  53 |       await userMenu.click()
  54 |       const logoutBtn = page.locator('text=退出, text=Logout, text=登出').first()
  55 |       if (await logoutBtn.count() > 0) {
  56 |         await logoutBtn.click()
  57 |         await page.waitForTimeout(2000)
  58 |         expect(page.url()).toContain('/login')
  59 |       }
  60 |     }
  61 |   })
  62 | })
  63 | 
```