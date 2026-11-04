const { test, expect } = require('@playwright/test');

test.beforeAll(() => {
  if (process.env.WEEVCRAFTS_QA_DATABASE !== 'seller_workflows_20261017') throw new Error('Use the isolated seller_workflows_20261017 preview on port 18080.');
});
async function login(page, account) {
  const response = await page.request.post('/api/v1/auth/login', { data: { email: `demo.${account}@weevcrafts.local`, password: 'WeeVCrafts-Demo-2026!' } });
  expect(response.ok()).toBeTruthy();
}
async function fits(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
}

test('customer can open the latest owned order and return to support', async ({ page }) => {
  await login(page, 'customer01'); await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/account');
  const action = page.getByRole('link', { name: 'View latest order' });
  const destination = await action.getAttribute('href');
  expect(destination).toMatch(/^\/orders\/.+/);
  await action.focus(); await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(destination.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$'));
  await expect(page.locator('main')).toContainText('Order'); await fits(page);
  await page.goto('/account'); await page.getByRole('link', { name: 'Open support', exact: true }).click();
  await expect(page).toHaveURL(/\/account\/support$/);
});

test('mobile catalogue exposes the first products without a full-screen banner', async ({ page }) => {
  await login(page, 'customer01'); await page.goto('/products');
  for (const width of [320, 390, 720, 1024, 1440]) {
    await page.setViewportSize({ width, height: 844 }); await fits(page);
    if (width <= 720) {
      const first = await page.locator('.product-grid .product-card').first().boundingBox();
      expect(first.y).toBeLessThan(720);
      const banner = await page.locator('.listing-banner-copy').boundingBox();
      expect(banner.width).toBeGreaterThan(width - 70);
    }
  }
  const targets = await page.locator('.site-footer .footer-grid > div > a:not(.wordmark)').evaluateAll(links => links.map(a => a.getBoundingClientRect().height));
  expect(targets.every(height => height >= 32)).toBeTruthy();
});

test('seller stock guidance opens the filtered queue and profile has a current marker', async ({ page }) => {
  await login(page, 'owner01'); await page.goto('/seller-admin');
  await page.getByRole('link', { name: 'Review low stock', exact: true }).click();
  await expect(page).toHaveURL(/\/seller-admin\/inventory\?stock=low$/);
  await page.goto('/seller-admin/settings');
  await expect(page.locator('#seller-sidebar').getByRole('link', { name: 'Store profile', exact: true })).toHaveAttribute('aria-current', 'page');
});

test('seller header searches a supported queue from settings and preserves order filters', async ({ page }) => {
  await login(page, 'owner01'); await page.goto('/seller-admin/settings');
  await page.getByRole('searchbox', { name: 'Search products', exact: true }).fill('unmatched-ui-audit-query');
  await page.locator('.seller-search').getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page).toHaveURL(/\/seller-admin\/products\?q=unmatched-ui-audit-query$/);
  await expect(page.locator('main')).toContainText('No products');
  await page.goto('/seller-admin/orders?tab=processing');
  await page.getByRole('searchbox', { name: 'Search orders', exact: true }).fill('WC-SELLER');
  await page.locator('.seller-search').getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page).toHaveURL(/tab=processing/); await expect(page).toHaveURL(/q=WC-SELLER/);
});

test('Escape keeps seller form focus and restores an unsaved-confirmation trigger', async ({ page }) => {
  await login(page, 'owner01'); await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/seller-admin/products/new');
  const title = page.locator('[data-product-builder] input[name=name]');
  await title.fill('Unsaved UI audit title'); await page.keyboard.press('Escape'); await expect(title).toBeFocused();
  const cancel = page.locator('.seller-form-actions').getByRole('link', { name: 'Cancel', exact: true });
  await cancel.click(); const dialog = page.getByRole('dialog'); await expect(dialog).toBeVisible();
  await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0); await expect(cancel).toBeFocused();
  await expect(title).toHaveValue('Unsaved UI audit title');
});

test('seller failed save retains product fields and offers correction without losing input', async ({ page }) => {
  await login(page, 'owner01'); await page.goto('/seller-admin/products/new');
  const form = page.locator('[data-product-builder]');
  await form.locator('[name=name]').fill('Audit retained draft');
  await form.locator('[name=description]').fill('A real field-retention browser check.');
  const category = await form.locator('[name=category_slug] option').nth(1).getAttribute('value');
  await form.locator('[name=category_slug]').selectOption(category);
  await form.locator('[name=price_rupees]').fill('199.25');
  await page.route('**/seller-admin/products', route => route.request().method() === 'POST' ? route.fulfill({ status: 503, contentType: 'text/plain', body: 'Unavailable' }) : route.continue());
  await form.getByRole('button', { name: 'Save product draft', exact: true }).click();
  await expect(form.locator('.ui-form-error')).toBeVisible();
  await expect(form.locator('[name=name]')).toHaveValue('Audit retained draft');
  await expect(form.locator('[name=price_rupees]')).toHaveValue('199.25');
  await expect(form.getByRole('button', { name: 'Save product draft', exact: true })).toBeEnabled();
});

for (const [account, root] of [['marketplace01', '/admin'], ['super01', '/super-admin']]) {
  test(`${root} category tools reflow and mobile navigation contains keyboard focus`, async ({ page }) => {
    await login(page, account); await page.setViewportSize({ width: 320, height: 844 });
    await page.goto(root + '/category-management'); await fits(page);
    if (root === '/admin') {
      await page.getByRole('link', { name: 'Edit category presentation', exact: true }).click();
      await expect(page).toHaveURL(new RegExp(root + '/storefront\\?tab=categories$')); await fits(page);
    } else {
      await expect(page.getByRole('link', { name: 'Edit category presentation', exact: true })).toHaveCount(0);
      await expect(page.locator('#admin-sidebar a[href*="/storefront"]')).toHaveCount(0);
      await page.goto('/super-admin/storefront?tab=categories');
      await expect(page).toHaveURL(/\/super-admin\?notice=/);
      await expect(page.locator('.admin-notice')).toContainText('Storefront editor has been removed');
      for (const route of ['/super-admin/storefront/save', '/super-admin/storefront/publish', '/admin/storefront/save']) {
        expect((await page.request.post(route, { maxRedirects: 0 })).status()).toBe(410);
      }
    }
    const menu = page.getByRole('button', { name: 'Open admin navigation', exact: true });
    await menu.focus(); await page.keyboard.press('Enter');
    await expect(page.locator('#admin-sidebar')).toHaveAttribute('aria-modal', 'true');
    await page.keyboard.press('Shift+Tab');
    expect(await page.evaluate(() => !!document.activeElement.closest('#admin-sidebar'))).toBeTruthy();
    await page.keyboard.press('Escape'); await expect(menu).toBeFocused();
  });
}

test('Super Admin dashboard links directly to accounts and roles; Admin remains denied', async ({ page }) => {
  await login(page, 'super01'); await page.goto('/super-admin');
  const navigation = page.getByRole('navigation', { name: 'Identity and access', exact: true });
  await navigation.getByRole('link', { name: 'Manage accounts', exact: true }).click(); await expect(page).toHaveURL(/\/super-admin\/users$/);
  await page.goto('/super-admin'); await navigation.getByRole('link', { name: 'Assign roles', exact: true }).click(); await expect(page).toHaveURL(/\/super-admin\/roles$/);
  await login(page, 'marketplace01'); await page.goto('/admin');
  await expect(page.getByRole('navigation', { name: 'Identity and access', exact: true })).toHaveCount(0);
  expect((await page.request.get('/admin/users')).status()).toBe(403);
});
