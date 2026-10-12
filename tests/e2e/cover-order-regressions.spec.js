const { test, expect } = require('@playwright/test');

const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080');
const isolated = target.hostname === '127.0.0.1' && target.port === '18089' && process.env.WEEVCRAFTS_QA_DATABASE === 'cover_browser';
const passwords = { customer: 'CoverQA-Customer-2026!', seller: 'CoverQA-Seller-2026!', admin: 'CoverQA-Admin-2026!', superadmin: 'CoverQA-Super-2026!' };
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64');

test('HTML payment errors never become customer-facing code', async ({ page }) => {
  test.skip(!isolated, 'Requires the disposable QA server.');
  await page.goto(target.origin + '/login');
  await page.route('**/api/v1/payments/demo-capture/*', route => route.fulfill({ status: 404, contentType: 'text/html', body: '<!doctype html><html>Internal error page</html>' }));
  const message = await page.evaluate(async () => {
    const backend = await window.WeevcraftsBackend.create();
    try { await backend.localPayment('test-order', 'capture'); } catch (error) { return error.message; }
  });
  expect(message).toContain('try again');
  expect(message).not.toMatch(/doctype|Internal error|<html>/);
});

test('persistent covers, clean demo confirmation and synchronized fulfilment', async ({ browser }) => {
  test.skip(!isolated, 'Requires the disposable cover_browser database and 18089 server.');
  test.setTimeout(360000);
  const sessions = {};
  for (const role of Object.keys(passwords)) {
    const context = await browser.newContext({ baseURL: target.origin });
    const login = await context.request.post('/api/v1/auth/login', { data: { email: role + '@weevcrafts.test', password: passwords[role] } });
    expect(login.status()).toBe(200);
    sessions[role] = { context, page: await context.newPage(), token: (await login.json()).csrf_token };
  }
  const { seller, customer, admin, superadmin } = sessions;
  const suffix = Date.now().toString();
  const slug = 'persistent-cover-' + suffix;
  try {
    const created = await seller.context.request.post('/seller-admin/products', {
      maxRedirects: 0,
      multipart: { csrf_token: seller.token, name: 'Persistent cover QA', slug, brand_name: 'Weevcrafts', brand_slug: 'weevcrafts', category_name: 'Crafts', category_slug: 'crafts', description: 'A product used only in the isolated image persistence test.', sku: 'COVER-' + suffix, price_rupees: '199.00', initial_stock: '10', image_url: '', image_file: { name: 'cover.png', mimeType: 'image/png', buffer: png } }
    });
    expect(created.status(), await created.text()).toBe(303);
    const editURL = created.headers().location;
    const id = /products\/(\d+)\/edit/.exec(editURL)[1];
    await seller.page.goto(editURL);
    const preview = seller.page.locator('img[src^="/media/products/"]').first();
    await expect(preview).toHaveAttribute('src', /\/media\/products\/\d+\/preview$/);
    const previewURL = await preview.getAttribute('src');
    const publicURL = previewURL.replace('/preview', '');
    expect(await (await seller.context.request.get(previewURL)).body()).toEqual(png);
    expect((await customer.context.request.get(previewURL)).status()).toBe(404);
    expect((await customer.context.request.get(publicURL)).status()).toBe(404);
    await seller.page.reload();
    expect(await (await seller.context.request.get(previewURL)).body()).toEqual(png);
    expect((await superadmin.context.request.post('/super-admin/products/submit', { form: { csrf_token: superadmin.token, product_id: id }, maxRedirects: 0 })).status()).toBe(303);
    expect((await superadmin.context.request.post('/super-admin/products/review', { form: { csrf_token: superadmin.token, product_id: id, decision: 'approve' }, maxRedirects: 0 })).status()).toBe(303);
    expect(await (await customer.context.request.get(publicURL)).body()).toEqual(png);
    await customer.page.goto('/products/' + slug);
    await expect(customer.page.locator('img[src="' + publicURL + '"]').first()).toBeVisible();
    const added = customer.page.waitForResponse(response => response.url().endsWith('/api/v1/cart/add') && response.request().method() === 'POST');
    await customer.page.locator('.detail-actions').getByRole('button', { name: 'Add to cart', exact: true }).click();
    expect((await added).ok()).toBeTruthy();
    await customer.page.goto('/checkout');
    for (const [name, value] of Object.entries({ recipient_name: 'Cover QA Customer', phone: '9876543210', line1: '12 QA Road', city: 'Hyderabad', state: 'Telangana', postal_code: '500001' })) {
      await customer.page.locator('#checkout-address-form [name="' + name + '"]').fill(value);
    }
    await customer.page.getByRole('button', { name: /Continue to delivery/ }).click();
    await customer.page.getByRole('button', { name: /Continue to payment/ }).click();
    await customer.page.getByRole('button', { name: /Continue to review/ }).click();
    await customer.page.getByRole('button', { name: /Place order/, exact: false }).click();
    await expect(customer.page).toHaveURL(/\/orders\/[^/]+\/confirmation$/);
    await expect(customer.page.locator('.confirmation-summary')).toContainText('Payment completed');
    await expect(customer.page.locator('main')).not.toContainText('<!doctype');
    const number = /\/orders\/([^/]+)\/confirmation/.exec(customer.page.url())[1];
    const captureURL = '/api/v1/payments/demo-capture/' + number;
    expect((await customer.context.request.get(captureURL)).status()).toBe(405);
    expect((await customer.context.request.post(captureURL, { headers: { 'X-CSRF-Token': 'invalid' } })).status()).toBe(403);
    expect((await seller.context.request.post(captureURL, { headers: { 'X-CSRF-Token': seller.token } })).status()).toBe(404);
    await customer.page.goto('/orders/' + number + '/confirmation?notice=' + encodeURIComponent('<!doctype html><html>RAW HTML ERROR</html>'));
    await expect(customer.page.locator('main')).not.toContainText('RAW HTML ERROR');
    await expect(customer.page).not.toHaveURL(/notice=/);
    await customer.page.goto('/orders');
    await seller.page.goto('/seller-admin/orders/' + number);
    await admin.page.goto('/admin/orders');
    await superadmin.page.goto('/super-admin/orders');
    const editing = await seller.context.newPage(); await editing.goto('/seller-admin/orders/' + number);
    await editing.locator('[data-fulfillment-form] [name="note"]').fill('Unsaved packing note');
    const row = p => p.locator('[data-admin-orders-region] tr').filter({ hasText: number });
    await expect(row(admin.page)).toHaveCount(1);
    await expect(row(superadmin.page)).toHaveCount(1);
    const sellerList = await seller.context.newPage(); await sellerList.goto('/seller-admin/orders');
    await sellerList.locator('#seller-order-stage').selectOption('pending');
    await sellerList.getByRole('button', { name: 'Apply filter', exact: true }).click();
    await expect(sellerList).toHaveURL(/tab=pending/);
    const stages = [['processing','Processing'], ['packing','Packing'], ['shipped','Shipped'], ['out_for_delivery','Ready to deliver'], ['delivered','Delivered']];
    for (const [stage, label] of stages) {
      const form = seller.page.locator('[data-fulfillment-form]');
      if (stage === 'processing') {
        expect(await form.locator('[name="status"] option').evaluateAll(options => options.map(o => o.textContent))).toEqual(['New','Processing','Packing','Shipped','Ready to deliver','Delivered']);
      }
      await form.locator('[name="status"]').selectOption(stage);
      if (stage === 'shipped') {
        await form.locator('[name="carrier"]').selectOption('India Post');
        await form.locator('[name="tracking_number"]').fill('QA-TRACK-' + suffix);
      }
      await form.getByRole('button', { name: 'Save fulfilment' }).click();
      await expect(seller.page).toHaveURL(/notice=Fulfilment/);
      await expect(seller.page.locator('[data-seller-order-detail]')).toContainText(label);
      await expect(row(admin.page).locator('[data-label="Fulfilment stage"]')).toHaveText(label, { timeout: 20000 });
      await expect(row(superadmin.page).locator('[data-label="Fulfilment stage"]')).toHaveText(label, { timeout: 20000 });
      await expect(customer.page.locator('.orders-list')).toContainText(label, { timeout: 20000 });
      if (stage === 'processing') {
        await expect(editing.locator('[data-order-conflict]')).toBeVisible({ timeout: 20000 });
        await expect(editing.locator('[name="note"]')).toHaveValue('Unsaved packing note');
        await editing.close();
      }
    }
    await superadmin.page.getByRole('button', { name: 'Pause updates', exact: true }).focus();
    await superadmin.page.keyboard.press('Enter');
    await expect(superadmin.page.getByRole('button', { name: 'Resume updates', exact: true })).toBeFocused();
    await expect(superadmin.page.locator('.order-refresh-state')).toContainText('paused');
    await superadmin.page.getByRole('button', { name: 'Resume updates', exact: true }).click();
    await superadmin.page.route('**/ui/admin/orders/status*', route => route.fulfill({ status: 503, body: 'Temporary failure' }));
    await expect(superadmin.page.getByRole('button', { name: 'Retry update' })).toBeVisible({ timeout: 20000 });
    await expect(row(superadmin.page)).toHaveCount(1);
    await superadmin.page.unroute('**/ui/admin/orders/status*');
    await superadmin.page.getByRole('button', { name: 'Retry update' }).click();
    await expect(superadmin.page.getByRole('button', { name: 'Retry update' })).toBeHidden();
    await superadmin.page.locator('#order-stage-filter').selectOption('delivered');
    await superadmin.page.getByRole('button', { name: 'Apply filter', exact: true }).click();
    await expect(superadmin.page).toHaveURL(/stage=delivered/);
    await expect(row(superadmin.page)).toHaveCount(1);
    for (const width of [1440,390,320]) {
      for (const session of [customer, seller, superadmin]) {
        await session.page.emulateMedia({ reducedMotion: 'reduce' });
        await session.page.setViewportSize({ width, height: 844 });
        await expect(session.page.locator('main')).toBeVisible();
        expect(await session.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
        await session.page.screenshot({ path: test.info().outputPath(session === customer ? 'customer-' + width + '.png' : session === seller ? 'seller-' + width + '.png' : 'admin-' + width + '.png'), fullPage: true });
      }
    }
  } finally { for (const {context} of Object.values(sessions)) await context.close(); }
});
