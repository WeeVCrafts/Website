const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');

const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
test('INT-027--qa-and-technical-operator', async ({ page }) => {
  test.skip(target.hostname !== '127.0.0.1' || target.port !== '18081', 'Requires the disposable production-configured QA guard instance.');
  await page.goto('/login');
  await page.locator('#page input[name="email"]').fill('demo.customer01@weevcrafts.local');
  await page.locator('#page input[name="password"]').fill('WeeVCrafts-Demo-2026!');
  await page.getByRole('button', { name: /sign in/i }).click();
  await page.waitForURL(/\/products\/?$/);
  const csrf = await page.locator('input[name="csrf_token"]').first().inputValue();
  const state = () => execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', 'weev_qa_20261009_guard', '-tAc', "SELECT status FROM orders WHERE order_number='WCDEMO1001'"], { encoding: 'utf8' }).trim();
  const before = state();
  for (const route of ['/api/v1/local/addresses', '/api/v1/local/settings']) {
    expect((await page.request.get(route)).status(), route).toBe(404);
  }
  const response = await page.request.post('/dev/payments/WCDEMO1001/capture', { headers: { 'X-CSRF-Token': csrf } });
  // Report the business effect as well as route status when the guard fails.
  expect.soft(state(), 'Disabled development capture changed persisted order state').toBe(before);
  expect(response.status(), 'Disabled development capture endpoint must not be registered').toBe(404);
});
