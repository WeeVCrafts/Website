const { test, expect } = require('@playwright/test');

// Use customer-layout.config.js to render the real templates with loopback fixtures.
test.use({ baseURL: 'http://127.0.0.1:18086' });

function deferred() {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
}

test('keyboard search ignores stale and dismissed suggestions', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const newQueryStarted = deferred();
  const newQueryReply = deferred();
  await page.route('**/ui/search-suggest?*', async route => {
    if (new URL(route.request().url()).searchParams.get('q') === 'mugs') {
      newQueryStarted.resolve();
      await newQueryReply.promise;
    }
    await route.continue();
  });

  await page.goto('/search');
  const search = page.locator('#global-search');
  const popup = page.locator('#search-suggestions');
  await search.fill('ceramic');
  await expect(popup.getByRole('option')).toHaveCount(2);
  await search.press('ArrowUp');
  await expect(search).toHaveAttribute('aria-activedescendant', 'search-suggestion-layout-bowl');
  await search.press('Escape');
  await expect(popup).toBeHidden();
  await search.press('ArrowDown');
  await expect(search).not.toHaveAttribute('aria-activedescendant');
  await search.press('Enter');
  await expect(page).toHaveURL(/\/search\?q=ceramic$/);

  await page.goto('/search');
  await search.fill('ceramic');
  await expect(popup.getByRole('option')).toHaveCount(2);
  await search.fill('mugs');
  await newQueryStarted.promise;
  try {
    await expect(popup).toBeHidden();
    await search.press('ArrowDown');
    await expect(search).not.toHaveAttribute('aria-activedescendant');
    await expect(page).toHaveURL(/\/search$/);
  } finally {
    newQueryReply.resolve();
  }
  await expect(popup.getByRole('option')).toHaveCount(1);
  await search.press('ArrowDown');
  await expect(search).toHaveAttribute('aria-activedescendant', 'search-suggestion-layout-mug');
  await search.press('Enter');
  await expect(page).toHaveURL(/\/products\/layout-mug$/);
});

test('switching product variants clamps quantity to available stock', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/products/layout-mug');
  const increase = page.getByRole('button', { name: 'Increase Ceramic mug quantity' });
  const quantity = page.locator('#detail-quantity');
  for (let i = 0; i < 3; i += 1) await increase.press('Enter');
  await expect(quantity).toHaveValue('4');
  await page.locator('#detail-variant').selectOption('102');
  await expect(quantity).toHaveValue('2');
  await expect(page.locator('[data-detail-quantity-status]')).toContainText('2');
  await expect(increase).toBeDisabled();
  await page.locator('#detail-variant').selectOption('103');
  await expect(page.locator('.detail-actions').getByRole('button', { name: 'Buy now' })).toBeDisabled();
  await expect(page.locator('.mobile-buy-bar').getByRole('button', { name: 'Buy now', includeHidden: true })).toBeDisabled();
});

test('delayed payment status does not interrupt keyboard focus', async ({ page }) => {
  const firstPoll = deferred();
  const statusRequested = deferred();
  const reply = deferred();
  let requests = 0;
  await page.route('**/ui/orders/status?*', async route => {
    if (++requests === 1) {
      await route.continue();
      firstPoll.resolve();
      return;
    }
    statusRequested.resolve();
    await reply.promise;
    const confirmed = await route.fetch({ url: route.request().url() + '&fixture_payment=captured' });
    await route.fulfill({ response: confirmed });
  });
  await page.goto('/orders/QA-PAYMENT/confirmation');
  const button = page.getByRole('button', { name: 'Check payment status' });
  await firstPoll.promise;
  await expect(button).toBeEnabled();
  await button.press('Enter');
  await statusRequested.promise;
  try {
    // This stable control is outside the subtree that the status request replaces.
    await page.locator('#global-search').focus();
  } finally {
    reply.resolve();
  }
  await expect(page.getByRole('heading', { name: 'Order confirmed' })).toBeVisible();
  await expect(page.locator('#global-search')).toBeFocused();
});

test('payment status resumes after a persisted page restore and accepts manual refresh', async ({ page }) => {
  await page.clock.install();
  await page.goto('/orders/QA-PAYMENT/confirmation');
  const button = page.getByRole('button', { name: 'Check payment status' });
  // Emit the browser lifecycle events directly to exercise the same document in every engine.
  const restored = page.waitForResponse(response => response.url().includes('/ui/orders/status?') && response.ok());
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
  });
  await restored;
  await expect(button).toBeEnabled();
  const manual = page.waitForResponse(response => response.url().includes('/ui/orders/status?') && response.ok());
  await button.press('Enter');
  await manual;
  await expect(button).toBeEnabled();
});

async function completeOrderInAnotherTab(context, path) {
  const other = await context.newPage();
  try {
    await other.goto(path);
    await other.getByRole('button', { name: 'Mark as delivered' }).click();
    await expect(other.locator('[data-seller-order-detail]')).toContainText('Fulfilment: Delivered');
    await expect(other.locator('[data-fulfillment-form]')).toHaveCount(0);
  } finally {
    await other.close();
  }
}

test('seller refresh preserves edited input when another tab completes fulfillment', async ({ page, context }, testInfo) => {
  const order = `QA-SELLER-${testInfo.testId}-${testInfo.retry}`;
  const path = `/seller-admin/orders/${order}`;
  const filters = '?q=mugs&tab=progress&page=2';
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(path + filters);
  const form = page.locator('[data-fulfillment-form]');
  await form.locator('[name=note]').fill('Draft tracking note');
  await page.getByRole('button', { name: 'Pause updates' }).click();

  await completeOrderInAnotherTab(context, path);

  await page.getByRole('button', { name: 'Resume updates' }).click();
  const conflict = page.locator('[data-order-conflict]');
  await expect(conflict).toBeVisible();
  await expect(form.locator('[name=note]')).toHaveValue('Draft tracking note');
  await expect(form.getByRole('button', { name: 'Mark as delivered' })).toBeDisabled();
  const reload = conflict.getByRole('link', { name: 'Reload latest order' });
  await expect(reload).toHaveAttribute('href', path + filters);
  await reload.click();
  await expect(page.locator('[data-seller-order-detail]')).toContainText('Fulfilment: Delivered');
  await expect(form).toHaveCount(0);
  await expect(page.locator('[data-print-document=label]')).toContainText('QA-NEW-TRACKING');
});

test('seller refresh updates a pristine view and print data together', async ({ page, context }, testInfo) => {
  const path = `/seller-admin/orders/QA-SELLER-${testInfo.testId}-${testInfo.retry}`;
  await page.addInitScript(() => { window.print = () => { window.fixturePrinted = document.body.dataset.printKind; }; });
  await page.goto(path);
  const detail = page.locator('[data-seller-order-detail]');
  const printing = detail.locator('[data-print-document=label]');
  await expect(printing).toContainText('QA-OLD-TRACKING');
  await page.getByRole('button', { name: 'Pause updates' }).click();
  await completeOrderInAnotherTab(context, path);
  await page.getByRole('button', { name: 'Resume updates' }).click();
  await expect(detail).toContainText('Fulfilment: Delivered');
  await expect(detail.locator('[data-fulfillment-form]')).toHaveCount(0);
  await expect(printing).toContainText('QA-NEW-TRACKING');
  const print = detail.getByRole('button', { name: 'Print address sheet', exact: true });
  await print.click();
  await expect.poll(() => page.evaluate(() => window.fixturePrinted)).toBe('label');
});

test('seller creates a parcel by keyboard and a declined waybill leaves it at packing', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const path = '/seller-admin/orders/QA-SHIPPING?shiprocket';
  await page.route('**/seller-admin/orders/shiprocket', async route => {
    const data = new URLSearchParams(route.request().postData());
    expect(data.get('seller_id')).toBe('42');
    if (data.get('action') === 'create') {
      expect(data.get('weight_grams')).toBe('500');
      expect(data.get('length_cm')).toBe('15');
      await route.continue();
    } else {
      expect(data.get('action')).toBe('assign');
      expect(data.get('courier_id')).toBe('51');
      await route.fulfill({ status: 422, contentType: 'text/plain', body: 'Shiprocket declined this action.' });
    }
  });
  await page.goto(path);
  const parcel = page.getByRole('region', { name: 'Shiprocket shipment' });
  await parcel.getByLabel('Packed weight (g)').fill('500');
  await parcel.getByLabel('Length (cm)').fill('15');
  await parcel.getByLabel('Width (cm)').fill('12');
  await parcel.getByLabel('Height (cm)').fill('8');
  await parcel.getByRole('button', { name: 'Create Shiprocket shipment' }).press('Enter');
  await expect(parcel).toContainText('Courier status: New');
  await expect(page.locator('[data-fulfillment-form]')).toHaveCount(0);
  await parcel.getByRole('link', { name: 'Check available couriers' }).press('Enter');
  const courier = parcel.getByRole('combobox', { name: 'Courier' });
  await expect(courier.locator('option[value="51"]')).toHaveText('Xpressbees Surface · ₹54.00 · 4 days');
  await courier.selectOption('51');
  await parcel.getByRole('button', { name: 'Assign courier and waybill' }).press('Enter');
  await expect(parcel.getByRole('alert')).toContainText('Shiprocket declined this action.');
  await expect(parcel.getByRole('alert')).toBeFocused();
  await expect(courier).toHaveValue('51');
  await expect(page.locator('[data-seller-order-detail]')).toContainText('Fulfilment: Packing');
  await expect(parcel.getByRole('button', { name: 'Schedule pickup' })).toHaveCount(0);
  await expect(parcel.getByRole('button', { name: 'Generate pickup manifest' })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});

test('seller generates the pickup manifest after pickup confirmation', async ({ page }) => {
  const path = '/seller-admin/orders/QA-SHIPPING?picked-up';
  await page.route('**/seller-admin/orders/shiprocket', async route => {
    const data = new URLSearchParams(route.request().postData());
    expect(data.get('action')).toBe('manifest');
    expect(data.get('order_number')).toBe('QA-SHIPPING');
    await route.continue();
  });
  await page.goto(path);
  const parcel = page.getByRole('region', { name: 'Shiprocket shipment' });
  await parcel.getByRole('button', { name: 'Generate pickup manifest' }).press('Enter');
  const manifest = parcel.getByRole('link', { name: /open pickup manifest/i });
  await expect(manifest).toHaveAttribute('href', 'https://shiprocket.in/test-manifest.pdf');
  await expect(parcel.getByRole('button', { name: 'Generate pickup manifest' })).toHaveCount(0);
});

test('admin order queue distinguishes empty, filtered, and unavailable', async ({ page }) => {
  const queue = page.locator('[data-admin-orders-region]');
  await page.goto('/admin/orders');
  await expect(queue).toContainText('No orders have been placed.');
  await page.goto('/admin/orders?stage=delivered');
  await expect(queue).toContainText('No orders match the current filters.');
  await queue.getByRole('link', { name: /clear filters/i }).click();
  expect(new URL(page.url()).search).toBe('');
  await expect(queue).toContainText('No orders have been placed.');
  await page.goto('/admin/orders?unavailable');
  await expect(queue).toContainText('Order queue unavailable.');
  await expect(queue).not.toContainText('No orders have been placed.');

  await page.goto('/admin/orders?populated');
  await expect(queue).toContainText('QA-ADMIN-ORDER');
  await page.route('**/ui/admin/orders/status?*', route => route.fulfill({ status: 503 }));
  await page.getByRole('button', { name: 'Pause updates' }).click();
  await page.getByRole('button', { name: 'Resume updates' }).click();
  const retry = page.getByRole('button', { name: 'Retry update' });
  await expect(retry).toBeVisible();
  await expect(queue).toContainText('QA-ADMIN-ORDER');
  await page.unroute('**/ui/admin/orders/status?*');
  await retry.click();
  await expect(retry).toBeHidden();
  await expect(queue).toContainText('QA-ADMIN-ORDER');
});

test('operator can reach the delivery queue and approve manual dispatch by keyboard', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route('**/admin/delivery-exceptions', async route => {
    if (route.request().method() !== 'POST') return route.continue();
    const request = route.request();
    const body = await new Response(request.postDataBuffer(), {
      headers: { 'Content-Type': request.headers()['content-type'] },
    }).formData();
    expect(body.get('action')).toBe('resolve');
    expect(body.get('kind')).toBe('cancelled_parcel');
    expect(body.get('reason')).toBe('Verified cancellation and booked the manual courier.');
    return route.continue();
  });
  await page.goto('/admin/orders');
  await page.getByRole('link', { name: 'Delivery exceptions' }).press('Enter');
  const shipment = page.getByRole('row').filter({ hasText: 'QA-SHIPPING' });
  await expect(shipment).toContainText('CANCELLED');
  await shipment.getByLabel('Resolution').fill('Verified cancellation and booked the manual courier.');
  await shipment.getByRole('button', { name: 'Allow manual dispatch' }).press('Enter');
  await expect(page).toHaveURL(/delivery-exceptions\?notice=saved$/);
});
