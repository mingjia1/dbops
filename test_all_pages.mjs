import { createRequire } from 'module';
const require = createRequire(import.meta.url);
const { chromium } = require('playwright');
import fs from 'fs';

const BASE_URL = 'http://localhost:3000';

const results = {
  pages: {},
  consoleErrors: [],
  apiErrors: [],
  buttonIssues: [],
  total: { ok: 0, fail: 0, warnings: 0 }
};

function record(bucket, msg) {
  results[bucket].push(msg);
  results.total.fail++;
}

function warn(bucket, msg) {
  results[bucket].push('[WARN] ' + msg);
  results.total.warnings++;
}

function ok(msg) {
  results.total.ok++;
}

async function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

async function checkPage(page, pageName, url) {
  console.log(`\n=== Checking: ${pageName} (${url}) ===`);
  const pageErrors = [];
  
  // Listen for console errors
  page.on('console', msg => {
    if (msg.type() === 'error' || msg.type() === 'warning') {
      const text = msg.text();
      if (text.includes('favicon.ico')) return; // ignore favicon
      if (text.includes('Failed to load resource')) return;
      pageErrors.push(`[${msg.type()}] ${text}`);
    }
  });

  // Listen for failed requests
  page.on('requestfailed', request => {
    const url2 = request.url();
    if (url2.includes('favicon.ico')) return;
    pageErrors.push(`[REQUEST_FAILED] ${request.method()} ${url2} - ${request.failure().errorText}`);
  });

  // Listen for responses with errors
  page.on('response', response => {
    const status = response.status();
    const url2 = response.url();
    if (status >= 400 && !url2.includes('favicon.ico')) {
      if (status === 404) {
        // 404 might be expected for some endpoints
        pageErrors.push(`[API_404] ${response.request().method()} ${url2}`);
      } else if (status >= 500) {
        pageErrors.push(`[API_${status}] ${response.request().method()} ${url2}`);
      }
    }
  });

  try {
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await sleep(3000); // wait for lazy-loaded components
    
    // Check for visible errors in the page
    const bodyText = await page.textContent('body');
    const visibleErrors = [];
    if (bodyText.includes('Error') || bodyText.includes('error')) {
      const errorElements = await page.locator('.ant-message-notice, .ant-notification-notice, .ant-alert-error').allTextContents();
      for (const err of errorElements) {
        visibleErrors.push(err);
      }
    }

    results.pages[pageName] = {
      url,
      consoleErrors: [...pageErrors],
      visibleErrors,
      buttons: await page.locator('button').count(),
    };

    if (pageErrors.length > 0) {
      console.log(`  ⚠️  Found ${pageErrors.length} issues:`);
      pageErrors.forEach(e => console.log(`    - ${e}`));
      for (const err of pageErrors) {
        if (err.startsWith('[API_500]') || err.startsWith('[API_404]')) {
          record('apiErrors', `${pageName}: ${err}`);
        } else {
          record('consoleErrors', `${pageName}: ${err}`);
        }
      }
    } else {
      ok(`${pageName}: OK (${results.pages[pageName].buttons} buttons)`);
      console.log(`  ✅ OK (${results.pages[pageName].buttons} buttons)`);
    }
  } catch (err) {
    record('consoleErrors', `${pageName}: PAGE LOAD FAILED - ${err.message}`);
    console.log(`  ❌ FAILED: ${err.message}`);
  }

  page.removeAllListeners('console');
  page.removeAllListeners('requestfailed');
  page.removeAllListeners('response');
  
  return pageErrors;
}

async function main() {
  console.log('Starting browser...');
  const browser = await chromium.launch({ 
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 }
  });
  const page = await context.newPage();

  try {
    // ---- LOGIN ----
    console.log('\n=== LOGIN ===');
    await page.goto(`${BASE_URL}/login`, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await sleep(2000);
    
    // Check login page
    const loginErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') loginErrors.push(`[CONSOLE_ERROR] ${msg.text()}`);
    });

    // Fill login form
    await page.fill('input[placeholder="用户名"]', 'admin');
    await page.fill('input[placeholder="密码"]', 'admin123');
    await page.click('button:has-text("登 录")');
    
    await sleep(3000);
    
    const currentUrl = page.url();
    console.log(`After login URL: ${currentUrl}`);
    
    if (currentUrl.includes('/dashboard') || currentUrl.includes('/home')) {
      console.log('✅ Login successful!');
      ok('Login: OK');
    } else {
      console.log(`⚠️ Login may have issues, current URL: ${currentUrl}`);
      warn('consoleErrors', 'Login: Redirect may not have worked');
    }

    page.removeAllListeners('console');

    // ---- NAVIGATE THROUGH ALL PAGES ----
    const pagesToCheck = [
      { name: '总览', url: `${BASE_URL}/dashboard/home` },
      { name: '监控仪表盘', url: `${BASE_URL}/dashboard/monitor` },
      { name: '主机管理', url: `${BASE_URL}/dashboard/hosts` },
      { name: '实例管理', url: `${BASE_URL}/dashboard/instances` },
      { name: '环境检查', url: `${BASE_URL}/dashboard/env-check` },
      { name: '备份管理', url: `${BASE_URL}/dashboard/backup` },
      { name: '集群部署', url: `${BASE_URL}/dashboard/cluster-deploy` },
      { name: '高可用管理', url: `${BASE_URL}/dashboard/ha` },
      { name: '角色切换', url: `${BASE_URL}/dashboard/role-switch` },
      { name: '升级管理', url: `${BASE_URL}/dashboard/upgrade` },
      { name: '数据迁移', url: `${BASE_URL}/dashboard/migration` },
      { name: '拓扑视图', url: `${BASE_URL}/dashboard/topology` },
      { name: '审批管理', url: `${BASE_URL}/dashboard/approvals` },
      { name: '审计日志', url: `${BASE_URL}/dashboard/audit-logs` },
      { name: '数据存储', url: `${BASE_URL}/dashboard/data-storage` },
      { name: 'Agent管理', url: `${BASE_URL}/dashboard/agent-manage` },
      { name: '插件管理', url: `${BASE_URL}/dashboard/plugins` },
      { name: '告警规则', url: `${BASE_URL}/dashboard/alert-rules` },
      { name: '参数模板', url: `${BASE_URL}/dashboard/parameter-templates` },
      { name: '系统设置', url: `${BASE_URL}/dashboard/security-settings` },
    ];

    for (const p of pagesToCheck) {
      await checkPage(page, p.name, p.url);
    }

  } catch (err) {
    console.error('Test script error:', err);
    record('consoleErrors', `SCRIPT_ERROR: ${err.message}`);
  } finally {
    await browser.close();
  }

  // ---- Write results to JSON ----
  fs.writeFileSync('test_results.json', JSON.stringify(results, null, 2));
  console.log('\n\n========== RESULTS SUMMARY ==========');
  console.log(`OK: ${results.total.ok}, Fail: ${results.total.fail}, Warnings: ${results.total.warnings}`);
  console.log(`Console errors: ${results.consoleErrors.length}`);
  console.log(`API errors: ${results.apiErrors.length}`);
  console.log('\nDetailed results written to test_results.json');
  
  // Print summary
  for (const [name, data] of Object.entries(results.pages)) {
    const errCount = data.consoleErrors.length + data.visibleErrors.length;
    const icon = errCount === 0 ? '✅' : '❌';
    console.log(`${icon} ${name}: ${errCount} errors, ${data.buttons} buttons`);
  }
}

main().catch(console.error);
