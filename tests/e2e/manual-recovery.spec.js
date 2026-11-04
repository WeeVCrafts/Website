const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const { resetAuthenticationQuota } = require('./qa-fixtures');

// These SQL operations create disposable identities and control expiration
// only in the dedicated QA database. Recovery and sign-in use the real app.
const qaDatabase = process.env.WEEVCRAFTS_QA_DATABASE;
const local = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
const allowed = qaDatabase === 'weev_qa_20261009_browser' && local.hostname === '127.0.0.1' && local.port === '18080';
const password = 'WeeVCrafts-Demo-2026!';
const roles = [
  ['customer', 'customer01', '/account'], ['seller-owner', 'owner01', '/seller-admin'],
  ['seller-staff', 'staff01', '/seller-admin'], ['marketplace-admin', 'marketplace01', '/marketplace-admin'],
  ['super-admin', 'super01', '/super-admin'], ['support-agent', 'support01', '/support-portal'],
  ['finance-operator', 'finance01', '/finance-portal'], ['returns-operations', 'operations01', '/operations-portal'],
];
function sql(query) {
  if (!allowed) throw new Error('Disposable recovery fixtures require the dedicated local QA deployment.');
  return execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', qaDatabase, '-v', 'ON_ERROR_STOP=1', '-tAc', query], { encoding: 'utf8' }).trim();
}
function fixture(role, account, caseId) {
  const email = `qa.recovery.${caseId.toLowerCase()}.${role}.${Date.now()}@weevcrafts.local`;
  const source = `demo.${account}@weevcrafts.local`;
  const id = sql(`INSERT INTO users(email,display_name,status) VALUES('${email}','QA ${role}','active') RETURNING id` ).split(/\r?\n/)[0];
  if (!/^\d+$/.test(id)) throw new Error('Invalid fixture user identifier');
  sql(`INSERT INTO credentials(user_id,password_hash) SELECT ${id},c.password_hash FROM credentials c JOIN users u ON u.id=c.user_id WHERE u.email='${source}'; INSERT INTO user_roles(user_id,role_slug) SELECT ${id},r.role_slug FROM user_roles r JOIN users u ON u.id=r.user_id WHERE u.email='${source}'; INSERT INTO seller_users(seller_id,user_id,status,permissions) SELECT su.seller_id,${id},su.status,su.permissions FROM seller_users su JOIN users u ON u.id=su.user_id WHERE u.email='${source}';`);
  if (role === 'seller-owner') sql(`INSERT INTO sellers(owner_user_id,display_name,status) VALUES(${id},'QA recovery seller ${id}','active')`);
  test.info().annotations.push({ type: 'account', description: `user_id=${id}; email=${email}; role copied from ${source}` });
  return { email, id };
}
async function csrf(page) {
  return page.locator('input[name="csrf_token"]').first().inputValue();
}
async function login(page, email, root, value = password) {
  await page.goto('/login');
  await page.locator('#page input[name="email"]').fill(email);
  await page.locator('#page input[name="password"]').fill(value);
  await page.getByRole('button', { name: /sign in/i }).click();
  await page.waitForURL(url => url.pathname.replace(/\/$/, '') === (root === '/account' ? '/products' : root));
  if (root === '/account') await page.goto('/account');
}
async function recover(page, email) {
  await page.goto('/forgot-password');
  await page.locator('#page input[name="email"]').fill(email);
  await page.getByRole('button', { name: 'Send recovery link' }).click();
  await expect(page.locator('[data-backend-form-message]')).toContainText(/recovery instructions have been sent/i);
  return page.locator('[data-backend-form-message]').innerText();
}
async function mailLink(request, email) {
  let message;
  await expect.poll(async () => {
    const response = await request.get('http://127.0.0.1:8025/api/v1/messages?limit=500');
    expect(response.status()).toBe(200);
    const messages = (await response.json()).messages || [];
    message = messages.find(item => item.Subject?.includes('Reset your Weevcrafts password') && (item.To || []).some(to => to.Address === email));
    return !!message;
  }, { timeout: 15000 }).toBe(true);
  const detail = await (await request.get(`http://127.0.0.1:8025/api/v1/message/${message.ID}`)).json();
  const url = (`${detail.Text || ''}\n${detail.HTML || ''}`.match(/https?:\/\/[^\s<>"']+/g) || []).find(link => link.includes('/reset-password?token='));
  expect(!!url).toBe(true);
  const parsed = new URL(url.replaceAll('&amp;', '&'));
  expect(parsed.origin).toBe(local.origin);
  return parsed.href;
}
async function reset(page, url, value) {
  await page.goto(url);
  await page.locator('#page input[name="password"]').fill(value);
  await page.locator('#page input[name="confirm_password"]').fill(value);
  await page.getByRole('button', { name: 'Update password' }).click();
  await expect(page.locator('[data-backend-form-message]')).toContainText(/password updated/i);
}

test.beforeEach(() => {
  test.skip(!allowed, 'Requires explicit isolated QA database and port; never mutates a shared deployment.');
  resetAuthenticationQuota();
  test.info().annotations.push({ type: 'fixture', description: 'Reset the isolated Docker gateway authentication quota between independent cases; controlled session/token expiry uses test-only persistence fixtures.' });
});
for (const [role, account, root] of roles) {
  test(`AUTH-008--${role}`, async ({ page, request }) => {
    const user = fixture(role, account, 'AUTH-008');
    await recover(page, user.email);
    const url = await mailLink(request, user.email);
    await page.goto(url);
    await expect(page.getByRole('button', { name: 'Update password' })).toBeVisible();
    const rawToken = new URL(url).searchParams.get('token');
    expect(sql(`SELECT token_hash FROM auth_action_tokens WHERE user_id=${user.id} AND purpose='password_reset' ORDER BY id DESC LIMIT 1`)).toBe(createHash('sha256').update(rawToken).digest('hex'));
    const logs = execFileSync('docker', ['logs', 'weev-qa-20261009-app'], { encoding: 'utf8', stdio: ['pipe', 'pipe', 'ignore'] });
    expect(logs.includes(rawToken)).toBe(false);
  });
  test(`AUTH-009--${role}`, async ({ page, request }) => {
    const user = fixture(role, account, 'AUTH-009');
    const known = await recover(page, user.email);
    const unknownEmail = `qa.unregistered.${role}.${Date.now()}@example.invalid`;
    expect(await recover(page, unknownEmail)).toBe(known);
    await mailLink(request, user.email);
    const mailbox = await (await request.get('http://127.0.0.1:8025/api/v1/messages?limit=500')).json();
    expect((mailbox.messages || []).some(item => (item.To || []).some(to => to.Address === unknownEmail))).toBe(false);
    expect(sql(`SELECT count(*) FROM users WHERE email='${unknownEmail}'`)).toBe('0');
  });
  test(`AUTH-010--${role}`, async ({ page, browser, request }) => {
    test.setTimeout(120000);
    const user = fixture(role, account, 'AUTH-010');
    const first = await browser.newContext();
    const second = await browser.newContext();
    try {
      const a = await first.newPage(); const b = await second.newPage();
      await login(a, user.email, root); await login(b, user.email, root);
      const firstToken = await csrf(a); const secondToken = await csrf(b);
      await recover(page, user.email);
      const url = await mailLink(request, user.email);
      const changed = 'QA-New-Recovery-Password-2026!';
      await reset(page, url, changed);
      for (const [context, oldToken] of [[first, firstToken], [second, secondToken]]) {
        expect([302, 303, 401, 403]).toContain((await context.request.get(new URL(root, local).href, { maxRedirects: 0 })).status());
        expect((await context.request.post(new URL('/logout', local).href, { form: { csrf_token: oldToken }, maxRedirects: 0 })).status()).toBe(403);
      }
      await page.goto('/login');
      const old = await page.request.post('/login', { form: { email: user.email, password, csrf_token: await csrf(page) }, maxRedirects: 0 });
      expect(old.status()).toBe(401);
      await login(page, user.email, root, changed);
    } finally { await first.close(); await second.close(); }
  });
  test(`AUTH-014--${role}`, async ({ page }) => {
    const user = fixture(role, account, 'AUTH-014');
    await login(page, user.email, root);
    const oldToken = await csrf(page);
    const cookie = (await page.context().cookies()).find(item => item.name === 'wecratfs_session');
    const key = 'wecratfs:session:' + createHash('sha256').update(cookie.value).digest('hex');
    // Give only this disposable session a short server-side lifetime.
    expect(execFileSync('docker', ['exec', 'weev-qa-20261009-redis', 'redis-cli', 'PEXPIRE', key, '1'], { encoding: 'utf8' }).trim()).toBe('1');
    await expect.poll(async () => (await page.request.post('/logout', { form: { csrf_token: oldToken }, maxRedirects: 0 })).status()).toBe(403);
    expect([302, 303, 401, 403]).toContain((await page.request.get(root, { maxRedirects: 0 })).status());
    await page.goto(root);
    expect(new URL(page.url()).pathname).toBe('/login');
  });
  test(`AUTH-011--${role}`, async ({ page, request }) => {
    test.setTimeout(120000);
    const user = fixture(role, account, 'AUTH-011');
    const credential = () => sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`);
    const initial = credential();
    await recover(page, user.email);
    const link = await mailLink(request, user.email);
    const originalToken = new URL(link).searchParams.get('token');
    await page.goto(link);
    // Exercise the JSON endpoint used by the browser's recovery form.
    const attempt = value => page.request.post('/api/v1/auth/password-reset/confirm', { data: { token: value, password: 'QA-Token-Validation-2026!' } });
    expect((await attempt(originalToken + 'altered')).status()).toBe(422);
    expect(credential() === initial).toBe(true);
    sql(`UPDATE auth_action_tokens SET expires_at=NOW()-INTERVAL '1 minute' WHERE user_id=${user.id} AND purpose='password_reset'`);
    expect((await attempt(originalToken)).status()).toBe(422);
    expect(credential() === initial).toBe(true);
    await recover(page, user.email);
    const freshLink = await mailLink(request, user.email);
    const freshToken = new URL(freshLink).searchParams.get('token');
    await page.goto(freshLink);
    expect((await attempt(freshToken)).status()).toBe(200);
    const changed = credential();
    expect(changed === initial).toBe(false);
    expect((await attempt(freshToken)).status()).toBe(422);
    expect(credential() === changed).toBe(true);
  });
  test(`AUTH-012--${role}`, async ({ page, request }) => {
    test.setTimeout(120000);
    const user = fixture(role, account, 'AUTH-012');
    await recover(page, user.email);
    const link = await mailLink(request, user.email);
    const rawToken = new URL(link).searchParams.get('token');
    await page.goto(link);
    const before = sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`);
    for (const value of ['a'.repeat(11), 'a'.repeat(1025)]) {
      const response = await page.request.post('/api/v1/auth/password-reset/confirm', { data: { token: rawToken, password: value } });
      expect(response.status()).toBe(422);
      expect(await response.text()).toMatch(/reset token or password is invalid/i);
      expect(sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`) === before).toBe(true);
      expect(sql(`SELECT count(*) FROM auth_action_tokens WHERE user_id=${user.id} AND consumed_at IS NULL AND expires_at>NOW()`)).toBe('1');
    }
    await page.locator('#page input[name="password"]').fill('Valid-Min-12');
    await page.locator('#page input[name="confirm_password"]').fill('Different-12');
    await page.getByRole('button', { name: 'Update password' }).click();
    await expect(page.locator('[data-backend-form-message]')).toContainText(/match/i);
    expect(sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`) === before).toBe(true);
    // Exactly twelve ASCII bytes, matching the documented server boundary.
    await reset(page, link, 'Valid-Min-12');
    await login(page, user.email, root, 'Valid-Min-12');
  });
}
