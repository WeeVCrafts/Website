const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');
const path = require('node:path');

const database = process.env.WEEVCRAFTS_QA_DATABASE;
const target = new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://localhost:8080');
const isolated = database === 'weevcrafts_roles_browser' && target.port === '8081' && ['localhost', '127.0.0.1'].includes(target.hostname);
const password = 'WeeVCrafts-QA-2026!';
function sql(query) {
  if (!isolated) throw new Error('Four-role fixtures require the disposable roles_browser database and port 8081.');
  return execFileSync('docker', ['compose', '-p', 'weevcrafts', 'exec', '-T', 'postgres', 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'wecratfs', '-d', database, '-tAc', query], { encoding: 'utf8' }).trim();
}
async function login(page, account) {
  const response = await page.request.post('/api/v1/auth/login', { data: { email: `${account}@weevcrafts.test`, password } });
  expect(response.ok()).toBeTruthy();
}
async function token(page, route) {
  await page.goto(route);
  return page.locator('input[name="csrf_token"]').first().inputValue();
}

test.describe('Four roles and private company coordination', () => {
  test.skip(!isolated, 'Only runs against an explicitly isolated QA deployment.');
  test.beforeAll(() => {
    if (sql("SELECT count(*) FROM support_tickets WHERE ticket_number='CASE-FOUR-ROLE-QA'") === '1') {
      sql("DELETE FROM seller_confirmation_messages WHERE confirmation_id IN (SELECT id FROM seller_confirmations WHERE ticket_id=(SELECT id FROM support_tickets WHERE ticket_number='CASE-FOUR-ROLE-QA')); DELETE FROM seller_confirmations WHERE ticket_id=(SELECT id FROM support_tickets WHERE ticket_number='CASE-FOUR-ROLE-QA'); DELETE FROM support_ticket_messages WHERE ticket_id=(SELECT id FROM support_tickets WHERE ticket_number='CASE-FOUR-ROLE-QA'); UPDATE support_tickets SET status='open',agent_note='' WHERE ticket_number='CASE-FOUR-ROLE-QA';");
      return;
    }
    sql(`BEGIN;
      INSERT INTO users(email,display_name,status,email_verified_at) VALUES('seller2@weevcrafts.test','Independent second seller','active',NOW());
      INSERT INTO credentials(user_id,password_hash) SELECT u.id,c.password_hash FROM users u CROSS JOIN credentials c JOIN users original ON original.id=c.user_id WHERE u.email='seller2@weevcrafts.test' AND original.email='seller@weevcrafts.test';
      INSERT INTO user_roles(user_id,role_slug) SELECT id,'seller' FROM users WHERE email='seller2@weevcrafts.test';
      INSERT INTO sellers(owner_user_id,display_name,status) SELECT id,'Second independent maker','active' FROM users WHERE email='seller2@weevcrafts.test';
      INSERT INTO products(slug,brand_slug,category_slug,name,description,price_cents,image_url,seller_id,status) SELECT 'qa-second-maker','weevcrafts','crafts','Second seller private piece','QA isolation item',1000,'',id,'approved' FROM sellers WHERE display_name='Second independent maker';
      INSERT INTO product_variants(product_id,sku,price_cents) SELECT id,'QA-SECOND-PRIVATE',1000 FROM products WHERE slug='qa-second-maker';
      INSERT INTO inventory_stock(variant_id,available_quantity) SELECT id,5 FROM product_variants WHERE sku='QA-SECOND-PRIVATE';
      INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) SELECT 'WC-FOUR-ROLE-QA',id,'paid',2000,2000,'{}','qa-four-role','qa-order-key' FROM users WHERE email='customer@weevcrafts.test';
      INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) SELECT o.id,v.id,'First maker private piece',v.sku,1000,1,1000 FROM orders o CROSS JOIN product_variants v WHERE o.order_number='WC-FOUR-ROLE-QA' AND v.sku='WC-TREE-001';
      INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) SELECT o.id,v.id,'Second seller private piece',v.sku,1000,1,1000 FROM orders o CROSS JOIN product_variants v WHERE o.order_number='WC-FOUR-ROLE-QA' AND v.sku='QA-SECOND-PRIVATE';
      INSERT INTO support_tickets(ticket_number,user_id,order_id,subject,message) SELECT 'CASE-FOUR-ROLE-QA',user_id,id,'Confirm dispatch with both makers','Please confirm when my order will be sent.' FROM orders WHERE order_number='WC-FOUR-ROLE-QA';
      COMMIT;`);
  });

  for (const [account, landing] of [['customer', '/products'], ['seller', '/seller-admin'], ['admin', '/admin'], ['superadmin', '/super-admin']]) {
    test(`${account} has its canonical login destination`, async ({ page }) => {
      await page.goto('/login');
      await page.locator('#page input[name="email"]').fill(`${account}@weevcrafts.test`);
      await page.locator('#page input[name="password"]').fill(password);
      await page.getByRole('button', { name: 'Continue with email' }).click();
      await expect(page).toHaveURL(new RegExp(`${landing}$`));
    });
  }

  test('company boundaries, retired bookmarks, Team removal and CSRF', async ({ page }) => {
    await login(page, 'admin');
    for (const route of ['/admin/users', '/admin/roles', '/admin/security', '/admin/settings', '/super-admin']) {
      expect((await page.request.get(route)).status(), route).toBe(403);
    }
    for (const route of ['/admin/orders', '/admin/support', '/admin/products', '/admin/sellers', '/admin/inventory', '/admin/returns', '/admin/finance', '/admin/analytics']) {
      const response = await page.request.get(route);
      expect(response.ok(), route).toBeTruthy();
      expect(await response.text(), route).not.toContain('Internal Server Error');
    }
    await page.goto('/finance-portal');
    await expect(page).toHaveURL(/\/admin\/finance$/);
    expect((await page.request.post('/admin/support/confirmations/create', { form: { ticket_id: '1', seller_id: '1', message: 'No CSRF', request_key: 'qa-csrf-request-key' }, maxRedirects: 0 })).status()).toBe(403);
    await login(page, 'seller');
    for (const route of ['/seller-admin/team', '/seller-admin/team/add']) expect((await page.request.get(route)).status()).toBe(404);
    expect((await page.request.get('/admin')).status()).toBe(403);
  });

  test('registration rejects forged roles; confirmed role replacement revokes the old session', async ({ page, browser }) => {
    const email = `four-role-registration-${Date.now()}@example.invalid`;
    const data = { email, display_name: 'Four role registration QA', phone: '9876543210', password, accepted_terms: true };
    expect((await page.request.post('/api/v1/auth/register', { data: { ...data, role: 'super_admin' } })).status()).toBe(400);
    expect((await page.request.post('/api/v1/auth/register', { data })).status()).toBe(201);
    const id = sql(`SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.email='${email}' AND ur.role_slug='customer'`);
    expect(id).not.toBe('');
    const context = await browser.newContext({ baseURL: target.origin });
    const admin = await context.newPage();
    await login(admin, 'superadmin');
    const csrf = await token(admin, '/super-admin/roles');
    const assignment = { csrf_token: csrf, user_id: id, role: 'admin', action: 'grant', reason: 'QA confirmed operations assignment' };
    expect((await admin.request.post('/super-admin/roles/update', { form: assignment, maxRedirects: 0 })).status()).toBe(422);
    expect((await admin.request.post('/super-admin/roles/update', { form: { ...assignment, confirm: 'yes', role: 'finance_operator' }, maxRedirects: 0 })).status()).toBe(409);
    expect((await admin.request.post('/super-admin/roles/update', { form: { ...assignment, confirm: 'yes' }, maxRedirects: 0 })).status()).toBe(303);
    expect(sql(`SELECT role_slug FROM user_roles WHERE user_id=${id}`)).toBe('admin');
    await page.goto('/account'); await expect(page).toHaveURL(/\/login/);
    const signedIn = await page.request.post('/api/v1/auth/login', { data: { email, password } });
    expect(signedIn.ok()).toBeTruthy();
    expect((await signedIn.json()).redirect_to).toBe('/admin');
    await context.close();
  });

  test('Super Admin can create each canonical role with confirmation and a reason', async ({ page, browser }) => {
    await login(page, 'superadmin');
    const csrf = await token(page, '/super-admin/users');
    for (const [role, destination] of [['customer', '/products'], ['seller', '/seller-admin'], ['admin', '/admin'], ['super_admin', '/super-admin']]) {
      const email = `create-${role}-${Date.now()}@example.invalid`;
      const form = { csrf_token: csrf, email, display_name: `QA ${role}`, password, role, reason: 'Verify confirmed canonical account creation' };
      expect((await page.request.post('/super-admin/users/create', { form, maxRedirects: 0 })).status()).toBe(422);
      expect((await page.request.post('/super-admin/users/create', { form: { ...form, confirm: 'yes' }, maxRedirects: 0 })).status()).toBe(303);
      const id = sql(`SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.email='${email}' AND ur.role_slug='${role}'`);
      expect(id).not.toBe('');
      const context = await browser.newContext({ baseURL: target.origin });
      try {
        const response = await context.request.post('/api/v1/auth/login', { data: { email, password } });
        expect(response.ok()).toBeTruthy(); expect((await response.json()).redirect_to).toBe(destination);
        if (role === 'seller') {
          const seller = await context.newPage(); await seller.goto('/seller-admin/products/new');
          await expect(seller).toHaveURL(/\/seller-admin\/onboarding/);
        }
      } finally {
        await context.close();
        expect((await page.request.post('/super-admin/users/delete', { form: { csrf_token: csrf, user_id: id, reason: 'Remove disposable role creation QA account' }, maxRedirects: 0 })).status()).toBe(303);
      }
    }
  });

  test('multi-seller case completes without leaking private messages or fabricating delivery', async ({ page, browser }) => {
    const ticket = sql("SELECT id FROM support_tickets WHERE ticket_number='CASE-FOUR-ROLE-QA'");
    await login(page, 'admin');
    await page.goto(`/admin/support?ticket_id=${ticket}`);
    await expect(page.locator('.support-linked-order')).toContainText('WC-FOUR-ROLE-QA');
    await expect(page.getByRole('link', { name: 'Inspect linked order' })).toHaveAttribute('href', '/admin/orders?q=WC-FOUR-ROLE-QA');
    const sellers = await page.locator('#confirmation-seller option').evaluateAll(options => options.map(option => option.value));
    expect(sellers).toHaveLength(2);
    const form = page.locator('form[action="/admin/support/confirmations/create"]');
    const csrf = await form.locator('input[name="csrf_token"]').inputValue();
    const key = await form.locator('input[name="request_key"]').inputValue();
    const firstSeller = sql("SELECT id FROM sellers WHERE display_name='WeevCrafts Seller'");
    const secondSeller = sql("SELECT id FROM sellers WHERE display_name='Second independent maker'");
    await page.locator('#confirmation-seller').selectOption(firstSeller);
    await page.locator('#confirmation-question').fill('PRIVATE first maker dispatch check');
    await form.getByRole('button', { name: 'Send private request' }).click();
    await expect(page.locator('.coordination-card')).toContainText('PRIVATE first maker dispatch check');
    const duplicate = { csrf_token: csrf, ticket_id: ticket, seller_id: firstSeller, request_key: key, message: 'PRIVATE first maker dispatch check' };
    expect((await page.request.post('/admin/support/confirmations/create', { form: duplicate, maxRedirects: 0 })).status()).toBe(303);
    expect((await page.request.post('/admin/support/confirmations/create', { form: { ...duplicate, message: 'Changed duplicate' }, maxRedirects: 0 })).status()).toBe(409);
    await page.goto(`/admin/support?ticket_id=${ticket}`);
    await page.locator('#confirmation-seller').selectOption(secondSeller);
    await page.locator('#confirmation-question').fill('PRIVATE second maker dispatch check');
    await page.getByRole('button', { name: 'Send private request' }).click();
    const firstID = sql("SELECT id FROM seller_confirmations WHERE question='PRIVATE first maker dispatch check'");
    const sellerContext = await browser.newContext({ baseURL: target.origin });
    const sellerPage = await sellerContext.newPage();
    await login(sellerPage, 'seller');
    await sellerPage.goto('/seller-admin/support');
    await expect(sellerPage.locator('.coordination-card')).toHaveCount(1);
    await expect(sellerPage.locator('.coordination-card')).not.toContainText('Second seller private piece');
    await sellerPage.getByLabel('Reply to company support').fill('PRIVATE maker says dispatch tomorrow');
    await sellerPage.getByRole('button', { name: 'Send confirmation' }).click();
    await expect(sellerPage.locator('.coordination-card')).toContainText('Answered');
    await login(sellerPage, 'seller2');
    const secondCSRF = await token(sellerPage, '/seller-admin/support');
    await expect(sellerPage.locator('.coordination-card')).toHaveCount(1);
    await expect(sellerPage.locator('.coordination-card')).not.toContainText('PRIVATE maker says dispatch tomorrow');
    expect((await sellerPage.request.post('/seller-admin/support/confirmations/reply', { form: { csrf_token: secondCSRF, confirmation_id: firstID, message: 'Foreign reply', request_key: 'foreign-request-key-0001' }, maxRedirects: 0 })).status()).toBe(403);
    await sellerContext.close();
    await page.goto(`/admin/support?ticket_id=${ticket}`);
    const card = page.locator(`#confirmation-${firstID}`);
    await expect(card).toContainText('PRIVATE maker says dispatch tomorrow');
    await card.getByLabel('Delivery issue or follow-up outcome').fill('Admin contacted maker; customer informed of planned dispatch.');
    page.once('dialog', dialog => dialog.accept());
    await card.getByRole('button', { name: 'Record outcome & close request' }).click();
    const confirmationDialog = page.getByRole('dialog');
    if (await confirmationDialog.isVisible()) await confirmationDialog.locator('[data-confirm-accept]').click();
    await expect(page.locator(`#confirmation-${firstID}`)).toContainText('Closed');
    await page.locator('form[action="/admin/support/cases/reply"] textarea[name="message"]').fill('Your makers are confirming dispatch. We will share the tracking details once recorded.');
    await page.locator('form[action="/admin/support/cases/reply"] button[type="submit"]').click();
    await page.goto(`/admin/support?ticket_id=${ticket}`);
    await page.locator('form[action="/admin/support/cases/status"] select[name="status"]').selectOption('resolved');
    await page.locator('form[action="/admin/support/cases/status"] input[name="note"]').fill('PRIVATE internal company follow-up complete');
    await page.getByRole('button', { name: 'Save status' }).click();
    expect(sql(`SELECT status FROM support_tickets WHERE id=${ticket}`)).toBe('resolved');
    await login(page, 'customer');
    await page.goto('/account/support');
    await expect(page.locator('body')).not.toContainText('PRIVATE');
    await expect(page.locator('body')).toContainText('Your makers are confirming dispatch.');
    expect((await page.request.get('/admin/support')).status()).toBe(403);
    expect(sql("SELECT status FROM orders WHERE order_number='WC-FOUR-ROLE-QA'")).toBe('paid');
    expect(sql(`SELECT count(*) FROM seller_confirmation_messages WHERE confirmation_id=${firstID}`)).toBe('1');
  });

  test('390px company navigation is keyboard accessible and respects reduced motion', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await login(page, 'superadmin');
    await page.goto('/super-admin/support');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
    const menu = page.getByRole('button', { name: 'Open admin navigation' });
    await menu.focus(); await page.keyboard.press('Enter');
    await expect(page.locator('#admin-sidebar').getByRole('button', { name: 'Close admin navigation' })).toBeFocused();
    await page.keyboard.press('Shift+Tab');
    expect(await page.evaluate(() => document.activeElement.closest('#admin-sidebar') !== null)).toBeTruthy();
    await page.keyboard.press('Escape'); await expect(menu).toBeFocused();
    await expect(page.locator('.admin-header').getByRole('button', { name: /log out/i })).toBeVisible();
    await page.screenshot({ path: path.join(require('node:os').tmpdir(), 'weevcrafts-four-roles', 'superadmin-mobile.png'), fullPage: true });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto('/super-admin/support');
    await page.screenshot({ path: path.join(require('node:os').tmpdir(), 'weevcrafts-four-roles', 'superadmin-desktop.png'), fullPage: true });
  });
});
