const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

async function login(page, account, landing) {
  await page.goto('/login');
  await page.locator('#page input[name="email"]').fill(`demo.${account}01@weevcrafts.local`);
  await page.locator('#page input[name="password"]').fill('WeeVCrafts-Demo-2026!');
  await page.getByRole('button', { name: /continue with email|sign in/i }).click();
  await page.waitForURL(url => url.pathname === landing);
}

test('customer-facing options, stock states and removable filter context', async ({ page }) => {
  await page.goto('/products/blue-tote');
  await expect(page.locator('.detail-actions')).not.toContainText('Default variant');
  await expect(page.locator('.detail-actions')).not.toContainText('SKU');
  await page.goto('/products?category=arts&availability=in-stock');
  await expect(page.getByRole('link', { name: 'Remove In stock', exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Remove In stock', exact: true }).click();
  await expect(page).toHaveURL(/category=arts/);
  expect(new URL(page.url()).searchParams.has('availability')).toBe(false);
  await expect(page.locator('.filter-chips')).toContainText('Category:');
  await page.goto('/products');
  for (const card of await page.locator('.product-card').all()) {
    if (/Sold out/.test(await card.innerText())) {
      await expect(card.getByRole('button', { name: 'Sold out', exact: true })).toBeDisabled();
    }
  }
});

test('seller validation focuses errors and a rejected save retains fields without duplicate requests', async ({ page }) => {
  await login(page, 'owner', '/seller-admin');
  await page.goto('/seller-admin/products/new');
  const form = page.locator('form[data-product-builder]');
  await expect(form.locator('input[name="price_rupees"]')).toHaveCSS('font-size', '16px');
  await expect(form.locator('input[name="price_rupees"]')).toHaveCSS('min-height', '44px');
  await form.getByRole('button', { name: 'Save product draft', exact: true }).click();
  await expect(form.locator('input[name="name"]')).toBeFocused();
  await expect(form.locator('input[name="name"]')).toHaveAttribute('aria-invalid', 'true');
  await form.locator('input[name="name"]').fill('Unsaved UI regression draft');
  await form.locator('select[name="category_slug"]').selectOption({ index: 1 });
  await form.locator('input[name="sku"]').fill('UI-UNSAVED-ONLY');
  await form.locator('input[name="price_rupees"]').fill('2499.50');
  await form.locator('input[name="compare_at_rupees"]').fill('0');
  await form.locator('textarea[name="description"]').fill('This draft must stay in the form after a rejected save.');
  let posts = 0;
  let release;
  const barrier = new Promise(resolve => { release = resolve; });
  await page.route('**/seller-admin/products', async route => {
    posts++;
    await barrier;
    await route.fulfill({ status: 422, contentType: 'text/html', body: '<main><p>Review the product details and try again.</p></main>' });
  });
  const save = form.getByRole('button', { name: 'Save product draft', exact: true });
  await save.click();
  await expect(save).toBeDisabled();
  release();
  await expect(form.getByRole('alert')).toContainText('Review the product details');
  await expect(form.getByRole('alert')).toBeFocused();
  await expect(save).toBeEnabled();
  await expect(form.locator('input[name="price_rupees"]')).toHaveValue('2499.50');
  await expect(form.locator('input[name="name"]')).toHaveValue('Unsaved UI regression draft');
  expect(posts).toBe(1);
});

test('archive confirmation supports Escape, cancel focus and no mutation', async ({ page }) => {
  await login(page, 'owner', '/seller-admin');
  await page.goto('/seller-admin/products');
  const row = page.locator('tr').filter({ has: page.locator('form[action="/seller-admin/products/archive"]') }).first();
  await row.getByText('Manage product', { exact: true }).click();
  await row.locator('form[action="/seller-admin/products/archive"] input[name="reason"]').fill('Keyboard cancellation verification');
  let posts = 0;
  page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/products/archive')) posts++; });
  const archive = row.getByRole('button', { name: 'Archive', exact: true });
  await archive.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused();
  await dialog.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(archive).toBeFocused();
  expect(posts).toBe(0);
});

test('saved and new checkout addresses agree with fields and delivery location', async ({ page }) => {
  await login(page, 'customer', '/products');
  await page.goto('/cart');
  let added = false;
  try {
    if (await page.locator('.cart-line').count() === 0) {
      await page.goto('/products/blue-tote');
      const response = page.waitForResponse(r => r.request().method() === 'POST' && r.url().includes('/api/v1/cart/add'));
      await page.locator('.detail-actions').getByRole('button', { name: 'Add to cart', exact: true }).click();
      expect((await response).ok()).toBe(true);
      added = true;
    }
    await page.goto('/cart');
    await expect(page.locator('[data-backend-cart]')).toHaveAttribute('data-backend-ready', 'true');
    await expect(page.locator('.cart-items')).not.toContainText('Default variant');
    await page.goto('/checkout');
    await expect(page.locator('.checkout-items, .checkout-main')).not.toContainText('Default variant');
    const form = page.locator('#checkout-address-form');
    const select = page.locator('[data-checkout-address-select]');
    await expect(select).toBeVisible();
    await select.selectOption({ index: 1 });
    const city = await form.locator('[name="city"]').inputValue();
    const state = await form.locator('[name="state"]').inputValue();
    await expect(page.locator('.header-main .location small')).toHaveText(`${city}, ${state}`);
    await select.selectOption('');
    await expect(form.locator('[name="line1"]')).toHaveValue('');
    await expect(form.locator('[name="city"]')).toHaveValue('');
    await expect(page.locator('.header-main .location small')).toHaveText('Choose delivery location');
    await expect(form.locator('[name="line1"]')).toBeFocused();
  } finally {
    if (added) {
      await page.goto('/cart');
      const remove = page.locator('.cart-line').filter({ hasText: 'Indigo Everyday Tote' }).getByRole('button', { name: /Remove/ });
      await remove.click();
      await expect(page.locator('.cart-line').filter({ hasText: 'Indigo Everyday Tote' })).toHaveCount(0);
    }
  }
});

test('updated surfaces satisfy automated accessibility and narrow layout checks', async ({ page }, testInfo) => {
  test.setTimeout(180000);
  const axePath = path.resolve('tmp/ui-audit/package/axe.min.js');
  test.skip(!fs.existsSync(axePath), 'Local axe audit dependency is unavailable.');
  const results = [];
  for (const group of [
    { routes: ['/', '/products', '/products/blue-tote', '/makers'] },
    { account: 'customer', landing: '/products', routes: ['/cart', '/checkout', '/account'] },
    { account: 'owner', landing: '/seller-admin', routes: ['/seller-admin', '/seller-admin/products/new'] },
    { account: 'super', landing: '/super-admin', routes: ['/super-admin', '/super-admin/products'] },
    { account: 'support', landing: '/support-portal', routes: ['/support-portal/cases'] },
  ]) {
    if (group.account) {
      await page.context().clearCookies();
      await login(page, group.account, group.landing);
    }
    for (const route of group.routes) {
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 844 });
        const response = await page.goto(route, { waitUntil: 'networkidle' });
        expect(response.status(), route).toBe(200);
        await page.addScriptTag({ path: axePath });
        const violations = await page.evaluate(async () => (await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa'] } })).violations.map(v => ({ id: v.id, impact: v.impact, nodes: v.nodes.map(n => ({ target: n.target, summary: n.failureSummary })) })));
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2 || [...document.querySelectorAll('.home-rail .category-card')].some(card => { const action=card.querySelector('b'); return action && action.getClientRects().length && action.getBoundingClientRect().bottom > card.getBoundingClientRect().bottom + 1; }));
        results.push({ route, width, overflow, violations });
      }
    }
  }
  const output = path.resolve('docs/design/ui-ux-implementation-2026-10-10-accessibility.json');
  fs.writeFileSync(output, JSON.stringify(results, null, 2));
  await testInfo.attach('accessibility and layout', { path: output, contentType: 'application/json' });
  expect(results.filter(result => result.overflow || result.violations.length), 'See saved accessibility report').toEqual([]);
});
