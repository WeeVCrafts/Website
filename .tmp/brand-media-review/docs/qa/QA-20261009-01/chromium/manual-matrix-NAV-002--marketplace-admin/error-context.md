# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-matrix.spec.js >> NAV-002--marketplace-admin
- Location: tests\e2e\manual-matrix.spec.js:126:3

# Error details

```
Error: marketplace-admin accessing /account

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
      - link "Weevcrafts Marketplace Portal" [ref=f1e5] [cursor=pointer]:
        - /url: /marketplace-admin
        - generic [ref=f1e6]:
          - strong [ref=f1e7]: Weevcrafts
          - generic [ref=f1e8]: Crafted in India. Made to be cherished.
      - navigation "Admin navigation" [ref=f1e9]:
        - link "Dashboard" [ref=f1e10] [cursor=pointer]:
          - /url: /marketplace-admin
          - generic [aria-hidden] [ref=f1e11]: space_dashboard
        - link "Sellers" [ref=f1e13] [cursor=pointer]:
          - /url: /marketplace-admin/sellers
          - generic [aria-hidden] [ref=f1e14]: groups
        - link "Products" [ref=f1e16] [cursor=pointer]:
          - /url: /marketplace-admin/products
          - generic [aria-hidden] [ref=f1e17]: inventory_2
        - link "Brands" [ref=f1e19] [cursor=pointer]:
          - /url: /marketplace-admin/brands
          - generic [aria-hidden] [ref=f1e20]: store
        - link "Categories" [ref=f1e22] [cursor=pointer]:
          - /url: /marketplace-admin/category-management
          - generic [aria-hidden] [ref=f1e23]: category
        - link "Inventory" [ref=f1e25] [cursor=pointer]:
          - /url: /marketplace-admin/inventory
          - generic [aria-hidden] [ref=f1e26]: warehouse
        - link "Orders" [ref=f1e28] [cursor=pointer]:
          - /url: /marketplace-admin/orders
          - generic [aria-hidden] [ref=f1e29]: local_shipping
        - link "Returns" [ref=f1e31] [cursor=pointer]:
          - /url: /marketplace-admin/returns
          - generic [aria-hidden] [ref=f1e32]: undo
        - link "Customers" [ref=f1e34] [cursor=pointer]:
          - /url: /marketplace-admin/customers
          - generic [aria-hidden] [ref=f1e35]: people
        - link "Support" [ref=f1e37] [cursor=pointer]:
          - /url: /marketplace-admin/support
          - generic [aria-hidden] [ref=f1e38]: support_agent
        - link "Reviews" [ref=f1e40] [cursor=pointer]:
          - /url: /marketplace-admin/reviews
          - generic [aria-hidden] [ref=f1e41]: reviews
        - link "Marketing & Content" [ref=f1e43] [cursor=pointer]:
          - /url: /marketplace-admin/marketing
          - generic [aria-hidden] [ref=f1e44]: campaign
        - link "Analytics & Health" [ref=f1e46] [cursor=pointer]:
          - /url: /marketplace-admin/analytics
          - generic [aria-hidden] [ref=f1e47]: monitoring
        - link "Settings" [ref=f1e49] [cursor=pointer]:
          - /url: /marketplace-admin/settings
          - generic [aria-hidden] [ref=f1e50]: settings
      - generic [ref=f1e52]:
        - generic [aria-hidden] [ref=f1e53]: park
        - heading "Empowering Indian Crafts Across India" [level=2] [ref=f1e54]: EmpoweringIndian CraftsAcross India
        - paragraph [ref=f1e55]: More artisans.Brighter futures.
        - link "View marketplace" [ref=f1e56] [cursor=pointer]:
          - /url: /marketplace-admin/analytics
          - text: View marketplace
          - generic [aria-hidden] [ref=f1e57]: arrow_forward
    - generic [ref=f1e58]:
      - banner [ref=f1e59]:
        - generic [ref=f1e60]: Marketplace Portal
        - generic [ref=f1e61]:
          - generic [ref=f1e62]: Search the admin portal
          - generic [aria-hidden] [ref=f1e63]: search
          - textbox "Search the admin portal" [ref=f1e64]:
            - /placeholder: Search orders, sellers, products, customers or tickets...
        - link "Switch workspace" [ref=f1e65] [cursor=pointer]:
          - /url: /workspaces
          - generic [aria-hidden] [ref=f1e66]: apps
        - generic [ref=f1e67]:
          - generic [ref=f1e68]: MR
          - generic [ref=f1e69]:
            - strong [ref=f1e70]: Marketplace Reviewer
            - generic [ref=f1e71]: Marketplace Admin
        - button "Log out" [ref=f1e73] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e74]: logout
      - main [ref=f1e76]:
        - generic [ref=f1e77]:
          - generic [ref=f1e78]:
            - paragraph [ref=f1e79]: WEEVCRAFTS · Marketplace Portal
            - heading "Marketplace dashboard" [level=1] [ref=f1e80]
            - paragraph [ref=f1e81]: Live marketplace operations backed by PostgreSQL.
          - img "Indian craft detail" [ref=f1e83]
          - generic [ref=f1e85]:
            - text: More artisans.More stories.
            - strong [ref=f1e86]: A kindertomorrow.
        - generic [ref=f1e87]:
          - paragraph [ref=f1e88]: Marketplace Admin workspace
          - heading "Welcome back, Marketplace Reviewer" [level=2] [ref=f1e89]
          - paragraph [ref=f1e90]: Live work queues and account activity for your assigned Weevcrafts role.
        - generic [ref=f1e91]:
          - article [ref=f1e92]:
            - generic [aria-hidden] [ref=f1e93]: fact_check
            - generic [ref=f1e94]:
              - strong [ref=f1e95]: "0"
              - generic [ref=f1e96]: Products awaiting review
              - generic [ref=f1e97]:
                - generic [aria-hidden] [ref=f1e98]: arrow_upward
                - text: Live current
          - article [ref=f1e99]:
            - generic [aria-hidden] [ref=f1e100]: groups
            - generic [ref=f1e101]:
              - strong [ref=f1e102]: "0"
              - generic [ref=f1e103]: Seller applications
              - generic [ref=f1e104]:
                - generic [aria-hidden] [ref=f1e105]: arrow_upward
                - text: Live current
          - article [ref=f1e106]:
            - generic [aria-hidden] [ref=f1e107]: shopping_bag
            - generic [ref=f1e108]:
              - strong [ref=f1e109]: "8"
              - generic [ref=f1e110]: Orders
              - generic [ref=f1e111]:
                - generic [aria-hidden] [ref=f1e112]: arrow_downward
                - text: Live current
          - article [ref=f1e113]:
            - generic [aria-hidden] [ref=f1e114]: undo
            - generic [ref=f1e115]:
              - strong [ref=f1e116]: "1"
              - generic [ref=f1e117]: Pending returns
              - generic [ref=f1e118]:
                - generic [aria-hidden] [ref=f1e119]: arrow_downward
                - text: Live current
        - generic [ref=f1e120]:
          - article [ref=f1e121]:
            - generic [ref=f1e122]:
              - generic [ref=f1e123]:
                - generic [aria-hidden] [ref=f1e124]: fact_check
                - generic [ref=f1e125]:
                  - heading "Products awaiting review" [level=2] [ref=f1e126]
                  - paragraph [ref=f1e127]: Seller submissions requiring an approval decision.
              - link "Open catalog" [ref=f1e128] [cursor=pointer]:
                - /url: /marketplace-admin/products
            - generic [ref=f1e129]:
              - generic [ref=f1e130]:
                - generic [ref=f1e131]: Product
                - generic [ref=f1e132]: Seller
                - generic [ref=f1e133]: Status
              - status [ref=f1e134]:
                - generic [aria-hidden] [ref=f1e135]: search_off
                - strong [ref=f1e136]: No products are waiting for review.
                - generic [ref=f1e137]: New submissions from sellers will appear here.
          - article [ref=f1e138]:
            - generic [ref=f1e139]:
              - generic [ref=f1e140]:
                - generic [aria-hidden] [ref=f1e141]: groups
                - generic [ref=f1e142]:
                  - heading "Seller applications" [level=2] [ref=f1e143]
                  - paragraph [ref=f1e144]: Current application state in the seller directory.
              - link "Review sellers" [ref=f1e145] [cursor=pointer]:
                - /url: /marketplace-admin/sellers
            - generic [ref=f1e146]:
              - generic [ref=f1e147]:
                - generic [ref=f1e148]: Seller
                - generic [ref=f1e149]: Submitted
                - generic [ref=f1e150]: Status
              - generic [ref=f1e151]:
                - strong [ref=f1e152]: CraftVilla
                - generic [ref=f1e153]: 9 Oct 2026
                - generic [ref=f1e154]: active
              - generic [ref=f1e155]:
                - strong [ref=f1e156]: Mithila Arts
                - generic [ref=f1e157]: 9 Oct 2026
                - generic [ref=f1e158]: active
              - generic [ref=f1e159]:
                - strong [ref=f1e160]: Weaver's Touch
                - generic [ref=f1e161]: 9 Oct 2026
                - generic [ref=f1e162]: active
        - generic [ref=f1e163]:
          - article [ref=f1e164]:
            - generic [ref=f1e166]:
              - generic [aria-hidden] [ref=f1e167]: undo
              - generic [ref=f1e168]:
                - heading "Return requests" [level=2] [ref=f1e169]
                - paragraph [ref=f1e170]: Update return decisions in the persistent operations queue.
            - generic [ref=f1e172]:
              - generic [ref=f1e173]:
                - strong [ref=f1e174]: RTN-1 · Kavita Rao
                - paragraph [ref=f1e175]: Order WCDEMO1004 · Item arrived damaged
                - generic [ref=f1e176]: Requested · INR 2598.00
                - generic [ref=f1e177]: pending
              - generic [ref=f1e178]:
                - generic [ref=f1e179]:
                  - text: Status
                  - combobox "Status" [ref=f1e180]:
                    - option "Approve" [selected]
                    - option "Reject"
                - generic [ref=f1e181]:
                  - text: Reason
                  - textbox "Reason" [ref=f1e182]:
                    - /placeholder: Decision note
                - button "Save decision" [ref=f1e183] [cursor=pointer]
          - article [ref=f1e184]:
            - generic [ref=f1e186]:
              - generic [aria-hidden] [ref=f1e187]: local_shipping
              - generic [ref=f1e188]:
                - heading "Recent orders" [level=2] [ref=f1e189]
                - paragraph [ref=f1e190]: Marketplace order status and customer totals.
            - generic [ref=f1e191]:
              - generic [ref=f1e192]:
                - generic [ref=f1e193]: Order
                - generic [ref=f1e194]: Customer
                - generic [ref=f1e195]: Total
                - generic [ref=f1e196]: Status
              - generic [ref=f1e197]:
                - strong [ref=f1e198]: WCDEMO1008
                - generic [ref=f1e199]: Vikram Das
                - generic [ref=f1e200]: INR 799.00
                - generic [ref=f1e201]: processing
              - generic [ref=f1e202]:
                - strong [ref=f1e203]: WCDEMO1007
                - generic [ref=f1e204]: Meera Joshi
                - generic [ref=f1e205]: INR 1599.00
                - generic [ref=f1e206]: refunded
              - generic [ref=f1e207]:
                - strong [ref=f1e208]: WCDEMO1006
                - generic [ref=f1e209]: Rahul Menon
                - generic [ref=f1e210]: INR 899.00
                - generic [ref=f1e211]: payment_failed
              - generic [ref=f1e212]:
                - strong [ref=f1e213]: WCDEMO1005
                - generic [ref=f1e214]: Sophia Lee
                - generic [ref=f1e215]: INR 1298.00
                - generic [ref=f1e216]: cancelled
              - generic [ref=f1e217]:
                - strong [ref=f1e218]: WCDEMO1004
                - generic [ref=f1e219]: Kavita Rao
                - generic [ref=f1e220]: INR 2598.00
                - generic [ref=f1e221]: delivered
              - generic [ref=f1e222]:
                - strong [ref=f1e223]: WCDEMO1003
                - generic [ref=f1e224]: Arjun Nair
                - generic [ref=f1e225]: INR 5499.00
                - generic [ref=f1e226]: shipped
              - generic [ref=f1e227]:
                - strong [ref=f1e228]: WCDEMO1002
                - generic [ref=f1e229]: Sneha Iyer
                - generic [ref=f1e230]: INR 1899.00
                - generic [ref=f1e231]: paid
              - generic [ref=f1e232]:
                - strong [ref=f1e233]: WCDEMO1001
                - generic [ref=f1e234]: Priya Sharma
                - generic [ref=f1e235]: INR 2499.00
                - generic [ref=f1e236]: pending_payment
      - contentinfo [ref=f1e237]:
        - link "Weevcrafts Marketplace Portal" [ref=f1e238] [cursor=pointer]:
          - /url: /marketplace-admin
          - generic [ref=f1e239]:
            - strong [ref=f1e240]: Weevcrafts
            - generic [ref=f1e241]: Marketplace Portal
        - generic [ref=f1e242]: Handmade in India · Delivery to serviceable PIN codes across India
        - link "Switch workspace" [ref=f1e243] [cursor=pointer]:
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
> 129 |       expect((await page.request.get(root, { maxRedirects: 0 })).status(), `${role.id} accessing ${root}`).toBe(403);
      |                                                                                                            ^ Error: marketplace-admin accessing /account
  130 |     }
  131 |     await page.goto('/workspaces');
  132 |     const choices = page.getByRole('navigation', { name: 'Available workspaces' });
  133 |     if (await choices.count()) {
  134 |       for (const href of await choices.locator('a').evaluateAll(links => links.map(link => link.getAttribute('href')))) expect(href).toBe(role.root);
  135 |     }
  136 |   });
  137 | }
  138 | 
  139 | test('GST-020--guest', async ({ request }) => {
  140 |   const missing = `missing-qa-${Date.now()}`;
  141 |   for (const route of [`/products/${missing}`, `/category/${missing}`, `/brands/${missing}`, `/${missing}`]) {
  142 |     const response = await request.get(route);
  143 |     expect(response.status(), route).toBe(404);
  144 |     expect(await response.text()).not.toMatch(/panic:|SQLSTATE|password_hash/);
  145 |   }
  146 | });
  147 | 
  148 | for (const role of [null, ...roles]) {
  149 |   const id = role ? role.id : 'guest';
  150 |   const paths = role ? (role.id === 'customer' ? ['/account', '/account/addresses', '/orders', '/products/natural-soap'] : [role.root, `${role.root}/${role.id.startsWith('seller') ? 'products' : role.id === 'support-agent' ? 'cases' : role.id === 'returns-operations' ? 'returns' : role.id === 'finance-operator' ? 'payments' : 'products'}`]) : ['/', '/products', '/products/natural-soap', '/login'];
  151 |   test(`UI-001--${id}`, async ({ page }, testInfo) => {
  152 |     test.setTimeout(180_000);
  153 |     if (role) await login(page, role);
  154 |     for (const width of [360, 390, 768, 1024, 1440]) {
  155 |       await page.setViewportSize({ width, height: 1000 });
  156 |       for (const route of paths) {
  157 |         expect((await page.goto(route)).status(), route).toBe(200);
  158 |         const sizes = await page.evaluate(() => [document.documentElement.scrollWidth, document.body.scrollWidth]);
  159 |         expect(Math.max(...sizes), `${route} horizontal overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
  160 |         await expect(page.locator('main').first()).toBeVisible();
  161 |       }
  162 |       await page.screenshot({ path: testInfo.outputPath(`${id}-${width}.png`) });
  163 |     }
  164 |   });
  165 | }
  166 | 
```