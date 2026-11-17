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
  await admin.page.goto(`${base}${admin.root}?tab=setup`);
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


test('Super Admin setup uses names, saved profiles and the correct navigation on desktop and phone',async({browser})=>{
 for(const width of [1440,390]){
  const {context,page,root}=await rolePage(browser,'super_admin',width);
  await page.goto(`${base}${root}`);
  await expect(page.getByRole('link',{name:'Fulfillment & exceptions',exact:true})).toHaveAttribute('aria-current','page');
  await expect(page.getByRole('link',{name:'Orders & delivery',exact:true})).not.toHaveAttribute('aria-current','page');
  await expect(page.getByRole('heading',{name:'Recent orders',exact:true})).toBeVisible();
  await page.getByRole('link',{name:'Setup',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Seller and product setup',exact:true})).toBeVisible();
  await expect(page.getByRole('heading',{name:'Recent orders',exact:true})).toHaveCount(0);
  await page.getByLabel('Seller',{exact:true}).selectOption(String(fixture.SellerID));
  await page.getByRole('button',{name:'Load seller',exact:true}).click();
  await page.getByLabel('Product option',{exact:true}).selectOption(String(fixture.VariantID));
  await page.getByRole('button',{name:'Load product option',exact:true}).click();
  await expect(page.getByLabel('Legal name',{exact:true})).toHaveValue('Verified craft business');
  await expect(page.getByLabel('Invoice prefix',{exact:true})).toHaveAttribute('readonly','');
  const state=page.getByLabel('State / Union Territory',{exact:true});
  await state.focus();await page.keyboard.press('Home');await page.keyboard.press('t');await page.keyboard.press('Enter');
  await state.selectOption('36');await expect(state).toHaveValue('36');
  await page.getByLabel('GST registration',{exact:true}).selectOption('registered');
  await expect(page.getByLabel('GSTIN (for registered sellers)',{exact:true})).toBeVisible();
  await page.getByLabel('GST registration',{exact:true}).selectOption('unregistered');
  await expect(page.getByLabel('GSTIN (for registered sellers)',{exact:true})).toBeHidden();
  await page.getByRole('link',{name:'Operations',exact:true}).click();
  await page.getByRole('button',{name:'Keep editing',exact:true}).click();
  await expect(state).toHaveValue('36');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await page.screenshot({path:path.resolve(`.tmp/fulfillment-setup-${width}.png`),fullPage:true});
  await page.getByRole('link',{name:'Operations',exact:true}).click();
  await page.getByRole('button',{name:'Discard changes',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Recent orders',exact:true})).toBeVisible();
  await context.close();
 }
});

test('Setup retains invalid values and saves exact tax percentages without changing launch mode',async({browser})=>{
 const {context,page,root}=await rolePage(browser,'super_admin');
 await page.goto(`${base}${root}?tab=setup&seller_id=${fixture.SellerID}&variant_id=${fixture.VariantID}`);
 const tax=page.locator('form[data-fulfillment-setup-action="save_tax_profile"]');
 // Bypass the browser pattern intentionally to exercise server-side failure recovery.
 await tax.getByLabel('HSN (4–8 digits)',{exact:true}).fill('44');
 await tax.getByLabel('Tax rate (%)',{exact:true}).fill('5.555');
 await tax.getByLabel('Verification reason',{exact:true}).fill('Checked fixture classification');
 const rejected=await page.request.post(`${base}${root}/action`,{form:{action:'save_tax_profile',csrf_token:fixture.Roles.super_admin.CSRF,variant_id:String(fixture.VariantID),hsn:'44',tax_rate_percent:'5.555',reason:'Checked fixture classification',setup_context:`seller_id=${fixture.SellerID}&variant_id=${fixture.VariantID}`}});
 expect(rejected.status()).toBe(422);
 const html=await rejected.text();expect(html).toContain('up to two decimal places');expect(html).toContain('value="5.555"');expect(html).toContain('aria-invalid="true"');
 await tax.getByLabel('HSN (4–8 digits)',{exact:true}).evaluate(field=>field.removeAttribute('pattern'));
 await tax.getByLabel('Tax rate (%)',{exact:true}).evaluate(field=>field.removeAttribute('pattern'));
 const failedSave=page.waitForResponse(response=>response.url().endsWith('/fulfillment/action')&&response.request().method()==='POST');
 await tax.getByRole('button',{name:'Verify and save product tax settings'}).click();
 expect((await failedSave).status()).toBe(422);
 await expect(tax.locator('.ui-error-summary')).toContainText('up to two decimal places');
 await expect(tax.getByLabel('Tax rate (%)',{exact:true})).toHaveValue('5.555');

 await tax.getByLabel('HSN (4–8 digits)',{exact:true}).fill('4420');
 await tax.getByLabel('Tax rate (%)',{exact:true}).fill('5.50');
 await save(page,tax.getByRole('button',{name:'Verify and save product tax settings'}));
 await expect(page.getByLabel('Tax rate (%)',{exact:true})).toHaveValue('5.50');
 await expect(page.getByText('Product tax settings verified and saved.',{exact:true})).toBeVisible();
 await expect(page.locator('#setup-policy')).toContainText('off');
 expect(page.url()).not.toContain('Checked');
 await context.close();
});

test('Verified pickup lookup offers retry and named choices',async({browser})=>{
 const {context,page,root}=await rolePage(browser,'super_admin');
 await page.goto(`${base}${root}?tab=setup&seller_id=${fixture.SellerID}`);
 await page.getByRole('button',{name:'Load or refresh verified Shiprocket pickups'}).click();
 await expect(page.getByText('Shiprocket pickup locations could not be loaded. Retry when the provider is available.',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Load or refresh verified Shiprocket pickups'}).click();
 const choices=page.getByLabel('Verified pickup location',{exact:true});
 await expect(choices).toBeVisible();await choices.selectOption('Fixture Depot');
 await expect(choices.locator('option:checked')).toHaveText('Fixture Depot · PIN 411001');
 await context.close();
});


test('Invoice state saves by selection and other roles cannot verify profiles',async({browser})=>{
 const admin=await rolePage(browser,'super_admin');
 await admin.page.goto(`${base}${admin.root}?tab=setup&seller_id=${fixture.SellerID}&variant_id=${fixture.VariantID}`);
 const invoice=admin.page.locator('[data-fulfillment-setup-action="save_invoice_profile"]');
 await invoice.getByLabel('State / Union Territory',{exact:true}).selectOption('36');
 await invoice.getByLabel('Verification reason',{exact:true}).fill('Fixture operator checked legal evidence');
 await save(admin.page,invoice.getByRole('button',{name:'Verify and save invoice details'}));
 await expect(admin.page.getByLabel('State / Union Territory',{exact:true})).toHaveValue('36');
 await expect(admin.page.getByText('Seller invoice details verified and saved.',{exact:true})).toBeVisible();
 expect(admin.page.url()).not.toContain('Fixture+operator');
 await admin.context.close();
 for(const role of ['customer','seller','admin']){
  const {context,page,root}=await rolePage(browser,role);
  const surface=await page.request.get(`${base}${root}?tab=setup`);
  expect(surface.status()).toBe(200);expect(await surface.text()).not.toContain('name="legal_name"');
  const denied=await page.request.post(`${base}${root}/action`,{form:{action:'save_tax_profile',csrf_token:fixture.Roles[role].CSRF,variant_id:String(fixture.VariantID),hsn:'4420',tax_rate_percent:'0',reason:'Forbidden fixture attempt'}});
  expect(denied.status()).toBe(403);
  await context.close();
 }
});
