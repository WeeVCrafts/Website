const { test, expect } = require('@playwright/test');
const { readFileSync } = require('node:fs');
const { createHmac } = require('node:crypto');
const path = require('node:path');

const baseURL = process.env.WEEVCRAFTS_BASE_URL || '';
const isolated = process.env.WEEVCRAFTS_RAZORPAY_QA === 'true' &&
  /^http:\/\/(localhost|127\.0\.0\.1):(18083|18089)$/.test(baseURL);
const fixturePassword = process.env.WEEVCRAFTS_QA_PASSWORD || 'Razorpay-QA-2026!';
const sellerEmails = (process.env.WEEVCRAFTS_QA_SELLERS || '').split(',').filter(Boolean);

async function checkLayout(page, action) {
  const measurements = {};
  // 683 CSS pixels approximate the reflow of a 1366px viewport at 200% zoom.
  for (const [width, height] of [[320, 740], [390, 844], [1366, 768], [683, 384]]) {
    await page.setViewportSize({ width, height });
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width + 1);
    if (action) await expect(action).toBeVisible();
    measurements[width] = await page.locator('.site-header').evaluate(header => Math.round(header.getBoundingClientRect().height));
  }
  return measurements;
}

async function signIn(context, email) {
  const result = await context.request.post('/api/v1/auth/login', { data: { email, password: fixturePassword } });
  expect(result.ok()).toBeTruthy();
}

test.describe('Razorpay sandbox in the isolated payment QA store', () => {
  test.skip(!isolated, 'Requires the disposable payment database, a loopback QA port and explicit sandbox opt-in.');

  test('real provider checkout, verified callback and signed capture replay', async ({ page, context, request, browser }, testInfo) => {
    test.setTimeout(240_000);
    const env = Object.fromEntries(readFileSync(path.join(__dirname, '../../.env'), 'utf8')
      .split(/\r?\n/).filter(line => /^RAZORPAY_[A-Z_]+=/.test(line))
      .map(line => { const split = line.indexOf('='); return [line.slice(0, split), line.slice(split + 1).trim()]; }));
    expect(env.RAZORPAY_KEY_ID).toMatch(/^rzp_test_/);
    expect(Boolean(env.RAZORPAY_KEY_SECRET && env.RAZORPAY_WEBHOOK_SECRET)).toBeTruthy();
    const pageErrors = [];
    page.on('pageerror', error => pageErrors.push(error.message));
    await signIn(context, process.env.WEEVCRAFTS_QA_CUSTOMER || 'customer@weevcrafts.test');
    const product = process.env.WEEVCRAFTS_QA_PRODUCT || 'weevcrafts-wooden-box';
    const secondProduct = process.env.WEEVCRAFTS_QA_SECOND_PRODUCT;
    const layout = {};
    await page.goto('/products/' + product);
    await page.waitForFunction(() => window.weevcraftsBackend);
    layout.product = await checkLayout(page);
    if (secondProduct) {
      expect(sellerEmails).toHaveLength(2);
      await page.setViewportSize({ width: 1366, height: 768 });
      for (const slug of [product, secondProduct]) {
        await page.goto('/products/' + slug);
        await page.waitForFunction(() => window.weevcraftsBackend);
        const added = page.waitForResponse(r => r.request().method() === 'POST' && /\/cart\/(add|items)/.test(new URL(r.url()).pathname));
        await page.locator('.detail-actions').getByRole('button', { name: 'Add to cart', exact: true }).click();
        expect((await added).ok()).toBeTruthy();
      }
      await page.goto('/cart');
      layout.cart = await checkLayout(page);
      await page.goto('/checkout');
    } else {
      await page.setViewportSize({ width: 390, height: 844 });
      await page.locator('.mobile-buy-bar').getByRole('button', { name: /Buy now/i }).click();
      await expect(page).toHaveURL(/\/checkout\?intent=/);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    for (const [name, value] of Object.entries({ recipient_name: 'Payment Test Customer', phone: '9000090000', line1: '1 Test Lane', city: 'Bengaluru', state: 'Karnataka', postal_code: '560034' })) {
      await page.locator(`#checkout-address-form [name="${name}"]`).fill(value);
    }
    await page.getByRole('button', { name: 'Review & pay', exact: true }).press('Enter');

    await expect(page.getByText('Razorpay test mode.', { exact: false })).toBeVisible();
    const payButton = page.getByRole('button', { name: /^Pay ₹[\d,.]+$/, exact: true });
    layout.checkout = await checkLayout(page, payButton);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: testInfo.outputPath('checkout-test-mode-390.png'), fullPage: true });

    await payButton.click();
    const provider = page.frameLocator('iframe.razorpay-checkout-frame');
    await expect(page.locator('iframe.razorpay-checkout-frame')).toBeVisible({ timeout: 30_000 });
    if (secondProduct) {
      await page.setViewportSize({ width: 1366, height: 768 });
      await provider.getByRole('button', { name: 'Close Checkout' }).click();
      await provider.getByRole('button', { name: 'Yes, exit', exact: true }).click();
      await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation\?payment=incomplete/, { timeout: 15_000 });
      const savedPath = new URL(page.url()).pathname.replace(/\/confirmation$/, '');
      await expect(page.getByRole('heading', { name: 'Awaiting payment confirmation' })).toBeVisible();
      await expect(page.getByRole('link', { name: /receipt/i })).toHaveCount(0);
      expect((await context.request.get(savedPath + '/invoice')).status()).toBe(409);
      expect((await context.request.get(savedPath + '/invoice?format=pdf')).status()).toBe(409);
      const refreshButton = page.getByRole('button', { name: 'Check payment status' });
      layout.pending = await checkLayout(page, refreshButton);
      await page.setViewportSize({ width: 320, height: 740 });
      const refreshed = page.waitForResponse(r => r.url().includes('/ui/orders/status?') && r.ok());
      await expect(refreshButton).toBeEnabled();
      await refreshButton.press('Enter');
      await refreshed;
      await expect(refreshButton).toBeFocused();
      await expect(page.locator('[data-payment-refresh-feedback]')).toContainText('Last updated');
      const sellerContext = await browser.newContext({ baseURL });
      try {
        await signIn(sellerContext, sellerEmails[0]);
        const sellerPage = await sellerContext.newPage();
        const orderNumber = decodeURIComponent(savedPath.split('/').pop());
        await sellerPage.goto('/seller-admin/orders?q=' + encodeURIComponent(orderNumber));
        await expect(sellerPage.locator('[data-seller-orders-region]').getByRole('link', { name: orderNumber, exact: true })).toHaveCount(0);
        await sellerPage.goto('/seller-admin/orders/' + encodeURIComponent(orderNumber));
        await expect(sellerPage.getByRole('button', { name: 'Start processing' })).toHaveCount(0);
      } finally {
        await sellerContext.close();
      }
      await page.setViewportSize({ width: 1366, height: 768 });
      await page.getByRole('button', { name: 'Continue payment' }).click();
      expect(new URL(page.url()).pathname).toBe(savedPath + '/confirmation');
    } else {
      await page.setViewportSize({ width: 1366, height: 768 });
    }
    const contactField = provider.getByPlaceholder('Mobile number', { exact: true });
    if (await contactField.isVisible()) await contactField.fill('9000090000');
    // Razorpay also advances automatically after a valid test mobile number.
    const contactContinue = provider.getByTestId('contact-overlay-container').getByRole('button', { name: 'Continue', exact: true });
    if (await contactContinue.isVisible()) await contactContinue.click({ timeout: 3000 }).catch(() => {});
    await provider.getByText('Netbanking', { exact: true }).click();
    const bankPromise = context.waitForEvent('page');
    await provider.getByText('Bank of Baroda - Retail Banking', { exact: true }).first().click();
    const bank = await bankPromise;
    await expect(bank.locator('body')).toContainText('This is just a demo bank page.');
    const verificationPromise = page.waitForRequest(r => r.url().endsWith('/payment/verify') && r.method() === 'POST');
    await bank.getByRole('button', { name: 'Success', exact: true }).click();
    const verification = (await verificationPromise).postDataJSON();
    await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation(?:\?|$)/, { timeout: 30_000 });

    // Localhost cannot receive provider deliveries. Fetch the real sandbox
    // capture and replay a signed webhook locally; this is not evidence of
    // hosted Razorpay-to-application webhook delivery.
    const authorization = 'Basic ' + Buffer.from(`${env.RAZORPAY_KEY_ID}:${env.RAZORPAY_KEY_SECRET}`).toString('base64');
    let payment;
    await expect.poll(async () => {
      const response = await request.get('https://api.razorpay.com/v1/payments/' + encodeURIComponent(verification.provider_payment_id), { headers: { Authorization: authorization } });
      expect(response.ok()).toBeTruthy(); payment = await response.json(); return payment.status;
    }, { timeout: 30_000 }).toBe('captured');
    expect(payment.order_id).toBe(verification.provider_order_id);
    expect(payment.currency).toBe('INR');
    // Recovery must obtain capture from the real provider even without a webhook.
    const refresh = page.getByRole('button', { name: 'Check payment status' });
    if (await refresh.isVisible()) await refresh.press('Enter', { timeout: 5000 }).catch(async error => {
      if (!(await page.getByRole('heading', { name: 'Order confirmed' }).isVisible())) throw error;
    });
    await expect(page.getByRole('heading', { name: 'Order confirmed' })).toBeVisible({timeout:75000});
    const raw = JSON.stringify({ entity: 'event', event: 'payment.captured', payload: { payment: { entity: payment } } });
    const headers = { 'Content-Type': 'application/json', 'X-Razorpay-Event-Id': 'qa_capture_' + payment.id,
      'X-Razorpay-Signature': createHmac('sha256', env.RAZORPAY_WEBHOOK_SECRET).update(raw).digest('hex') };
    const altered = await request.post('/webhooks/razorpay', { headers, data: raw + ' ' });
    expect(altered.status()).toBe(401);
    const first = await request.post('/webhooks/razorpay', { headers, data: raw });
    expect(first.ok()).toBeTruthy(); expect((await first.json()).processed).toBe(true);
    const duplicate = await request.post('/webhooks/razorpay', { headers, data: raw });
    expect(duplicate.ok()).toBeTruthy(); expect((await duplicate.json()).processed).toBe(false);
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Order confirmed' })).toBeVisible();
    await expect(page.getByText('Test payment. No real money is charged.', { exact: true })).toBeVisible();
    layout.confirmation = await checkLayout(page, page.getByRole('link', { name: 'View order', exact: true }));
    await page.setViewportSize({ width: 1366, height: 768 });
    await page.screenshot({ path: testInfo.outputPath('sandbox-payment-confirmed-1366.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath('sandbox-payment-confirmed-390.png'), fullPage: true });
    if (secondProduct) {
      const orderPath = new URL(page.url()).pathname.replace(/\/confirmation$/, '');
      const orderNumber = decodeURIComponent(orderPath.split('/').pop());
      expect((await context.request.get(orderPath + '/invoice')).status()).toBe(200);
      const pdf = await context.request.get(orderPath + '/invoice?format=pdf');
      expect(pdf.ok()).toBeTruthy();
      expect(pdf.headers()['content-type']).toContain('application/pdf');
      await page.goto(orderPath);
      layout.order = await checkLayout(page, page.getByRole('link', { name: 'Track order', exact: true }));
      for (const [i, sellerEmail] of sellerEmails.entries()) {
        const sellerContext = await browser.newContext({ baseURL });
        try {
          await signIn(sellerContext, sellerEmail);
          const seller = await sellerContext.newPage();
          const queue = '/seller-admin/orders?q=' + encodeURIComponent(orderNumber);
          await seller.goto(queue);
          const queueEntry = seller.locator('[data-seller-orders-region]').getByRole('link', { name: orderNumber, exact: true });
          await expect(queueEntry).toBeVisible();
          for (const action of ['Start processing', 'Mark as packed', 'Mark as shipped', 'Mark ready to deliver', 'Mark as delivered']) {
            await seller.goto('/seller-admin/orders/' + encodeURIComponent(orderNumber));
            const form = seller.locator('[data-fulfillment-form]');
            if (action === 'Mark as shipped') {
              const carrier = form.locator('select[name="carrier"]');
              await expect(carrier).toHaveAttribute('required', '');
              await expect(form.getByLabel('Tracking number')).toHaveAttribute('required', '');
              await carrier.selectOption('Other');
              await form.getByLabel('Other carrier name').fill('Sandbox Carrier');
              await form.getByLabel('Tracking number').fill('WC-QA-TRACK-' + i);
            }
            const saved = seller.waitForResponse(r => r.url().endsWith('/seller-admin/orders/fulfill') && r.request().method() === 'POST');
            await form.getByRole('button', { name: action, exact: true }).click();
            expect([200, 303]).toContain((await saved).status());
            await expect(seller).toHaveURL(url => url.pathname === '/seller-admin/orders/' + encodeURIComponent(orderNumber) && url.searchParams.get('notice') === 'Fulfilment update saved.');
          }
        } finally {
          await sellerContext.close();
        }
        await page.goto(orderPath + '/tracking');
        await expect(page.locator('.shipment-card').filter({ hasText: 'WC-QA-TRACK-' + i }).locator('.ui-status')).toHaveText('Delivered');
        const order = await context.request.get('/api/v1' + orderPath);
        expect(order.ok()).toBeTruthy();
        expect((await order.json()).data.status).toBe(i === 0 ? 'processing' : 'delivered');
      }
      await expect(page.locator('.shipment-card')).toHaveCount(2);
    }
    await testInfo.attach('Header heights by CSS viewport width', { body: JSON.stringify(layout, null, 2), contentType: 'application/json' });
    expect(pageErrors).toEqual([]);
  });

  test('published public content and legacy URL migrations remain crawlable', async ({ request }) => {
    const sitemap = await request.get('/sitemap.xml');
    expect(sitemap.ok()).toBeTruthy();
    const urls = [...(await sitemap.text()).matchAll(/<loc>([^<]+)<\/loc>/g)].map(match => match[1]);
    expect(urls).toContain(baseURL + '/pages/returns');
    expect(urls).toContain(baseURL + '/sustainability');
    for (const url of urls) {
      const response = await request.get(url);
      expect(response.status(), url).toBe(200);
      const html = await response.text();
      expect(html, url).toContain('name="robots" content="index,follow"');
      expect(html, url).toContain(`rel="canonical" href="${url}"`);
    }
    for (const [source, target] of Object.entries({ '/pages/about-us': '/about', '/collections/sarees': '/categories/textiles', '/collections/arts-collection': '/categories/arts', '/collections/frontpage': '/products' })) {
      const response = await request.get(source, { maxRedirects: 0 });
      expect(response.status()).toBe(301); expect(response.headers().location).toBe(target);
    }
    expect((await request.get('/brands/not-a-real-brand')).status()).toBe(404);
  });
});
