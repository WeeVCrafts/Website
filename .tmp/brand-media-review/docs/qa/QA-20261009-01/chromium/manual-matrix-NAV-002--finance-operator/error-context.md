# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-matrix.spec.js >> NAV-002--finance-operator
- Location: tests\e2e\manual-matrix.spec.js:126:3

# Error details

```
Error: finance-operator accessing /account

expect(received).toBe(expected) // Object.is equality

Expected: 403
Received: 303
```

# Page snapshot

```yaml
- generic [active] [ref=f1e1]:
  - link "Skip to main content" [ref=f1e2] [cursor=pointer]:
    - /url: "#admin-main"
  - generic [ref=f1e3]:
    - complementary [ref=f1e4]:
      - link "Weevcrafts Finance Portal" [ref=f1e5] [cursor=pointer]:
        - /url: /finance-portal
        - generic [ref=f1e6]:
          - strong [ref=f1e7]: Weevcrafts
          - generic [ref=f1e8]: Crafted in India. Made to be cherished.
      - navigation "Admin navigation" [ref=f1e9]:
        - link "Payments & Finance" [ref=f1e10] [cursor=pointer]:
          - /url: /finance-portal
          - generic [aria-hidden] [ref=f1e11]: payments
        - link "Payments" [ref=f1e13] [cursor=pointer]:
          - /url: /finance-portal/payments
          - generic [aria-hidden] [ref=f1e14]: receipt_long
        - link "Failed Payments" [ref=f1e16] [cursor=pointer]:
          - /url: /finance-portal/failed-payments
          - generic [aria-hidden] [ref=f1e17]: error
        - link "Refunds" [ref=f1e19] [cursor=pointer]:
          - /url: /finance-portal/refunds
          - generic [aria-hidden] [ref=f1e20]: undo
        - link "Reconciliation" [ref=f1e22] [cursor=pointer]:
          - /url: /finance-portal/finance-reconciliation
          - generic [aria-hidden] [ref=f1e23]: fact_check
        - link "Settlements" [ref=f1e25] [cursor=pointer]:
          - /url: /finance-portal/settlements
          - generic [aria-hidden] [ref=f1e26]: account_balance
        - link "Commission rules" [ref=f1e28] [cursor=pointer]:
          - /url: /finance-portal/commission-rules
          - generic [aria-hidden] [ref=f1e29]: percent
      - generic [ref=f1e31]:
        - generic [aria-hidden] [ref=f1e32]: park
        - heading "Empowering Indian Crafts Across India" [level=2] [ref=f1e33]: EmpoweringIndian CraftsAcross India
        - paragraph [ref=f1e34]: More artisans.Brighter futures.
        - link "View marketplace" [ref=f1e35] [cursor=pointer]:
          - /url: /finance-portal/analytics
          - text: View marketplace
          - generic [aria-hidden] [ref=f1e36]: arrow_forward
    - generic [ref=f1e37]:
      - banner [ref=f1e38]:
        - generic [ref=f1e39]: Finance Portal
        - generic [ref=f1e40]:
          - generic [ref=f1e41]: Search the admin portal
          - generic [aria-hidden] [ref=f1e42]: search
          - textbox "Search the admin portal" [ref=f1e43]:
            - /placeholder: Search orders, sellers, products, customers or tickets...
        - link "Switch workspace" [ref=f1e44] [cursor=pointer]:
          - /url: /workspaces
          - generic [aria-hidden] [ref=f1e45]: apps
        - generic [ref=f1e46]:
          - generic [ref=f1e47]: FO
          - generic [ref=f1e48]:
            - strong [ref=f1e49]: Finance Operator
            - generic [ref=f1e50]: Finance Operator
        - button "Log out" [ref=f1e52] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e53]: logout
      - main [ref=f1e55]:
        - generic [ref=f1e56]:
          - generic [ref=f1e57]:
            - paragraph [ref=f1e58]: WEEVCRAFTS · Finance Portal
            - heading "Finance" [level=1] [ref=f1e59]
            - paragraph [ref=f1e60]: Live marketplace operations backed by PostgreSQL.
          - img "Indian craft detail" [ref=f1e62]
          - generic [ref=f1e64]:
            - text: More artisans.More stories.
            - strong [ref=f1e65]: A kindertomorrow.
        - generic [ref=f1e66]:
          - article [ref=f1e67]:
            - generic [aria-hidden] [ref=f1e68]: payments
            - generic [ref=f1e69]:
              - strong [ref=f1e70]: "4"
              - generic [ref=f1e71]: Captured payments
              - generic [ref=f1e72]:
                - generic [aria-hidden] [ref=f1e73]: arrow_downward
                - text: Live current
          - article [ref=f1e74]:
            - generic [aria-hidden] [ref=f1e75]: error
            - generic [ref=f1e76]:
              - strong [ref=f1e77]: "2"
              - generic [ref=f1e78]: Failed payments
              - generic [ref=f1e79]:
                - generic [aria-hidden] [ref=f1e80]: arrow_downward
                - text: Live current
          - article [ref=f1e81]:
            - generic [aria-hidden] [ref=f1e82]: account_balance
            - generic [ref=f1e83]:
              - strong [ref=f1e84]: "0"
              - generic [ref=f1e85]: Pending settlements
              - generic [ref=f1e86]:
                - generic [aria-hidden] [ref=f1e87]: arrow_upward
                - text: Live current
          - article [ref=f1e88]:
            - generic [aria-hidden] [ref=f1e89]: currency_rupee
            - generic [ref=f1e90]:
              - strong [ref=f1e91]: "0"
              - generic [ref=f1e92]: Approved returns
              - generic [ref=f1e93]:
                - generic [aria-hidden] [ref=f1e94]: arrow_upward
                - text: Live current
        - generic [ref=f1e95]:
          - article [ref=f1e96]:
            - generic [ref=f1e97]:
              - generic [ref=f1e98]:
                - generic [aria-hidden] [ref=f1e99]: account_balance
                - generic [ref=f1e100]:
                  - heading "Seller settlements" [level=2] [ref=f1e101]
                  - paragraph [ref=f1e102]: Payable amounts reconciled from captured orders and commission rules.
              - button "Create settlement batches" [ref=f1e104] [cursor=pointer]
            - generic [ref=f1e105]:
              - generic [ref=f1e106]:
                - generic [ref=f1e107]: Seller
                - generic [ref=f1e108]: Reference
                - generic [ref=f1e109]: Orders
                - generic [ref=f1e110]: Gross
                - generic [ref=f1e111]: Commission
                - generic [ref=f1e112]: Payable
                - generic [ref=f1e113]: Status
                - generic [ref=f1e114]: Period
                - generic [ref=f1e115]: Actions
              - status [ref=f1e116]:
                - generic [aria-hidden] [ref=f1e117]: search_off
                - strong [ref=f1e118]: No settlement records yet.
                - generic [ref=f1e119]: Create a batch after captured seller orders appear in the ledger.
          - article [ref=f1e120]:
            - generic [ref=f1e122]:
              - generic [aria-hidden] [ref=f1e123]: receipt_long
              - generic [ref=f1e124]:
                - heading "Payment attempts" [level=2] [ref=f1e125]
                - paragraph [ref=f1e126]: Transactions from the configured mock or payment provider.
            - generic [ref=f1e127]:
              - generic [ref=f1e128]:
                - generic [ref=f1e129]: Date
                - generic [ref=f1e130]: Order
                - generic [ref=f1e131]: Amount
                - generic [ref=f1e132]: Status
              - generic [ref=f1e133]:
                - generic [ref=f1e134]: 9 Oct 2026
                - strong [ref=f1e135]: WCDEMO1008
                - generic [ref=f1e136]: INR 799.00
                - generic [ref=f1e137]: captured
              - generic [ref=f1e138]:
                - generic [ref=f1e139]: 9 Oct 2026
                - strong [ref=f1e140]: WCDEMO1007
                - generic [ref=f1e141]: INR 1599.00
                - generic [ref=f1e142]: refunded
              - generic [ref=f1e143]:
                - generic [ref=f1e144]: 9 Oct 2026
                - strong [ref=f1e145]: WCDEMO1006
                - generic [ref=f1e146]: INR 899.00
                - generic [ref=f1e147]: failed
              - generic [ref=f1e148]:
                - generic [ref=f1e149]: 9 Oct 2026
                - strong [ref=f1e150]: WCDEMO1005
                - generic [ref=f1e151]: INR 1298.00
                - generic [ref=f1e152]: failed
              - generic [ref=f1e153]:
                - generic [ref=f1e154]: 9 Oct 2026
                - strong [ref=f1e155]: WCDEMO1004
                - generic [ref=f1e156]: INR 2598.00
                - generic [ref=f1e157]: captured
              - generic [ref=f1e158]:
                - generic [ref=f1e159]: 9 Oct 2026
                - strong [ref=f1e160]: WCDEMO1003
                - generic [ref=f1e161]: INR 5499.00
                - generic [ref=f1e162]: captured
              - generic [ref=f1e163]:
                - generic [ref=f1e164]: 9 Oct 2026
                - strong [ref=f1e165]: WCDEMO1002
                - generic [ref=f1e166]: INR 1899.00
                - generic [ref=f1e167]: captured
              - generic [ref=f1e168]:
                - generic [ref=f1e169]: 9 Oct 2026
                - strong [ref=f1e170]: WCDEMO1001
                - generic [ref=f1e171]: INR 2499.00
                - generic [ref=f1e172]: created
      - contentinfo [ref=f1e173]:
        - link "Weevcrafts Finance Portal" [ref=f1e174] [cursor=pointer]:
          - /url: /finance-portal
          - generic [ref=f1e175]:
            - strong [ref=f1e176]: Weevcrafts
            - generic [ref=f1e177]: Finance Portal
        - generic [ref=f1e178]: Handmade in India · Delivery to serviceable PIN codes across India
        - link "Switch workspace" [ref=f1e179] [cursor=pointer]:
          - /url: /workspaces
```

# Test source

```ts
  29  |   return meta.getAttribute('content');
  30  | }
  31  | async function denied(page, root) {
  32  |   const response = await page.request.get(root, { maxRedirects: 0 });
  33  |   expect([302, 303, 401, 403]).toContain(response.status());
  34  | }
  35  | 
  36  | for (const role of roles) {
  37  |   test(`AUTH-001--${role.id}`, async ({ page }) => {
  38  |     await login(page, role);
  39  |     await page.reload();
  40  |     expect(new URL(page.url()).pathname.replace(/\/$/, '')).toBe(role.root);
  41  |     await expect(page.locator('body')).toContainText(role.name);
  42  |     await expect(page.locator('body')).toContainText(role.label);
  43  |   });
  44  | 
  45  |   test(`AUTH-002--${role.id}`, async ({ page }) => {
  46  |     await page.goto('/login');
  47  |     await page.locator('#page input[name="email"]').fill(role.email);
  48  |     await page.locator('#page input[name="password"]').fill('Definitely-Incorrect-2026!');
  49  |     await page.getByRole('button', { name: /sign in/i }).click();
  50  |     await expect(page.locator('body')).toContainText(/email or password is incorrect/i);
  51  |     expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
  52  |     await denied(page, role.root);
  53  |   });
  54  | 
  55  |   test(`AUTH-003--${role.id}`, async ({ page }) => {
  56  |     await page.goto('/login');
  57  |     await page.locator('#page input[name="email"]').fill(`qa.unknown.${role.id}.${Date.now()}@example.invalid`);
  58  |     await page.locator('#page input[name="password"]').fill(password);
  59  |     await page.getByRole('button', { name: /sign in/i }).click();
  60  |     await expect(page.locator('body')).toContainText(/email or password is incorrect/i);
  61  |     expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
  62  |   });
  63  | 
  64  |   test(`AUTH-004--${role.id}`, async ({ page }) => {
  65  |     for (const [email, pass] of [['', ''], [role.email, ''], ['', password]]) {
  66  |       await page.goto('/login');
  67  |       await page.locator('#page input[name="email"]').fill(email);
  68  |       await page.locator('#page input[name="password"]').fill(pass);
  69  |       await page.getByRole('button', { name: /sign in/i }).click();
  70  |       const invalid = await page.locator('#page input:invalid').count();
  71  |       if (!invalid) await expect(page.locator('body')).toContainText(/required|email or password|enter/i);
  72  |       expect(new URL(page.url()).pathname).toBe('/login');
  73  |       expect((await page.context().cookies()).some(cookie => cookie.name === 'wecratfs_session')).toBe(false);
  74  |     }
  75  |   });
  76  | 
  77  |   test(`AUTH-007--${role.id}`, async ({ page }) => {
  78  |     // Submit at the actual HTTP boundary, without following unsafe redirects.
  79  |     for (const next of [role.root, 'https://example.invalid/steal', '//example.invalid/steal', '\\example.invalid/steal']) {
  80  |       await page.context().clearCookies();
  81  |       await page.goto(`/login?next=${encodeURIComponent(next)}`);
  82  |       const response = await page.request.post('/login', {
  83  |         form: { email: role.email, password, csrf_token: await token(page), next }, maxRedirects: 0,
  84  |       });
  85  |       expect(response.status()).toBe(303);
  86  |       const location = response.headers().location;
  87  |       expect(location).toBe(role.root);
  88  |       expect(new URL(location, page.url()).origin).toBe(new URL(page.url()).origin);
  89  |     }
  90  |   });
  91  | 
  92  |   test(`AUTH-013--${role.id}`, async ({ page, browser }) => {
  93  |     await login(page, role);
  94  |     const oldCookies = await page.context().cookies();
  95  |     const oldSession = oldCookies.find(cookie => cookie.name === 'wecratfs_session');
  96  |     expect(oldSession).toBeTruthy();
  97  |     const csrf = await token(page);
  98  |     const response = await page.request.post('/logout', { form: { csrf_token: csrf }, maxRedirects: 0 });
  99  |     expect(response.status()).toBe(303);
  100 |     await page.reload();
  101 |     await denied(page, role.root);
  102 |     await page.goBack();
  103 |     const replay = await browser.newContext();
  104 |     try {
  105 |       await replay.addCookies(oldCookies);
  106 |       expect([302, 303, 401, 403]).toContain((await replay.request.get(new URL(role.root, page.url()).href, { maxRedirects: 0 })).status());
  107 |       expect([302, 303, 401, 403]).toContain((await replay.request.post(new URL('/logout', page.url()).href, { form: { csrf_token: csrf }, maxRedirects: 0 })).status());
  108 |     } finally { await replay.close(); }
  109 |   });
  110 | 
  111 |   test(`AUTH-016--${role.id}`, async ({ page, browser }) => {
  112 |     await login(page, role);
  113 |     const second = await browser.newContext();
  114 |     try {
  115 |       const other = await second.newPage();
  116 |       await login(other, roles.find(candidate => candidate.id !== role.id));
  117 |       for (const csrf of ['', 'altered-invalid-csrf', await token(other)]) {
  118 |         const response = await page.request.post('/logout', { form: { csrf_token: csrf }, maxRedirects: 0 });
  119 |         expect(response.status()).toBe(403);
  120 |         await page.goto(role.root);
  121 |         await expect(page.locator('body')).toContainText(role.name);
  122 |       }
  123 |     } finally { await second.close(); }
  124 |   });
  125 | 
  126 |   test(`NAV-002--${role.id}`, async ({ page }) => {
  127 |     await login(page, role);
  128 |     for (const root of roots.filter(root => root !== role.root)) {
> 129 |       const response = await page.request.get(root, { maxRedirects: 0 });
      |                                                                                                            ^ Error: finance-operator accessing /account
  130 |       if (root === '/account' && response.status() === 303) {
  131 |         // The generic account entry redirects non-customers to their own
  132 |         // assigned portal; it must never serve customer workspace data.
  133 |         expect(response.headers().location).toBe(role.root);
  134 |         const followed = await page.request.get(root);
  135 |         expect(new URL(followed.url()).pathname.replace(/\/$/, '')).toBe(role.root);
  136 |       } else {
  137 |         expect(response.status(), `${role.id} accessing ${root}`).toBe(403);
  138 |       }
  139 |     }
  140 |     await page.goto('/workspaces');
  141 |     const choices = page.getByRole('navigation', { name: 'Available workspaces' });
  142 |     if (await choices.count()) {
  143 |       for (const href of await choices.locator('a').evaluateAll(links => links.map(link => link.getAttribute('href')))) expect(href).toBe(role.root);
  144 |     }
  145 |   });
  146 | }
  147 | 
  148 | test('GST-020--guest', async ({ request }) => {
  149 |   const missing = `missing-qa-${Date.now()}`;
  150 |   for (const route of [`/products/${missing}`, `/category/${missing}`, `/brands/${missing}`, `/${missing}`]) {
  151 |     const response = await request.get(route);
  152 |     expect(response.status(), route).toBe(404);
  153 |     expect(await response.text()).not.toMatch(/panic:|SQLSTATE|password_hash/);
  154 |   }
  155 | });
  156 | 
  157 | for (const role of [null, ...roles]) {
  158 |   const id = role ? role.id : 'guest';
  159 |   const paths = role ? (role.id === 'customer' ? ['/account', '/account/addresses', '/orders', '/products/natural-soap'] : [role.root, `${role.root}/${role.id.startsWith('seller') ? 'products' : role.id === 'support-agent' ? 'cases' : role.id === 'returns-operations' ? 'returns' : role.id === 'finance-operator' ? 'payments' : 'products'}`]) : ['/', '/products', '/products/natural-soap', '/login'];
  160 |   test(`UI-001--${id}`, async ({ page }, testInfo) => {
  161 |     test.setTimeout(180_000);
  162 |     if (role) await login(page, role);
  163 |     for (const width of [360, 390, 768, 1024, 1440]) {
  164 |       await page.setViewportSize({ width, height: 1000 });
  165 |       for (const route of paths) {
  166 |         expect((await page.goto(route)).status(), route).toBe(200);
  167 |         const sizes = await page.evaluate(() => [document.documentElement.scrollWidth, document.body.scrollWidth]);
  168 |         expect(Math.max(...sizes), `${route} horizontal overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
  169 |         await expect(page.locator('main').first()).toBeVisible();
  170 |       }
  171 |       await page.screenshot({ path: testInfo.outputPath(`${id}-${width}.png`) });
  172 |     }
  173 |   });
  174 | }
  175 | 
```