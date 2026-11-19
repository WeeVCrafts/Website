const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

test.setTimeout(120000);
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
    await page.getByRole('navigation',{name:'Order sections'}).getByRole('link',{name:'Returns & refunds',exact:true}).click();
    await page.getByRole('link',{name:'View request',exact:true}).first().click();
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
  await expect(admin.page.getByRole('heading',{name:'Finish product setup'})).toBeVisible();
  await expect(admin.page.locator('form[data-fulfillment-setup-action]')).toHaveCount(0);
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
        await expect(page.locator('.commerce-workspace')).toBeVisible();
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
    const qc=page.getByLabel('Pre-dispatch quality-check evidence',{exact:true});if(await qc.count())await qc.fill('Verified isolated pre-dispatch packaging and item inspection');
    await save(page, page.getByRole('button', { name: 'Ready to ship', exact: true }));
    const handover = page.locator('form').filter({ has: page.getByRole('button', { name: 'Record courier handover' }) });
    await handover.getByLabel('Actual carrier', { exact: true }).fill('Isolated test carrier');
    await handover.getByLabel('Actual tracking number', { exact: true }).fill('TEST-TRACK-001');
    await handover.getByLabel('Courier handover / delivery evidence').fill('Isolated browser fixture confirms physical handover');
    await save(page, handover.getByRole('button', { name: 'Record courier handover' }));
    await expect(page.getByText(/partially shipped/i).first()).toBeVisible();
    await context.close();
  });
  test('company verifies manual delivery and customer downloads a real invoice PDF', async ({ browser }) => {
    const admin = await rolePage(browser, 'admin');
    await openOrder(admin.page, admin.root);
    const delivery = admin.page.locator('form').filter({ has: admin.page.getByRole('button', { name: 'Confirm verified delivery' }) });
    await delivery.getByLabel('Courier handover / delivery evidence').fill('Isolated fixture: operator verified courier delivery evidence');
    await save(admin.page, delivery.getByRole('button', { name: 'Confirm verified delivery' }));
    await expect(admin.page.getByText('Order: Completed', { exact: true }).first()).toBeVisible();
    await admin.context.close();
    const customer = await rolePage(browser, 'customer', 390);
    await openOrder(customer.page, customer.root);
    await expect(customer.page.getByRole('button', { name: 'Cancel before courier pickup' })).toHaveCount(0);
    await customer.page.getByRole('navigation',{name:'Order sections'}).getByRole('link',{name:'Documents',exact:true}).click();
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



const setupURL=(root,step='')=>`${base}${root}?tab=setup&seller_id=${fixture.SellerID}&variant_id=${fixture.VariantID}${step?'&step='+step:''}`;

test('Guided setup shows one form, saves each task, and explains checkout gates',async({browser})=>{
 const {context,page,root}=await rolePage(browser,'super_admin',1536);await page.setViewportSize({width:1536,height:1024});
 await page.goto(`${base}${root}?tab=setup`);
 await expect(page.getByRole('heading',{name:'Finish product setup',exact:true})).toBeVisible();
 await expect(page.getByRole('link',{name:'Fulfillment & exceptions',exact:true})).toHaveAttribute('aria-current','page');
 await expect(page.locator('form[data-fulfillment-setup-action]')).toHaveCount(0);
 await page.getByLabel('Find product',{exact:true}).fill('no-such-approved-product');await page.getByRole('button',{name:'Apply filters',exact:true}).click();
 await expect(page.getByRole('heading',{name:'No approved products match'})).toBeVisible();
 await page.goto(`${base}${root}?tab=setup`);await page.screenshot({path:path.resolve('.tmp/guided-overview-1536.png')});
 await page.goto(setupURL(root,'invoice'));
 const invoice=page.locator('[data-fulfillment-setup-action="save_invoice_profile"]');
 await expect(page.locator('form[data-fulfillment-setup-action]')).toHaveCount(1);
 await expect(invoice.getByLabel('Legal name',{exact:true})).not.toHaveValue('');
 await expect(invoice.getByLabel('Invoice prefix',{exact:true})).toHaveAttribute('readonly','');
 await invoice.getByLabel('GST registration',{exact:true}).selectOption('registered');await expect(invoice.getByLabel('GSTIN (for registered sellers)')).toBeVisible();
 await invoice.getByLabel('GST registration',{exact:true}).selectOption('unregistered');await expect(invoice.getByLabel('GSTIN (for registered sellers)')).toBeHidden();
 await invoice.getByLabel('State / Union Territory',{exact:true}).selectOption('36');
 await invoice.getByLabel('Legal name',{exact:true}).fill('Verified craft business updated');
 await page.getByRole('link',{name:'Back to products',exact:true}).click();await page.getByRole('button',{name:'Keep editing',exact:true}).click();
 await expect(invoice.getByLabel('State / Union Territory',{exact:true})).toHaveValue('36');
 await invoice.getByLabel('Verification reason',{exact:true}).fill('Verified isolated fixture legal evidence');
 await page.screenshot({path:path.resolve('.tmp/guided-invoice-1536.png')});
 await save(page,invoice.getByRole('button',{name:'Save and continue',exact:true}));
 expect(page.url()).not.toContain('Verified');expect(page.url()).not.toContain('legal_name');
 await expect(page.getByRole('heading',{name:'Add packed measurements'})).toBeVisible();
 await page.goto(setupURL(root,'pickup'));
 await expect(page.getByRole('heading',{name:'Confirm seller pickup location'})).toBeVisible();
 await page.getByRole('button',{name:'Load or refresh verified Shiprocket pickups'}).click();
 await expect(page.getByText('Shiprocket pickup locations could not be loaded. Retry when the provider is available.',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Load or refresh verified Shiprocket pickups'}).click();
 const pickup=page.locator('[data-fulfillment-setup-action="save_shipping_profile"]');await pickup.getByLabel('Verified pickup location').selectOption('Fixture Depot');
 await pickup.getByLabel('Verification reason').fill('Verified isolated seller pickup');await save(page,pickup.getByRole('button',{name:'Save and continue'}));
 await expect(page.getByRole('heading',{name:'Add packed measurements'})).toBeVisible();
 await page.goto(setupURL(root,'parcel'));
 const parcel=page.locator('[data-fulfillment-setup-action="save_setup_parcel"]');
 await parcel.getByLabel('Packed weight (g)').fill('750');await parcel.getByLabel('Length (cm)').fill('30');await parcel.getByLabel('Width (cm)').fill('20');await parcel.getByLabel('Height (cm)').fill('10');
 await page.screenshot({path:path.resolve('.tmp/guided-parcel-1536.png')});await save(page,parcel.getByRole('button',{name:'Save and continue'}));
 await page.goto(setupURL(root,'tax'));await page.goto(setupURL(root,'tax'));await expect(page.getByRole('heading',{name:'Verify tax & return policy'})).toBeVisible();
 const tax=page.locator('[data-fulfillment-setup-action="save_setup_tax"]');await tax.getByLabel('HSN (4–8 digits)').fill('4420');await tax.getByLabel('Tax rate (%)').fill('0');await tax.getByLabel('Return category').selectOption('textile_craft');await tax.getByLabel('Verification reason').fill('Verified isolated product classification');
 await save(page,tax.getByRole('button',{name:'Save and continue'}));await page.goto(setupURL(root,'review'));await expect(page.getByText('Product setup complete',{exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'Checkout is still paused'})).toBeVisible();await expect(page.locator('form[data-fulfillment-setup-action]')).toHaveCount(0);
 await page.screenshot({path:path.resolve('.tmp/guided-review-1536.png')});
 await page.getByRole('link',{name:'Continue to pilot setup'}).click();await expect(page.getByRole('heading',{name:'One genuine order pilot'})).toBeVisible();await expect(page.locator('form[data-fulfillment-setup-action]')).toHaveCount(1);
 await context.close();
});

test('Guided validation retains invalid values, focuses errors and preserves exact tax precision',async({browser})=>{
 const {context,page,root}=await rolePage(browser,'super_admin');await page.goto(setupURL(root,'tax'));
 const tax=page.locator('[data-fulfillment-setup-action="save_setup_tax"]');
 await tax.getByLabel('HSN (4–8 digits)').fill('44');await tax.getByLabel('Tax rate (%)').fill('5.555');await tax.getByLabel('Return category').selectOption('fragile_art');await tax.getByLabel('Verification reason').fill('Isolated validation evidence');
 await tax.locator('[pattern]').evaluateAll(fields=>fields.forEach(field=>field.removeAttribute('pattern')));
 const rejected=page.waitForResponse(r=>r.url().endsWith('/fulfillment/action')&&r.request().method()==='POST');await tax.getByRole('button',{name:'Save and continue'}).click();expect((await rejected).status()).toBe(422);
 await expect(tax.locator('.ui-error-summary')).toContainText('up to two decimal places');await expect(tax.getByLabel('Tax rate (%)')).toHaveValue('5.555');await expect(page.locator('form[data-fulfillment-setup-action]')).toHaveCount(1);
 await expect(tax.locator('[aria-invalid="true"]').first()).toBeFocused();
 const parcel=await page.request.post(`${base}${root}/action`,{form:{action:'save_setup_parcel',csrf_token:fixture.Roles.super_admin.CSRF,variant_id:String(fixture.VariantID),weight_grams:'0',length_cm:'30',width_cm:'20',height_cm:'10',setup_context:`seller_id=${fixture.SellerID}&variant_id=${fixture.VariantID}&step=parcel`}});
 expect(parcel.status()).toBe(422);expect(await parcel.text()).toContain('Enter a whole number from 100 to 50000.');
 await context.close();
});

test('Guided screens fit phone widths and keyboard navigation preserves unfinished work',async({browser})=>{
 const {context,page,root}=await rolePage(browser,'super_admin',390);
 for(const step of ['', 'invoice','pickup','parcel','tax','review']){
  await page.goto(step?setupURL(root,step):`${base}${root}?tab=setup`);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await page.keyboard.press('Tab');expect(await page.evaluate(()=>document.activeElement.tagName!=='BODY')).toBe(true);
  await expect(page.locator('form[data-fulfillment-setup-action]')).toHaveCount(['invoice','tax','parcel'].includes(step)?1:0);
  await page.keyboard.press('Tab');
  if(step){await expect(page.locator('[data-guided-progress]')).not.toHaveAttribute('open','');await page.getByText('Setup progress',{exact:true}).focus();await page.keyboard.press('Space');await expect(page.locator('[data-guided-progress]')).toHaveAttribute('open','');await page.keyboard.press('Space');}
  await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});
  const audit=await page.evaluate(async()=>{const result=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']}});return result.violations.map(v=>({id:v.id,impact:v.impact,nodes:v.nodes.map(n=>n.target)}))});expect(audit).toEqual([]);
  await page.screenshot({path:path.resolve(`.tmp/guided-${step||'overview'}-390.png`),fullPage:true});
 }
 await page.keyboard.press('Tab');
 await page.getByRole('button',{name:'Open admin navigation'}).click();await expect(page.getByRole('link',{name:'Fulfillment & exceptions',exact:true})).toBeVisible();await page.keyboard.press('Escape');await expect(page.getByRole('button',{name:'Open admin navigation'})).toBeFocused();
 await page.goto(setupURL(root,'invoice'));await page.getByLabel('Legal name',{exact:true}).fill('Unsaved isolated edit');await page.getByRole('link',{name:'Back to products',exact:true}).click();await page.getByRole('button',{name:'Discard changes',exact:true}).click();await expect(page.getByRole('heading',{name:'Finish product setup'})).toBeVisible();
 await context.close();
});

test('Only Super Admin can save guided profiles and production activation stays guarded',async({browser})=>{
 for(const role of ['customer','seller','admin']){
  const {context,page,root}=await rolePage(browser,role);
  for(const action of ['save_setup_parcel','save_setup_tax']){
   const denied=await page.request.post(`${base}${root}/action`,{form:{action,csrf_token:fixture.Roles[role].CSRF,variant_id:String(fixture.VariantID),weight_grams:'750',length_cm:'30',width_cm:'20',height_cm:'10',hsn:'4420',tax_rate_percent:'0',return_class:'textile_craft',reason:'Forbidden isolated attempt'}});expect(denied.status()).toBe(403);
  }await context.close();
 }
 const {context,page,root}=await rolePage(browser,'super_admin');await page.goto(setupURL(root,'pilot'));
 const pilot=page.locator('[data-fulfillment-setup-action="configure_pilot"]');await pilot.getByLabel('Customer account').selectOption({index:1});await pilot.getByLabel('Activation reason').fill('Isolated test must preserve paused bookings');await pilot.getByLabel(/Wallet is funded/).check();
 const failed=page.waitForResponse(r=>r.url().endsWith('/fulfillment/action')&&r.request().method()==='POST');await pilot.getByRole('button',{name:'Enable one-order pilot'}).click();expect((await failed).status()).toBe(422);await expect(pilot.locator('.ui-error-summary')).toContainText('paused');
 await context.close();
});
