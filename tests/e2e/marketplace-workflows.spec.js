const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

const fixturePath = process.env.MARKETPLACE_UI_FIXTURE_FILE;
const urlPath = process.env.MARKETPLACE_PREVIEW_URL_FILE;
test.skip(!fixturePath || !urlPath, 'Requires the dedicated authenticated marketplace preview and disposable fixtures.');
let fixture, base;
test.beforeAll(() => {
  fixture = JSON.parse(fs.readFileSync(fixturePath, 'utf8'));
  base = fs.readFileSync(urlPath, 'utf8').trim();
  expect(new URL(base).hostname).toBe('127.0.0.1');
});

test('HTML policy and claim evidence work at phone and desktop widths', async ({browser}) => {
  test.skip(!fixture.PolicyOrder, 'Policy fixture is opt-in.');
  for(const width of [1440,390]){
    const {context,page,root}=await rolePage(browser,'customer',width);
    const policy=await page.goto(`${base}/returns-replacements-authenticity`);
    expect(policy.status()).toBe(200);
    await expect(page.getByRole('heading',{name:'Returns, replacements & authenticity',exact:true})).toBeVisible();
    await expect(page.getByRole('heading',{name:'Using store credit',exact:true})).toBeVisible();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    await page.keyboard.press('Tab');
    expect(await page.evaluate(()=>document.activeElement.tagName!=='BODY')).toBe(true);
    await page.screenshot({path:path.resolve(`.tmp/return-policy-${width}.png`),fullPage:true});
    await page.goto(`${base}${root}`);
    await expect(page.getByRole('heading',{name:'Your store credit',exact:true})).toBeVisible();
    await page.goto(`${base}${root}?order=${encodeURIComponent(fixture.PolicyOrder)}`);
    await expect(page.getByRole('button',{name:'Submit claim for review'})).toBeVisible();
    const upload=page.locator('form').filter({has:page.getByRole('button',{name:'Upload private evidence',exact:true})});
    await upload.getByLabel('Unboxing video or photo').setInputFiles({name:'unboxing.mp4',mimeType:'video/mp4',buffer:Buffer.from([0,0,0,24,...Buffer.from('ftypisom'),0,0,0,0,...Buffer.from('fixture-video')])});
    await save(page,upload.getByRole('button',{name:'Upload private evidence'}));
    const download=page.getByRole('link',{name:'unboxing.mp4',exact:true}).last();
    const result=await page.request.get(new URL(await download.getAttribute('href'),base).href);
    expect(result.status()).toBe(200);
    expect(result.headers()['cache-control']).toContain('no-store');
    expect(result.headers()['content-disposition']).toContain('attachment');
    expect(result.headers()['x-content-type-options']).toBe('nosniff');
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    await page.screenshot({path:path.resolve(`.tmp/return-claim-${width}.png`),fullPage:true});
    await context.close();
  }
  const admin=await rolePage(browser,'super_admin',390);
  await admin.page.goto(`${base}${admin.root}`);
  await expect(admin.page.getByRole('heading',{name:'Policy and live shipping readiness'})).toBeVisible();
  await expect(admin.page.getByRole('button',{name:'Enable one-order pilot'})).toBeVisible();
  expect(await admin.page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await admin.page.screenshot({path:path.resolve('.tmp/policy-super-admin-390.png'),fullPage:true});
  await admin.context.close();
});
async function rolePage(browser, role, width = 1440) {
  const context = await browser.newContext({ viewport: { width, height: 1000 } });
  await context.addCookies([{ name: 'wecratfs_session', value: fixture.Roles[role].Token, url: base, httpOnly: true, sameSite: 'Lax' }]);
  const page = await context.newPage();
  const root = role === 'customer' ? '/account/fulfillment' : `/${role === 'super_admin' ? 'super-admin' : role === 'seller' ? 'seller-admin' : 'admin'}/fulfillment`;
  return { context, page, root };
}
async function openOrder(page, root) {
  const response = await page.goto(`${base}${root}?order=${encodeURIComponent(fixture.Order)}`);
  expect(response.status()).toBe(200);
  await expect(page.getByRole('heading', { name: `Order ${fixture.Order}`, exact: true })).toBeVisible();
}
async function save(page, button) {
  const navigation = page.waitForEvent('framenavigated', { predicate: frame => frame === page.mainFrame() });
  const response = page.waitForResponse(r => r.url().endsWith('/fulfillment/action') && r.request().method() === 'POST');
  await button.click();
  expect((await response).status()).toBe(303);
  await navigation;
  await page.waitForLoadState('networkidle');
}

test.describe.serial('Seller-controlled fulfillment', () => {
  test('four role surfaces render at desktop and phone widths', async ({ browser }) => {
    for (const role of ['customer', 'seller', 'admin', 'super_admin']) {
      for (const width of [1440, 390]) {
        const { context, page, root } = await rolePage(browser, role, width);
        await openOrder(page, root);
        await expect(page.locator('.marketplace-workspace')).toBeVisible();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
        await page.keyboard.press('Tab');
        expect(await page.evaluate(() => document.activeElement.tagName !== 'BODY')).toBe(true);
        await page.screenshot({ path: path.resolve(`.tmp/marketplace-${role}-${width}.png`), fullPage: true });
        await context.close();
      }
    }
  });
  test('seller accepts, packs, confirms a package and records actual fallback handover', async ({ browser }) => {
    const { context, page, root } = await rolePage(browser, 'seller');
    await openOrder(page, root);
    await save(page, page.getByRole('button', { name: 'Accept order', exact: true }));
    await expect(page.getByRole('button', { name: 'Start packing' })).toBeVisible();
    await save(page, page.getByRole('button', { name: 'Start packing' }));
    const packageForm = page.locator('form').filter({ has: page.getByRole('button', { name: 'Save package', exact: true }) });
    await packageForm.locator('input[name^="item_"]').fill('3');
    await packageForm.getByLabel('Weight (grams)').fill('500');
    await packageForm.getByLabel('Length (cm)').fill('15');
    await packageForm.getByLabel('Width (cm)').fill('12');
    await packageForm.getByLabel('Height (cm)').fill('8');
    await save(page, packageForm.getByRole('button', { name: 'Save package' }));
    await page.getByLabel(/I confirm the package contents/).check();
    await save(page, page.getByRole('button', { name: 'Ready to ship', exact: true }));
    const handover = page.locator('form').filter({ has: page.getByRole('button', { name: 'Record courier handover' }) });
    await handover.getByLabel('Actual carrier', { exact: true }).fill('Isolated test carrier');
    await handover.getByLabel('Actual tracking number', { exact: true }).fill('TEST-TRACK-001');
    await handover.getByLabel('Courier handover / delivery evidence').fill('Isolated browser fixture confirms physical handover');
    await save(page, handover.getByRole('button', { name: 'Record courier handover' }));
    await expect(page.getByText(/partially shipped/).first()).toBeVisible();
    await context.close();
  });
  test('company verifies manual delivery and customer downloads a real invoice PDF', async ({ browser }) => {
    const admin = await rolePage(browser, 'admin');
    await openOrder(admin.page, admin.root);
    const delivery = admin.page.locator('form').filter({ has: admin.page.getByRole('button', { name: 'Confirm verified delivery' }) });
    await delivery.getByLabel('Courier handover / delivery evidence').fill('Isolated fixture: operator verified courier delivery evidence');
    await save(admin.page, delivery.getByRole('button', { name: 'Confirm verified delivery' }));
    await expect(admin.page.getByText('completed', { exact: true }).first()).toBeVisible();
    await admin.context.close();
    const customer = await rolePage(browser, 'customer', 390);
    await openOrder(customer.page, customer.root);
    await expect(customer.page.getByRole('button', { name: 'Cancel before courier pickup' })).toHaveCount(0);
    const invoice = customer.page.getByRole('link', { name: /^Download invoice/ });
    await expect(invoice).toBeVisible();
    const response = await customer.page.request.get(new URL(await invoice.getAttribute('href'), base).href);
    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/pdf');
    expect((await response.body()).subarray(0, 4).toString()).toBe('%PDF');
    expect(response.headers()['cache-control']).toContain('no-store');
    const denied = await customer.page.request.get(`${base}/super-admin/fulfillment`);
    expect(denied.status()).toBe(403);
    await customer.context.close();
  });
});
