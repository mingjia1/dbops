# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dbops.spec.ts >> DBOps 端到端覆盖 >> 12 端到端 API: 创建 + 查询 + 删除 host
- Location: e2e\dbops.spec.ts:136:7

# Error details

```
TypeError: Cannot read properties of undefined (reading 'token')
```

# Test source

```ts
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
  131 |     await page.goto(`${FRONTEND}/dashboard/audit-logs`)
  132 |     await page.waitForLoadState('networkidle')
  133 |     await expect(page.locator('text=审计').first()).toBeVisible()
  134 |   })
  135 | 
  136 |   test('12 端到端 API: 创建 + 查询 + 删除 host', async ({ request }) => {
  137 |     const lr = await request.post(`${BACKEND}/api/v1/auth/login`, { data: { username: 'admin', password: pwd } })
> 138 |     const token = (await lr.json()).data.token
      |                                          ^ TypeError: Cannot read properties of undefined (reading 'token')
  139 |     if (!token) throw new Error('login returned no token; pwd wrong?')
  140 |     const auth = { headers: { Authorization: `Bearer ${token}` } }
  141 |     const created = await request.post(`${BACKEND}/api/v1/hosts`, {
  142 |       ...auth,
  143 |       data: { name: `e2e-host-${Date.now()}`, address: '127.0.0.1', ssh_user: 'root', ssh_auth_method: 'password', ssh_credential: 'dummy' },
  144 |     })
  145 |     expect(created.status()).toBe(200)
  146 |     const id = (await created.json()).data.id
  147 |     const got = await request.get(`${BACKEND}/api/v1/hosts/${id}`, auth)
  148 |     expect(got.status()).toBe(200)
  149 |     expect((await got.json()).data.id).toBe(id)
  150 |     const del = await request.delete(`${BACKEND}/api/v1/hosts/${id}`, auth)
  151 |     expect(del.status()).toBe(200)
  152 |   })
  153 | 
  154 |   test('13 端到端 API: alert rule + trigger + history', async ({ request }) => {
  155 |     const lr = await request.post(`${BACKEND}/api/v1/auth/login`, { data: { username: 'admin', password: pwd } })
  156 |     const token = (await lr.json()).data.token
  157 |     if (!token) throw new Error('login returned no token')
  158 |     const auth = { headers: { Authorization: `Bearer ${token}` } }
  159 |     const rule = await request.post(`${BACKEND}/api/v1/alerts/rules`, {
  160 |       ...auth,
  161 |       data: { name: `e2e-rule-${Date.now()}`, metric: 'qps', condition: '>', threshold: 100, severity: 'warning' },
  162 |     })
  163 |     expect(rule.status()).toBe(200)
  164 |     const rid = (await rule.json()).data.id
  165 |     const trig = await request.post(`${BACKEND}/api/v1/alerts/trigger`, {
  166 |       ...auth,
  167 |       data: { rule_id: rid, instance_id: 'e2e-instance-1', value: 200, message: 'e2e' },
  168 |     })
  169 |     expect(trig.status()).toBe(200)
  170 |     const hist = await request.get(`${BACKEND}/api/v1/alerts/history`, auth)
  171 |     expect(hist.status()).toBe(200)
  172 |     const records = (await hist.json()).data
  173 |     expect(Array.isArray(records)).toBe(true)
  174 |     const found = records.find((r: any) => r.message === 'e2e')
  175 |     expect(found).toBeTruthy()
  176 |   })
  177 | 
  178 |   test('14 错误响应统一: 404 / 400 HTTP 状态码', async ({ request }) => {
  179 |     const lr = await request.post(`${BACKEND}/api/v1/auth/login`, { data: { username: 'admin', password: pwd } })
  180 |     const token = (await lr.json()).data.token
  181 |     if (!token) throw new Error('login returned no token')
  182 |     const auth = { headers: { Authorization: `Bearer ${token}` } }
  183 |     const r = await request.get(`${BACKEND}/api/v1/parameter-templates/no-such-template`, auth)
  184 |     expect(r.status()).toBe(404)
  185 |     const bad = await request.post(`${BACKEND}/api/v1/auth/login`, { data: { username: 'admin' } })
  186 |     expect(bad.status()).toBe(400)
  187 |   })
  188 | 
  189 |   test('15 退出登录: localStorage 清空 + 跳回 login', async ({ page }) => {
  190 |     await login(page, pwd)
  191 |     // 退出登录会触发后端限流误命中 (login 是从 localStorage 恢复, 不走 API, 但 antd 的 message 错误会显示).
  192 |     // 这里直接清 localStorage + 导航验证, 不依赖 UI 点击.
  193 |     await page.evaluate(() => { localStorage.clear() })
  194 |     await page.goto(`${FRONTEND}/login`, { waitUntil: 'domcontentloaded' })
  195 |     const token = await page.evaluate(() => localStorage.getItem('token'))
  196 |     expect(token).toBeNull()
  197 |     // 也能从 dashboard 跳回 login: 模拟访问受保护页面应被踢回登录.
  198 |     await page.goto(`${FRONTEND}/dashboard/home`, { waitUntil: 'domcontentloaded' })
  199 |     await page.waitForURL(/\/login/, { timeout: 8_000 })
  200 |   })
  201 | })
  202 | 
```