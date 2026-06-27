import { test, expect, type Page } from '@playwright/test'

const FRONTEND = 'http://127.0.0.1:3000'
const BACKEND = 'http://127.0.0.1:8080'

// 全部 18 个受保护页面 (App.tsx 中声明).
const PAGES: { path: string; name: string; expectText: string[] }[] = [
  { path: '/dashboard/home', name: '总览', expectText: ['概览', '快捷操作'] },
  { path: '/dashboard/monitor', name: '监控仪表盘', expectText: ['监控'] },
  { path: '/dashboard/hosts', name: '主机管理', expectText: ['主机'] },
  { path: '/dashboard/instances', name: '实例管理', expectText: ['实例'] },
  { path: '/dashboard/env-check', name: '环境检测', expectText: ['环境'] },
  { path: '/dashboard/backup', name: '备份管理', expectText: ['备份'] },
  { path: '/dashboard/cluster-deploy', name: '集群部署', expectText: ['集群'] },
  { path: '/dashboard/ha', name: '高可用管理', expectText: ['高可用'] },
  { path: '/dashboard/role-switch', name: '角色切换', expectText: ['角色'] },
  { path: '/dashboard/data-storage', name: '数据存储', expectText: ['数据存储管理'] },
  { path: '/dashboard/agent-manage', name: 'Agent 管理', expectText: ['Agent 管理'] },
  { path: '/dashboard/alert-rules', name: '告警规则', expectText: ['告警'] },
  { path: '/dashboard/upgrade', name: '升级管理', expectText: ['升级'] },
  { path: '/dashboard/migration', name: '数据迁移', expectText: ['迁移'] },
  { path: '/dashboard/topology', name: '拓扑视图', expectText: ['拓扑'] },
  { path: '/dashboard/parameter-templates', name: '参数模板', expectText: ['参数模板'] },
  { path: '/dashboard/approvals', name: '审批管理', expectText: ['审批'] },
  { path: '/dashboard/audit-logs', name: '审计日志', expectText: ['审计'] },
]

async function readAdminPassword(): Promise<string> {
  if (process.env.E2E_ADMIN_PASSWORD) return process.env.E2E_ADMIN_PASSWORD
  try {
    const fs = await import('fs/promises')
    const t = await fs.readFile('d:/test_tmple/new_dbops/dbops/platform-backend/data/.admin_password', 'utf8').catch(() => '')
    if (t.trim()) return t.trim()
  } catch {}
  return 'admin123'
}

let cachedLoginBody: any

async function login(page: Page, pwd: string) {
  if (!cachedLoginBody?.data?.token) {
    const candidates = Array.from(new Set([process.env.E2E_ADMIN_PASSWORD, 'admin123', pwd, 'Tv@gTFz8HHMhYArjOhk2', '2xUegKvuXr3WwNqjxKfD'].filter(Boolean)))
    for (const candidate of candidates) {
      const resp = await page.request.post(`${BACKEND}/api/v1/auth/login`, {
        data: { username: 'admin', password: candidate },
      })
      if (resp.status() === 200) {
        cachedLoginBody = await resp.json()
        break
      }
    }
  }
  expect(cachedLoginBody?.data?.token, 'admin login should return a token').toBeTruthy()
  await page.goto(`${FRONTEND}/login`, { waitUntil: 'domcontentloaded' })
  await page.evaluate((b: any) => {
    localStorage.setItem('token', b.data.token)
    localStorage.setItem('user', JSON.stringify(b.data.user))
  }, cachedLoginBody)
  await page.goto(`${FRONTEND}/dashboard/home`, { waitUntil: 'networkidle' })
  await page.waitForSelector('.apple-stat', { timeout: 10_000 })
}

test.describe('全量页面 + 按钮覆盖', () => {
  let pwd: string
  test.beforeAll(async () => { pwd = await readAdminPassword() })

  for (const p of PAGES) {
    test(`P ${p.name}: ${p.path} 渲染 + 关键文案`, async ({ page }) => {
      await login(page, pwd)
      await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
      for (const t of p.expectText) {
        await expect(page.locator(`text=${t}`).first()).toBeVisible({ timeout: 8_000 })
      }
    })
  }

  // 轻量点击可安全重复的按钮，验证页面不会白屏或丢失主体内容。
  test('BUTTON 轻量扫描: 每页安全按钮点击后页面仍可交互', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page, pwd)
    const issues: string[] = []
    const unsafeWords = ['退出', '删除', '提交', '创建', '保存', '安装', '部署', '执行', '重置', '修改', '启动', '停止', '销毁', '恢复', '授权', '回收', 'Logout', 'Delete']
    for (const p of PAGES) {
      await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
      const btns = page.locator('button:visible, .ant-btn:visible')
      const count = Math.min(await btns.count(), 6)
      const beforeUrl = page.url()
      for (let i = 0; i < count; i++) {
        const btn = btns.nth(i)
        const text = (await btn.textContent().catch(() => ''))?.trim() || ''
        if (!text) continue
        if (await btn.isDisabled().catch(() => false)) continue
        if (unsafeWords.some(word => text.includes(word))) continue
        try {
          await btn.click({ timeout: 1500, force: true })
          await page.waitForTimeout(80)
        } catch (e: any) {
          // 弹窗可能已打开, 关闭后继续
          await page.keyboard.press('Escape').catch(() => {})
        }
        // 任何 modal 一律 Esc 关闭
        await page.keyboard.press('Escape').catch(() => {})
      }
      // 关闭可能打开的 modal, 然后回到原页面
      await page.keyboard.press('Escape').catch(() => {})
      // 关闭可能跳走的 modal 后, 不应出现白屏: 主体内容还可见
      const bodyHasContent = await page.locator('body').innerText().then(t => t.length > 50)
      if (!bodyHasContent) issues.push(`${p.name}: 点击按钮后页面空白 (URL=${page.url()})`)
      // 重置到原页面
      if (page.url() !== beforeUrl) {
        await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
      }
    }
    if (issues.length > 0) {
      console.log('按钮扫描发现以下问题:\n' + issues.join('\n'))
    }
    expect(issues.length).toBe(0)
  })

  // 路由可达性: 客户端路由通通返回 200 + 非 404 错误.
  test('ROUTE 全部路由可达', async ({ page }) => {
    await login(page, pwd)
    for (const p of PAGES) {
      const resp = await page.goto(`${FRONTEND}${p.path}`, { waitUntil: 'networkidle' })
      expect(resp?.status(), `route ${p.path} should be 200`).toBe(200)
    }
  })

  test('MENU 系统管理可展开并进入子菜单', async ({ page }) => {
    await login(page, pwd)
    await page.getByText('系统管理').click()
    await expect(page.getByText('数据存储')).toBeVisible({ timeout: 8_000 })
    await expect(page.getByText('Agent 管理')).toBeVisible({ timeout: 8_000 })
    await expect(page.getByText('告警规则')).toBeVisible({ timeout: 8_000 })
    await expect(page.getByText('参数模板')).toBeVisible({ timeout: 8_000 })

    await page.getByText('Agent 管理').click()
    await page.waitForURL(/\/dashboard\/agent-manage/, { timeout: 8_000 })
    await expect(page.locator('text=Agent 管理').first()).toBeVisible({ timeout: 8_000 })
  })
})
