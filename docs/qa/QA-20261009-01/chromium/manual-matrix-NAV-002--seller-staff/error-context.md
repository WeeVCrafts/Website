# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-matrix.spec.js >> NAV-002--seller-staff
- Location: tests\e2e\manual-matrix.spec.js:126:3

# Error details

```
Error: seller-staff accessing /account

expect(received).toBe(expected) // Object.is equality

Expected: 403
Received: 303
```

# Page snapshot

```yaml
- generic [active] [ref=f1e1]:
  - link "Skip to main content" [ref=f1e2] [cursor=pointer]:
    - /url: "#seller-main"
  - banner [ref=f1e3]:
    - generic [ref=f1e4]:
      - link "Weevcrafts seller portal" [ref=f1e5] [cursor=pointer]:
        - /url: /seller-admin
        - generic [ref=f1e6]:
          - strong [ref=f1e7]: Weevcrafts
          - generic [ref=f1e8]: Crafted in India. Made to be cherished.
      - generic [ref=f1e9]: Seller Portal
      - generic [ref=f1e10]:
        - generic [ref=f1e11]: Search seller portal
        - generic [aria-hidden] [ref=f1e12]: search
        - textbox "Search seller portal" [ref=f1e13]:
          - /placeholder: Search orders, products, payouts, and more...
        - button "Search" [ref=f1e14] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e15]: search
      - link "Switch workspace" [ref=f1e16] [cursor=pointer]:
        - /url: /workspaces
        - generic [aria-hidden] [ref=f1e17]: apps
        - generic [ref=f1e18]: Workspaces
      - link "AS Arjun Nair Staff Seller Staff" [ref=f1e19] [cursor=pointer]:
        - /url: /seller-admin
        - generic [ref=f1e20]: AS
        - generic [ref=f1e21]:
          - strong [ref=f1e22]: Arjun Nair Staff
          - generic [ref=f1e23]: Seller Staff
    - navigation "Store navigation" [ref=f1e24]:
      - link "Categories" [ref=f1e25] [cursor=pointer]:
        - /url: /categories
        - text: Categories
        - generic [aria-hidden] [ref=f1e26]: expand_more
      - link "Offers" [ref=f1e27] [cursor=pointer]:
        - /url: /products?category=offers
      - link "Makers" [ref=f1e28] [cursor=pointer]:
        - /url: /brands
      - link "Storefront" [ref=f1e29] [cursor=pointer]:
        - /url: /
      - link "Switch workspace" [ref=f1e30] [cursor=pointer]:
        - /url: /workspaces
  - generic [ref=f1e31]:
    - complementary [ref=f1e32]:
      - generic [ref=f1e33]:
        - generic [ref=f1e34]: AS
        - generic [ref=f1e35]:
          - strong [ref=f1e36]: Weaver's Touch
          - generic [ref=f1e37]: demo.staff01@weevcrafts.local
          - link "Seller application status" [ref=f1e38] [cursor=pointer]:
            - /url: /seller-admin/onboarding
      - navigation "Seller portal navigation" [ref=f1e39]:
        - link "Dashboard" [ref=f1e40] [cursor=pointer]:
          - /url: /seller-admin
          - generic [aria-hidden] [ref=f1e41]: dashboard
        - link "Products" [ref=f1e43] [cursor=pointer]:
          - /url: /seller-admin/products
          - generic [aria-hidden] [ref=f1e44]: category
        - link "Inventory" [ref=f1e46] [cursor=pointer]:
          - /url: /seller-admin/inventory
          - generic [aria-hidden] [ref=f1e47]: inventory
        - link "Orders & Fulfilment" [ref=f1e49] [cursor=pointer]:
          - /url: /seller-admin/orders
          - generic [aria-hidden] [ref=f1e50]: inventory_2
        - link "Earnings" [ref=f1e52] [cursor=pointer]:
          - /url: /seller-admin/earnings
          - generic [aria-hidden] [ref=f1e53]: account_balance_wallet
        - link "Analytics" [ref=f1e55] [cursor=pointer]:
          - /url: /seller-admin/analytics
          - generic [aria-hidden] [ref=f1e56]: bar_chart
        - link "Go to storefront" [ref=f1e58] [cursor=pointer]:
          - /url: /
          - generic [aria-hidden] [ref=f1e59]: storefront
          - generic [ref=f1e60]: Storefront
        - link "Switch workspace" [ref=f1e61] [cursor=pointer]:
          - /url: /workspaces
          - generic [aria-hidden] [ref=f1e62]: apps
        - button "Log out" [ref=f1e65] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e66]: logout
      - generic [ref=f1e68]:
        - generic [aria-hidden] [ref=f1e69]: park
        - paragraph [ref=f1e70]: Grow togetherCreate with purpose.
        - strong [ref=f1e71]: Weevcraftssupports artisanbusinesses.
        - link "List New Product" [ref=f1e72] [cursor=pointer]:
          - /url: /seller/products/new
          - text: List New Product
          - generic [aria-hidden] [ref=f1e73]: arrow_forward
    - main [ref=f1e75]:
      - generic [ref=f1e76]:
        - generic [ref=f1e77]:
          - link "Home" [ref=f1e78] [cursor=pointer]:
            - /url: /seller-admin
          - generic [aria-hidden] [ref=f1e79]: chevron_right
          - generic [ref=f1e80]: Seller Portal
          - generic [aria-hidden] [ref=f1e81]: chevron_right
          - generic [ref=f1e82]: Dashboard
        - generic [ref=f1e83]:
          - paragraph [ref=f1e84]: WEEVCRAFTS SELLER PORTAL
          - heading "Dashboard" [level=1] [ref=f1e85]
          - paragraph [ref=f1e86]: Weaver's Touch · seller workspace
        - img "Handmade craft detail" [ref=f1e88]
        - generic [ref=f1e90]:
          - text: More artisans.
          - strong [ref=f1e91]: Happier homes.
      - generic [ref=f1e92]:
        - article [ref=f1e93]:
          - generic [aria-hidden] [ref=f1e94]: shopping_bag
          - generic [ref=f1e95]:
            - strong [ref=f1e96]: "0"
            - generic [ref=f1e97]: Orders · This week
            - generic [ref=f1e98]:
              - generic [aria-hidden] [ref=f1e99]: arrow_upward
              - text: Paid orders
              - emphasis [ref=f1e100]: This week
        - article [ref=f1e101]:
          - generic [aria-hidden] [ref=f1e102]: payments
          - generic [ref=f1e103]:
            - strong [ref=f1e104]: INR 0.00
            - generic [ref=f1e105]: Revenue · This week
            - generic [ref=f1e106]:
              - generic [aria-hidden] [ref=f1e107]: arrow_upward
              - text: Seller item sales
              - emphasis [ref=f1e108]: This week
        - article [ref=f1e109]:
          - generic [aria-hidden] [ref=f1e110]: account_balance_wallet
          - generic [ref=f1e111]:
            - strong [ref=f1e112]: INR 0.00
            - generic [ref=f1e113]: Pending payout
            - generic [ref=f1e114]:
              - generic [aria-hidden] [ref=f1e115]: arrow_upward
              - text: Open seller ledger
              - emphasis [ref=f1e116]: current
        - article [ref=f1e117]:
          - generic [aria-hidden] [ref=f1e118]: category
          - generic [ref=f1e119]:
            - strong [ref=f1e120]: "4"
            - generic [ref=f1e121]: Products
            - generic [ref=f1e122]:
              - generic [aria-hidden] [ref=f1e123]: arrow_upward
              - text: Active and draft
              - emphasis [ref=f1e124]: current
      - generic [ref=f1e125]:
        - article [ref=f1e126]:
          - generic [ref=f1e127]:
            - generic [ref=f1e128]:
              - generic [aria-hidden] [ref=f1e129]: category
              - generic [ref=f1e130]:
                - heading "Products" [level=2] [ref=f1e131]
                - paragraph [ref=f1e132]: Current catalogue for Weaver's Touch
            - link "Open products" [ref=f1e133] [cursor=pointer]:
              - /url: /seller-admin/products
          - generic [ref=f1e135]:
            - generic [ref=f1e136]:
              - generic [ref=f1e137]: Product
              - generic [ref=f1e138]: SKU
              - generic [ref=f1e139]: Price
              - generic [ref=f1e140]: Stock
              - generic [ref=f1e141]: Status
            - generic [ref=f1e142]:
              - generic [ref=f1e143]: Neem and Sandalwood Soap
              - generic [ref=f1e145]: FR-SOP-012
              - generic [ref=f1e146]: INR 499.00
              - generic [ref=f1e147]: "50"
              - generic [ref=f1e148]: approved
            - generic [ref=f1e149]:
              - generic [ref=f1e150]: Handwoven Wicker Basket
              - generic [ref=f1e152]: BR-BSK-009
              - generic [ref=f1e153]: INR 1099.00
              - generic [ref=f1e154]: "20"
              - generic [ref=f1e155]: approved
            - generic [ref=f1e156]:
              - generic [ref=f1e157]: Indigo Everyday Tote
              - generic [ref=f1e159]: WT-TOT-006
              - generic [ref=f1e160]: INR 899.00
              - generic [ref=f1e161]: "35"
              - generic [ref=f1e162]: approved
            - generic [ref=f1e163]:
              - generic [ref=f1e164]: Madhubani Painting - Tree of Life
              - generic [ref=f1e166]: MA-TREE-001
              - generic [ref=f1e167]: INR 2499.00
              - generic [ref=f1e168]: "17"
              - generic [ref=f1e169]: approved
        - article [ref=f1e170]:
          - generic [ref=f1e171]:
            - generic [ref=f1e172]:
              - generic [aria-hidden] [ref=f1e173]: local_shipping
              - generic [ref=f1e174]:
                - heading "Recent orders" [level=2] [ref=f1e175]
                - paragraph [ref=f1e176]: Fulfil orders from this seller account.
            - link "Open orders" [ref=f1e177] [cursor=pointer]:
              - /url: /seller-admin/orders
          - generic [ref=f1e179]:
            - generic [ref=f1e180]:
              - generic [ref=f1e181]: Order
              - generic [ref=f1e182]: Customer
              - generic [ref=f1e183]: Items
              - generic [ref=f1e184]: Value
              - generic [ref=f1e185]: Status
            - generic [ref=f1e186]:
              - generic [ref=f1e187]: WCDEMO1001
              - generic [ref=f1e188]: Priya Sharma
              - generic [ref=f1e189]: "1"
              - strong [ref=f1e190]: INR 2499.00
              - generic [ref=f1e191]: pending
      - generic [ref=f1e192]:
        - generic [ref=f1e194]:
          - generic [aria-hidden] [ref=f1e195]: bolt
          - generic [ref=f1e196]:
            - heading "Available actions" [level=2] [ref=f1e197]
            - paragraph [ref=f1e198]: Actions are shown only when your seller account has the required permission.
        - generic [ref=f1e199]:
          - link "Create product draft Submit a new listing for marketplace review." [ref=f1e200] [cursor=pointer]:
            - /url: /seller-admin/products/new
            - generic [aria-hidden] [ref=f1e201]: add
            - generic [ref=f1e202]: Create product draft
            - generic [ref=f1e203]: Submit a new listing for marketplace review.
          - link "Manage stock Review available and reserved units." [ref=f1e204] [cursor=pointer]:
            - /url: /seller-admin/inventory
            - generic [aria-hidden] [ref=f1e205]: inventory_2
            - generic [ref=f1e206]: Manage stock
            - generic [ref=f1e207]: Review available and reserved units.
          - link "Fulfil customer orders Record processing and shipment details." [ref=f1e208] [cursor=pointer]:
            - /url: /seller-admin/orders
            - generic [aria-hidden] [ref=f1e209]: local_shipping
            - generic [ref=f1e210]: Fulfil customer orders
            - generic [ref=f1e211]: Record processing and shipment details.
  - contentinfo "Seller legal links" [ref=f1e212]:
    - generic [ref=f1e213]: © 2026 Weevcrafts
    - navigation "Policies" [ref=f1e214]:
      - link "Seller Terms & Conditions" [ref=f1e215] [cursor=pointer]:
        - /url: /seller-admin/terms
      - link "Privacy Policy" [ref=f1e216] [cursor=pointer]:
        - /url: /privacy
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
      |                                                                                                            ^ Error: seller-staff accessing /account
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