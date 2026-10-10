# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-matrix.spec.js >> NAV-002--super-admin
- Location: tests\e2e\manual-matrix.spec.js:126:3

# Error details

```
Error: super-admin accessing /account

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
      - link "Weevcrafts Super Admin Portal" [ref=f1e5] [cursor=pointer]:
        - /url: /super-admin
        - generic [ref=f1e6]:
          - strong [ref=f1e7]: Weevcrafts
          - generic [ref=f1e8]: Crafted in India. Made to be cherished.
      - navigation "Admin navigation" [ref=f1e9]:
        - link "Dashboard" [ref=f1e10] [cursor=pointer]:
          - /url: /super-admin
          - generic [aria-hidden] [ref=f1e11]: space_dashboard
        - link "Sellers" [ref=f1e13] [cursor=pointer]:
          - /url: /super-admin/sellers
          - generic [aria-hidden] [ref=f1e14]: groups
        - link "Products" [ref=f1e16] [cursor=pointer]:
          - /url: /super-admin/products
          - generic [aria-hidden] [ref=f1e17]: inventory_2
        - link "Brands" [ref=f1e19] [cursor=pointer]:
          - /url: /super-admin/brands
          - generic [aria-hidden] [ref=f1e20]: store
        - link "Categories" [ref=f1e22] [cursor=pointer]:
          - /url: /super-admin/category-management
          - generic [aria-hidden] [ref=f1e23]: category
        - link "Inventory" [ref=f1e25] [cursor=pointer]:
          - /url: /super-admin/inventory
          - generic [aria-hidden] [ref=f1e26]: warehouse
        - link "Orders" [ref=f1e28] [cursor=pointer]:
          - /url: /super-admin/orders
          - generic [aria-hidden] [ref=f1e29]: local_shipping
        - link "Returns" [ref=f1e31] [cursor=pointer]:
          - /url: /super-admin/returns
          - generic [aria-hidden] [ref=f1e32]: undo
        - link "Customers" [ref=f1e34] [cursor=pointer]:
          - /url: /super-admin/customers
          - generic [aria-hidden] [ref=f1e35]: people
        - link "Support" [ref=f1e37] [cursor=pointer]:
          - /url: /super-admin/support
          - generic [aria-hidden] [ref=f1e38]: support_agent
        - link "Reviews" [ref=f1e40] [cursor=pointer]:
          - /url: /super-admin/reviews
          - generic [aria-hidden] [ref=f1e41]: reviews
        - link "Marketing & Content" [ref=f1e43] [cursor=pointer]:
          - /url: /super-admin/marketing
          - generic [aria-hidden] [ref=f1e44]: campaign
        - link "Finance" [ref=f1e46] [cursor=pointer]:
          - /url: /super-admin/finance
          - generic [aria-hidden] [ref=f1e47]: payments
        - link "Commission rules" [ref=f1e49] [cursor=pointer]:
          - /url: /super-admin/commission-rules
          - generic [aria-hidden] [ref=f1e50]: percent
        - link "Security & Roles" [ref=f1e52] [cursor=pointer]:
          - /url: /super-admin/security
          - generic [aria-hidden] [ref=f1e53]: admin_panel_settings
        - link "Analytics & Health" [ref=f1e55] [cursor=pointer]:
          - /url: /super-admin/analytics
          - generic [aria-hidden] [ref=f1e56]: monitoring
        - link "Settings" [ref=f1e58] [cursor=pointer]:
          - /url: /super-admin/settings
          - generic [aria-hidden] [ref=f1e59]: settings
      - generic [ref=f1e61]:
        - generic [aria-hidden] [ref=f1e62]: park
        - heading "Empowering Indian Crafts Across India" [level=2] [ref=f1e63]: EmpoweringIndian CraftsAcross India
        - paragraph [ref=f1e64]: More artisans.Brighter futures.
        - link "View marketplace" [ref=f1e65] [cursor=pointer]:
          - /url: /super-admin/analytics
          - text: View marketplace
          - generic [aria-hidden] [ref=f1e66]: arrow_forward
    - generic [ref=f1e67]:
      - banner [ref=f1e68]:
        - generic [ref=f1e69]: Super Admin Portal
        - generic [ref=f1e70]:
          - generic [ref=f1e71]: Search the admin portal
          - generic [aria-hidden] [ref=f1e72]: search
          - textbox "Search the admin portal" [ref=f1e73]:
            - /placeholder: Search orders, sellers, products, customers or tickets...
        - link "Switch workspace" [ref=f1e74] [cursor=pointer]:
          - /url: /workspaces
          - generic [aria-hidden] [ref=f1e75]: apps
        - generic [ref=f1e76]:
          - generic [ref=f1e77]: SA
          - generic [ref=f1e78]:
            - strong [ref=f1e79]: Super Admin One
            - generic [ref=f1e80]: Super Admin
        - button "Log out" [ref=f1e82] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e83]: logout
      - main [ref=f1e85]:
        - generic [ref=f1e86]:
          - generic [ref=f1e87]:
            - paragraph [ref=f1e88]: WEEVCRAFTS · Super Admin Portal
            - heading "Marketplace dashboard" [level=1] [ref=f1e89]
            - paragraph [ref=f1e90]: Live marketplace operations backed by PostgreSQL.
          - img "Indian craft detail" [ref=f1e92]
          - generic [ref=f1e94]:
            - text: More artisans.More stories.
            - strong [ref=f1e95]: A kindertomorrow.
        - generic [ref=f1e96]:
          - paragraph [ref=f1e97]: Super Admin workspace
          - heading "Welcome back, Super Admin One" [level=2] [ref=f1e98]
          - paragraph [ref=f1e99]: Live work queues and account activity for your assigned Weevcrafts role.
        - generic [ref=f1e100]:
          - article [ref=f1e101]:
            - generic [aria-hidden] [ref=f1e102]: groups
            - generic [ref=f1e103]:
              - strong [ref=f1e104]: "24"
              - generic [ref=f1e105]: User accounts
              - generic [ref=f1e106]:
                - generic [aria-hidden] [ref=f1e107]: arrow_downward
                - text: Live current
          - article [ref=f1e108]:
            - generic [aria-hidden] [ref=f1e109]: admin_panel_settings
            - generic [ref=f1e110]:
              - strong [ref=f1e111]: "16"
              - generic [ref=f1e112]: Operational role assignments
              - generic [ref=f1e113]:
                - generic [aria-hidden] [ref=f1e114]: arrow_downward
                - text: Live current
          - article [ref=f1e115]:
            - generic [aria-hidden] [ref=f1e116]: history
            - generic [ref=f1e117]:
              - strong [ref=f1e118]: "11"
              - generic [ref=f1e119]: Recorded audit events
              - generic [ref=f1e120]:
                - generic [aria-hidden] [ref=f1e121]: arrow_downward
                - text: Live current
          - article [ref=f1e122]:
            - generic [aria-hidden] [ref=f1e123]: support_agent
            - generic [ref=f1e124]:
              - strong [ref=f1e125]: "2"
              - generic [ref=f1e126]: Open support cases
              - generic [ref=f1e127]:
                - generic [aria-hidden] [ref=f1e128]: arrow_downward
                - text: Live current
        - generic [ref=f1e129]:
          - article [ref=f1e130]:
            - generic [ref=f1e131]:
              - generic [ref=f1e132]:
                - generic [aria-hidden] [ref=f1e133]: fact_check
                - generic [ref=f1e134]:
                  - heading "Products awaiting review" [level=2] [ref=f1e135]
                  - paragraph [ref=f1e136]: Seller submissions requiring an approval decision.
              - link "Open catalog" [ref=f1e137] [cursor=pointer]:
                - /url: /super-admin/products
            - generic [ref=f1e138]:
              - generic [ref=f1e139]:
                - generic [ref=f1e140]: Product
                - generic [ref=f1e141]: Seller
                - generic [ref=f1e142]: Status
              - status [ref=f1e143]:
                - generic [aria-hidden] [ref=f1e144]: search_off
                - strong [ref=f1e145]: No products are waiting for review.
                - generic [ref=f1e146]: New submissions from sellers will appear here.
          - article [ref=f1e147]:
            - generic [ref=f1e149]:
              - generic [aria-hidden] [ref=f1e150]: history
              - generic [ref=f1e151]:
                - heading "Recent audit events" [level=2] [ref=f1e152]
                - paragraph [ref=f1e153]: Recorded administrative and operational changes.
            - generic [ref=f1e154]:
              - generic [ref=f1e155]:
                - generic [ref=f1e156]: Time
                - generic [ref=f1e157]: Actor
                - generic [ref=f1e158]: Action
                - generic [ref=f1e159]: Record
              - generic [ref=f1e160]:
                - generic [ref=f1e161]: 9 Oct 2026, 10:21
                - generic [ref=f1e162]: "18"
                - strong [ref=f1e163]: auth.login_failed
                - generic [ref=f1e164]: "user #18"
              - generic [ref=f1e165]:
                - generic [ref=f1e166]: 9 Oct 2026, 10:20
                - generic [ref=f1e167]: "15"
                - strong [ref=f1e168]: auth.login_failed
                - generic [ref=f1e169]: "user #15"
              - generic [ref=f1e170]:
                - generic [ref=f1e171]: 9 Oct 2026, 10:20
                - generic [ref=f1e172]: "12"
                - strong [ref=f1e173]: auth.login_failed
                - generic [ref=f1e174]: "user #12"
              - generic [ref=f1e175]:
                - generic [ref=f1e176]: 9 Oct 2026, 10:19
                - generic [ref=f1e177]: "9"
                - strong [ref=f1e178]: auth.login_failed
                - generic [ref=f1e179]: "user #9"
              - generic [ref=f1e180]:
                - generic [ref=f1e181]: 9 Oct 2026, 10:18
                - generic [ref=f1e182]: "1"
                - strong [ref=f1e183]: auth.login_failed
                - generic [ref=f1e184]: "user #1"
              - generic [ref=f1e185]:
                - generic [ref=f1e186]: 9 Oct 2026, 10:18
                - generic [ref=f1e187]: "18"
                - strong [ref=f1e188]: management.save
                - generic [ref=f1e189]: "settings #"
              - generic [ref=f1e190]:
                - generic [ref=f1e191]: 9 Oct 2026, 10:18
                - generic [ref=f1e192]: "18"
                - strong [ref=f1e193]: management.save
                - generic [ref=f1e194]: "settings #"
              - generic [ref=f1e195]:
                - generic [ref=f1e196]: 9 Oct 2026, 10:18
                - generic [ref=f1e197]: "18"
                - strong [ref=f1e198]: management.delete
                - generic [ref=f1e199]: "brands #backend-qa-brand-1791541089204"
              - generic [ref=f1e200]:
                - generic [ref=f1e201]: 9 Oct 2026, 10:18
                - generic [ref=f1e202]: "18"
                - strong [ref=f1e203]: management.update
                - generic [ref=f1e204]: "brands #backend-qa-brand-1791541089204"
              - generic [ref=f1e205]:
                - generic [ref=f1e206]: 9 Oct 2026, 10:18
                - generic [ref=f1e207]: "18"
                - strong [ref=f1e208]: management.create
                - generic [ref=f1e209]: "brands #backend-qa-brand-1791541089204"
              - generic [ref=f1e210]:
                - generic [ref=f1e211]: 9 Oct 2026, 10:14
                - generic [ref=f1e212]: "18"
                - strong [ref=f1e213]: demo.seeded
                - generic [ref=f1e214]: "platform #local-demo"
        - generic [ref=f1e215]:
          - article [ref=f1e216]:
            - generic [ref=f1e217]:
              - generic [ref=f1e218]:
                - generic [aria-hidden] [ref=f1e219]: groups
                - generic [ref=f1e220]:
                  - heading "Seller applications" [level=2] [ref=f1e221]
                  - paragraph [ref=f1e222]: Current application state in the seller directory.
              - link "Review sellers" [ref=f1e223] [cursor=pointer]:
                - /url: /super-admin/sellers
            - generic [ref=f1e224]:
              - generic [ref=f1e225]:
                - generic [ref=f1e226]: Seller
                - generic [ref=f1e227]: Submitted
                - generic [ref=f1e228]: Status
              - generic [ref=f1e229]:
                - strong [ref=f1e230]: CraftVilla
                - generic [ref=f1e231]: 9 Oct 2026
                - generic [ref=f1e232]: active
              - generic [ref=f1e233]:
                - strong [ref=f1e234]: Mithila Arts
                - generic [ref=f1e235]: 9 Oct 2026
                - generic [ref=f1e236]: active
              - generic [ref=f1e237]:
                - strong [ref=f1e238]: Weaver's Touch
                - generic [ref=f1e239]: 9 Oct 2026
                - generic [ref=f1e240]: active
          - article [ref=f1e241]:
            - generic [ref=f1e243]:
              - generic [aria-hidden] [ref=f1e244]: security
              - generic [ref=f1e245]:
                - heading "Identity and access" [level=2] [ref=f1e246]
                - paragraph [ref=f1e247]: Review user roles and audited access changes.
            - link "Manage roles and security" [ref=f1e248] [cursor=pointer]:
              - /url: /super-admin/security
      - contentinfo [ref=f1e249]:
        - link "Weevcrafts Super Admin Portal" [ref=f1e250] [cursor=pointer]:
          - /url: /super-admin
          - generic [ref=f1e251]:
            - strong [ref=f1e252]: Weevcrafts
            - generic [ref=f1e253]: Super Admin Portal
        - generic [ref=f1e254]: Handmade in India · Delivery to serviceable PIN codes across India
        - link "Switch workspace" [ref=f1e255] [cursor=pointer]:
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
      |                                                                                                            ^ Error: super-admin accessing /account
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