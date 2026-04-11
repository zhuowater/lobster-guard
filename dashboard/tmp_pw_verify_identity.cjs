const { chromium } = require('@playwright/test');

const checks = [
  { path: '/#/audit', mustContain: ['新接入Bot', '创新实验室'] },
  { path: '/#/user-profiles', mustContain: ['李四', '研发二部'] },
  { path: '/#/user-profiles/user-001', mustContain: ['张三', '研发一部'] },
  { path: '/#/agent', mustContain: ['数据治理Bot', '数据工程部'] },
  { path: '/#/sessions', mustContain: ['张明', '安全团队'] },
];

(async() => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on('console', msg => { if (msg.type() === 'error') console.log('console-error', msg.text()); });
  page.on('pageerror', err => console.log('pageerror', err.message));

  await page.goto('http://127.0.0.1:9090/#/login', { waitUntil: 'networkidle', timeout: 30000 });
  await page.locator('#username').fill('admin');
  await page.locator('#password').fill('admin123');
  await page.locator('button[type="submit"]').click();
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(1000);

  const results = [];
  for (const check of checks) {
    await page.goto('http://127.0.0.1:9090' + check.path, { waitUntil: 'networkidle', timeout: 30000 });
    await page.waitForTimeout(1500);
    const text = await page.locator('body').innerText();
    const ok = check.mustContain.every(x => text.includes(x));
    const shot = '/root/lobster-guard/dashboard' + check.path.replace(/[^a-z0-9]+/gi, '_') + '.png';
    await page.screenshot({ path: shot, fullPage: true });
    results.push({ path: check.path, ok, mustContain: check.mustContain, screenshot: shot, bodySample: text.slice(0, 1200) });
  }

  console.log(JSON.stringify(results, null, 2));
  await browser.close();
})();
