# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dbops.spec.ts >> DBOps 端到端覆盖 >> 06 快捷操作 + 跳转
- Location: e2e\dbops.spec.ts:94:7

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: 200
Received: 429
```

# Test source

```ts
  1   | import { test, expect, type Page } from '@playwright/test'
  2   | 
  3   | const FRONTEND = 'http://127.0.0.1:3000'
  4   | const BACKEND = 'http://127.0.0.1:8080'
  5   | 
  6   | async function readAdminPassword(): Promise<string> {
  7   |   // 1) 环境变量
  8   |   if (process.env.E2E_ADMIN_PASSWORD) return process.env.E2E_ADMIN_PASSWORD
  9   |   // 2) 后端写入的 fixture 文件 (data/.admin_password)
  10  |   try {
  11  |     const fs = await import('fs/promises')
  12  |     const candidates = [
  13  |       'd:/test_tmple/new_dbops/dbops/platform-backend/data/.admin_password',
  14  |       'C:/Users/jia.jia/AppData/Local/Temp/.admin_password',
  15  |     ]
  16  |     for (const p of candidates) {
  17  |       const t = await fs.readFile(p, 'utf8').catch(() => '')
  18  |       if (t.trim()) return t.trim()
  19  |     }
  20  |   } catch {}
  21  |   return 'Tv@gTFz8HHMhYArjOhk2'
  22  | }
  23  | 
  24  | async function login(page: Page, pwd: string) {
  25  |   // 直连 API 拿 token + user, 写进 localStorage, 然后导航到 dashboard.
  26  |   // 这种方式绕开 antd Form 的事件依赖, 适合 e2e.
  27  |   const resp = await page.request.post(`${BACKEND}/api/v1/auth/login`, {
  28  |     data: { username: 'admin', password: pwd },
  29  |   })
> 30  |   expect(resp.status()).toBe(200)
      |                         ^ Error: expect(received).toBe(expected) // Object.is equality
  31  |   const body = await resp.json()
  32  |   await page.goto(`${FRONTEND}/login`, { waitUntil: 'domcontentloaded' })
  33  |   await page.evaluate((b: any) => {
  34  |     localStorage.setItem('token', b.data.token)
  35  |     localStorage.setItem('user', JSON.stringify(b.data.user))
  36  |   }, body)
  37  |   await page.goto(`${FRONTEND}/dashboard/home`, { waitUntil: 'networkidle' })
  38  |   await page.waitForSelector('.apple-stat', { timeout: 10_000 })
  39  | }
  40  | 
  41  | test.describe('DBOps 端到端覆盖', () => {
  42  |   let pwd: string
  43  | 
  44  |   test.beforeAll(async () => {
  45  |     pwd = await readAdminPassword()
  46  |   })
  47  | 
  48  |   test('01 健康检查 + 探活分流', async ({ request }) => {
  49  |     const live = await request.get(`${BACKEND}/health/live`)
  50  |     expect(live.status()).toBe(200)
  51  |     expect((await live.json()).data.status).toBe('alive')
  52  |     const ready = await request.get(`${BACKEND}/health/ready`)
  53  |     expect(ready.status()).toBe(200)
  54  |     const rb = await ready.json()
  55  |     expect(rb.data.checks.db).toBe('ok')
  56  |   })
  57  | 
  58  |   test('02 未鉴权 401', async ({ request }) => {
  59  |     const r = await request.get(`${BACKEND}/api/v1/instances`)
  60  |     expect(r.status()).toBe(401)
  61  |   })
  62  | 
  63  |   test('03 错误密码 401, 正确密码 200', async ({ request }) => {
  64  |     const bad = await request.post(`${BACKEND}/api/v1/auth/login`, {
  65  |       data: { username: 'admin', password: 'wrong-password-123' },
  66  |     })
  67  |     expect(bad.status()).toBe(401)
  68  |     const ok = await request.post(`${BACKEND}/api/v1/auth/login`, {
  69  |       data: { username: 'admin', password: pwd },
  70  |     })
  71  |     expect(ok.status()).toBe(200)
  72  |     const body = await ok.json()
  73  |     expect(body.data.token).toBeTruthy()
  74  |   })
  75  | 
  76  |   test('04 登录页 UI 渲染 + Apple 主题', async ({ page }) => {
  77  |     await page.goto(`${FRONTEND}/login`)
  78  |     await expect(page.locator('text=MySQL 运维平台').first()).toBeVisible()
  79  |     await expect(page.locator('text=欢迎回来').first()).toBeVisible()
  80  |     const btn = page.locator('button:has-text("登 录")')
  81  |     await expect(btn.first()).toBeVisible()
  82  |     const bg = await btn.first().evaluate(el => getComputedStyle(el).backgroundImage)
  83  |     expect(bg).toContain('linear-gradient')
  84  |   })
  85  | 
  86  |   test('05 登录后 Home 显示 6 个 stat 卡片', async ({ page }) => {
  87  |     await login(page, pwd)
  88  |     const labels = ['主机', '实例', '告警历史', '待审批', '审计日志', '存储后端']
  89  |     for (const l of labels) {
  90  |       await expect(page.locator('.apple-stat-label').filter({ hasText: l }).first()).toBeVisible({ timeout: 8_000 })
  91  |     }
  92  |   })
  93  | 
  94  |   test('06 快捷操作 + 跳转', async ({ page }) => {
  95  |     await login(page, pwd)
  96  |     await page.locator('.apple-card:has-text("添加主机")').first().click()
  97  |     await page.waitForURL(/\/hosts\/new/, { timeout: 8_000 })
  98  |   })
  99  | 
  100 |   test('07 数据存储页: 显示 SQLite 状态', async ({ page }) => {
  101 |     await login(page, pwd)
  102 |     await page.goto(`${FRONTEND}/dashboard/data-storage`)
  103 |     await page.waitForLoadState('networkidle')
  104 |     await expect(page.locator('text=数据存储管理').first()).toBeVisible()
  105 |     await expect(page.locator('text=SQLite').first()).toBeVisible({ timeout: 8_000 })
  106 |   })
  107 | 
  108 |   test('08 主机管理页', async ({ page }) => {
  109 |     await login(page, pwd)
  110 |     await page.goto(`${FRONTEND}/dashboard/hosts`)
  111 |     await page.waitForLoadState('networkidle')
  112 |     await expect(page.locator('text=主机').first()).toBeVisible()
  113 |   })
  114 | 
  115 |   test('09 实例管理页', async ({ page }) => {
  116 |     await login(page, pwd)
  117 |     await page.goto(`${FRONTEND}/dashboard/instances`)
  118 |     await page.waitForLoadState('networkidle')
  119 |     await expect(page.locator('text=实例').first()).toBeVisible()
  120 |   })
  121 | 
  122 |   test('10 告警规则页', async ({ page }) => {
  123 |     await login(page, pwd)
  124 |     await page.goto(`${FRONTEND}/dashboard/alert-rules`)
  125 |     await page.waitForLoadState('networkidle')
  126 |     await expect(page.locator('text=告警').first()).toBeVisible()
  127 |   })
  128 | 
  129 |   test('11 审计日志页', async ({ page }) => {
  130 |     await login(page, pwd)
```