# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dbops.spec.ts >> DBOps 端到端覆盖 >> 13 端到端 API: alert rule + trigger + history
- Location: e2e\dbops.spec.ts:173:7

# Error details

```
Error: login failed after retries (rate limited)
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
  21  |   return 'admin123'
  22  | }
  23  | 
  24  | let cachedLoginBody: any
  25  | 
  26  | async function loginViaPage(page: Page, pwd: string) {
  27  |   // 使用 localStorage 注入 token, 绕开 antd Form 事件依赖.
  28  |   if (!cachedLoginBody?.data?.token) {
  29  |     const resp = await page.request.post(`${BACKEND}/api/v1/auth/login`, {
  30  |       data: { username: 'admin', password: pwd },
  31  |     })
  32  |     expect(resp.status(), 'login API should return 200').toBe(200)
  33  |     cachedLoginBody = await resp.json()
  34  |   }
  35  |   await page.goto(`${FRONTEND}/login`, { waitUntil: 'domcontentloaded' })
  36  |   await page.evaluate((b: any) => {
  37  |     localStorage.setItem('token', b.data.token)
  38  |     localStorage.setItem('user', JSON.stringify(b.data.user))
  39  |   }, cachedLoginBody)
  40  |   await page.goto(`${FRONTEND}/dashboard/home`, { waitUntil: 'networkidle' })
  41  |   await page.waitForSelector('.apple-stat', { timeout: 10_000 })
  42  | }
  43  | 
  44  | async function loginViaRequest(request: any, pwd: string): Promise<string> {
  45  |   // 直接 API 调用, 带重试应对 429 限流.
  46  |   for (let attempt = 0; attempt < 3; attempt++) {
  47  |     const resp = await request.post(`${BACKEND}/api/v1/auth/login`, {
  48  |       data: { username: 'admin', password: pwd },
  49  |     })
  50  |     if (resp.status() === 429) {
  51  |       await new Promise(r => setTimeout(r, 2000 * (attempt + 1)))
  52  |       continue
  53  |     }
  54  |     expect(resp.status()).toBe(200)
  55  |     const body = await resp.json()
  56  |     return body.data.token
  57  |   }
> 58  |   throw new Error('login failed after retries (rate limited)')
      |         ^ Error: login failed after retries (rate limited)
  59  | }
  60  | 
  61  | test.describe('DBOps 端到端覆盖', () => {
  62  |   let pwd: string
  63  | 
  64  |   test.beforeAll(async () => {
  65  |     pwd = await readAdminPassword()
  66  |   })
  67  | 
  68  |   test('01 健康检查 + 探活分流', async ({ request }) => {
  69  |     const live = await request.get(`${BACKEND}/health/live`)
  70  |     expect(live.status()).toBe(200)
  71  |     expect((await live.json()).data.status).toBe('alive')
  72  |     const ready = await request.get(`${BACKEND}/health/ready`)
  73  |     expect(ready.status()).toBe(200)
  74  |     const rb = await ready.json()
  75  |     expect(rb.data.checks.db).toBe('ok')
  76  |   })
  77  | 
  78  |   test('02 未鉴权 401', async ({ request }) => {
  79  |     const r = await request.get(`${BACKEND}/api/v1/instances`)
  80  |     expect(r.status()).toBe(401)
  81  |   })
  82  | 
  83  |   test('03 错误密码 401, 正确密码 200', async ({ request }) => {
  84  |     const bad = await request.post(`${BACKEND}/api/v1/auth/login`, {
  85  |       data: { username: 'admin', password: 'wrong-password-123' },
  86  |     })
  87  |     // 429 也是可接受的 (限流)
  88  |     expect([401, 429]).toContain(bad.status())
  89  |     if (bad.status() === 401) {
  90  |       const token = await loginViaRequest(request, pwd)
  91  |       expect(token).toBeTruthy()
  92  |     }
  93  |   })
  94  | 
  95  |   test('04 登录页 UI 渲染 + Apple 主题', async ({ page }) => {
  96  |     await page.goto(`${FRONTEND}/login`)
  97  |     await expect(page.locator('text=MySQL 运维平台').first()).toBeVisible()
  98  |     await expect(page.locator('text=欢迎回来').first()).toBeVisible()
  99  |     const btn = page.locator('button:has-text("登 录")')
  100 |     await expect(btn.first()).toBeVisible()
  101 |     const bg = await btn.first().evaluate(el => getComputedStyle(el).backgroundImage)
  102 |     expect(bg).toContain('linear-gradient')
  103 |   })
  104 | 
  105 |   test('05 登录后 Home 显示 6 个 stat 卡片', async ({ page }) => {
  106 |     await loginViaPage(page, pwd)
  107 |     const labels = ['主机', '实例', '告警历史', '待审批', '审计日志', '存储后端']
  108 |     for (const l of labels) {
  109 |       await expect(page.locator('.apple-stat-label').filter({ hasText: l }).first()).toBeVisible({ timeout: 8_000 })
  110 |     }
  111 |   })
  112 | 
  113 |   test('06 快捷操作 + 跳转', async ({ page }) => {
  114 |     await loginViaPage(page, pwd)
  115 |     const card = page.locator('.quick-action-card:has-text("添加空主机")').first()
  116 |     await card.waitFor({ state: 'visible', timeout: 10_000 })
  117 |     await card.click()
  118 |     await page.waitForURL(/\/hosts\/new/, { timeout: 8_000 })
  119 |   })
  120 | 
  121 |   test('07 数据存储页: 显示 SQLite 状态', async ({ page }) => {
  122 |     await loginViaPage(page, pwd)
  123 |     await page.goto(`${FRONTEND}/dashboard/data-storage`)
  124 |     await page.waitForLoadState('networkidle')
  125 |     await expect(page.locator('text=数据存储管理').first()).toBeVisible()
  126 |     await expect(page.locator('text=SQLite').first()).toBeVisible({ timeout: 8_000 })
  127 |   })
  128 | 
  129 |   test('08 主机管理页', async ({ page }) => {
  130 |     await loginViaPage(page, pwd)
  131 |     await page.goto(`${FRONTEND}/dashboard/hosts`)
  132 |     await page.waitForLoadState('networkidle')
  133 |     await expect(page.locator('text=主机').first()).toBeVisible()
  134 |   })
  135 | 
  136 |   test('09 实例管理页', async ({ page }) => {
  137 |     await loginViaPage(page, pwd)
  138 |     await page.goto(`${FRONTEND}/dashboard/instances`)
  139 |     await page.waitForLoadState('networkidle')
  140 |     await expect(page.locator('text=实例').first()).toBeVisible()
  141 |   })
  142 | 
  143 |   test('10 告警规则页', async ({ page }) => {
  144 |     await loginViaPage(page, pwd)
  145 |     await page.goto(`${FRONTEND}/dashboard/alert-rules`)
  146 |     await page.waitForLoadState('networkidle')
  147 |     await expect(page.locator('text=告警').first()).toBeVisible()
  148 |   })
  149 | 
  150 |   test('11 审计日志页', async ({ page }) => {
  151 |     await loginViaPage(page, pwd)
  152 |     await page.goto(`${FRONTEND}/dashboard/audit-logs`)
  153 |     await page.waitForLoadState('networkidle')
  154 |     await expect(page.locator('text=审计').first()).toBeVisible()
  155 |   })
  156 | 
  157 |   test('12 端到端 API: 创建 + 查询 + 删除 host', async ({ request }) => {
  158 |     const token = await loginViaRequest(request, pwd)
```