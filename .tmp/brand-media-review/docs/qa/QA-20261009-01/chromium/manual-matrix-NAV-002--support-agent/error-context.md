# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-matrix.spec.js >> NAV-002--support-agent
- Location: tests\e2e\manual-matrix.spec.js:126:3

# Error details

```
Error: support-agent accessing /account

expect(received).toBe(expected) // Object.is equality

Expected: 403
Received: 303
```

# Page snapshot

```yaml
- generic [active] [ref=f1e1]:
  - link "Skip to main content" [ref=f1e2] [cursor=pointer]:
    - /url: "#support-main"
  - banner [ref=f1e3]:
    - generic [ref=f1e4]:
      - link "Weevcrafts support portal" [ref=f1e5] [cursor=pointer]:
        - /url: /support-portal
        - generic [ref=f1e6]:
          - strong [ref=f1e7]: Weevcrafts
          - generic [ref=f1e8]: Crafted in India. Made to be cherished.
      - generic [ref=f1e9]: Support Agent
      - generic [ref=f1e10]:
        - generic [ref=f1e11]: Search support cases
        - generic [aria-hidden] [ref=f1e12]: search
        - textbox "Search support cases" [ref=f1e13]:
          - /placeholder: Search cases, orders, customers, sellers or anything...
      - link "Switch workspace" [ref=f1e14] [cursor=pointer]:
        - /url: /workspaces
        - generic [aria-hidden] [ref=f1e15]: apps
      - link "AS Aditi Support Support Agent" [ref=f1e16] [cursor=pointer]:
        - /url: /workspaces
        - generic [ref=f1e17]: AS
        - generic [ref=f1e18]:
          - strong [ref=f1e19]: Aditi Support
          - generic [ref=f1e20]: Support Agent
        - generic [aria-hidden] [ref=f1e21]: expand_more
  - generic [ref=f1e22]:
    - complementary [ref=f1e23]:
      - generic [ref=f1e24]:
        - generic [ref=f1e25]: AS
        - generic [ref=f1e26]:
          - strong [ref=f1e27]: Aditi Support
          - generic [ref=f1e28]: Support Agent
          - link "Switch workspace" [ref=f1e29] [cursor=pointer]:
            - /url: /workspaces
      - navigation "Support portal navigation" [ref=f1e30]:
        - link "Dashboard" [ref=f1e31] [cursor=pointer]:
          - /url: /support-portal
          - generic [aria-hidden] [ref=f1e32]: home
        - link "Cases" [ref=f1e34] [cursor=pointer]:
          - /url: /support-portal/cases
          - generic [aria-hidden] [ref=f1e35]: work_outline
        - link "Customers" [ref=f1e37] [cursor=pointer]:
          - /url: /support-portal/customers
          - generic [aria-hidden] [ref=f1e38]: person_outline
        - link "Orders" [ref=f1e40] [cursor=pointer]:
          - /url: /support-portal/orders
          - generic [aria-hidden] [ref=f1e41]: inventory_2
        - link "Switch workspace" [ref=f1e43] [cursor=pointer]:
          - /url: /workspaces
          - generic [aria-hidden] [ref=f1e44]: apps
        - button "Log out" [ref=f1e47] [cursor=pointer]:
          - generic [aria-hidden] [ref=f1e48]: logout
      - generic [ref=f1e50]:
        - generic [aria-hidden] [ref=f1e51]: support_agent
        - paragraph [ref=f1e52]: Resolve customer issues with care and keep every update on the ticket record.
        - link "Open case queue" [ref=f1e53] [cursor=pointer]:
          - /url: /support-portal/cases
          - text: Open case queue
          - generic [aria-hidden] [ref=f1e54]: arrow_forward
    - main [ref=f1e55]:
      - generic [ref=f1e56]:
        - generic [ref=f1e57]:
          - generic [ref=f1e58]:
            - heading "Support dashboard" [level=1] [ref=f1e59]
            - paragraph [ref=f1e60]: Support cases backed by live PostgreSQL records.
          - img "Indian craft detail" [ref=f1e62]
          - generic [ref=f1e64]:
            - text: Every questionis an opportunityto create a better
            - strong [ref=f1e65]: tomorrow.
        - generic [ref=f1e66]:
          - article [ref=f1e67]:
            - generic [aria-hidden] [ref=f1e68]: support_agent
            - generic [ref=f1e69]:
              - strong [ref=f1e70]: "2"
              - generic [ref=f1e71]: Open tickets
              - generic [ref=f1e72]:
                - generic [aria-hidden] [ref=f1e73]: arrow_downward
                - emphasis [ref=f1e74]: PostgreSQL
          - article [ref=f1e75]:
            - generic [aria-hidden] [ref=f1e76]: warning
            - generic [ref=f1e77]:
              - strong [ref=f1e78]: "0"
              - generic [ref=f1e79]: Urgent tickets
              - generic [ref=f1e80]:
                - generic [aria-hidden] [ref=f1e81]: arrow_downward
                - emphasis [ref=f1e82]: PostgreSQL
          - article [ref=f1e83]:
            - generic [aria-hidden] [ref=f1e84]: task_alt
            - generic [ref=f1e85]:
              - strong [ref=f1e86]: "1"
              - generic [ref=f1e87]: Resolved or closed
              - generic [ref=f1e88]:
                - generic [aria-hidden] [ref=f1e89]: arrow_downward
                - emphasis [ref=f1e90]: PostgreSQL
        - generic [ref=f1e91]:
          - generic [ref=f1e92]:
            - generic [ref=f1e93]:
              - generic [aria-hidden] [ref=f1e94]: support_agent
              - generic [ref=f1e95]:
                - heading "Live case queue" [level=2] [ref=f1e96]
                - paragraph [ref=f1e97]: Customer names and order details are masked for support.
            - link "Open queue" [ref=f1e98] [cursor=pointer]:
              - /url: /support-portal/cases
          - region "Live support case queue" [ref=f1e99]:
            - generic [ref=f1e100]:
              - generic [ref=f1e101]: Case
              - generic [ref=f1e102]: Subject
              - generic [ref=f1e103]: Customer
              - generic [ref=f1e104]: Priority
              - generic [ref=f1e105]: Order
              - generic [ref=f1e106]: Status
              - generic [ref=f1e107]: Latest reply
              - generic [ref=f1e108]: Created
              - generic [ref=f1e109]: Update
            - generic [ref=f1e110]:
              - strong [ref=f1e111]:
                - link "WCDSPWCDEMO1004" [ref=f1e112] [cursor=pointer]:
                  - /url: /support-portal/cases?ticket_id=3
              - generic [ref=f1e113]: Return received
              - generic [ref=f1e114]: Kavita R.
              - generic [ref=f1e115]: normal
              - generic [ref=f1e116]: WCDEMO1004
              - generic [ref=f1e117]: resolved
              - generic [ref=f1e118]: 9 Oct 2026 10:14
              - generic [ref=f1e119]:
                - generic [ref=f1e120]: Status for WCDSPWCDEMO1004
                - combobox "Status for WCDSPWCDEMO1004" [ref=f1e121]:
                  - option "Resolved"
                  - option "Closed" [selected]
                - generic [ref=f1e122]: Internal note for WCDSPWCDEMO1004
                - textbox "Internal note for WCDSPWCDEMO1004" [ref=f1e123]:
                  - /placeholder: Internal note
                - button "Save" [ref=f1e124] [cursor=pointer]
            - generic [ref=f1e125]:
              - strong [ref=f1e126]:
                - link "WCDSPWCDEMO1003" [ref=f1e127] [cursor=pointer]:
                  - /url: /support-portal/cases?ticket_id=2
              - generic [ref=f1e128]: Tracking update requested
              - generic [ref=f1e129]: Arjun N.
              - generic [ref=f1e130]: normal
              - generic [ref=f1e131]: WCDEMO1003
              - generic [ref=f1e132]: in_progress
              - generic [ref=f1e133]: 9 Oct 2026 10:14
              - generic [ref=f1e134]:
                - generic [ref=f1e135]: Status for WCDSPWCDEMO1003
                - combobox "Status for WCDSPWCDEMO1003" [ref=f1e136]:
                  - option "In progress"
                  - option "Waiting for customer"
                  - option "Resolved" [selected]
                - generic [ref=f1e137]: Internal note for WCDSPWCDEMO1003
                - textbox "Internal note for WCDSPWCDEMO1003" [ref=f1e138]:
                  - /placeholder: Internal note
                - button "Save" [ref=f1e139] [cursor=pointer]
            - generic [ref=f1e140]:
              - strong [ref=f1e141]:
                - link "WCDSPWCDEMO1001" [ref=f1e142] [cursor=pointer]:
                  - /url: /support-portal/cases?ticket_id=1
              - generic [ref=f1e143]: Where is my payment link?
              - generic [ref=f1e144]: Priya S.
              - generic [ref=f1e145]: normal
              - generic [ref=f1e146]: WCDEMO1001
              - generic [ref=f1e147]: open
              - generic [ref=f1e148]: 9 Oct 2026 10:14
              - generic [ref=f1e149]:
                - generic [ref=f1e150]: Status for WCDSPWCDEMO1001
                - combobox "Status for WCDSPWCDEMO1001" [ref=f1e151]:
                  - option "Open"
                  - option "In progress"
                  - option "Waiting for customer"
                  - option "Resolved" [selected]
                - generic [ref=f1e152]: Internal note for WCDSPWCDEMO1001
                - textbox "Internal note for WCDSPWCDEMO1001" [ref=f1e153]:
                  - /placeholder: Internal note
                - button "Save" [ref=f1e154] [cursor=pointer]
        - generic [ref=f1e155]:
          - generic [ref=f1e156]:
            - generic [ref=f1e157]:
              - generic [aria-hidden] [ref=f1e158]: forum
              - generic [ref=f1e159]:
                - heading "Latest replies" [level=2] [ref=f1e160]
                - paragraph [ref=f1e161]: Recent support responses loaded from PostgreSQL.
            - link "Open a case" [ref=f1e162] [cursor=pointer]:
              - /url: /support-portal/cases
          - status [ref=f1e163]:
            - heading "No replies yet" [level=2] [ref=f1e164]
            - paragraph [ref=f1e165]: Replies sent to customers will appear here.
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
      |                                                                                                            ^ Error: support-agent accessing /account
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