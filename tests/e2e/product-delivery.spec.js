const { test, expect } = require('@playwright/test');
const axe = require('axe-core');

test('product PIN check uses the current option and quantity and announces carrier results', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const requests = [];
  await page.route('**/api/v1/products/layout-mug/delivery?*', async route => {
    const query = new URL(route.request().url()).searchParams;
    requests.push(Object.fromEntries(query));
    await route.fulfill({ json: { state: 'available', transit_days: 4 } });
  });
  await page.goto('/products/layout-mug');
  const form = page.locator('[data-product-delivery]');
  await expect(form).toBeVisible();
  const pin = page.getByLabel('Check delivery', { exact: true });
  const status = form.getByRole('status');
  await pin.fill('050079');
  await form.getByRole('button', { name: 'Check PIN' }).press('Enter');
  await expect(pin).toBeFocused();
  await expect(pin).toHaveAttribute('aria-invalid', 'true');
  expect(requests).toHaveLength(0);
  await page.locator('#detail-variant').selectOption('102');
  await pin.fill('500079');
  await page.getByRole('button', { name: /Increase.*quantity/ }).press('Enter');
  await form.getByRole('button', { name: 'Check PIN' }).press('Enter');
  await expect(status).toContainText('Estimated transit after dispatch: 4 days.');
  expect(requests).toEqual([{ postal_code: '500079', variant_id: '102', quantity: '2' }]);
  await page.getByRole('button', { name: /Decrease.*quantity/ }).press('Enter');
  await expect(status).toBeEmpty();
  await pin.fill('110030');
  await page.unroute('**/api/v1/products/layout-mug/delivery?*');
  await page.route('**/api/v1/products/layout-mug/delivery?*', route => route.fulfill({ json: { state: 'unserviceable' } }));
  await form.getByRole('button', { name: 'Check PIN' }).press('Enter');
  await expect(status).toContainText('No courier currently serves PIN 110030');
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2)).toBe(false);
  await page.addScriptTag({ content: axe.source });
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    const failures = await page.evaluate(async () => {
      const result = await axe.run(document.querySelector('.site-header'), { runOnly: ['label-content-name-mismatch', 'link-name', 'button-name'] });
      return result.violations.map(v => ({ id: v.id, targets: v.nodes.map(n => n.target) }));
    });
    expect(failures).toEqual([]);
  }
});

test('changing PIN discards an earlier serviceability response', async ({ page }) => {
  let release;
  const pending = new Promise(resolve => { release = resolve; });
  await page.route('**/api/v1/products/layout-mug/delivery?*', async route => {
    await pending;
    await route.fulfill({ json: { state: 'available', transit_days: 2 } }).catch(() => {});
  });
  await page.goto('/products/layout-mug');
  const form = page.locator('[data-product-delivery]');
  const pin = page.getByLabel('Check delivery', { exact: true });
  await pin.fill('500079');
  await form.getByRole('button', { name: 'Check PIN' }).click();
  await expect(form.getByRole('status')).toContainText('Checking delivery');
  await pin.fill('110030');
  release();
  await expect(form.getByRole('status')).toBeEmpty();
  await expect(form.getByRole('button', { name: 'Check PIN' })).toBeEnabled();
});

test('email delivery history keeps its records and keyboard access on a phone', async ({ page }) => {
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/super-admin/support/email');
    await expect(page.getByRole('heading', { name: 'Order emails', exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: 'Delivered to mail server' })).toBeVisible();
    await expect(page.getByRole('cell', { name: /Bounced$/ })).toBeVisible();
    const region = page.getByRole('region', { name: 'Order email delivery records' });
    await region.focus();
    await expect(region).toBeFocused();
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2)).toBe(false);
  }
});
