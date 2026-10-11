const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const password = 'WeeVCrafts-Demo-2026!';
const database = 'weev_qa_secure_20261009';
const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
const isolated = process.env.WEEVCRAFTS_QA_DATABASE === database && target.hostname === '127.0.0.1' && target.port === '18082';
function sql(query) {
  if (!isolated) throw new Error('Requires the isolated secure customer QA deployment');
  return execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', database, '-v', 'ON_ERROR_STOP=1', '-tAc', query], { encoding: 'utf8' }).trim();
}
async function login(page, account) {
  const response = await page.request.post('/api/v1/auth/login', { data: { email: `demo.${account}@weevcrafts.local`, password } });
  expect(response.status()).toBe(200);
  expect(response.headers()['cache-control']).toBe('private, no-store');
  return response.json();
}
async function customer(page) {
  const email = `secure.${Date.now()}.${Math.random().toString(36).slice(2)}@example.invalid`;
  const response = await page.request.post('/api/v1/auth/register', { data: { email, password, display_name: 'QA Customer', phone: '+919876543210', accepted_terms: true } });
  expect(response.status()).toBe(201);
  return response.json();
}
test.beforeEach(() => {
  test.skip(!isolated, 'Requires disposable QA database and private Redis instance.');
  // Reset only the QA instance's authentication counters between scenarios.
  const keys = execFileSync('docker', ['exec', 'weev-secure-qa-redis', 'redis-cli', '--scan', '--pattern', 'wecratfs:rate:*'], { encoding: 'utf8' }).trim().split(/\r?\n/).filter(Boolean);
  if (keys.length) execFileSync('docker', ['exec', 'weev-secure-qa-redis', 'redis-cli', 'DEL', ...keys], { stdio: 'ignore' });
});

test('customer login, wishlist toggle persistence, failure and rapid-click states', async ({ page }) => {
  await page.goto('/login');
  await page.locator('#page [name=email]').fill('demo.customer01@weevcrafts.local');
  await page.locator('#page [name=password]').fill(password);
  await page.getByRole('button', { name: 'Continue with email', exact: true }).click();
  await page.waitForURL('**/products');
  await expect(page.locator('a[href="/workspaces"]')).toHaveCount(0);
  await page.waitForFunction(() => window.weevcraftsBackend);
  const product = page.locator('.product-card').first();
  const slug = await product.getAttribute('data-product');
  const heart = product.locator('.heart');
  if (await heart.getAttribute('aria-pressed') === 'true') await heart.click();
  await expect(heart).toHaveAttribute('aria-pressed', 'false');
  const badRequests = [];
  page.on('request', r => { if (r.url().includes('undefined')) badRequests.push(r.url()); });
  await heart.click(); await expect(heart).toHaveAttribute('aria-pressed', 'true');
  await heart.click(); await expect(heart).toHaveAttribute('aria-pressed', 'false');
  await heart.click(); await expect(heart).toHaveAttribute('aria-pressed', 'true');
  await page.reload(); await page.waitForFunction(() => window.weevcraftsBackend);
  await expect(page.locator(`.product-card[data-product="${slug}"] .heart`)).toHaveAttribute('aria-pressed', 'true');
  await page.goto(`/products/${slug}`); await page.waitForFunction(() => window.weevcraftsBackend);
  const detailHeart = page.locator('.gallery-heart');
  await detailHeart.click(); await expect(detailHeart).toHaveAttribute('aria-pressed', 'false');
  await page.route('**/api/v1/wishlist/add', r => r.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { message: 'Temporary QA failure' } }) }));
  await detailHeart.click(); await expect(detailHeart).toHaveAttribute('aria-pressed', 'false');
  await expect(detailHeart).toBeEnabled();
  await page.unroute('**/api/v1/wishlist/add');
  let adds = 0;
  await page.route('**/api/v1/wishlist/add', async r => { adds++; await new Promise(resolve => setTimeout(resolve, 200)); await r.continue(); });
  await detailHeart.evaluate(button => { button.click(); button.click(); button.click(); });
  await expect(detailHeart).toHaveAttribute('aria-pressed', 'true'); expect(adds).toBe(1);
  await page.goto('/wishlist'); await page.waitForFunction(() => window.weevcraftsBackend);
  const saved = page.locator('.saved-card').filter({ has: page.locator(`[data-product-slug="${slug}"]`) });
  const remove = saved.locator('[data-backend-action="wishlist-remove"]').first();
  await remove.click(); await expect(remove).toHaveCount(0);
  await page.goto(`/products/${slug}`); await page.waitForFunction(() => window.weevcraftsBackend);
  await page.locator('.gallery-heart').click();
  await expect(page.locator('.gallery-heart')).toHaveAttribute('aria-pressed', 'true');
  await page.goto('/wishlist'); await page.waitForFunction(() => window.weevcraftsBackend);
  const move = page.locator(`.saved-card [data-backend-action="wishlist-to-cart"][data-product-slug="${slug}"]`);
  await move.click(); await expect(move).toHaveCount(0);
  const cart = (await (await page.request.get('/api/v1/cart')).json()).data;
  expect(JSON.stringify(cart)).toContain(slug);
  expect(badRequests).toEqual([]);
});

test('private profile upload, preview, persistence, reminder and validation', async ({ page, browser }) => {
  const identity = await customer(page);
  sql(`UPDATE users SET phone='' WHERE id=${identity.data.id}`);
  await page.goto('/products');
  await page.getByRole('button', { name: 'Open profile menu', exact: true }).click();
  await expect(page.locator('.notification-action em')).toBeVisible();
  await page.getByRole('link', { name: 'Notifications', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Complete your profile', exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Complete your profile', exact: true }).click();
  const file = path.resolve('web/assets/images/customer/blue-tote.webp');
  await page.locator('[name=phone]').fill('+91 98765 43210');
  await page.locator('[name=photo]').setInputFiles(file);
  await expect(page.locator('[data-photo-preview] img')).toHaveAttribute('src', /^blob:/);
  await page.getByRole('button', { name: 'Save changes', exact: true }).click();
  await expect(page.locator('[role=status]').filter({ hasText: 'Profile updated' }).first()).toBeVisible();
  const photo = await page.request.get('/account/profile/photo');
  expect(photo.status()).toBe(200); expect(photo.headers()['content-type']).toBe('image/jpeg');
  expect(photo.headers()['cache-control']).toBe('private, no-store');
  await page.reload(); await expect(page.locator('[data-photo-preview] img')).toHaveAttribute('src', '/account/profile/photo');
  await expect(page.locator('.profile-toggle img')).toHaveAttribute('src', '/account/profile/photo');
  await expect.poll(() => page.locator('.profile-toggle img').evaluate(img => img.complete && img.naturalWidth > 0)).toBeTruthy();
  await page.goto('/account/notifications');
  await expect(page.getByRole('link', { name: 'Complete your profile', exact: true })).toHaveCount(0);
  await expect(page.locator('.notification-action em')).toHaveCount(0);
  const other = await browser.newContext();
  try {
    const second = await other.newPage(); await customer(second);
    expect((await second.request.get(`/account/profile/photo?user_id=${identity.data.id}`)).status()).toBe(404);
    expect((await second.request.get('/account/profile/photo')).status()).toBe(404);
  } finally { await other.close(); }
  const anonymous = await browser.newContext();
  try { expect((await anonymous.request.get(target.origin + '/account/profile/photo')).status()).toBe(403); } finally { await anonymous.close(); }
  const fields = { csrf_token: identity.csrf_token, name: 'QA Customer', phone: '+919876543210' };
  expect((await page.request.post('/account/profile', { multipart: { ...fields, photo: { name: 'unsafe.svg', mimeType: 'image/svg+xml', buffer: Buffer.from('<svg onload="alert(1)"></svg>') } } })).status()).toBe(422);
  expect((await page.request.post('/account/profile', { multipart: { ...fields, photo: { name: 'large.png', mimeType: 'image/png', buffer: Buffer.alloc(5 * 1024 * 1024 + 1) } } })).status()).toBe(422);
  expect((await page.request.post('/account/profile', { form: { ...fields, csrf_token: '' } })).status()).toBe(403);
  expect((await page.request.get('/account/profile/photo')).status()).toBe(200);
  const before = sql(`SELECT object_key FROM user_avatars WHERE user_id=${identity.data.id}`);
  expect((await page.request.post('/account/profile', { multipart: { ...fields, photo: { name: 'new.webp', mimeType: 'image/webp', buffer: fs.readFileSync(file) } }, maxRedirects: 0 })).status()).toBe(303);
  expect(sql(`SELECT count(*) FROM avatar_cleanup WHERE object_key='${before}'`)).toBe('1');
  expect((await page.request.post('/account/profile', { form: { ...fields, remove_photo: '1' }, maxRedirects: 0 })).status()).toBe(303);
  expect((await page.request.get('/account/profile/photo')).status()).toBe(404);
  await page.reload();
  await expect(page.locator('.profile-toggle img')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Open profile menu', exact: true })).toBeVisible();
});

test('social settings are persistent, safe and Super Admin controlled', async ({ page, browser }) => {
  const admin = await login(page, 'super01');
  await page.goto('/super-admin/settings');
  const form = { csrf_token: admin.csrf_token, support_email: 'support@example.invalid', shipping_information: 'Delivery options appear at checkout.', reason: 'Isolated secure settings verification', save_social_links: '1', instagram_url: 'https://www.instagram.com/weevcrafts', facebook_url: '', youtube_url: 'https://www.youtube.com/@weevcrafts', x_url: '', whatsapp_url: '' };
  expect((await page.request.post('/super-admin/settings/save', { form, maxRedirects: 0 })).status()).toBe(303);
  await page.goto('/products');
  await expect(page.getByRole('link', { name: 'Weevcrafts on Instagram' })).toHaveAttribute('href', form.instagram_url);
  await expect(page.getByRole('link', { name: 'Weevcrafts on Facebook' })).toHaveCount(0);
  expect((await page.request.post('/super-admin/settings/save', { form: { ...form, instagram_url: 'javascript:alert(1)' }, maxRedirects: 0 })).status()).toBe(400);
  const context = await browser.newContext();
  try {
    const marketplace = await context.newPage(); const actor = await login(marketplace, 'marketplace01');
    expect((await marketplace.request.post('/marketplace-admin/settings/save', { form: { ...form, csrf_token: actor.csrf_token }, maxRedirects: 0 })).status()).toBe(403);
    const { save_social_links, instagram_url, ...otherFields } = form;
    expect((await marketplace.request.post('/marketplace-admin/settings/save', { form: { ...otherFields, csrf_token: actor.csrf_token }, maxRedirects: 0 })).status()).toBe(303);
  } finally { await context.close(); }
  await page.reload(); await expect(page.getByRole('link', { name: 'Weevcrafts on Instagram' })).toHaveAttribute('href', form.instagram_url);
});

test('role replacement removes prior privileges and revokes existing sessions', async ({ page, browser }) => {
  const identity = await customer(page);
  const adminContext = await browser.newContext();
  try {
    const adminPage = await adminContext.newPage(); const admin = await login(adminPage, 'super01');
    const form = { csrf_token: admin.csrf_token, user_id: String(identity.data.id), role: 'finance_operator', action: 'grant', reason: 'Isolated role replacement verification' };
    expect((await adminPage.request.post('/super-admin/roles/update', { form, maxRedirects: 0 })).status()).toBe(303);
    expect(sql(`SELECT string_agg(role_slug,',') FROM user_roles WHERE user_id=${identity.data.id}`)).toBe('finance_operator');
    expect((await page.request.get('/api/v1/wishlist')).status()).toBe(401);
    sql(`INSERT INTO user_roles(user_id,role_slug) VALUES(${identity.data.id},'customer')`);
    const email = sql(`SELECT email FROM users WHERE id=${identity.data.id}`);
    const response = await page.request.post('/api/v1/auth/login', { data: { email, password } });
    expect((await response.json()).redirect_to).toBe('/access-unavailable');
    expect((await page.request.get('/api/v1/wishlist')).status()).toBe(403);
    await page.goto('/finance-portal'); await expect(page).toHaveURL(/access-unavailable/);
    await expect(page.getByRole('heading', { name: 'Account access needs attention' })).toBeVisible();
    expect((await adminPage.request.post('/super-admin/roles/update', { form: { ...form, role: 'customer' }, maxRedirects: 0 })).status()).toBe(303);
    expect(sql(`SELECT count(*) FROM user_roles WHERE user_id=${identity.data.id}`)).toBe('1');
  } finally { await adminContext.close(); }
});

test('cross-account orders, addresses, wishlist and privileged operations stay isolated', async ({ page, browser }) => {
  const first = await login(page, 'customer01');
  const context = await browser.newContext();
  try {
    const other = await context.newPage(); const second = await login(other, 'customer02');
    const orders = (await (await page.request.get('/api/v1/orders')).json()).data;
    const order = orders[0]?.number || orders[0]?.order_number || 'WCDEMO1001';
    expect((await other.request.get(`/api/v1/orders/${order}`)).status()).toBe(404);
    const address = sql(`SELECT id FROM addresses WHERE user_id=${first.data.id} LIMIT 1`);
    if (address) {
      expect((await other.request.post('/account/addresses/delete', { form: { csrf_token: second.csrf_token, address_id: address }, maxRedirects: 0 })).status()).toBe(404);
      expect(sql(`SELECT count(*) FROM addresses WHERE id=${address}`)).toBe('1');
    }
    const product = (await (await page.request.get('/api/v1/products')).json()).data[0];
    expect((await page.request.post('/api/v1/wishlist/add', { data: { product_id: product.id }, headers: { 'X-CSRF-Token': first.csrf_token } })).status()).toBe(201);
    expect((await other.request.delete(`/api/v1/wishlist/${product.id}`, { headers: { 'X-CSRF-Token': second.csrf_token } })).status()).toBe(200);
    expect((await (await page.request.get('/api/v1/wishlist')).json()).data.some(item => item.product_id === product.id)).toBe(true);
    expect((await page.request.post('/api/v1/wishlist/add', { data: { product_id: product.id } })).status()).toBe(403);
    expect((await other.request.get('/super-admin/security', { maxRedirects: 0 })).status()).toBe(403);
    expect((await other.request.post('/super-admin/roles/update', { form: { csrf_token: second.csrf_token, user_id: String(first.data.id), action: 'grant', role: 'finance_operator', reason: 'Must be denied' }, maxRedirects: 0 })).status()).toBe(403);
  } finally { await context.close(); }
});

test('all role portals remove switching and customer mobile polish stays usable', async ({ page }, testInfo) => {
  for (const [account, destination] of [['customer01', '/products'], ['owner01', '/seller-admin'], ['staff01', '/seller-admin'], ['marketplace01', '/marketplace-admin'], ['super01', '/super-admin'], ['support01', '/support-portal'], ['finance01', '/finance-portal'], ['operations01', '/operations-portal']]) {
    await page.context().clearCookies(); const identity = await login(page, account);
    expect(identity.redirect_to).toBe(destination);
    await page.goto(destination); await expect(page.locator('a[href="/workspaces"]')).toHaveCount(0);
    const legacy = await page.request.get('/workspaces', { maxRedirects: 0 }); expect(legacy.status()).toBe(303); expect(legacy.headers().location).toBe(destination);
  }
  await page.context().clearCookies(); await login(page, 'customer01');
  await page.emulateMedia({ reducedMotion: 'reduce' });
  // 720 CSS pixels also exercises reflow at 200% of a 1440px desktop viewport.
  for (const width of [1440, 720, 390, 320]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const route of ['/products', '/account/profile', '/account/notifications', '/cart']) {
      await page.goto(route);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
      expect(await page.locator('body').evaluate(el => parseFloat(getComputedStyle(el).fontSize))).toBeGreaterThanOrEqual(16);
      await page.screenshot({ path: path.join('docs/qa/secure-customer', `${testInfo.project.name}-${width}-${route.replaceAll('/', '-')}.png`), fullPage: true });
    }
  }
  await page.goto('/account/profile'); await page.keyboard.press('Tab');
  expect(await page.evaluate(() => document.activeElement !== document.body)).toBe(true);
});

test('two sellers, staff permissions, private support attachments and receipts enforce boundaries', async ({ page, browser }) => {
  const first = await login(page, 'owner01');
  const product = sql(`SELECT p.id FROM products p JOIN sellers s ON s.id=p.seller_id WHERE s.owner_user_id=${first.data.id} LIMIT 1`);
  expect(product).toMatch(/^\d+$/);
  const otherContext = await browser.newContext();
  try {
    const other = await otherContext.newPage(); const second = await login(other, 'owner02');
    const productStatus = sql(`SELECT status FROM products WHERE id=${product}`);
    expect((await page.request.get(`/seller-admin/products/${product}/edit`)).status()).toBe(200);
    expect((await other.request.get(`/seller-admin/products/${product}/edit`)).status()).toBe(404);
    expect((await other.request.post('/seller-admin/products/archive', { form: { csrf_token: second.csrf_token, product_id: product, reason: 'Cross-seller request must be denied' }, maxRedirects: 0 })).status()).toBe(403);
    expect(sql(`SELECT status FROM products WHERE id=${product}`)).toBe(productStatus);
    expect((await page.request.post('/seller-admin/products/archive', { form: { product_id: product }, maxRedirects: 0 })).status()).toBe(403);
    await otherContext.clearCookies(); await login(other, 'staff01');
    expect((await other.request.get('/seller-admin/team', { maxRedirects: 0 })).status()).toBe(403);
    expect((await other.request.post('/seller-admin/team/update', { form: { user_id: String(first.data.id) }, maxRedirects: 0 })).status()).toBe(403);
    await page.context().clearCookies(); const customerIdentity = await login(page, 'customer01');
    const subject = `Private support boundary ${Date.now()}`;
    const created = await page.request.post('/support/create', { headers: { 'X-CSRF-Token': customerIdentity.csrf_token }, multipart: { csrf_token: customerIdentity.csrf_token, subject, message: 'Private support attachment ownership verification.', attachments: { name: 'qa.webp', mimeType: 'image/webp', buffer: fs.readFileSync('web/assets/images/customer/blue-tote.webp') } }, maxRedirects: 0 });
    expect(created.status()).toBe(303);
    const attachment = sql(`SELECT a.id FROM support_attachments a JOIN support_tickets t ON t.id=a.ticket_id WHERE t.user_id=${customerIdentity.data.id} AND t.subject='${subject}' LIMIT 1`);
    expect(attachment).toMatch(/^\d+$/);
    const own = await page.request.get(`/support/attachments/${attachment}`);
    expect(own.status()).toBe(200); expect(own.headers()['cache-control']).toBe('private, no-store');
    await otherContext.clearCookies(); await login(other, 'customer02');
    expect((await other.request.get(`/support/attachments/${attachment}`)).status()).toBe(404);
    const number = sql(`SELECT order_number FROM orders WHERE user_id=${customerIdentity.data.id} LIMIT 1`);
    expect((await page.request.get(`/orders/${number}/invoice`)).status()).toBe(200);
    expect((await other.request.get(`/orders/${number}/invoice`)).status()).toBe(404);
    expect((await other.request.get('/finance-portal/finance-reconciliation', { maxRedirects: 0 })).status()).toBe(403);
    await otherContext.clearCookies(); await login(other, 'finance01');
    expect((await other.request.get('/finance-portal/finance-reconciliation')).status()).toBe(200);
    await otherContext.clearCookies(); await login(other, 'support01');
    expect((await other.request.get(`/support/attachments/${attachment}`)).status()).toBe(200);
  } finally { await otherContext.close(); }
});

test('suspension, password replacement and deletion invalidate existing sessions', async ({ page, browser }) => {
  const context = await browser.newContext();
  try {
    const adminPage = await context.newPage(); const admin = await login(adminPage, 'super01');
    for (const action of ['suspension', 'password', 'deletion']) {
      await page.context().clearCookies(); const identity = await customer(page);
      const email = sql(`SELECT email FROM users WHERE id=${identity.data.id}`);
      const form = { csrf_token: admin.csrf_token, user_id: String(identity.data.id), email, display_name: 'QA Customer', status: action === 'suspension' ? 'suspended' : 'active', reason: `Isolated ${action} revocation verification` };
      if (action === 'password') form.password = 'Replaced-QA-Password-2026!';
      const endpoint = action === 'deletion' ? '/super-admin/users/delete' : '/super-admin/users/update';
      expect((await adminPage.request.post(endpoint, { form, maxRedirects: 0 })).status()).toBe(303);
      expect((await page.request.get('/api/v1/orders')).status()).toBe(401);
    }
  } finally { await context.close(); }
});

test('JavaScript and server forms share safe customer return destinations', async ({ page, browser }) => {
  for (const next of ['/checkout?step=address', '//outside.example', '/super-admin/security']) {
    await page.context().clearCookies();
    const response = await page.request.post('/api/v1/auth/login', { data: { email: 'demo.customer01@weevcrafts.local', password, next } });
    expect((await response.json()).redirect_to).toBe(next.startsWith('/checkout') ? next : '/products');
  }
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    const serverPage = await context.newPage();
    await serverPage.goto('/login?next=%2Fcheckout');
    await serverPage.locator('#page [name=email]').fill('demo.customer01@weevcrafts.local');
    await serverPage.locator('#page [name=password]').fill(password);
    await serverPage.getByRole('button', { name: /sign in/i }).click();
    await expect(serverPage).toHaveURL(/\/checkout$/);
  } finally { await context.close(); }
});
