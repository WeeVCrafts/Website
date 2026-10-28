const { test, expect } = require('@playwright/test');
const { readFileSync } = require('node:fs');
const { createHmac } = require('node:crypto');
const path = require('node:path');

const baseURL = process.env.WEEVCRAFTS_BASE_URL || '';
const isolated = process.env.WEEVCRAFTS_RAZORPAY_QA === 'true' &&
  /^http:\/\/(localhost|127\.0\.0\.1):18083$/.test(baseURL);

test.describe('Razorpay sandbox in the isolated payment QA store', () => {
  test.skip(!isolated, 'Requires the disposable payment database, port 18083 and explicit sandbox opt-in.');

  test('real provider checkout, verified callback and signed capture replay', async ({ page, context, request }, testInfo) => {
    test.setTimeout(150_000);
    const env = Object.fromEntries(readFileSync(path.join(__dirname, '../../.env'), 'utf8')
      .split(/\r?\n/).filter(line => /^RAZORPAY_[A-Z_]+=/.test(line))
      .map(line => { const split = line.indexOf('='); return [line.slice(0, split), line.slice(split + 1).trim()]; }));
    expect(env.RAZORPAY_KEY_ID).toMatch(/^rzp_test_/);
    expect(Boolean(env.RAZORPAY_KEY_SECRET && env.RAZORPAY_WEBHOOK_SECRET)).toBeTruthy();
    const pageErrors = [];
    page.on('pageerror', error => pageErrors.push(error.message));
    const login = await context.request.post('/api/v1/auth/login', {
      data: { email: 'customer@weevcrafts.test', password: process.env.WEEVCRAFTS_QA_PASSWORD || 'Razorpay-QA-2026!' },
    });
    expect(login.ok()).toBeTruthy();
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/products/weevcrafts-wooden-box');
    await page.waitForFunction(() => window.weevcraftsBackend);
    await page.locator('.mobile-buy-bar').getByRole('button', { name: /Buy now/i }).click();
    await expect(page).toHaveURL(/\/checkout\?intent=/);
    for (const [name, value] of Object.entries({ recipient_name: 'Payment Test Customer', phone: '9000090000', line1: '1 Test Lane', city: 'Bengaluru', state: 'Karnataka', postal_code: '560034' })) {
      await page.locator(`#checkout-address-form [name="${name}"]`).fill(value);
    }
    await page.getByRole('button', { name: 'Continue to delivery', exact: true }).press('Enter');
    await page.getByRole('button', { name: 'Continue to payment', exact: true }).click();
    await expect(page.getByText('Razorpay test mode.', { exact: false })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath('checkout-test-mode-390.png'), fullPage: true });
    await page.getByRole('button', { name: 'Continue to review', exact: true }).click();
    await page.getByRole('button', { name: 'Place order', exact: true }).click();
    await page.setViewportSize({ width: 1440, height: 1000 });
    const provider = page.frameLocator('iframe.razorpay-checkout-frame');
    await provider.getByPlaceholder('Mobile number', { exact: true }).fill('9000090000');
    // Razorpay also advances automatically after a valid test mobile number.
    const contactContinue = provider.getByTestId('contact-overlay-container').getByRole('button', { name: 'Continue', exact: true });
    if (await contactContinue.isVisible()) await contactContinue.click({ timeout: 3000 }).catch(() => {});
    await provider.getByText('Netbanking', { exact: true }).click();
    const bankPromise = context.waitForEvent('page');
    await provider.getByText('Canara Bank', { exact: true }).first().click();
    const bank = await bankPromise;
    await expect(bank.locator('body')).toContainText('This is just a demo bank page.');
    const verificationPromise = page.waitForRequest(r => r.url().endsWith('/payment/verify') && r.method() === 'POST');
    await bank.getByRole('button', { name: 'Success', exact: true }).click();
    const verification = (await verificationPromise).postDataJSON();
    await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation$/, { timeout: 30_000 });
    await expect(page.getByText('Payment pending', { exact: true }).first()).toBeVisible();

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
    await expect(page.getByRole('heading', { name: 'Order placed successfully!' })).toBeVisible();
    await expect(page.getByText('Razorpay test payment. No real money has been charged.', { exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath('sandbox-payment-confirmed-1440.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath('sandbox-payment-confirmed-390.png'), fullPage: true });
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
