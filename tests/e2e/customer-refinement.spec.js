const {test, expect} = require('@playwright/test');
const {execFileSync} = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const base = process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18083';
const database = 'customer_workflows_20261017';
const password = 'Customer-Workflow-2026!';
const product = 'customer-workflow-piece';
const output = path.join(os.tmpdir(), 'weev-customer-refinement-images');
function guard() {
  const url = new URL(base);
  if (process.env.WEEVCRAFTS_QA_DATABASE !== database || url.hostname !== '127.0.0.1' || url.port !== '18083') throw new Error('Customer fixtures require the dedicated database and loopback port 18083.');
}
function sql(query) {
  guard();
  return execFileSync('docker', ['exec', 'weevcrafts-storefront-qa-db', 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'storefrontqa', '-d', database, '-tAc', query], {encoding: 'utf8'}).trim();
}
const email = kind => `customer.workflow.${kind}@example.invalid`;
async function login(page, kind = 'journey') {
  expect((await page.request.post('/api/v1/auth/login', {data: {email: email(kind), password}})).ok()).toBeTruthy();
}
async function ready(page) {
  try {
    await page.waitForFunction(() => window.weevcraftsBackend && window.WVUI, null, {timeout: 45000});
  } catch (error) {
    const state = await page.evaluate(() => ({title:document.title, scripts:!!window.WVUI, backendError:window.weevcraftsBackendError?.message}));
    throw new Error('Customer scripts did not initialize at '+page.url()+': '+JSON.stringify(state));
  }
  await page.evaluate(() => document.fonts.ready);
}
async function screenshot(page, name) { await page.screenshot({path: path.join(output, name + '.png'), fullPage: true}); }
async function checkoutReview(page) {
  await page.goto('/checkout'); await ready(page);
  const form = page.locator('#checkout-address-form');
  if (await page.locator('[data-checkout-address-select]').count()) await page.locator('[data-checkout-address-select]').selectOption('');
  for (const [name, value] of Object.entries({recipient_name: 'Customer workflow QA', phone: '+919876543210', line1: '12 Craft Lane', city: 'Pune', state: 'Maharashtra', postal_code: '411001'})) await form.locator(`[name=${name}]`).fill(value);
  await page.getByRole('button', {name: 'Continue to delivery'}).click();
  await page.getByRole('button', {name: 'Continue to payment'}).click();
  await page.getByRole('button', {name: 'Continue to review'}).click();
}
async function addItem(page) {
  await page.goto('/products/' + product); await ready(page);
  await page.locator('#detail-variant').selectOption(sql("SELECT id FROM product_variants WHERE sku='CUSTOMER-QA-BLUE'"));
  await page.locator('.detail-actions').getByRole('button', {name: 'Add to cart', exact: true}).click();
  await expect(page).toHaveURL(/\/cart$/);
}
async function placeOrder(page, outcome = 'capture') {
  if (outcome !== 'capture') await page.route('**/api/v1/payments/demo-capture/*', async route => {
    const number = new URL(route.request().url()).pathname.split('/').pop();
    const response = await route.fetch({url: base + '/dev/payments/' + number + '/' + outcome});
    await route.fulfill({response});
  }, {times: 1});
  await page.getByRole('button', {name: 'Place order', exact: true}).click();
  await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation$/, {timeout: 30000});
  return decodeURIComponent(new URL(page.url()).pathname.split('/')[2]);
}

test.beforeAll(async ({request}) => {
  guard(); await fs.mkdir(output, {recursive: true});
  for (const kind of ['journey', 'queue', 'failure', 'profile', 'security', 'retry']) {
    if (sql("SELECT count(*) FROM users WHERE email='" + email(kind) + "'") === '0') {
      const response = await request.post(base + '/api/v1/auth/register', {data: {email: email(kind), password, display_name: 'Customer workflow ' + kind, phone: '+919876543210', accepted_terms: true}});
      expect(response.ok()).toBeTruthy();
    }
  }
  sql(`INSERT INTO products(slug,brand_slug,category_slug,name,description,price_cents,seller_id,status,image_url)
    SELECT '${product}',(SELECT slug FROM brands LIMIT 1),(SELECT slug FROM categories LIMIT 1),'Customer workflow ceramic mugs','Ceramic mugs for isolated workflow verification.',89900,id,'approved','/assets/images/customer/ceramic-mugs.webp'
    FROM sellers WHERE owner_user_id=(SELECT id FROM users WHERE email='demo.owner01@weevcrafts.local') ON CONFLICT(slug) DO NOTHING;
    INSERT INTO product_variants(product_id,sku,name,price_cents,status)
    SELECT id,v.sku,v.name,89900,'active' FROM products CROSS JOIN (VALUES('CUSTOMER-QA-BLUE','Blue pair'),('CUSTOMER-QA-SAND','Sand pair'))v(sku,name) WHERE slug='${product}' ON CONFLICT(sku) DO NOTHING;
    INSERT INTO inventory_stock(variant_id,available_quantity) SELECT id,100 FROM product_variants WHERE sku LIKE 'CUSTOMER-QA-%' ON CONFLICT DO NOTHING;
    INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key,created_at)
    SELECT 'WC-CUSTOMER-QUEUE-'||n,(SELECT id FROM users WHERE email='${email('queue')}'),CASE WHEN n<=45 THEN 'paid' WHEN n<=50 THEN 'delivered' ELSE 'cancelled' END,89900,89900,'{}','customer-queue',n::text,now()-n*interval '1 minute' FROM generate_series(1,55)n ON CONFLICT DO NOTHING;
    INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) SELECT 'WC-CUSTOMER-PRIVATE',id,'paid',89900,89900,'{}','customer-private','1' FROM users WHERE email='${email('profile')}' ON CONFLICT DO NOTHING;
    UPDATE orders SET status='pending_payment' WHERE order_number='WC-CUSTOMER-QUEUE-1' AND user_id=(SELECT id FROM users WHERE email='${email('queue')}');
    INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents)
    SELECT o.id,v.id,'Customer workflow ceramic mugs',v.sku,89900,1,89900 FROM orders o CROSS JOIN product_variants v WHERE o.order_number LIKE 'WC-CUSTOMER-QUEUE-%' AND v.sku='CUSTOMER-QA-BLUE' AND NOT EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id);`);
});

test('order history filters before paging, retains navigation, and never invents payment results', async ({page, browserName}) => {
  test.skip(browserName !== 'chromium', 'Business mutations run once against isolated fixtures.');
  await login(page, 'queue'); await page.goto('/orders'); await ready(page);
  await expect(page.locator('[data-server-orders] .order-card')).toHaveCount(20);
  const sidebar = await page.locator('.account-sidebar').boundingBox();
  const queue = await page.locator('[data-server-orders]').boundingBox();
  expect(queue.x).toBeGreaterThanOrEqual(sidebar.x + sidebar.width);
  expect(queue.y).toBeLessThan(sidebar.y + 150);
  await page.locator('[data-server-orders]').evaluate(element => element.dataset.qaRefresh = 'old');
  await page.getByRole('button',{name:'Pause updates',exact:true}).click();
  await page.getByRole('button',{name:'Resume updates',exact:true}).click();
  await expect(page.locator('[data-server-orders]')).not.toHaveAttribute('data-qa-refresh','old');
  const cancel = page.locator('[data-server-orders]').getByRole('button',{name:'Cancel order',exact:true});
  await cancel.click(); await expect(page.getByRole('dialog')).toBeVisible(); await page.keyboard.press('Escape'); await expect(cancel).toBeFocused();

  await expect(page.locator('.order-result-summary')).toHaveText('Showing 1–20 of 55 orders');
  await expect(page.getByRole('link', {name: 'All orders (55)', exact: true})).toBeVisible();
  await page.getByRole('link', {name: 'Next page', exact: true}).click();
  await expect(page.locator('.order-result-summary')).toHaveText('Showing 21–40 of 55 orders');
  await page.locator('.compact-order a').first().click();
  await expect(page.getByRole('link', {name: 'Back to orders', exact: true})).toHaveAttribute('href', '/orders?page=2');
  await page.getByRole('link', {name: 'Back to orders', exact: true}).click(); await expect(page).toHaveURL(/page=2/);
  await page.getByRole('link', {name: 'Delivered (5)', exact: true}).click();
  await expect(page.locator('[data-server-orders] .order-card')).toHaveCount(5);
  await page.goto('/orders?status=delivered&page=999'); await expect(page.locator('.order-result-summary')).toHaveText('Showing 1–5 of 5 orders');
  for (const state of ['success', 'failed', 'pending']) {
    await page.goto('/payments/' + state); await expect(page).toHaveURL(base + '/payments');
    await expect(page.locator('.payment-state-links')).toHaveCount(0);
    await expect(page.locator('#page')).not.toContainText('#PAY256');
  }
  await page.goto('/payments/success?order=WC-CUSTOMER-QUEUE-55');
  await expect(page).toHaveURL(/WC-CUSTOMER-QUEUE-55\/confirmation/);
  await expect(page.locator('.confirmation-summary')).toContainText('Order cancelled');
  expect((await page.request.get('/payments/success?order=DOES-NOT-EXIST')).status()).toBe(404);
});

test('shopping and checkout keep chosen options, prevent duplicates, and deliver an owned order', async ({page, browser, browserName}) => {
  test.skip(browserName !== 'chromium');
  await login(page); await page.goto('/cart'); await ready(page);
  await page.evaluate(({slug,id}) => window.weevcraftsBackend.removeCart(slug,id), {slug:product,id:sql("SELECT id FROM product_variants WHERE sku='CUSTOMER-QA-BLUE'")});
  await page.goto('/products/' + product); await ready(page);
  const heart = page.locator('.gallery-heart');
  const saved = await heart.getAttribute('aria-pressed');
  await page.route('**/api/v1/wishlist/*', route => route.request().method() === 'POST' ? route.fulfill({status: 503, contentType: 'application/json', body: '{"error":{"message":"Temporary QA wishlist failure"}}'}) : route.continue());
  if (saved === 'false') { await heart.click(); await expect(heart).toHaveAttribute('aria-pressed', 'false'); await expect(heart).toBeEnabled(); }
  await page.unroute('**/api/v1/wishlist/*');
  if (await heart.getAttribute('aria-pressed') === 'false') await heart.click();
  await expect(heart).toHaveAttribute('aria-pressed', 'true');
  await page.goto('/wishlist'); await ready(page); await expect(page.locator('#wishlist-grid')).toContainText('Customer workflow ceramic mugs');
  await page.goto('/products?q=Customer+workflow'); await ready(page);
  const quickLink = page.locator(`[data-product="${product}"] [data-quick-view]`);
  await quickLink.click(); const dialog = page.locator('#product-quick-view');
  await expect(dialog.locator('[data-quick-add]')).toBeDisabled();
  await dialog.locator('[data-quick-variant]').selectOption(sql("SELECT id FROM product_variants WHERE sku='CUSTOMER-QA-BLUE'"));
  let additions = 0;
  await page.route('**/api/v1/cart/add', async route => { additions++; await new Promise(resolve => setTimeout(resolve, 200)); await route.continue(); });
  await dialog.locator('[data-quick-add]').evaluate(button => {button.click(); button.click();});
  await expect(page.locator('#cart-drawer')).toBeVisible(); expect(additions).toBe(1);
  await expect(page.locator('[data-cart-drawer-items]')).toContainText('Blue pair');
  await page.keyboard.press('Escape'); await expect(quickLink).toBeFocused();
  await page.goto('/cart'); await ready(page);
  const row = page.locator('#cart-layout .cart-line').filter({hasText: 'Customer workflow ceramic mugs'});
  await expect(row).toContainText('Blue pair');
  await row.getByRole('button', {name: 'Increase Customer workflow ceramic mugs quantity', exact: true}).click(); await expect(row.locator('output')).toHaveText('2');
  await checkoutReview(page);
  await page.getByRole('button', {name: 'Edit address', exact: true}).click();
  await expect(page.locator('[name=line1]')).toHaveValue('12 Craft Lane');
  await page.locator('[name=line1]').fill('14 Craft Lane');
  await page.getByRole('button', {name: 'Continue to delivery'}).click();
  await page.getByRole('button', {name: 'Back to address'}).press('Enter'); await expect(page.locator('[data-step="1"] h2')).toBeFocused();
  await page.getByRole('button', {name: 'Continue to delivery'}).click(); await page.getByRole('button', {name: 'Continue to payment'}).click(); await page.getByRole('button', {name: 'Continue to review'}).click();
  await expect(page.locator('[data-checkout-address-summary]')).toContainText('14 Craft Lane');
  await page.setViewportSize({width: 390, height: 844}); await screenshot(page, 'checkout-390');
  const number = await placeOrder(page); await expect(page.locator('.confirmation-summary')).toContainText('Payment completed');
  await page.reload(); await expect(page.locator('.confirmation-summary')).toContainText(number);
  await page.getByRole('link', {name: 'View order', exact: true}).click();
  const before = sql(`SELECT count(*) FROM orders WHERE user_id=(SELECT id FROM users WHERE email='${email('journey')}')`);
  const sellerContext = await browser.newContext({baseURL: base}); const seller = await sellerContext.newPage();
  expect((await seller.request.post('/api/v1/auth/login', {data: {email: 'demo.owner01@weevcrafts.local', password: 'WeeVCrafts-Demo-2026!'}})).ok()).toBeTruthy();
  await seller.goto('/seller-admin/orders/' + number);
  for (const action of ['Start processing', 'Mark as packed', 'Mark as shipped', 'Mark ready to deliver', 'Mark as delivered']) {
    if (action === 'Mark as shipped') {await seller.locator('[name=carrier]').selectOption('Other'); await seller.locator('[name=custom_carrier]').fill('Customer QA courier'); await seller.locator('[name=tracking_number]').fill('CUSTOMER-QA-TRACK');}
    await seller.getByRole('button', {name: action, exact: true}).click();
    await expect(seller.locator('[data-seller-order-detail]')).toBeVisible();
  }
  await sellerContext.close();
  await expect(page.locator('.order-detail-status')).toContainText('arrived', {timeout: 25000});
  expect(sql(`SELECT count(*) FROM orders WHERE user_id=(SELECT id FROM users WHERE email='${email('journey')}')`)).toBe(before);
  await page.getByRole('link', {name: 'Track order', exact: true}).click(); await expect(page.locator('.shipment-card')).toContainText('CUSTOMER-QA-TRACK');
  await page.goto('/orders/' + number + '/confirmation'); await expect(page.locator('.confirmation-progress li.done')).toHaveCount(4);
  await page.goto('/orders/' + number + '/invoice');
  const pdf = await page.request.get('/orders/' + number + '/invoice?format=pdf'); expect(pdf.headers()['content-type']).toContain('application/pdf');
  await page.goto('/account/reviews');
  const review = page.locator('[data-review-product="'+sql("SELECT id FROM products WHERE slug='"+product+"'")+'"] form').first();
  if (await review.count()) {
    await review.locator('[name=rating]').selectOption('5'); await review.locator('[name=title]').fill('Careful handmade finish'); await review.locator('[name=body]').fill('The mugs arrived safely with a beautiful handmade finish.');
    await review.getByRole('button', {name: 'Submit verified review'}).click();
    await expect(page).toHaveURL(/\/account\/reviews\?notice=Review/);await ready(page);
  }
  await expect(page.locator('.review-list').last()).toContainText('pending');
  await page.goto('/orders/' + number); await page.locator('[name=reason]').fill('The mugs do not fit our shelves. Please review this return request.');
  await page.getByRole('button', {name: 'Submit return request', exact: true}).click(); await expect(page.locator('.order-return-card')).toContainText('Return request');
  await page.goto('/returns'); await expect(page.locator('.returns-main')).toContainText(number);
  await page.goto('/orders/' + number); await page.getByRole('link', {name: 'Need help?', exact: true}).click();
  await expect(page.locator('[name=order_number]')).toHaveValue(number);
  const support = page.locator('[data-customer-support-create]'); await support.locator('[name=subject]').fill('Customer QA order support'); await support.locator('[name=message]').fill('Please help me follow the return request for this order.');
  await page.route('**/support/create', route => route.fulfill({status: 503, contentType: 'text/plain', body: 'Temporary support failure. Try again.'}));
  await support.locator('[name="attachments[]"]').setInputFiles(path.join(__dirname, '../../web/assets/images/brand/favicon.png'));
  await support.getByRole('button', {name: 'Send support request'}).click(); await expect(support.locator('.ui-form-error')).toContainText('Temporary support failure'); await expect(support.locator('[name=message]')).toHaveValue('Please help me follow the return request for this order.');
  await page.unroute('**/support/create'); await support.getByRole('button', {name: 'Send support request'}).click(); await expect(page).toHaveURL(/\/account\/support/);
  const ticket = page.locator('.customer-support-ticket').filter({hasText: 'Customer QA order support'}).first();
  const attachment = await ticket.locator('.support-attachments a').first().getAttribute('href');expect((await page.request.get(attachment)).ok()).toBeTruthy();
  const stranger=await browser.newContext({baseURL:base});const strangerPage=await stranger.newPage();await login(strangerPage,'queue');expect((await stranger.request.get(attachment)).status()).toBe(404);await stranger.close();
  await ticket.locator('[name=message]').fill('Please also confirm the expected response time.'); await ticket.getByRole('button', {name: 'Send reply'}).click(); await expect(page).toHaveURL(/\/account\/support\?ticket_id=/); await expect(page.locator('.customer-support-ticket').filter({hasText:'Customer QA order support'}).first().locator('.customer-support-messages')).toContainText('expected response time');
  await screenshot(page, 'support-390');
});

test('payment interruption retries existing order; failed and cancelled orders stay distinct', async ({page, browserName}) => {
  test.skip(browserName !== 'chromium'); await login(page, 'failure'); await addItem(page); await checkoutReview(page);
  const pending = await placeOrder(page, 'pending'); await expect(page.locator('h1')).toHaveText('Your order is saved');
  await page.getByRole('link', {name: 'View order', exact: true}).click(); await page.getByRole('button', {name: 'Continue payment', exact: true}).click();
  await expect(page).toHaveURL(new RegExp('/orders/' + pending + '/confirmation')); await expect(page.locator('.confirmation-summary')).toContainText('Payment completed');
  await addItem(page); await checkoutReview(page); const failed = await placeOrder(page, 'fail'); await expect(page.locator('.confirmation-summary')).toContainText('Payment failed');
  await page.goto('/orders/' + failed); await expect(page.getByRole('button', {name: 'Continue payment', exact: true})).toHaveCount(0);
  await addItem(page); await checkoutReview(page); const cancelled = await placeOrder(page, 'pending'); await page.goto('/orders/' + cancelled);
  await page.getByRole('button', {name: 'Cancel order', exact: true}).click(); await expect(page.getByRole('dialog')).toBeVisible(); await page.getByRole('button', {name: 'Keep editing', exact: true}).count().then(async count => {if (count) await page.getByRole('button', {name: 'Keep editing', exact: true}).click(); else await page.keyboard.press('Escape');});
  await page.getByRole('button', {name: 'Cancel order', exact: true}).click(); await page.getByRole('dialog').getByRole('button', {name: 'Cancel order', exact: true}).click();
  await expect(page.locator('.order-detail-status')).toContainText('cancelled');
});

test('profile, addresses, preferences and privacy retain entries and protect ownership', async ({page, browser, browserName}) => {
  test.skip(browserName !== 'chromium'); await login(page, 'profile'); await page.goto('/account/profile'); await ready(page);
  const form = page.locator('[data-profile-form]'); await form.locator('[name=name]').fill('Customer QA profile saved'); await form.locator('[name=phone]').fill('+919876543210');
  await page.getByRole('link', {name: 'Addresses', exact: true}).click(); await expect(page.getByRole('dialog')).toBeVisible(); await page.getByRole('button', {name: 'Keep editing', exact: true}).click();
  await form.locator('[name=photo]').setInputFiles(path.join(__dirname, '../../web/assets/images/brand/favicon.png'));
  await form.getByRole('button', {name: 'Save changes', exact: true}).click(); await expect(page.locator('.utility-notice')).toContainText('Profile updated');
  const photo = await page.request.get('/account/profile/photo'); expect(photo.ok()).toBeTruthy(); expect(photo.headers()['cache-control']).toContain('private');
  await page.goto('/account/addresses'); const address = page.locator('[data-address-create]');
  for (const [name, value] of Object.entries({recipient_name: 'Customer address QA', phone: '+919876543210', line1: '18 Long Street', city: 'Pune', state: 'Maharashtra', postal_code: '411001'})) await address.locator(`[name=${name}]`).fill(value);
  await page.route('**/account/addresses', route => route.request().method() === 'POST' ? route.fulfill({status: 422, contentType: 'text/plain', body: 'Check this address and retry.'}) : route.continue());
  await address.getByRole('button', {name: 'Add address', exact: true}).click(); await expect(address.locator('.ui-form-error')).toContainText('Check this address'); await expect(address.locator('[name=line1]')).toHaveValue('18 Long Street');
  await page.unroute('**/account/addresses'); await address.getByRole('button', {name: 'Add address', exact: true}).click(); await expect(page).toHaveURL(/\/account\/addresses\?notice=Address\+saved/); await ready(page); await expect(page.locator('.saved-address-list')).toContainText('Customer address QA');
  const deleteButton = page.locator('[data-address-delete]').first().getByRole('button', {name: 'Delete address'}); await deleteButton.click(); await expect(page.getByRole('dialog')).toBeVisible(); await page.keyboard.press('Escape'); await expect(deleteButton).toBeFocused();
  await page.goto('/account/notifications'); const preferences = page.locator('form[action="/account/notifications"]'); await preferences.locator('[name=offers]').uncheck(); await preferences.getByRole('button', {name: 'Save preferences'}).click(); await page.reload(); await expect(preferences.locator('[name=offers]')).not.toBeChecked();
  await page.goto('/account/privacy'); await page.getByRole('link', {name: 'Manage privacy choices and data requests'}).click(); await expect(page.locator('h1')).toBeVisible();
  const other = await browser.newContext({baseURL: base}); const otherPage = await other.newPage(); await login(otherPage, 'queue');
  const privateNumber = 'WC-CUSTOMER-PRIVATE';
  for (const route of ['/orders/' + privateNumber, '/orders/' + privateNumber + '/invoice?format=pdf', '/payments/success?order=' + privateNumber, '/contact?order=' + privateNumber]) expect((await other.request.get(route)).status(), route).toBe(404);
  await other.close();
});

test('responsive customer routes, keyboard drawers, filters and reduced motion', async ({page, browserName}) => {
  await login(page); const errors = []; page.on('pageerror', error => errors.push(error.message));
  const routes = ['/', '/products', '/products/' + product, '/wishlist', '/cart', '/checkout', '/orders', '/tracking', '/returns', '/payments', '/account', '/account/profile', '/account/addresses', '/account/notifications', '/notifications', '/account/security', '/account/sessions', '/account/privacy', '/privacy/center', '/account/reviews', '/account/support', '/contact'];
  const widths = browserName === 'chromium' ? [320, 390, 768, 1024, 1440] : [390, 1440];
  for (const width of widths) {
    await page.setViewportSize({width, height: width < 800 ? 844 : 1000});
    for (const route of routes) {
      const response = await page.goto(route);
      expect(response.status(), route).toBeLessThan(400); await ready(page);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${route} overflow at ${width}`).toBeTruthy();
      if (['/account', '/orders', '/account/support'].includes(route) && [390, 1440].includes(width)) await screenshot(page, `${browserName}-${route.replaceAll('/', '-')}-${width}`);
    }
  }
  await page.setViewportSize({width: 390, height: 844}); await page.goto('/account'); await ready(page);
  const accountMenu = page.locator('[data-account-navigation] summary'); await accountMenu.press('Enter'); await expect(page.getByRole('navigation', {name: 'Account sections'})).toBeVisible(); await accountMenu.press('Enter');
  const menu = page.locator('.menu-toggle'); await menu.press('Enter'); await expect(page.locator('#mobile-menu')).toBeVisible(); await page.keyboard.press('Escape'); await expect(menu).toBeFocused();
  await page.emulateMedia({reducedMotion: 'reduce'}); await page.goto('/products?q=impossible-customer-query'); await expect(page.getByRole('heading', {name: 'No matching pieces'})).toBeVisible();
  await page.setViewportSize({width: 844, height: 390}); await page.goto('/account/security'); await ready(page); expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
  await page.setViewportSize({width: 720, height: 500}); await page.goto('/orders'); await ready(page); expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
  await page.addScriptTag({path: require.resolve('axe-core')});
  const violations = await page.evaluate(async () => (await axe.run(document.querySelector('#page'), {runOnly: {type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']}})).violations.map(item => item.id));
  expect(violations).toEqual([]); expect(errors).toEqual([]);
});

test('security verifies current password, CSRF and session revocation', async ({page, browser, browserName}) => {
  test.skip(browserName !== 'chromium'); await login(page, 'security'); const other = await browser.newContext({baseURL: base}); const otherPage = await other.newPage(); await login(otherPage, 'security');
  await page.goto('/account/security'); await ready(page); const form = page.locator('[data-customer-password]'); const csrf = await form.locator('[name=csrf_token]').inputValue();
  const submit = (current, token) => page.request.post('/account/password', {headers: {Accept: 'application/json'}, form: {csrf_token: token, current_password: current, new_password: 'Customer-Changed-2026!', confirm_password: 'Customer-Changed-2026!'}});
  expect((await submit(password, 'invalid')).status()).toBe(403); expect((await submit('wrong-password', csrf)).status()).toBe(422);
  expect((await submit(password, csrf)).ok()).toBeTruthy(); await otherPage.goto('/account'); await expect(otherPage).toHaveURL(/\/login/); await other.close();
  expect((await page.request.post('/api/v1/auth/login',{data:{email:email('security'),password:'Customer-Changed-2026!'}})).ok()).toBeTruthy();await page.goto('/account/security');const nextToken=await page.locator('[data-customer-password] [name=csrf_token]').inputValue();
  expect((await page.request.post('/account/password',{headers:{Accept:'application/json'},form:{csrf_token:nextToken,current_password:'Customer-Changed-2026!',new_password:password,confirm_password:password}})).ok()).toBeTruthy();
});


test('unavailable payments and storage provide honest, saved-record recovery', async ({browser, browserName}) => {
  test.skip(browserName !== 'chromium' || process.env.WEEVCRAFTS_FAILURE_BASE_URL !== 'http://127.0.0.1:18084', 'Requires the second isolated app with unavailable integrations.');
  const context = await browser.newContext({baseURL: process.env.WEEVCRAFTS_FAILURE_BASE_URL});
  const page = await context.newPage(); await login(page, 'profile');
  await page.goto('/products/' + product); await ready(page);
  const variant = sql("SELECT id FROM product_variants WHERE sku='CUSTOMER-QA-SAND'");
  await page.evaluate(({slug,id}) => window.weevcraftsBackend.addCart(slug,1,id), {slug:product,id:variant});
  await page.goto('/checkout'); await ready(page);
  const form = page.locator('#checkout-address-form');
  for (const [name,value] of Object.entries({recipient_name:'Unavailable QA',phone:'+919876543210',line1:'19 Craft Lane',city:'Pune',state:'Maharashtra',postal_code:'411001'})) await form.locator(`[name=${name}]`).fill(value);
  await page.getByRole('button',{name:'Continue to delivery'}).click();await page.getByRole('button',{name:'Continue to payment'}).click();
  await expect(page.getByRole('button',{name:'Payment unavailable',exact:true})).toBeDisabled();await expect(page.locator('[data-step="3"]')).toContainText('Your cart is saved');
  await page.goto('/contact'); await ready(page); const support=page.locator('[data-customer-support-create]');const subject='Saved support after media failure '+Date.now();
  await support.locator('[name=subject]').fill(subject);await support.locator('[name=message]').fill('This request must remain saved when object storage fails.');await support.locator('[name="attachments[]"]').setInputFiles(path.join(__dirname,'../../web/assets/images/brand/favicon.png'));
  await support.getByRole('button',{name:'Send support request'}).click();await expect(page).toHaveURL(/\/account\/support\?ticket_id=/);
  await expect(page.locator('.utility-notice')).toContainText('Your request was saved');expect(sql("SELECT count(*) FROM support_tickets WHERE subject='"+subject+"'")).toBe('1');
  await expect(page.locator('.customer-support-ticket')).toContainText(subject);
  await context.close();
});


test('stock changes, uncertain checkout completion and session expiry retain safe work', async ({page, browserName}) => {
  test.skip(browserName !== 'chromium'); await login(page,'retry');await addItem(page);await checkoutReview(page);
  const id=sql("SELECT id FROM product_variants WHERE sku='CUSTOMER-QA-BLUE'");const available=sql('SELECT available_quantity FROM inventory_stock WHERE variant_id='+id);
  const before=sql("SELECT count(*) FROM orders WHERE user_id=(SELECT id FROM users WHERE email='"+email('retry')+"')");
  try {
    sql('UPDATE inventory_stock SET available_quantity=0 WHERE variant_id='+id);
    await page.getByRole('button',{name:'Place order',exact:true}).click();
    await expect(page.getByRole('button',{name:'Place order',exact:true})).toBeEnabled();
    expect(sql("SELECT count(*) FROM orders WHERE user_id=(SELECT id FROM users WHERE email='"+email('retry')+"')")).toBe(before);
    await expect(page.locator('[name=line1]')).toHaveValue('12 Craft Lane');
  } finally {sql('UPDATE inventory_stock SET available_quantity='+available+' WHERE variant_id='+id);}
  await page.route('**/api/v1/checkout',async route=>{await route.fetch();await route.abort('connectionfailed');},{times:1});
  await page.getByRole('button',{name:'Place order',exact:true}).click();await expect(page.getByRole('button',{name:'Place order',exact:true})).toBeEnabled();
  const number=await placeOrder(page);await expect(page.locator('.confirmation-summary')).toContainText('Payment completed');
  expect(Number(sql("SELECT count(*) FROM orders WHERE user_id=(SELECT id FROM users WHERE email='"+email('retry')+"')"))).toBe(Number(before)+1);
  expect(sql("SELECT count(*) FROM addresses WHERE user_id=(SELECT id FROM users WHERE email='"+email('retry')+"') AND line1='12 Craft Lane'")).toBe('1');
  await page.goto('/contact?order='+number);await ready(page);const form=page.locator('[data-customer-support-create]');await form.locator('[name=subject]').fill('Retained after session expiry');await form.locator('[name=message]').fill('This draft remains in the page while I restore my session.');
  await page.context().clearCookies();await form.getByRole('button',{name:'Send support request'}).click();await expect(form.locator('.ui-form-error')).toContainText('Your session ended');
  await expect(form.locator('[name=message]')).toHaveValue('This draft remains in the page while I restore my session.');await expect(form.getByRole('link',{name:'Sign in again'})).toBeVisible();
});


test('returned orders enable one local refund only after operations receives them', async ({page,browser,browserName}) => {
  test.skip(browserName!=='chromium');await login(page);
  const number=sql("SELECT o.order_number FROM orders o JOIN return_requests r ON r.order_id=o.id WHERE o.user_id=(SELECT id FROM users WHERE email='"+email('journey')+"') AND r.status='pending' ORDER BY r.id DESC LIMIT 1");expect(number).not.toBe('');
  await page.goto('/orders/'+number);await expect(page.getByRole('button',{name:'Request full refund',exact:true})).toHaveCount(0);
  const operations=await browser.newContext({baseURL:base});const admin=await operations.newPage();
  expect((await admin.request.post('/api/v1/auth/login',{data:{email:'demo.marketplace01@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}})).ok()).toBeTruthy();
  await admin.goto('/admin/returns?q='+encodeURIComponent(number));const item=admin.locator('.portal-return-item').filter({hasText:number});await item.locator('[name=status]').selectOption('approved');await item.locator('[name=reason]').fill('Approved isolated QA return.');await item.getByRole('button',{name:'Save decision',exact:true}).click();
  await item.getByRole('button',{name:'Mark returned order received',exact:true}).click();await expect(item.locator('.ui-status')).toContainText('Received');await operations.close();
  await page.reload();await page.locator('#refund-reason').fill('The returned mugs were received by operations. Please complete the refund.');
  await page.getByRole('button',{name:'Request full refund',exact:true}).click();await expect(page.locator('.order-return-card[role=status]')).toContainText('processed');
  expect(sql("SELECT count(*) FROM payment_refunds WHERE order_id=(SELECT id FROM orders WHERE order_number='"+number+"')")).toBe('1');
  await page.goto('/orders/'+number+'/confirmation');await expect(page.locator('.confirmation-summary')).toContainText('Payment refunded');await expect(page.locator('.confirmation-progress li.done')).toHaveCount(4);
});

test('quick view retries failed reads and supports native selection and keyboard dismissal',async({page})=>{
  await login(page);await page.setViewportSize({width:390,height:844});await page.goto('/products?q=Customer+workflow');await ready(page);
  const link=page.locator('[data-product="'+product+'"] [data-quick-view]');
  await page.route('**/products/'+product,route=>route.request().headers()['hx-target']==='product-quick-view-content'?route.fulfill({status:503,body:'Temporary quick view failure'}):route.continue());
  await link.press('Enter');const dialog=page.locator('#product-quick-view');await expect(dialog.getByRole('button',{name:'Try again',exact:true})).toBeVisible();
  await page.unroute('**/products/'+product);await dialog.getByRole('button',{name:'Try again',exact:true}).click();const select=dialog.locator('[data-quick-variant]');await expect(select).toBeVisible();
  await select.focus();await select.press('ArrowDown');await select.press('Enter');await expect(select).not.toHaveValue('');await expect(dialog.locator('[data-quick-add]')).toBeEnabled();
  await page.keyboard.press('Escape');
  // A native select popup consumes Escape before its containing dialog.
  if(await dialog.isVisible())await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();await expect(link).toBeFocused();
  const filter=page.locator('[data-filter-open]');if(await filter.count()){await filter.press('Enter');await page.keyboard.press('Escape');await expect(filter).toBeFocused();}
});


test('privacy requests, consent and deliberate deletion work for an isolated customer',async({page,browserName})=>{
  test.skip(browserName!=='chromium');const kind='privacy-'+Date.now();
  expect((await page.request.post('/api/v1/auth/register',{data:{email:email(kind),password,display_name:'Privacy QA customer',phone:'+919876543210',accepted_terms:true}})).ok()).toBeTruthy();await login(page,kind);await page.goto('/privacy/center');await ready(page);
  const request=page.locator('form[action="/privacy/requests"]');await request.locator('[name=request_type]').selectOption('access');await request.locator('[name=details]').fill('Please provide access to my isolated QA account data.');await request.getByRole('button',{name:'Submit request',exact:true}).click();await expect(page.locator('.saved-address').filter({hasText:'access · open'}).first()).toBeVisible();
  const consent=page.locator('form[action="/privacy/consent"]').filter({has:page.locator('[name=purpose][value=analytics]')});await consent.getByRole('button').click();await expect(page.getByText('analytics · granted',{exact:true})).toBeVisible();
  await request.locator('[name=request_type]').selectOption('deletion');await request.locator('[name=details]').fill('Delete this disposable QA account and its eligible data.');await request.getByRole('button',{name:'Submit request',exact:true}).click();
  const deletion=page.getByRole('button',{name:'Delete eligible data',exact:true});await expect(deletion).toBeVisible();await deletion.click();await expect(page.getByRole('dialog')).toBeVisible();await page.keyboard.press('Escape');await expect(deletion).toBeFocused();
  await deletion.click();await page.getByRole('dialog').getByRole('button',{name:'Delete eligible data',exact:true}).click();await expect(page).toHaveURL(/\/login/);
  expect((await page.request.post('/api/v1/auth/login',{data:{email:email(kind),password}})).ok()).toBeFalsy();
});
