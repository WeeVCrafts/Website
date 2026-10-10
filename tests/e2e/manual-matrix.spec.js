const { test, expect } = require('@playwright/test');

// Execution IDs correspond to docs/test-execution-tracker.csv. Keep each
// assertion set limited to the complete procedure named by that execution ID.
const roles = [
  ['customer', 'customer01', '/account', 'Priya Sharma', 'My account'],
  ['seller-owner', 'owner01', '/seller-admin', 'Aditi Rao', 'Seller Owner'],
  ['seller-staff', 'staff01', '/seller-admin', 'Arjun Nair Staff', 'Seller Staff'],
  ['marketplace-admin', 'marketplace01', '/marketplace-admin', 'Marketplace Reviewer', 'Marketplace Admin'],
  ['super-admin', 'super01', '/super-admin', 'Super Admin One', 'Super Admin'],
  ['support-agent', 'support01', '/support-portal', 'Aditi Support', 'Support Agent'],
  ['finance-operator', 'finance01', '/finance-portal', 'Finance Operator', 'Finance Operator'],
  ['returns-operations', 'operations01', '/operations-portal', 'Returns Operator', 'Returns Operations'],
].map(([id, account, root, name, label]) => ({ id, email: `demo.${account}@weevcrafts.local`, root, name, label }));
const roots = [...new Set(roles.map(role => role.root))];
const password = 'WeeVCrafts-Demo-2026!';

async function login(page, role) {
  await page.goto('/login');
  await page.locator('#page input[name="email"]').fill(role.email);
  await page.locator('#page input[name="password"]').fill(password);
  await page.getByRole('button', { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, '') === (role.root === '/account' ? '/products' : role.root));
  if (role.root === '/account') await page.goto('/account');
}
async function token(page) {
  const input = page.locator('input[name="csrf_token"]').first();
  if (await input.count()) return input.inputValue();
  const meta = page.locator('meta[name="csrf-token"]');
  return meta.getAttribute('content');
}
async function denied(page, root) {
  const response = await page.request.get(root, { maxRedirects: 0 });
  expect([302, 303, 401, 403]).toContain(response.status());
}

for (const role of roles) {
  test(`AUTH-001--${role.id}`, async ({ page }) => {
    await login(page, role);
    await page.reload();
    expect(new URL(page.url()).pathname.replace(/\/$/, '')).toBe(role.root);
    await expect(page.locator('body')).toContainText(role.name);
    await expect(page.locator('body')).toContainText(role.label);
  });

  test(`AUTH-002--${role.id}`, async ({ page }) => {
    await page.goto('/login');
    await page.locator('#page input[name="email"]').fill(role.email);
    await page.locator('#page input[name="password"]').fill('Definitely-Incorrect-2026!');
    await page.getByRole('button', { name: /sign in/i }).click();
    await expect(page.locator('body')).toContainText(/email or password is incorrect/i);
    expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
    await denied(page, role.root);
  });

  test(`AUTH-003--${role.id}`, async ({ page }) => {
    await page.goto('/login');
    await page.locator('#page input[name="email"]').fill(`qa.unknown.${role.id}.${Date.now()}@example.invalid`);
    await page.locator('#page input[name="password"]').fill(password);
    await page.getByRole('button', { name: /sign in/i }).click();
    await expect(page.locator('body')).toContainText(/email or password is incorrect/i);
    expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
  });

  test(`AUTH-004--${role.id}`, async ({ page }) => {
    for (const [email, pass] of [['', ''], [role.email, ''], ['', password]]) {
      await page.goto('/login');
      await page.locator('#page input[name="email"]').fill(email);
      await page.locator('#page input[name="password"]').fill(pass);
      await page.getByRole('button', { name: /sign in/i }).click();
      const invalid = await page.locator('#page input:invalid').count();
      if (!invalid) await expect(page.locator('body')).toContainText(/required|email or password|enter/i);
      expect(new URL(page.url()).pathname).toBe('/login');
      expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
    }
  });

  test(`AUTH-007--${role.id}`, async ({ page }) => {
    // Submit at the actual HTTP boundary, without following unsafe redirects.
    for (const next of [role.root, 'https://example.invalid/steal', '//example.invalid/steal', '\\example.invalid/steal']) {
      await page.context().clearCookies();
      await page.goto(`/login?next=${encodeURIComponent(next)}`);
      const response = await page.request.post('/login', {
        form: { email: role.email, password, csrf_token: await token(page), next }, maxRedirects: 0,
      });
      expect(response.status()).toBe(303);
      const location = response.headers().location;
      expect(location).toBe(role.root);
      expect(new URL(location, page.url()).origin).toBe(new URL(page.url()).origin);
    }
  });

  test(`AUTH-013--${role.id}`, async ({ page, browser }) => {
    await login(page, role);
    const oldCookies = await page.context().cookies();
    const oldSession = oldCookies.find(cookie => cookie.name === 'wecratfs_session');
    expect(oldSession).toBeTruthy();
    const csrf = await token(page);
    const response = await page.request.post('/logout', { form: { csrf_token: csrf }, maxRedirects: 0 });
    expect(response.status()).toBe(303);
    await page.reload();
    await denied(page, role.root);
    await page.goBack();
    const replay = await browser.newContext();
    try {
      await replay.addCookies(oldCookies);
      expect([302, 303, 401, 403]).toContain((await replay.request.get(new URL(role.root, page.url()).href, { maxRedirects: 0 })).status());
      expect([302, 303, 401, 403]).toContain((await replay.request.post(new URL('/logout', page.url()).href, { form: { csrf_token: csrf }, maxRedirects: 0 })).status());
    } finally { await replay.close(); }
  });

  test(`AUTH-016--${role.id}`, async ({ page, browser }) => {
    await login(page, role);
    const second = await browser.newContext();
    try {
      const other = await second.newPage();
      await login(other, roles.find(candidate => candidate.id !== role.id));
      for (const csrf of ['', 'altered-invalid-csrf', await token(other)]) {
        const response = await page.request.post('/logout', { form: { csrf_token: csrf }, maxRedirects: 0 });
        expect(response.status()).toBe(403);
        await page.goto(role.root);
        await expect(page.locator('body')).toContainText(role.name);
      }
    } finally { await second.close(); }
  });

  test(`NAV-002--${role.id}`, async ({ page }) => {
    await login(page, role);
    for (const root of roots.filter(root => root !== role.root)) {
      const response = await page.request.get(root, { maxRedirects: 0 });
      if (root === '/account' && response.status() === 303) {
        // The generic account entry redirects non-customers to their own
        // assigned portal; it must never serve customer workspace data.
        expect(response.headers().location).toBe(role.root);
        const followed = await page.request.get(root);
        expect(new URL(followed.url()).pathname.replace(/\/$/, '')).toBe(role.root);
      } else {
        expect(response.status(), `${role.id} accessing ${root}`).toBe(403);
      }
    }
    await page.goto('/workspaces');
    await expect(page).toHaveURL(new RegExp((role.root === '/account' ? '/products' : role.root) + '$'));
    await expect(page.locator('a[href="/workspaces"]')).toHaveCount(0);
  });
}

test('GST-020--guest', async ({ request }) => {
  const missing = `missing-qa-${Date.now()}`;
  for (const route of [`/products/${missing}`, `/category/${missing}`, `/brands/${missing}`, `/${missing}`]) {
    const response = await request.get(route);
    expect.soft(response.status(), route).toBe(404);
    expect(await response.text()).not.toMatch(/panic:|SQLSTATE|password_hash/);
  }
});

for (const role of [null, ...roles]) {
  const id = role ? role.id : 'guest';
  const paths = role ? (role.id === 'customer' ? ['/account', '/account/addresses', '/orders', '/products/natural-soap'] : [role.root, `${role.root}/${role.id.startsWith('seller') ? 'products' : role.id === 'support-agent' ? 'cases' : role.id === 'returns-operations' ? 'returns' : role.id === 'finance-operator' ? 'payments' : 'products'}`]) : ['/', '/products', '/products/natural-soap', '/login'];
  test(`UI-001--${id}`, async ({ page }, testInfo) => {
    test.setTimeout(180_000);
    if (role) await login(page, role);
    for (const width of [360, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 1000 });
      for (const route of paths) {
        expect((await page.goto(route)).status(), route).toBe(200);
        const sizes = await page.evaluate(() => [document.documentElement.scrollWidth, document.body.scrollWidth]);
        expect(Math.max(...sizes), `${route} horizontal overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
        await expect(page.locator('main').first()).toBeVisible();
      }
      await page.screenshot({ path: testInfo.outputPath(`${id}-${width}.png`) });
    }
  });
}
