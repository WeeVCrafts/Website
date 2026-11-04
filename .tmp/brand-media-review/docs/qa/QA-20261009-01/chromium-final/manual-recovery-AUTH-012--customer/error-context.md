# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-recovery.spec.js >> AUTH-012--customer
- Location: tests\e2e\manual-recovery.spec.js:157:3

# Error details

```
Test timeout of 120000ms exceeded.
```

```
Error: locator.fill: Test timeout of 120000ms exceeded.
Call log:
  - waiting for locator('#page input[name="email"]')

```

# Page snapshot

```yaml
- generic [active] [ref=e1]: Too many authentication attempts.
```

# Test source

```ts
  1   | const { test, expect } = require('@playwright/test');
  2   | const { execFileSync } = require('node:child_process');
  3   | const { createHash } = require('node:crypto');
  4   | 
  5   | // These SQL operations create disposable identities and control expiration
  6   | // only in the dedicated QA database. Recovery and sign-in use the real app.
  7   | const qaDatabase = process.env.WEEVCRAFTS_QA_DATABASE;
  8   | const local = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
  9   | const allowed = qaDatabase === 'weev_qa_20261009_browser' && local.hostname === '127.0.0.1' && local.port === '18080';
  10  | const password = 'WeeVCrafts-Demo-2026!';
  11  | const roles = [
  12  |   ['customer', 'customer01', '/account'], ['seller-owner', 'owner01', '/seller-admin'],
  13  |   ['seller-staff', 'staff01', '/seller-admin'], ['marketplace-admin', 'marketplace01', '/marketplace-admin'],
  14  |   ['super-admin', 'super01', '/super-admin'], ['support-agent', 'support01', '/support-portal'],
  15  |   ['finance-operator', 'finance01', '/finance-portal'], ['returns-operations', 'operations01', '/operations-portal'],
  16  | ];
  17  | function sql(query) {
  18  |   if (!allowed) throw new Error('Disposable recovery fixtures require the dedicated local QA deployment.');
  19  |   return execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'wecratfs', '-d', qaDatabase, '-v', 'ON_ERROR_STOP=1', '-tAc', query], { encoding: 'utf8' }).trim();
  20  | }
  21  | function fixture(role, account, caseId) {
  22  |   const email = `qa.recovery.${caseId.toLowerCase()}.${role}.${Date.now()}@weevcrafts.local`;
  23  |   const source = `demo.${account}@weevcrafts.local`;
  24  |   const id = sql(`INSERT INTO users(email,display_name,status) VALUES('${email}','QA ${role}','active') RETURNING id` ).split(/\r?\n/)[0];
  25  |   if (!/^\d+$/.test(id)) throw new Error('Invalid fixture user identifier');
  26  |   sql(`INSERT INTO credentials(user_id,password_hash) SELECT ${id},c.password_hash FROM credentials c JOIN users u ON u.id=c.user_id WHERE u.email='${source}'; INSERT INTO user_roles(user_id,role_slug) SELECT ${id},r.role_slug FROM user_roles r JOIN users u ON u.id=r.user_id WHERE u.email='${source}'; INSERT INTO seller_users(seller_id,user_id,status,permissions) SELECT su.seller_id,${id},su.status,su.permissions FROM seller_users su JOIN users u ON u.id=su.user_id WHERE u.email='${source}';`);
  27  |   if (role === 'seller-owner') sql(`INSERT INTO sellers(owner_user_id,display_name,status) VALUES(${id},'QA recovery seller ${id}','active')`);
  28  |   test.info().annotations.push({ type: 'account', description: `user_id=${id}; email=${email}; role copied from ${source}` });
  29  |   return { email, id };
  30  | }
  31  | async function csrf(page) {
  32  |   return page.locator('input[name="csrf_token"]').first().inputValue();
  33  | }
  34  | async function login(page, email, root, value = password) {
  35  |   await page.goto('/login');
  36  |   await page.locator('#page input[name="email"]').fill(email);
  37  |   await page.locator('#page input[name="password"]').fill(value);
  38  |   await page.getByRole('button', { name: /sign in/i }).click();
  39  |   await page.waitForURL(url => url.pathname.replace(/\/$/, '') === root);
  40  | }
  41  | async function recover(page, email) {
  42  |   await page.goto('/forgot-password');
> 43  |   await page.locator('#page input[name="email"]').fill(email);
      |                                                   ^ Error: locator.fill: Test timeout of 120000ms exceeded.
  44  |   await page.getByRole('button', { name: 'Send recovery link' }).click();
  45  |   await expect(page.locator('[data-backend-form-message]')).toContainText(/recovery instructions have been sent/i);
  46  |   return page.locator('[data-backend-form-message]').innerText();
  47  | }
  48  | async function mailLink(request, email) {
  49  |   let message;
  50  |   await expect.poll(async () => {
  51  |     const response = await request.get('http://127.0.0.1:8025/api/v1/messages?limit=500');
  52  |     expect(response.status()).toBe(200);
  53  |     const messages = (await response.json()).messages || [];
  54  |     message = messages.find(item => item.Subject?.includes('Reset your Weevcrafts password') && (item.To || []).some(to => to.Address === email));
  55  |     return !!message;
  56  |   }, { timeout: 15000 }).toBe(true);
  57  |   const detail = await (await request.get(`http://127.0.0.1:8025/api/v1/message/${message.ID}`)).json();
  58  |   const url = (`${detail.Text || ''}\n${detail.HTML || ''}`.match(/https?:\/\/[^\s<>"']+/g) || []).find(link => link.includes('/reset-password?token='));
  59  |   expect(!!url).toBe(true);
  60  |   const parsed = new URL(url.replaceAll('&amp;', '&'));
  61  |   expect(parsed.origin).toBe(local.origin);
  62  |   return parsed.href;
  63  | }
  64  | async function reset(page, url, value) {
  65  |   await page.goto(url);
  66  |   await page.locator('#page input[name="password"]').fill(value);
  67  |   await page.locator('#page input[name="confirm_password"]').fill(value);
  68  |   await page.getByRole('button', { name: 'Update password' }).click();
  69  |   await expect(page.locator('[data-backend-form-message]')).toContainText(/password updated/i);
  70  | }
  71  | 
  72  | test.beforeEach(() => { test.skip(!allowed, 'Requires explicit isolated QA database and port; never mutates a shared deployment.'); });
  73  | for (const [role, account, root] of roles) {
  74  |   test(`AUTH-008--${role}`, async ({ page, request }) => {
  75  |     const user = fixture(role, account, 'AUTH-008');
  76  |     await recover(page, user.email);
  77  |     const url = await mailLink(request, user.email);
  78  |     await page.goto(url);
  79  |     await expect(page.getByRole('button', { name: 'Update password' })).toBeVisible();
  80  |     const rawToken = new URL(url).searchParams.get('token');
  81  |     expect(sql(`SELECT token_hash FROM auth_action_tokens WHERE user_id=${user.id} AND purpose='password_reset' ORDER BY id DESC LIMIT 1`)).toBe(createHash('sha256').update(rawToken).digest('hex'));
  82  |     const logs = execFileSync('docker', ['logs', 'weev-qa-20261009-app'], { encoding: 'utf8', stdio: ['pipe', 'pipe', 'ignore'] });
  83  |     expect(logs.includes(rawToken)).toBe(false);
  84  |   });
  85  |   test(`AUTH-009--${role}`, async ({ page, request }) => {
  86  |     const user = fixture(role, account, 'AUTH-009');
  87  |     const known = await recover(page, user.email);
  88  |     const unknownEmail = `qa.unregistered.${role}.${Date.now()}@example.invalid`;
  89  |     expect(await recover(page, unknownEmail)).toBe(known);
  90  |     await mailLink(request, user.email);
  91  |     const mailbox = await (await request.get('http://127.0.0.1:8025/api/v1/messages?limit=500')).json();
  92  |     expect((mailbox.messages || []).some(item => (item.To || []).some(to => to.Address === unknownEmail))).toBe(false);
  93  |     expect(sql(`SELECT count(*) FROM users WHERE email='${unknownEmail}'`)).toBe('0');
  94  |   });
  95  |   test(`AUTH-010--${role}`, async ({ page, browser, request }) => {
  96  |     test.setTimeout(120000);
  97  |     const user = fixture(role, account, 'AUTH-010');
  98  |     const first = await browser.newContext();
  99  |     const second = await browser.newContext();
  100 |     try {
  101 |       const a = await first.newPage(); const b = await second.newPage();
  102 |       await login(a, user.email, root); await login(b, user.email, root);
  103 |       const firstToken = await csrf(a); const secondToken = await csrf(b);
  104 |       await recover(page, user.email);
  105 |       const url = await mailLink(request, user.email);
  106 |       const changed = 'QA-New-Recovery-Password-2026!';
  107 |       await reset(page, url, changed);
  108 |       for (const [context, oldToken] of [[first, firstToken], [second, secondToken]]) {
  109 |         expect([302, 303, 401, 403]).toContain((await context.request.get(new URL(root, local).href, { maxRedirects: 0 })).status());
  110 |         expect((await context.request.post(new URL('/logout', local).href, { form: { csrf_token: oldToken }, maxRedirects: 0 })).status()).toBe(403);
  111 |       }
  112 |       await page.goto('/login');
  113 |       const old = await page.request.post('/login', { form: { email: user.email, password, csrf_token: await csrf(page) }, maxRedirects: 0 });
  114 |       expect(old.status()).toBe(401);
  115 |       await login(page, user.email, root, changed);
  116 |     } finally { await first.close(); await second.close(); }
  117 |   });
  118 |   test(`AUTH-014--${role}`, async ({ page }) => {
  119 |     const user = fixture(role, account, 'AUTH-014');
  120 |     await login(page, user.email, root);
  121 |     const oldToken = await csrf(page);
  122 |     const cookie = (await page.context().cookies()).find(item => item.name === 'wecratfs_session');
  123 |     const key = 'wecratfs:session:' + createHash('sha256').update(cookie.value).digest('hex');
  124 |     // Give only this disposable session a short server-side lifetime.
  125 |     expect(execFileSync('docker', ['exec', 'weev-qa-20261009-redis', 'redis-cli', 'PEXPIRE', key, '1'], { encoding: 'utf8' }).trim()).toBe('1');
  126 |     await expect.poll(async () => (await page.request.post('/logout', { form: { csrf_token: oldToken }, maxRedirects: 0 })).status()).toBe(403);
  127 |     expect([302, 303, 401, 403]).toContain((await page.request.get(root, { maxRedirects: 0 })).status());
  128 |     await page.goto(root);
  129 |     expect(new URL(page.url()).pathname).toBe('/login');
  130 |   });
  131 |   test(`AUTH-011--${role}`, async ({ page, request }) => {
  132 |     test.setTimeout(120000);
  133 |     const user = fixture(role, account, 'AUTH-011');
  134 |     const credential = () => sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`);
  135 |     const initial = credential();
  136 |     await recover(page, user.email);
  137 |     const link = await mailLink(request, user.email);
  138 |     const originalToken = new URL(link).searchParams.get('token');
  139 |     await page.goto(link);
  140 |     // Exercise the JSON endpoint used by the browser's recovery form.
  141 |     const attempt = value => page.request.post('/api/v1/auth/password-reset/confirm', { data: { token: value, password: 'QA-Token-Validation-2026!' } });
  142 |     expect((await attempt(originalToken + 'altered')).status()).toBe(422);
  143 |     expect(credential() === initial).toBe(true);
```