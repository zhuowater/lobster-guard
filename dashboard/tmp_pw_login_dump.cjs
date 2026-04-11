const { chromium } = require('@playwright/test');
(async() => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  await page.goto('http://127.0.0.1:9090/#/user-profiles', { waitUntil: 'networkidle', timeout: 30000 });
  console.log('BODY', await page.locator('body').innerText());
  console.log('APP', await page.locator('#app').innerHTML());
  await page.screenshot({ path:'/root/lobster-guard/dashboard/tmp-login.png', fullPage:true});
  await browser.close();
})();