const { test, expect } = require('@playwright/test');
const path = require('node:path');
const fs = require('node:fs/promises');
const os = require('node:os');

const base = process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18080';
const output = process.env.WEEVCRAFTS_UI_ARTIFACTS || path.join(os.tmpdir(), 'weevcrafts-reference-20261015');
test.use({ baseURL: base });

test.beforeEach(async () => {
  if (new URL(base).hostname !== '127.0.0.1' || new URL(base).port !== '18080') throw new Error('Reference QA only runs against the isolated app at 127.0.0.1:18080.');
  await fs.mkdir(output, { recursive: true });
});
async function login(page) {
  const result = await page.request.post('/api/v1/auth/login', { data: { email: 'demo.customer08@weevcrafts.local', password: 'WeeVCrafts-Demo-2026!' } });
  expect(result.ok()).toBeTruthy();
}
async function ready(page) {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForFunction(() => !!document.querySelector('.site-header')?._x_dataStack);
}

test('five slides advance every four seconds and pause accessibly', async ({ page }) => {
  const instant = new Date('2026-10-15T08:00:00Z');
  await page.clock.install({ time: instant });
  await page.clock.pauseAt(new Date(instant.getTime() + 1000));
  await page.goto('/'); await ready(page);
  await expect(page.locator('[data-hero-slide]')).toHaveCount(5);
  await expect(page.locator('[data-hero-autoplay]')).toHaveAccessibleName('Pause slideshow');
  await page.clock.runFor(3999);
  await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
  await page.clock.runFor(1);
  await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
  for (const count of [3, 4, 5, 1]) {
    await page.clock.runFor(4000);
    await expect(page.locator('[data-hero-count]')).toHaveText(`${count} / 5`);
  }
  await page.locator('[data-hero-autoplay]').press('Enter');
  await page.clock.runFor(8000);
  await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
  await page.locator('[data-hero-autoplay]').click();
  await page.clock.runFor(4000);
  await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
  await page.locator('[data-hero-next]').press('Enter');
  await page.clock.runFor(8000);
  await expect(page.locator('[data-hero-count]')).toHaveText('3 / 5');
  await expect(page.locator('[data-hero-autoplay]')).toHaveAccessibleName('Play slideshow');
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.reload(); await ready(page);
  await page.clock.runFor(8000);
  await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
  await expect(page.locator('[data-hero-autoplay]')).toHaveAccessibleName('Play slideshow');
});

test('reference layouts reflow and retain accessible navigation', async ({ page }) => {
  test.setTimeout(180_000);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  for (const width of [1448, 1440, 1024, 390, 320]) {
    await page.setViewportSize({ width, height: width > 800 ? 1086 : 844 });
    for (const route of ['/', '/products?category=home', '/products']) {
      await page.goto(route); await ready(page);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${route} overflow at ${width}`).toBeTruthy();
      const label = route === '/' ? 'home' : route.includes('home') ? 'home-living' : 'catalog';
      await page.screenshot({ path: path.join(output, `${label}-${width}.png`), fullPage: true });
    }
    const menu = page.locator(width <= 800 ? '.menu-toggle' : '.all-categories');
    await menu.press('Enter'); await expect(page.locator('#mobile-menu')).toBeVisible();
    await page.keyboard.press('Escape'); await expect(menu).toBeFocused();
  }
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/'); await ready(page);
  await page.locator('[data-hero-next]').press('Enter');
  await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
  expect(errors).toEqual([]);
});

test('quick view uses live variants, rejects unavailable choices and adds once', async ({ page }) => {
  test.setTimeout(90_000);
  await login(page);
  await page.goto('/products'); await ready(page);
  const link = page.locator('[data-product="blue-tote"] [data-quick-view]');
  await link.click();
  await expect(page.locator('[data-quick-product]')).toBeVisible();
  const modal = page.locator('#product-quick-view');
  const select = modal.locator('[data-quick-variant]');
  const add = modal.locator('[data-quick-add]');
  await expect(add).toBeDisabled();
  const unavailable = await select.locator('option').filter({ hasText: 'Extra large' }).getAttribute('value');
  await select.selectOption(unavailable); await expect(add).toBeDisabled();
  const chosen = await select.locator('option').filter({ hasText: 'Large · natural cotton' }).getAttribute('value');
  await page.evaluate(id => window.weevcraftsBackend.removeCart('blue-tote', id), chosen);
  await select.selectOption(chosen); await expect(add).toBeEnabled();
  await expect(modal.locator('[data-quick-price]')).toContainText('1049');
  await page.addScriptTag({ path: require.resolve('axe-core') });
  expect(await modal.evaluate(async element => (await axe.run(element, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'] } })).violations.map(item => item.id))).toEqual([]);
  await page.route('**/api/v1/cart/add', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { message: 'QA temporary cart failure' } }) }));
  await add.click();
  await expect(modal.locator('[data-quick-feedback]')).not.toBeEmpty();
  await expect(modal).toBeVisible(); await expect(select).toHaveValue(chosen);
  await page.unroute('**/api/v1/cart/add');
  let additions = 0;
  await page.route('**/api/v1/cart/add', async route => { additions++; await new Promise(resolve => setTimeout(resolve, 150)); await route.continue(); });
  await add.evaluate(button => { button.click(); button.click(); });
  await expect(page.locator('#cart-drawer')).toBeVisible();
  expect(additions).toBe(1);
  await expect(modal).not.toBeVisible();
  await expect(page.locator('[data-cart-drawer-items]')).toContainText('Large · natural cotton');
  await page.keyboard.press('Escape'); await expect(link).toBeFocused();
  await page.evaluate(id => window.weevcraftsBackend.removeCart('blue-tote', id), chosen);
  await link.click(); await expect(page.locator('[data-quick-product]')).toBeVisible();
  await page.keyboard.press('Escape'); await expect(link).toBeFocused();
});

test('quick view recovers from failed reads and preserves native full-page routes', async ({ page }) => {
  await page.goto('/products'); await ready(page);
  const link = page.locator('[data-product="blue-tote"] [data-quick-view]');
  await page.route('**/products/blue-tote', async route => {
    if (route.request().headers()['hx-target'] === 'product-quick-view-content') await route.fulfill({ status: 503, body: 'Unavailable' });
    else await route.continue();
  });
  await link.click(); await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
  await page.unroute('**/products/blue-tote');
  await page.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(page.locator('[data-quick-product]')).toBeVisible();
  await page.getByRole('button', { name: 'Close quick view', exact: true }).click();
  await expect(link).toBeFocused();
  const fragment = await page.request.get('/products/blue-tote', { headers: { 'HX-Request': 'true', 'HX-Target': 'product-quick-view-content' } });
  expect(fragment.ok()).toBeTruthy();
  expect(fragment.headers()['cache-control']).toContain('no-store');
  expect(await fragment.text()).not.toContain('<html');
  const full = await page.request.get('/products/blue-tote');
  expect(await full.text()).toContain('<html');
  await link.click(); await expect(page.locator('[data-quick-product]')).toBeVisible();
  const select = page.locator('#quick-view-variant');
  await select.selectOption(await select.locator('option[data-in-stock="true"]').first().getAttribute('value'));
  await page.locator('[data-quick-add]').click();
  await expect(page.locator('#guest-login-dialog')).toBeVisible();
  await expect(page.locator('#product-quick-view')).not.toBeVisible();
  await page.keyboard.press('Escape'); await expect(link).toBeFocused();
});

test('server wishlist swaps preserve compact and actionable cards', async ({ page }) => {
  await login(page); await page.goto('/products'); await ready(page);
  const original = await page.locator('[data-product="blue-tote"] .heart').getAttribute('aria-pressed');
  const csrf = await page.locator('body').getAttribute('hx-headers').then(value => JSON.parse(value)['X-CSRF-Token']);
  for (const actions of ['false', 'true']) {
    const response = await page.request.post('/ui/wishlist/blue-tote', { headers: { 'X-CSRF-Token': csrf }, form: { card_actions: actions } });
    expect(response.ok()).toBeTruthy();
    const html = await response.text();
    expect(html.includes('data-quick-view')).toBe(actions === 'true');
    expect(html.includes('product-card--compact')).toBe(actions === 'false');
    expect(html).toContain('wishlist-count');
  }
  if (original === 'false') await page.evaluate(() => window.weevcraftsBackend.removeWishlist('blue-tote'));
});

test('all customer route families use the reference shell without overflow', async ({ page }) => {
  test.setTimeout(240_000);
  await login(page);
  const routes = ['/categories', '/makers', '/brands', '/deals', '/products/blue-tote', '/wishlist', '/cart', '/checkout', '/orders', '/tracking', '/returns', '/payments', '/account', '/account/profile', '/account/addresses', '/account/notifications', '/account/security', '/account/sessions', '/account/privacy', '/account/reviews', '/account/support', '/notifications', '/faq', '/contact', '/shipping', '/our-story', '/terms', '/privacy'];
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const route of routes) {
      const response = await page.goto(route); await ready(page);
      expect(response.status(), route).toBeLessThan(400);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${route} overflow at ${width}`).toBeTruthy();
      await expect(page.locator('.storefront-nav')).toHaveCount(1);
      if (['/account', '/account/support', '/cart', '/products/blue-tote', '/orders'].includes(route)) await page.screenshot({ path: path.join(output, route.replaceAll('/', '-') + `-${width}.png`), fullPage: true });
    }
  }
});
