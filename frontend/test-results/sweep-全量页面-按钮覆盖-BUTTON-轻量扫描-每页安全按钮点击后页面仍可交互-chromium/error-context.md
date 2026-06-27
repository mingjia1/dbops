# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: sweep.spec.ts >> 全量页面 + 按钮覆盖 >> BUTTON 轻量扫描: 每页安全按钮点击后页面仍可交互
- Location: e2e\sweep.spec.ts:78:7

# Error details

```
Error: admin login should return a token

expect(received).toBeTruthy()

Received: undefined
```

# Test source

```ts
  1   | import { test, expect, type Page } from '@playwright/test'
  2   | 
  3   | const FRONTEND = 'http://127.0.0.1:3000'
  4   | const BACKEND = 'http://127.0.0.1:8080'
  5   | 
  6   | // 全部 18 个受保护页面 (App.tsx 中声明).
  7   | const PAGES: { path: string; name: string; expectText: string[] }[] = [
  8   |   { path: '/dashboard/home', name: '总览', expectText: ['概览', '快捷操作'] },
  9   |   { path: '/dashboard/monitor', name: '监控仪表盘', expectText: ['监控'] },
  10  |   { path: '/dashboard/hosts', name: '主机管理', expectText: ['主机'] },
  11  |   { path: '/dashboard/instances', name: '实例管理', expectText: ['实例'] },
  12  |   { path: '/dashboard/env-check', name: '环境检测', expectText: ['环境'] },
  13  |   { path: '/dashboard/backup', name: '备份管理', expectText: ['备份'] },
  14  |   { path: '/dashboard/cluster-deploy', name: '集群部署', expectText: ['集群'] },
  15  |   { path: '/dashboard/ha', name: '高可用管理', expectText: ['高可用'] },
  16  |   { path: '/dashboard/role-switch', name: '角色切换', expectText: ['角色'] },
  17  |   { path: '/dashboard/data-storage', name: '数据存储', expectText: ['数据存储管理'] },
  18  |   { path: '/dashboard/agent-manage', name: 'Agent 管理', expectText: ['Agent 管理'] },
  19  |   { path: '/dashboard/alert-rules', name: '告警规则', expectText: ['告警'] },
  20  |   { path: '/dashboard/upgrade', name: '升级管理', expectText: ['升级'] },
  21  |   { path: '/dashboard/migration', name: '数据迁移', expectText: ['迁移'] },
  22  |   { path: '/dashboard/topology', name: '拓扑视图', expectText: ['拓扑'] },
  23  |   { path: '/dashboard/parameter-templates', name: '参数模板', expectText: ['参数模板'] },
  24  |   { path: '/dashboard/approvals', name: '审批管理', expectText: ['审批'] },
  25  |   { path: '/dashboard/audit-logs', name: '审计日志', expectText: ['审计'] },
  26  | ]
  27  | 
  28  | async function readAdminPassword(): Promise<string> {
  29  |   if (process.env.E2E_ADMIN_PASSWORD) return process.env.E2E_ADMIN_PASSWORD
  30  |   try {
  31  |     const fs = await import('fs/promises')
  32  |     const t = await fs.readFile('d:/test_tmple/new_dbops/dbops/platform-backend/data/.admin_password', 'utf8').catch(() => '')
  33  |     if (t.trim()) return t.trim()
  34  |   } catch {}
  35  |   return '2xUegKvuXr3WwNqjxKfD'
  36  | }
  37  | 
  38  | let cachedLoginBody: any
  39  | 
  40  | async function login(page: Page, pwd: string) {
  41  |   if (!cachedLoginBody?.data?.token) {
  42  |     const candidates = Array.from(new Set([process.env.E2E_ADMIN_PASSWORD, '123456', pwd, 'Tv@gTFz8HHMhYArjOhk2', '2xUegKvuXr3WwNqjxKfD'].filter(Boolean)))
  43  |     for (const candidate of candidates) {
  44  |       const resp = await page.request.post(`${BACKEND}/api/v1/auth/login`, {
  45  |         data: { username: 'admin', password: candidate },
  46  |       })
  47  |       if (resp.status() === 200) {
  48  |         cachedLoginBody = await resp.json()
  49  |         break
  50  |       }
  51  |     }
  52  |   }
> 53  |   expect(cachedLoginBody?.data?.token, 'admin login should return a token').toBeTruthy()
      |                                                                             ^ Error: admin login should return a token
  54  |   await page.goto(`${FRONTEND}/login`, { waitUntil: 'domcontentloaded' })
  55  |   await page.evaluate((b: any) => {
  56  |     localStorage.setItem('token', b.data.token)
  57  |     localStorage.setItem('user', JSON.stringify(b.data.user))
  58  |   }, cachedLoginBody)
  59  |   await page.goto(`${FRONTEND}/dashboard/home`, { waitUntil: 'networkidle' })
  60  |   await page.waitForSelector('.apple-stat', { timeout: 10_000 })
  61  | }
  62  | 
  63  | test.describe('全量页面 + 按钮覆盖', () => {
  64  |   let pwd: string
  65  |   test.beforeAll(async () => { pwd = await readAdminPassword() })
  66  | 
  67  |   for (const p of PAGES) {
  68  |     test(`P ${p.name}: ${p.path} 渲染 + 关键文案`, async ({ page }) => {
  69  |       await login(page, pwd)
  70  |       await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
  71  |       for (const t of p.expectText) {
  72  |         await expect(page.locator(`text=${t}`).first()).toBeVisible({ timeout: 8_000 })
  73  |       }
  74  |     })
  75  |   }
  76  | 
  77  |   // 轻量点击可安全重复的按钮，验证页面不会白屏或丢失主体内容。
  78  |   test('BUTTON 轻量扫描: 每页安全按钮点击后页面仍可交互', async ({ page }) => {
  79  |     test.setTimeout(90_000)
  80  |     await login(page, pwd)
  81  |     const issues: string[] = []
  82  |     const unsafeWords = ['退出', '删除', '提交', '创建', '保存', '安装', '部署', '执行', '重置', '修改', '启动', '停止', '销毁', '恢复', '授权', '回收', 'Logout', 'Delete']
  83  |     for (const p of PAGES) {
  84  |       await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
  85  |       const btns = page.locator('button:visible, .ant-btn:visible')
  86  |       const count = Math.min(await btns.count(), 6)
  87  |       const beforeUrl = page.url()
  88  |       for (let i = 0; i < count; i++) {
  89  |         const btn = btns.nth(i)
  90  |         const text = (await btn.textContent().catch(() => ''))?.trim() || ''
  91  |         if (!text) continue
  92  |         if (await btn.isDisabled().catch(() => false)) continue
  93  |         if (unsafeWords.some(word => text.includes(word))) continue
  94  |         try {
  95  |           await btn.click({ timeout: 1500, force: true })
  96  |           await page.waitForTimeout(80)
  97  |         } catch (e: any) {
  98  |           // 弹窗可能已打开, 关闭后继续
  99  |           await page.keyboard.press('Escape').catch(() => {})
  100 |         }
  101 |         // 任何 modal 一律 Esc 关闭
  102 |         await page.keyboard.press('Escape').catch(() => {})
  103 |       }
  104 |       // 关闭可能打开的 modal, 然后回到原页面
  105 |       await page.keyboard.press('Escape').catch(() => {})
  106 |       // 关闭可能跳走的 modal 后, 不应出现白屏: 主体内容还可见
  107 |       const bodyHasContent = await page.locator('body').innerText().then(t => t.length > 50)
  108 |       if (!bodyHasContent) issues.push(`${p.name}: 点击按钮后页面空白 (URL=${page.url()})`)
  109 |       // 重置到原页面
  110 |       if (page.url() !== beforeUrl) {
  111 |         await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
  112 |       }
  113 |     }
  114 |     if (issues.length > 0) {
  115 |       console.log('按钮扫描发现以下问题:\n' + issues.join('\n'))
  116 |     }
  117 |     expect(issues.length).toBe(0)
  118 |   })
  119 | 
  120 |   // 路由可达性: 客户端路由通通返回 200 + 非 404 错误.
  121 |   test('ROUTE 全部路由可达', async ({ page }) => {
  122 |     await login(page, pwd)
  123 |     for (const p of PAGES) {
  124 |       const resp = await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
  125 |       expect(resp?.status(), `route ${p.path} should be 200`).toBe(200)
  126 |     }
  127 |   })
  128 | 
  129 |   test('MENU 系统管理可展开并进入子菜单', async ({ page }) => {
  130 |     await login(page, pwd)
  131 |     await page.getByText('系统管理').click()
  132 |     await expect(page.getByText('数据存储')).toBeVisible({ timeout: 8_000 })
  133 |     await expect(page.getByText('Agent 管理')).toBeVisible({ timeout: 8_000 })
  134 |     await expect(page.getByText('告警规则')).toBeVisible({ timeout: 8_000 })
  135 |     await expect(page.getByText('参数模板')).toBeVisible({ timeout: 8_000 })
  136 | 
  137 |     await page.getByText('Agent 管理').click()
  138 |     await page.waitForURL(/\/dashboard\/agent-manage/, { timeout: 8_000 })
  139 |     await expect(page.locator('text=Agent 管理').first()).toBeVisible({ timeout: 8_000 })
  140 |   })
  141 | })
  142 | 
```