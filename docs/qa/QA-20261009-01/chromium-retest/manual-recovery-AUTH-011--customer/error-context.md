# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-recovery.spec.js >> AUTH-011--customer
- Location: tests\e2e\manual-recovery.spec.js:131:3

# Error details

```
Test timeout of 120000ms exceeded.
```

```
Error: locator.inputValue: Test timeout of 120000ms exceeded.
Call log:
  - waiting for locator('input[name="csrf_token"]').first()

```

# Page snapshot

```yaml
- generic [active] [ref=f1e1]:
  - banner [ref=f1e2]:
    - link "Skip to content" [ref=f1e3] [cursor=pointer]:
      - /url: "#page"
    - generic [ref=f1e4]:
      - generic [ref=f1e5]:
        - button "Open navigation menu" [ref=f1e6] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e7]: menu
        - link "Delivery location" [ref=f1e8] [cursor=pointer]:
          - /url: /account/addresses?notice=Delivery+location+opened
          - generic [aria-hidden] [ref=f1e9]: location_on
          - generic [ref=f1e10]: Deliver toBengaluru, Karnataka
      - link "Weevcrafts home" [ref=f1e11] [cursor=pointer]:
        - /url: /
        - generic [ref=f1e12]:
          - strong [ref=f1e13]: Weevcrafts
          - generic [ref=f1e14]: Crafted in India. Made to be cherished.
      - generic [ref=f1e15]:
        - generic [ref=f1e17]:
          - generic [ref=f1e18]: Search products
          - combobox "Search products" [ref=f1e19]
          - button "Search" [ref=f1e20] [cursor=pointer]:
            - generic [aria-hidden] [ref=f1e21]: search
        - navigation "Account actions" [ref=f1e22]:
          - link "Account" [ref=f1e23] [cursor=pointer]:
            - /url: /account
            - generic [aria-hidden] [ref=f1e24]: person
          - link [ref=f1e25] [cursor=pointer]:
            - /url: /wishlist
            - generic [aria-hidden] [ref=f1e26]: favorite_border
            - generic:
              - text: Wishlist
              - emphasis [ref=f1e27]: "0"
          - link [ref=f1e28] [cursor=pointer]:
            - /url: /cart
            - generic [aria-hidden] [ref=f1e29]: shopping_bag
            - generic:
              - text: Cart
              - emphasis [ref=f1e30]: "0"
  - main [ref=f1e31]:
    - generic [ref=f1e33]:
      - paragraph [ref=f1e34]: ACCOUNT RECOVERY
      - heading "Choose a new password" [level=1] [ref=f1e35]
      - paragraph [ref=f1e36]: Use a long, unique password to keep your account safe.
      - generic [ref=f1e37]:
        - generic [ref=f1e38]:
          - text: New password
          - generic [ref=f1e39]:
            - textbox "New password Show password" [ref=f1e40]
            - button "Show password" [ref=f1e41] [cursor=pointer]:
              - generic [aria-hidden] [ref=f1e42]: visibility
        - generic [ref=f1e43]:
          - text: Confirm password
          - generic [ref=f1e44]:
            - textbox "Confirm password Show password" [ref=f1e45]
            - button "Show password" [ref=f1e46] [cursor=pointer]:
              - generic [aria-hidden] [ref=f1e47]: visibility
        - button "Update password" [ref=f1e48] [cursor=pointer]:
          - text: Update password
          - generic [aria-hidden] [ref=f1e49]: lock_reset
      - paragraph [ref=f1e50]:
        - generic [aria-hidden] [ref=f1e51]: verified_user
        - text: Your account and recovery requests are handled securely by Weevcrafts.
  - generic [ref=f1e52]:
    - generic [ref=f1e53]:
      - generic [aria-hidden] [ref=f1e54]: groups
      - generic [ref=f1e55]: Crafted in India
      - generic [ref=f1e56]: Discover independent makers
    - generic [ref=f1e57]:
      - generic [aria-hidden] [ref=f1e58]: star
      - generic [ref=f1e59]: Customer reviews
      - generic [ref=f1e60]: Read published product reviews
    - generic [ref=f1e61]:
      - generic [aria-hidden] [ref=f1e62]: volunteer_activism
      - generic [ref=f1e63]: Artisan craft
      - generic [ref=f1e64]: Explore maker collections
    - generic [ref=f1e65]:
      - generic [aria-hidden] [ref=f1e66]: verified_user
      - generic [ref=f1e67]: Secure Payments
      - generic [ref=f1e68]: Status shown in your orders
    - generic [ref=f1e69]:
      - generic [aria-hidden] [ref=f1e70]: autorenew
      - generic [ref=f1e71]: Easy Returns
      - generic [ref=f1e72]: See the returns policy
    - generic [ref=f1e73]:
      - generic [aria-hidden] [ref=f1e74]: public
      - generic [ref=f1e75]: Domestic Delivery
      - generic [ref=f1e76]: To serviceable PIN codes
  - contentinfo [ref=f1e77]:
    - generic [ref=f1e78]:
      - generic [ref=f1e79]:
        - link "Weevcrafts Crafted in India. Made to be cherished." [ref=f1e80] [cursor=pointer]:
          - /url: /
          - generic [ref=f1e81]:
            - strong [ref=f1e82]: Weevcrafts
            - generic [ref=f1e83]: Crafted in India. Made to be cherished.
        - paragraph [ref=f1e84]: A marketplace for Indian arts, crafts and sarees.Supporting artisans. Preserving traditions.Building a brighter, more inclusive tomorrow.
        - generic [ref=f1e85]:
          - generic [aria-hidden] [ref=f1e86]: photo_camera
          - generic [aria-hidden] [ref=f1e87]: play_circle
          - generic [aria-hidden] [ref=f1e88]: alternate_email
          - generic [aria-hidden] [ref=f1e89]: language
      - generic [ref=f1e90]:
        - heading "Shop" [level=3] [ref=f1e91]
        - link "All Categories" [ref=f1e92] [cursor=pointer]:
          - /url: /categories
        - link "Arts" [ref=f1e93] [cursor=pointer]:
          - /url: /products?category=arts
        - link "Crafts" [ref=f1e94] [cursor=pointer]:
          - /url: /products?category=crafts
        - link "Sarees" [ref=f1e95] [cursor=pointer]:
          - /url: /products?category=sarees
        - link "Offers" [ref=f1e96] [cursor=pointer]:
          - /url: /products?category=offers
        - link "New Arrivals" [ref=f1e97] [cursor=pointer]:
          - /url: /products
      - generic [ref=f1e98]:
        - heading "Help" [level=3] [ref=f1e99]
        - link "Track Order" [ref=f1e100] [cursor=pointer]:
          - /url: /orders
        - link "Returns & Refunds" [ref=f1e101] [cursor=pointer]:
          - /url: /returns
        - link "Shipping Info" [ref=f1e102] [cursor=pointer]:
          - /url: /shipping
        - link "Size Guide" [ref=f1e103] [cursor=pointer]:
          - /url: /size-guide
        - link "FAQs" [ref=f1e104] [cursor=pointer]:
          - /url: /faq
        - link "Contact Us" [ref=f1e105] [cursor=pointer]:
          - /url: /contact
      - generic [ref=f1e106]:
        - heading "About" [level=3] [ref=f1e107]
        - link "Our Story" [ref=f1e108] [cursor=pointer]:
          - /url: /our-story
        - link "Makers" [ref=f1e109] [cursor=pointer]:
          - /url: /makers/mithila-arts
        - link "Sustainability" [ref=f1e110] [cursor=pointer]:
          - /url: /sustainability
        - link "Press" [ref=f1e111] [cursor=pointer]:
          - /url: /press
        - link "Careers" [ref=f1e112] [cursor=pointer]:
          - /url: /careers
        - link "Sell on Weevcrafts" [ref=f1e113] [cursor=pointer]:
          - /url: /seller
      - generic [ref=f1e114]:
        - heading "Email preferences" [level=3] [ref=f1e115]
        - paragraph [ref=f1e116]: Choose whether you want offers, new arrivals and maker stories.
        - link "Manage notification preferences" [ref=f1e117] [cursor=pointer]:
          - /url: /account/notifications
          - text: Manage notification preferences
          - generic [aria-hidden] [ref=f1e118]: arrow_forward
        - generic [ref=f1e119]: Sign in to save your choices. Email delivery depends on store email settings.
        - generic [ref=f1e120]: Made with care in India | A more handmade tomorrow
    - generic [ref=f1e121]:
      - generic [ref=f1e122]: Copyright 2024 Weevcrafts. All rights reserved.
      - generic [ref=f1e123]: Terms & Conditions | Privacy Policy | Cookie Policy
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
> 32  |   return page.locator('input[name="csrf_token"]').first().inputValue();
      |                                                           ^ Error: locator.inputValue: Test timeout of 120000ms exceeded.
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
  43  |   await page.locator('#page input[name="email"]').fill(email);
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
```