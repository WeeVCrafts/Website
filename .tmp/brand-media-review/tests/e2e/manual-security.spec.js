const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');
const { resetAuthenticationQuota } = require('./qa-fixtures');
const local = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
const qaDatabase = process.env.WEEVCRAFTS_QA_DATABASE;
const allowed = qaDatabase === 'weev_qa_20261009_browser' && local.hostname === '127.0.0.1' && local.port === '18080';
function sql(query) {
  if (!allowed) throw new Error('Security mutations require the dedicated QA database.');
  return execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', qaDatabase, '-v', 'ON_ERROR_STOP=1', '-tAc', query], { encoding: 'utf8' }).trim();
}
const roles = [
  ['customer', 'customer01', '/account'], ['seller-owner', 'owner01', '/seller-admin'],
  ['seller-staff', 'staff01', '/seller-admin'], ['marketplace-admin', 'marketplace01', '/marketplace-admin'],
  ['super-admin', 'super01', '/super-admin'], ['support-agent', 'support01', '/support-portal'],
  ['finance-operator', 'finance01', '/finance-portal'], ['returns-operations', 'operations01', '/operations-portal'],
];
async function login(page, email, root) {
  await page.goto('/login');
  await page.locator('#page input[name="email"]').fill(email);
  await page.locator('#page input[name="password"]').fill('WeeVCrafts-Demo-2026!');
  await page.getByRole('button', { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, '') === (root === '/account' ? '/products' : root));
  if (root === '/account') await page.goto('/account');
}
test.beforeEach(() => {
  test.skip(!allowed, 'Requires the isolated QA database and deployment.');
  resetAuthenticationQuota();
});
for (const [role, account, root] of roles) {
  test(`AUTH-006--${role}`, async ({ page, browser }, testInfo) => {
    test.setTimeout(120000);
    const email = `qa.suspension.${role}.${Date.now()}@weevcrafts.local`;
    const source = `demo.${account}@weevcrafts.local`;
    const id = sql(`INSERT INTO users(email,display_name,status) VALUES('${email}','QA suspension ${role}','active') RETURNING id`).split(/\r?\n/)[0];
    if (!/^\d+$/.test(id)) throw new Error('Invalid disposable user identifier');
    sql(`INSERT INTO credentials(user_id,password_hash) SELECT ${id},c.password_hash FROM credentials c JOIN users u ON u.id=c.user_id WHERE u.email='${source}'; INSERT INTO user_roles(user_id,role_slug) SELECT ${id},r.role_slug FROM user_roles r JOIN users u ON u.id=r.user_id WHERE u.email='${source}'; INSERT INTO seller_users(seller_id,user_id,status,permissions) SELECT su.seller_id,${id},su.status,su.permissions FROM seller_users su JOIN users u ON u.id=su.user_id WHERE u.email='${source}';`);
    if (role === 'seller-owner') sql(`INSERT INTO sellers(owner_user_id,display_name,status) VALUES(${id},'QA suspension seller ${id}','active')`);
    testInfo.annotations.push({ type: 'account', description: `user_id=${id}; email=${email}; actor=demo.super01@weevcrafts.local` });
    await login(page, email, root);
    const oldToken = await page.locator('input[name="csrf_token"]').first().inputValue();
    const administrator = await browser.newContext();
    try {
      const admin = await administrator.newPage();
      await login(admin, 'demo.super01@weevcrafts.local', '/super-admin');
      await admin.goto('/super-admin/security');
      const csrf = await admin.locator('input[name="csrf_token"]').first().inputValue();
      const update = await admin.request.post('/super-admin/users/update', { form: { csrf_token: csrf, user_id: id, email, display_name: `QA suspension ${role}`, status: 'suspended', reason: 'Disposable QA account suspension test' }, maxRedirects: 0 });
      expect(update.status()).toBe(303);
      expect(sql(`SELECT status FROM users WHERE id=${id}`)).toBe('suspended');
      expect([302, 303, 401, 403]).toContain((await page.request.get(root, { maxRedirects: 0 })).status());
      expect((await page.request.post('/logout', { form: { csrf_token: oldToken }, maxRedirects: 0 })).status()).toBe(403);
      await page.context().clearCookies();
      await page.goto('/login');
      const response = await page.request.post('/login', { form: { csrf_token: await page.locator('input[name="csrf_token"]').first().inputValue(), email, password: 'WeeVCrafts-Demo-2026!' }, maxRedirects: 0 });
      expect(response.status()).toBe(401);
      expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
    } finally { await administrator.close(); }
  });
}

test('enabled local payment helper still processes an owned QA order', async ({ page }, testInfo) => {
  const number = `QAHELPER${Date.now()}`;
  // This boundary check needs a fresh pending payment, rather than the seed's
  // 30-minute inventory reservation, which expires during a long campaign.
  sql(`WITH fixture AS (INSERT INTO orders(order_number,user_id,status,currency,subtotal_cents,shipping_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) SELECT '${number}',user_id,'pending_payment',currency,subtotal_cents,shipping_cents,total_cents,address_snapshot,'qa-helper','${number}' FROM orders WHERE order_number='WCDEMO1001' RETURNING id,total_cents,currency) INSERT INTO payment_attempts(order_id,provider,provider_order_id,provider_payment_id,status,amount_cents,currency) SELECT id,'local','local_order_${number}','local_payment_${number}','created',total_cents,currency FROM fixture`);
  testInfo.annotations.push({ type: 'fixture', description: `Fresh isolated pending order ${number}; helper boundary only, no inventory reservation or provider charge.` });
  await login(page, 'demo.customer01@weevcrafts.local', '/account');
  const csrf = await page.locator('input[name="csrf_token"]').first().inputValue();
  expect(sql(`SELECT status FROM orders WHERE order_number='${number}'`)).toBe('pending_payment');
  const response = await page.request.post(`/dev/payments/${number}/capture`, { headers: { 'X-CSRF-Token': csrf } });
  expect(response.status()).toBe(200);
  expect((await response.json()).processed).toBe(true);
  expect(sql(`SELECT status FROM orders WHERE order_number='${number}'`)).toBe('paid');
});
