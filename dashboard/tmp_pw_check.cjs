const { chromium } = require('@playwright/test');
(async() => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on('console', msg => console.log('console', msg.type(), msg.text()));
  page.on('pageerror', err => console.log('pageerror', err.message));
  await page.goto('http://127.0.0.1:9090/#/user-profiles', { waitUntil: 'networkidle', timeout: 30000 });
  await page.screenshot({ path: '/root/lobster-guard/dashboard/tmp-user-profiles.png', fullPage: true });
  const bodyText = await page.locator('body').innerText();
  console.log('TITLE', await page.title());
  console.log('URL', page.url());
  console.log('BODY', bodyText.slice(0, 2000));
  console.log('APPHTML', (await page.locator('#app').innerHTML()).slice(0, 2000));
  await browser.close();
})();
