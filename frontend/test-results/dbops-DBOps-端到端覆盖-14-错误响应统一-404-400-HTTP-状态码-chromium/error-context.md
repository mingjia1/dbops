# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dbops.spec.ts >> DBOps 端到端覆盖 >> 14 错误响应统一: 404 / 400 HTTP 状态码
- Location: e2e\dbops.spec.ts:200:7

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: 400
Received: 429
```

# Test source

```ts
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
  159 |     const auth = { headers: { Authorization: `Bearer ${token}` } }
  160 |     const created = await request.post(`${BACKEND}/api/v1/hosts`, {
  161 |       ...auth,
  162 |       data: { name: `e2e-host-${Date.now()}`, address: '127.0.0.1', ssh_user: 'root', ssh_auth_method: 'password', ssh_credential: 'dummy' },
  163 |     })
  164 |     expect(created.status()).toBe(200)
  165 |     const id = (await created.json()).data.id
  166 |     const got = await request.get(`${BACKEND}/api/v1/hosts/${id}`, auth)
  167 |     expect(got.status()).toBe(200)
  168 |     expect((await got.json()).data.id).toBe(id)
  169 |     const del = await request.delete(`${BACKEND}/api/v1/hosts/${id}`, auth)
  170 |     expect(del.status()).toBe(200)
  171 |   })
  172 | 
  173 |   test('13 端到端 API: alert rule + trigger + history', async ({ request }) => {
  174 |     const token = await loginViaRequest(request, pwd)
  175 |     const auth = { headers: { Authorization: `Bearer ${token}` } }
  176 |     const rule = await request.post(`${BACKEND}/api/v1/alerts/rules`, {
  177 |       ...auth,
  178 |       data: { name: `e2e-rule-${Date.now()}`, metric: 'qps', condition: '>', threshold: 100, severity: 'warning' },
  179 |     })
  180 |     expect(rule.status()).toBe(200)
  181 |     const rid = (await rule.json()).data.id
  182 |     // trigger endpoint may return 500 if notifier is misconfigured; treat as known issue
  183 |     const trig = await request.post(`${BACKEND}/api/v1/alerts/trigger`, {
  184 |       ...auth,
  185 |       data: { rule_id: rid, instance_id: 'e2e-instance-1', value: 200, message: 'e2e' },
  186 |     })
  187 |     if (trig.status() === 500) {
  188 |       console.log('NOTE: /alerts/trigger returned 500 (notifier may be misconfigured), skipping trigger verification')
  189 |     } else {
  190 |       expect(trig.status()).toBe(200)
  191 |       const hist = await request.get(`${BACKEND}/api/v1/alerts/history`, auth)
  192 |       expect(hist.status()).toBe(200)
  193 |       const records = (await hist.json()).data
  194 |       expect(Array.isArray(records)).toBe(true)
  195 |       const found = records.find((r: any) => r.message === 'e2e')
  196 |       expect(found).toBeTruthy()
  197 |     }
  198 |   })
  199 | 
  200 |   test('14 错误响应统一: 404 / 400 HTTP 状态码', async ({ request }) => {
  201 |     const token = await loginViaRequest(request, pwd)
  202 |     const auth = { headers: { Authorization: `Bearer ${token}` } }
  203 |     const r = await request.get(`${BACKEND}/api/v1/parameter-templates/no-such-template`, auth)
  204 |     expect(r.status()).toBe(404)
  205 |     const bad = await request.post(`${BACKEND}/api/v1/auth/login`, { data: { username: 'admin' } })
> 206 |     expect(bad.status()).toBe(400)
      |                          ^ Error: expect(received).toBe(expected) // Object.is equality
  207 |   })
  208 | 
  209 |   test('15 退出登录: localStorage 清空 + 跳回 login', async ({ page }) => {
  210 |     await loginViaPage(page, pwd)
  211 |     // 退出登录会触发后端限流误命中 (login 是从 localStorage 恢复, 不走 API, 但 antd 的 message 错误会显示).
  212 |     // 这里直接清 localStorage + 导航验证, 不依赖 UI 点击.
  213 |     await page.evaluate(() => { localStorage.clear() })
  214 |     await page.goto(`${FRONTEND}/login`, { waitUntil: 'domcontentloaded' })
  215 |     const token = await page.evaluate(() => localStorage.getItem('token'))
  216 |     expect(token).toBeNull()
  217 |     // 也能从 dashboard 跳回 login: 模拟访问受保护页面应被踢回登录.
  218 |     await page.goto(`${FRONTEND}/dashboard/home`, { waitUntil: 'domcontentloaded' })
  219 |     await page.waitForURL(/\/login/, { timeout: 8_000 })
  220 |   })
  221 | })
  222 | 
```