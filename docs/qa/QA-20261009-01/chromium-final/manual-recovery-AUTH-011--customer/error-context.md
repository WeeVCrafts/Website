# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-recovery.spec.js >> AUTH-011--customer
- Location: tests\e2e\manual-recovery.spec.js:131:3

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: 422
Received: 429
```

# Page snapshot

```yaml
- generic [active] [ref=f3e1]:
  - banner [ref=f3e2]:
    - link "Skip to content" [ref=f3e3] [cursor=pointer]:
      - /url: "#page"
    - generic [ref=f3e4]:
      - generic [ref=f3e5]:
        - button "Open navigation menu" [ref=f3e6] [cursor=pointer]:
          - generic [aria-hidden] [ref=f3e7]: menu
        - link "Delivery location" [ref=f3e8] [cursor=pointer]:
          - /url: /account/addresses?notice=Delivery+location+opened
          - generic [aria-hidden] [ref=f3e9]: location_on
          - generic [ref=f3e10]: Deliver toBengaluru, Karnataka
      - link "Weevcrafts home" [ref=f3e11] [cursor=pointer]:
        - /url: /
        - generic [ref=f3e12]:
          - strong [ref=f3e13]: Weevcrafts
          - generic [ref=f3e14]: Crafted in India. Made to be cherished.
      - generic [ref=f3e15]:
        - generic [ref=f3e17]:
          - generic [ref=f3e18]: Search products
          - combobox "Search products" [ref=f3e19]
          - button "Search" [ref=f3e20] [cursor=pointer]:
            - generic [aria-hidden] [ref=f3e21]: search
        - navigation "Account actions" [ref=f3e22]:
          - link "Account" [ref=f3e23] [cursor=pointer]:
            - /url: /account
            - generic [aria-hidden] [ref=f3e24]: person
          - link [ref=f3e25] [cursor=pointer]:
            - /url: /wishlist
            - generic [aria-hidden] [ref=f3e26]: favorite_border
            - generic:
              - text: Wishlist
              - emphasis [ref=f3e27]: "0"
          - link [ref=f3e28] [cursor=pointer]:
            - /url: /cart
            - generic [aria-hidden] [ref=f3e29]: shopping_bag
            - generic:
              - text: Cart
              - emphasis [ref=f3e30]: "0"
  - main [ref=f3e31]:
    - generic [ref=f3e33]:
      - paragraph [ref=f3e34]: ACCOUNT RECOVERY
      - heading "Choose a new password" [level=1] [ref=f3e35]
      - paragraph [ref=f3e36]: Use a long, unique password to keep your account safe.
      - generic [ref=f3e37]:
        - generic [ref=f3e38]:
          - text: New password
          - generic [ref=f3e39]:
            - textbox "New password Show password" [ref=f3e40]
            - button "Show password" [ref=f3e41] [cursor=pointer]:
              - generic [aria-hidden] [ref=f3e42]: visibility
        - generic [ref=f3e43]:
          - text: Confirm password
          - generic [ref=f3e44]:
            - textbox "Confirm password Show password" [ref=f3e45]
            - button "Show password" [ref=f3e46] [cursor=pointer]:
              - generic [aria-hidden] [ref=f3e47]: visibility
        - button "Update password" [ref=f3e48] [cursor=pointer]:
          - text: Update password
          - generic [aria-hidden] [ref=f3e49]: lock_reset
      - paragraph [ref=f3e50]:
        - generic [aria-hidden] [ref=f3e51]: verified_user
        - text: Your account and recovery requests are handled securely by Weevcrafts.
  - generic [ref=f3e52]:
    - generic [ref=f3e53]:
      - generic [aria-hidden] [ref=f3e54]: groups
      - generic [ref=f3e55]: Crafted in India
      - generic [ref=f3e56]: Discover independent makers
    - generic [ref=f3e57]:
      - generic [aria-hidden] [ref=f3e58]: star
      - generic [ref=f3e59]: Customer reviews
      - generic [ref=f3e60]: Read published product reviews
    - generic [ref=f3e61]:
      - generic [aria-hidden] [ref=f3e62]: volunteer_activism
      - generic [ref=f3e63]: Artisan craft
      - generic [ref=f3e64]: Explore maker collections
    - generic [ref=f3e65]:
      - generic [aria-hidden] [ref=f3e66]: verified_user
      - generic [ref=f3e67]: Secure Payments
      - generic [ref=f3e68]: Status shown in your orders
    - generic [ref=f3e69]:
      - generic [aria-hidden] [ref=f3e70]: autorenew
      - generic [ref=f3e71]: Easy Returns
      - generic [ref=f3e72]: See the returns policy
    - generic [ref=f3e73]:
      - generic [aria-hidden] [ref=f3e74]: public
      - generic [ref=f3e75]: Domestic Delivery
      - generic [ref=f3e76]: To serviceable PIN codes
  - contentinfo [ref=f3e77]:
    - generic [ref=f3e78]:
      - generic [ref=f3e79]:
        - link "Weevcrafts Crafted in India. Made to be cherished." [ref=f3e80] [cursor=pointer]:
          - /url: /
          - generic [ref=f3e81]:
            - strong [ref=f3e82]: Weevcrafts
            - generic [ref=f3e83]: Crafted in India. Made to be cherished.
        - paragraph [ref=f3e84]: A marketplace for Indian arts, crafts and sarees.Supporting artisans. Preserving traditions.Building a brighter, more inclusive tomorrow.
        - generic [ref=f3e85]:
          - generic [aria-hidden] [ref=f3e86]: photo_camera
          - generic [aria-hidden] [ref=f3e87]: play_circle
          - generic [aria-hidden] [ref=f3e88]: alternate_email
          - generic [aria-hidden] [ref=f3e89]: language
      - generic [ref=f3e90]:
        - heading "Shop" [level=3] [ref=f3e91]
        - link "All Categories" [ref=f3e92] [cursor=pointer]:
          - /url: /categories
        - link "Arts" [ref=f3e93] [cursor=pointer]:
          - /url: /products?category=arts
        - link "Crafts" [ref=f3e94] [cursor=pointer]:
          - /url: /products?category=crafts
        - link "Sarees" [ref=f3e95] [cursor=pointer]:
          - /url: /products?category=sarees
        - link "Offers" [ref=f3e96] [cursor=pointer]:
          - /url: /products?category=offers
        - link "New Arrivals" [ref=f3e97] [cursor=pointer]:
          - /url: /products
      - generic [ref=f3e98]:
        - heading "Help" [level=3] [ref=f3e99]
        - link "Track Order" [ref=f3e100] [cursor=pointer]:
          - /url: /orders
        - link "Returns & Refunds" [ref=f3e101] [cursor=pointer]:
          - /url: /returns
        - link "Shipping Info" [ref=f3e102] [cursor=pointer]:
          - /url: /shipping
        - link "Size Guide" [ref=f3e103] [cursor=pointer]:
          - /url: /size-guide
        - link "FAQs" [ref=f3e104] [cursor=pointer]:
          - /url: /faq
        - link "Contact Us" [ref=f3e105] [cursor=pointer]:
          - /url: /contact
      - generic [ref=f3e106]:
        - heading "About" [level=3] [ref=f3e107]
        - link "Our Story" [ref=f3e108] [cursor=pointer]:
          - /url: /our-story
        - link "Makers" [ref=f3e109] [cursor=pointer]:
          - /url: /makers/mithila-arts
        - link "Sustainability" [ref=f3e110] [cursor=pointer]:
          - /url: /sustainability
        - link "Press" [ref=f3e111] [cursor=pointer]:
          - /url: /press
        - link "Careers" [ref=f3e112] [cursor=pointer]:
          - /url: /careers
        - link "Sell on Weevcrafts" [ref=f3e113] [cursor=pointer]:
          - /url: /seller
      - generic [ref=f3e114]:
        - heading "Email preferences" [level=3] [ref=f3e115]
        - paragraph [ref=f3e116]: Choose whether you want offers, new arrivals and maker stories.
        - link "Manage notification preferences" [ref=f3e117] [cursor=pointer]:
          - /url: /account/notifications
          - text: Manage notification preferences
          - generic [aria-hidden] [ref=f3e118]: arrow_forward
        - generic [ref=f3e119]: Sign in to save your choices. Email delivery depends on store email settings.
        - generic [ref=f3e120]: Made with care in India | A more handmade tomorrow
    - generic [ref=f3e121]:
      - generic [ref=f3e122]: Copyright 2024 Weevcrafts. All rights reserved.
      - generic [ref=f3e123]: Terms & Conditions | Privacy Policy | Cookie Policy
```

# Test source

```ts
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
  144 |     sql(`UPDATE auth_action_tokens SET expires_at=NOW()-INTERVAL '1 minute' WHERE user_id=${user.id} AND purpose='password_reset'`);
  145 |     expect((await attempt(originalToken)).status()).toBe(422);
  146 |     expect(credential() === initial).toBe(true);
  147 |     await recover(page, user.email);
  148 |     const freshLink = await mailLink(request, user.email);
  149 |     const freshToken = new URL(freshLink).searchParams.get('token');
  150 |     await page.goto(freshLink);
  151 |     expect((await attempt(freshToken)).status()).toBe(200);
  152 |     const changed = credential();
  153 |     expect(changed === initial).toBe(false);
> 154 |     expect((await attempt(freshToken)).status()).toBe(422);
      |                                                  ^ Error: expect(received).toBe(expected) // Object.is equality
  155 |     expect(credential() === changed).toBe(true);
  156 |   });
  157 |   test(`AUTH-012--${role}`, async ({ page, request }) => {
  158 |     test.setTimeout(120000);
  159 |     const user = fixture(role, account, 'AUTH-012');
  160 |     await recover(page, user.email);
  161 |     const link = await mailLink(request, user.email);
  162 |     const rawToken = new URL(link).searchParams.get('token');
  163 |     await page.goto(link);
  164 |     const before = sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`);
  165 |     for (const value of ['a'.repeat(11), 'a'.repeat(1025)]) {
  166 |       const response = await page.request.post('/api/v1/auth/password-reset/confirm', { data: { token: rawToken, password: value } });
  167 |       expect(response.status()).toBe(422);
  168 |       expect(await response.text()).toMatch(/reset token or password is invalid/i);
  169 |       expect(sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`) === before).toBe(true);
  170 |       expect(sql(`SELECT count(*) FROM auth_action_tokens WHERE user_id=${user.id} AND consumed_at IS NULL AND expires_at>NOW()`)).toBe('1');
  171 |     }
  172 |     await page.locator('#page input[name="password"]').fill('Valid-Min-12');
  173 |     await page.locator('#page input[name="confirm_password"]').fill('Different-12');
  174 |     await page.getByRole('button', { name: 'Update password' }).click();
  175 |     await expect(page.locator('[data-backend-form-message]')).toContainText(/match/i);
  176 |     expect(sql(`SELECT password_hash FROM credentials WHERE user_id=${user.id}`) === before).toBe(true);
  177 |     // Exactly twelve ASCII bytes, matching the documented server boundary.
  178 |     await reset(page, link, 'Valid-Min-12');
  179 |     await login(page, user.email, root, 'Valid-Min-12');
  180 |   });
  181 | }
  182 | 
```