const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');

const widths = [320, 390, 720, 768, 820, 1024, 1280, 1440];
const routes = ['','/products','/products/new','/inventory','/orders','/earnings','/analytics','/returns','/support','/settings','/onboarding','/terms'];

function requireQA(testInfo) {
  const url = new URL(testInfo.project.use.baseURL);
  if (url.hostname !== '127.0.0.1' || url.port !== '18080' || process.env.WEEVCRAFTS_QA_DATABASE !== 'seller_workflows_20261017') {
    throw new Error('Use the isolated seller_workflows_20261017 database and local port 18080.');
  }
}

function sql(query) {
  return execFileSync('docker', ['exec', 'weevcrafts-storefront-qa-db', 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'storefrontqa', '-d', 'seller_workflows_20261017', '-tAc', query], { encoding: 'utf8', windowsHide: true }).trim();
}

test.beforeAll(({}, testInfo) => {
  requireQA(testInfo);
  sql(`INSERT INTO seller_ledger(seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents,status,note)
    SELECT s.id,o.id,'sale',99999900,11999988,87999912,'open','Seller responsive QA'
    FROM sellers s JOIN users u ON u.id=s.owner_user_id
    CROSS JOIN (SELECT id FROM orders WHERE order_number LIKE 'WC-SELLER-WORKFLOW-%' ORDER BY id LIMIT 1) o
    WHERE u.email='demo.owner01@weevcrafts.local'
    AND NOT EXISTS(SELECT 1 FROM seller_ledger l WHERE l.seller_id=s.id AND l.order_id=o.id AND l.entry_type='sale')
    ON CONFLICT DO NOTHING;
    INSERT INTO support_tickets(ticket_number,user_id,subject,message)
    SELECT 'SUP-SELLER-RESPONSIVE-QA',id,'Seller responsive layout fixture',repeat('Please review this order and the delivery details. ',20)
    FROM users WHERE email='demo.owner01@weevcrafts.local'
    ON CONFLICT(ticket_number) DO NOTHING;`);
});

test.beforeEach(async ({ page }, testInfo) => {
  requireQA(testInfo);
  const response = await page.request.post('/api/v1/auth/login', { data: { email: 'demo.owner01@weevcrafts.local', password: 'WeeVCrafts-Demo-2026!' } });
  expect(response.ok()).toBeTruthy();
});

async function fits(page, label) {
  const problems = await page.evaluate(() => {
    const result = [];
    if (document.documentElement.scrollWidth > innerWidth + 1) result.push('document overflow');
    const main = document.querySelector('#seller-main');
    if (main && main.scrollWidth > main.clientWidth + 1) result.push('main content overflow');
    const header = Array.from(document.querySelector('.seller-header-main').children).filter(node => node.getClientRects().length);
    for (let i = 0; i < header.length; i++) {
      const a = header[i].getBoundingClientRect();
      if (a.left < -1 || a.right > innerWidth + 1) result.push('offscreen header control');
      for (const other of header.slice(i + 1)) {
        const b = other.getBoundingClientRect();
        if (Math.min(a.right, b.right) - Math.max(a.left, b.left) > 1 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 1) result.push('overlapping header controls');
      }
    }
    for (const node of document.querySelectorAll('.seller-toolbar .seller-button, .ui-actions .seller-button, .seller-header-logout button, .ui-table thead th')) {
      if (!node.getClientRects().length) continue;
      if (innerWidth <= 720 && node.closest('.ui-table--records thead')) continue;
      const style = getComputedStyle(node);
      if (style.whiteSpace !== 'nowrap') result.push(`wrapping label: ${node.textContent.trim()}`);
      if (node.scrollWidth > node.clientWidth + 2) result.push(`clipped label: ${node.textContent.trim()}`);
    }
    for (const node of document.querySelectorAll('.seller-toolbar input:not([type=hidden]), .seller-toolbar select, .seller-toolbar .seller-button')) {
      if (!node.getClientRects().length) continue;
      const box = node.getBoundingClientRect();
      if (box.width < 40 || box.height < 43) result.push(`small control: ${node.name || node.textContent.trim()}`);
      if (box.left < -1 || box.right > innerWidth + 1) result.push('offscreen toolbar control');
    }
    return result;
  });
  expect(problems, label).toEqual([]);
}

test('all seller routes reflow without clipped labels, controls or document overflow', async ({ page }, testInfo) => {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  for (const route of routes) {
    await page.setViewportSize({ width: 1440, height: 1000 });
    const response = await page.goto('/seller-admin' + route + (route === '/earnings' ? '?range=month' : ''));
    expect(response.status(), route).toBe(200);
    for (const width of widths) {
      await page.setViewportSize({ width, height: 1000 });
      await fits(page, `${route || 'dashboard'} at ${width}`);
      if (route === '/orders' && width > 720) {
        await expect(page.locator('[data-label="Customer"]').first()).toHaveCSS('overflow-wrap', 'normal');
        expect(await page.locator('[data-label="Customer"]').first().evaluate(node => node.clientWidth)).toBeGreaterThanOrEqual(160);
      }
      if (width <= 720 && await page.locator('.ui-table--records tbody tr').count()) {
        await expect(page.locator('.ui-table--records tbody tr').first()).toHaveCSS('display', 'grid');
        const labels = await page.locator('.ui-table--records tbody td').evaluateAll(nodes => nodes.every(node => !!node.dataset.label));
        expect(labels, `${route} mobile labels`).toBeTruthy();
      }
      if ([320, 1440].includes(width) && ['/products','/earnings','/support'].includes(route)) {
        await page.screenshot({ path: testInfo.outputPath(`${route.slice(1)}-${width}.png`) });
        if (route === '/support') await page.locator('.seller-support-create').screenshot({ path: testInfo.outputPath(`support-create-${width}.png`) });
      }
    }
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/seller-admin/earnings?range=month');
  const ledger = await page.locator('[aria-label="Seller ledger"]').boundingBox();
  const content = await page.locator('#seller-main').boundingBox();
  expect(ledger.width).toBeGreaterThan(content.width * .9);
  await page.goto('/seller-admin/products');
  const edit = page.locator('.seller-product-link').first();
  if (await edit.count()) { await edit.click(); await fits(page, 'product editor'); }
  await page.goto('/seller-admin/orders');
  await page.getByRole('link', { name: 'Open order', exact: true }).first().click();
  for (const width of widths) { await page.setViewportSize({ width, height: 1000 }); await fits(page, `order detail ${width}`); }
  expect(errors).toEqual([]);
});

test('navigation, search and product disclosure remain usable with keyboard input', async ({ page }) => {
  for (const width of [320, 720, 820]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/seller-admin/products');
    const menu = page.getByRole('button', { name: 'Open seller navigation', exact: true });
    await menu.focus(); await page.keyboard.press('Enter');
    await expect(page.locator('#seller-sidebar')).toHaveAttribute('aria-modal', 'true');
    await page.keyboard.press('Shift+Tab');
    expect(await page.evaluate(() => document.activeElement.closest('#seller-sidebar') !== null)).toBeTruthy();
    await page.keyboard.press('Escape'); await expect(menu).toBeFocused();
    await expect(page.locator('#seller-main')).not.toHaveAttribute('inert', '');
    const search = page.getByLabel('Search products', { exact: true });
    await search.fill('SELLER-WORKFLOW'); await search.press('Enter');
    await expect(page).toHaveURL(/q=SELLER-WORKFLOW/);
    await page.locator('.seller-toolbar').getByRole('link', { name: 'Clear', exact: true }).click();
    await expect(search).toHaveValue('');
    const summary = page.locator('.row-actions summary').first();
    await summary.focus(); await page.keyboard.press('Enter');
    await expect(summary.locator('..')).toHaveAttribute('open', '');
  }
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.getByRole('button', { name: 'Open seller navigation', exact: true }).click();
  await expect(page.locator('#seller-sidebar')).toHaveCSS('transition-duration', '0s');
});

test('support preserves entries on failure, saves replies, and renders closed and empty states', async ({ page }, testInfo) => {
  const subject = `Responsive support ${testInfo.project.name} ${Date.now()}`;
  await page.setViewportSize({ width: 390, height: 900 });
  await page.goto('/seller-admin/support');
  const form = page.locator('form[action="/seller-admin/support/create"]');
  await form.getByRole('button', { name: 'Send request' }).click();
  await expect(form.locator('[name=subject]')).toBeFocused();
  await form.getByLabel('Subject', { exact: true }).fill(subject);
  const text = 'A long seller message about an order. '.repeat(20) + '\nSecond paragraph with the complete order reference.';
  await form.getByLabel('Message', { exact: true }).fill(text);
  await form.locator('[type=file]').setInputFiles({ name: 'seller-order-evidence-with-a-long-readable-filename.png', mimeType: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==', 'base64') });
  let rejectCreate = true;
  await page.route('**/seller-admin/support/create', route => rejectCreate
    ? route.fulfill({ status: 503, contentType: 'text/plain', body: 'Please try again shortly.' })
    : route.continue());
  await form.getByRole('button', { name: 'Send request' }).click();
  await expect(form.locator('[role=alert]')).toContainText('Please try again');
  await expect(form.getByLabel('Message', { exact: true })).toHaveValue(text);
  expect(await form.locator('[type=file]').evaluate(node => node.files.length)).toBe(1);
  await expect(form.getByRole('button', { name: 'Send request' })).toBeEnabled();
  rejectCreate = false;
  await form.locator('[type=file]').setInputFiles([]);
  await form.getByRole('button', { name: 'Send request' }).click();
  const ticket = page.locator('.seller-support-ticket').filter({ hasText: subject });
  await expect(ticket).toContainText('Second paragraph');
  const reply = ticket.locator('form');
  await reply.getByLabel('Reply', { exact: true }).fill('Reply retained and saved across a full page reload.');
  await reply.getByRole('button', { name: 'Send reply' }).click();
  await expect(ticket.locator('.seller-support-message').last()).toContainText('Reply retained and saved');
  await page.reload(); await expect(ticket).toContainText('Reply retained and saved');
  const id = await ticket.getAttribute('id');
  // Only this test's ticket in the explicitly guarded QA database is changed.
  sql(`UPDATE support_tickets SET status='closed' WHERE id=${Number(id.replace('ticket-', ''))}`);
  await page.reload();
  await expect(ticket.locator('form')).toHaveCount(0);
  await expect(ticket).toContainText('This request is closed');
  await ticket.screenshot({ path: testInfo.outputPath('support-conversation-390.png'), style: '.seller-header,.seller-skip-link { visibility: hidden !important; }' });
  for (const width of widths) { await page.setViewportSize({ width, height: 1000 }); await fits(page, `support ${width}`); }
  await page.locator('.seller-toolbar [name=q]').fill(subject);
  await page.locator('.seller-toolbar').getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.locator('.seller-support-ticket')).toHaveCount(1);
  await page.locator('.seller-toolbar [name=q]').fill('no-such-ticket-' + Date.now());
  await page.locator('.seller-toolbar').getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.getByText('No support requests match these filters.')).toBeVisible();
  await fits(page, 'empty support');
});

test('long names, references and large amounts stay readable without widening the page', async ({ page }, testInfo) => {
  await page.goto('/seller-admin/orders');
  // Stress the real rendered row without editing an existing customer's records.
  await page.locator('.ui-table tbody tr').first().evaluate(row => {
    row.querySelector('[data-label="Customer"]').textContent = 'A very long customer name that must wrap between complete words';
    row.querySelector('[data-label="Value"]').textContent = 'INR 99999999.00';
    row.querySelector('th a').textContent = 'WC-2026-1013-12345678901234567890';
  });
  for (const width of widths) {
    await page.setViewportSize({ width, height: 1000 }); await fits(page, `long data ${width}`);
    await expect(page.locator('[data-label="Value"]').first()).toHaveCSS('white-space', 'nowrap');
  }
  await page.locator('.ui-table tbody tr').first().screenshot({ path: testInfo.outputPath('long-record-320.png') });
  await page.setViewportSize({ width: 640, height: 500 });
  await fits(page, 'reflow equivalent to 200% of a 1280px viewport');
});

test('seller forms and tables retain accessible names, labels and contrast', async ({ page }) => {
  for (const route of ['/products', '/earnings', '/support', '/products/new']) {
    await page.goto('/seller-admin' + route);
    await page.addScriptTag({ path: require.resolve('axe-core/axe.min.js') });
    const violations = await page.evaluate(async () => {
      const result = await axe.run(document.querySelector('#seller-main'), { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'] } });
      return result.violations.map(item => ({ id: item.id, nodes: item.nodes.map(node => node.target) }));
    });
    expect(violations, route).toEqual([]);
  }
});
