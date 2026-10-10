# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: manual-matrix.spec.js >> GST-020--guest
- Location: tests\e2e\manual-matrix.spec.js:148:1

# Error details

```
Error: /category/missing-qa-1791542886060

expect(received).toBe(expected) // Object.is equality

Expected: 404
Received: 200
```

```
Error: /brands/missing-qa-1791542886060

expect(received).toBe(expected) // Object.is equality

Expected: 404
Received: 200
```

# Test source

```ts
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
  129 |       const response = await page.request.get(root, { maxRedirects: 0 });
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
> 152 |     expect.soft(response.status(), route).toBe(404);
      |                                           ^ Error: /brands/missing-qa-1791542886060
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