const { chromium } = require('@playwright/test');
const pages = [
  '/#/audit',
  '/#/user-profiles',
  '/#/user-profiles/user-001',
  '/#/behavior',
  '/#/sessions'
];
(async() => {
  const browser = await chromium.launch({ headless: true });
  for (const path of pages) {
    const page = await browser.newPage();
    const errors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') errors.push(msg.text());
    });
    page.on('pageerror', err => errors.push('pageerror: ' + err.message));
    await page.goto('http://127.0.0.1:9090' + path, { waitUntil: 'networkidle', timeout: 30000 });
    const body = await page.locator('body').innerText();
    const appHtml = await page.locator('#app').innerHTML().catch(() => '');
    console.log('PAGE', path);
    console.log(JSON.stringify({
      title: await page.title(),
      body_len: body.length,
      app_len: appHtml.length,
      errors: errors.slice(0, 5)
    }));
    await page.close();
  }
  await browser.close();
})();
