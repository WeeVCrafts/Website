const {test,expect}=require('@playwright/test');
const fs=require('node:fs');
const path=require('node:path');
let fixture,base;
test.beforeAll(()=>{
 if(!process.env.MARKETPLACE_UI_FIXTURE_FILE||!process.env.MARKETPLACE_PREVIEW_URL_FILE) throw new Error('Authenticated cross-role fixtures are required.');
 fixture=JSON.parse(fs.readFileSync(process.env.MARKETPLACE_UI_FIXTURE_FILE,'utf8'));
 base=fs.readFileSync(process.env.MARKETPLACE_PREVIEW_URL_FILE,'utf8').trim();
 expect(new URL(base).hostname).toBe('127.0.0.1');
});
async function actor(browser,role,width=1440){
 const context=await browser.newContext({viewport:{width,height:1000}});
 await context.addCookies([{name:'wecratfs_session',value:fixture.Roles[role].Token,url:base,httpOnly:true,sameSite:'Lax'}]);
 const page=await context.newPage();
 const root=(role==='customer'||role==='legacy_customer')?'':`/${role.replace('_','-')==='super-admin'?'super-admin':(role==='seller'||role==='second_seller'||role==='legacy_seller')?'seller-admin':'admin'}`;
 return {context,page,root};
}
async function audit(page){
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 await page.keyboard.press('Tab');expect(await page.evaluate(()=>document.activeElement.tagName!=='BODY')).toBe(true);
 await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});
 expect(await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}})).violations.map(v=>({id:v.id,targets:v.nodes.map(n=>n.target)})))).toEqual([]);
}
test('Normal order links share role-scoped financial, package and request views',async({browser})=>{
 test.setTimeout(240000);
 for(const role of ['customer','seller','admin','super_admin']){
  for(const width of [1440,390]){
   const a=await actor(browser,role,width);
   const errors=[];a.page.on('pageerror',e=>errors.push(e.message));
   for(const tab of ['overview','packages','documents','returns','support']){
    const response=await a.page.goto(`${base}${a.root}/orders/${fixture.Order}?tab=${tab}`);
    expect(response.status()).toBe(200);expect(response.headers()['cache-control']).toContain('no-store');
    await expect(a.page.getByRole('heading',{name:`Order ${fixture.Order}`,exact:true})).toBeVisible();
    await expect(a.page.getByRole('navigation',{name:'Order sections'}).getByRole('link',{name:tab==='returns'?'Returns & refunds':new RegExp(tab,'i')})).toHaveAttribute('aria-current','page');
    if(tab==='overview')await expect(a.page.getByRole('heading',{name:'Financial summary',exact:true})).toBeVisible();
    if(tab==='documents'&&role==='seller')await expect(a.page.getByRole('link',{name:/^Download invoice/})).toBeVisible();
    await audit(a.page);
   }
   await a.page.screenshot({path:path.resolve(`.tmp/cross-role-${role}-${width}.png`),fullPage:true});
   expect(errors).toEqual([]);
   const requests=await a.page.goto(`${base}${a.root}/returns`);expect(requests.status()).toBe(200);
   await expect(a.page.getByRole('heading',{name:'Returns & refunds',exact:true})).toBeVisible();await audit(a.page);
   const foreign=await a.page.request.get(`${base}${role==='customer'?'/seller-admin':'/account'}/fulfillment`);expect(foreign.status()).toBe(403);
   await a.context.close();
  }
 }
});
test('Seller setup is discoverable, validates entries, preserves drafts and requires Super Admin review',async({browser})=>{
 const seller=await actor(browser,'seller',390);
 await seller.page.goto(`${base}/seller-admin/products`);
 await seller.page.getByRole('link',{name:'Open fulfillment setup',exact:true}).click();
 await expect(seller.page.getByRole('heading',{name:'Fulfillment setup',exact:true})).toBeVisible();await audit(seller.page);
 await seller.page.getByRole('link',{name:'Open invoice details',exact:true}).click();
 const form=seller.page.locator('form').filter({has:seller.page.getByRole('button',{name:'Save draft',exact:true})});
 await form.getByLabel('Legal name',{exact:true}).fill('');
 await form.getByRole('button',{name:'Submit for review',exact:true}).click();
 await expect(form.getByLabel('Legal name',{exact:true})).toHaveAttribute('aria-invalid','true');
 await expect(form.getByLabel('Legal name',{exact:true})).toBeFocused();
 await form.getByLabel('Legal name',{exact:true}).fill('Fixture reviewed legal identity');
 await form.getByLabel('State or union territory',{exact:true}).focus();await seller.page.keyboard.press('ArrowDown');await seller.page.keyboard.press('Escape');
 await form.getByLabel('State or union territory',{exact:true}).selectOption('27');
 await form.getByLabel('Registered invoice address',{exact:true}).fill('abc');
 await save(seller.page,form.getByRole('button',{name:'Save draft',exact:true}),'/seller-admin/setup');
 await expect(form.getByLabel('Legal name',{exact:true})).toHaveValue('Fixture reviewed legal identity');
 await expect(form.getByLabel('Registered invoice address',{exact:true})).toHaveValue('abc');
 await form.getByLabel('Registered invoice address',{exact:true}).fill('Verified isolated merchant address Pune 411001');
 await save(seller.page,form.getByRole('button',{name:'Submit for review',exact:true}),'/seller-admin/setup');
 await expect(seller.page.getByText('Submitted',{exact:true}).first()).toBeVisible();
 const id=new URL(seller.page.url()).searchParams.get('submission_id');expect(id).toMatch(/^\d+$/);
 const denied=await seller.page.request.get(`${base}/super-admin/fulfillment/reviews/${id}`);expect(denied.status()).toBe(403);
 const admin=await actor(browser,'super_admin',390);
 await admin.page.goto(`${base}/super-admin/fulfillment/reviews/${id}`);await audit(admin.page);
 await admin.page.screenshot({path:path.resolve('.tmp/cross-role-review-390.png'),fullPage:true});
 await expect(admin.page.locator('td[data-label="Submitted details"]').filter({hasText:'Fixture reviewed legal identity'})).toBeVisible();
 await admin.page.getByLabel('Review note',{exact:true}).fill('Verified isolated fixture legal evidence; no real merchant facts changed.');
 await admin.page.getByLabel(/I verified/).check();
 await save(admin.page,admin.page.getByRole('button',{name:'Approve verified details',exact:true}),`/fulfillment/reviews/${id}`);
 await seller.page.reload();await expect(seller.page.getByText('Approved',{exact:true}).first()).toBeVisible();
 await seller.page.goto(`${base}/seller-admin/setup?kind=tax&variant_id=${fixture.VariantID}`);
 const tax=seller.page.locator('form').filter({has:seller.page.getByRole('button',{name:'Save draft',exact:true})});
 await tax.getByLabel('Verified tax rate (%)',{exact:true}).fill('');
 await save(seller.page,tax.getByRole('button',{name:'Save draft',exact:true}),'/seller-admin/setup');
 await expect(tax.getByLabel('Verified tax rate (%)',{exact:true})).toHaveValue('');
 await tax.getByLabel('Verified tax rate (%)',{exact:true}).fill('5.555');
 await tax.getByRole('button',{name:'Save draft',exact:true}).click();
 await expect(tax.getByLabel('Verified tax rate (%)',{exact:true})).toHaveAttribute('aria-invalid','true');
 await expect(tax.getByLabel('Verified tax rate (%)',{exact:true})).toHaveValue('5.555');
 await tax.getByLabel('Verified tax rate (%)',{exact:true}).fill('0');
 await tax.getByLabel('HSN (4–8 digits)',{exact:true}).fill('4420');
 await tax.getByLabel('Return category',{exact:true}).selectOption('fragile_art');
 await save(seller.page,tax.getByRole('button',{name:'Submit for review',exact:true}),'/seller-admin/setup');
 const taxID=new URL(seller.page.url()).searchParams.get('submission_id');
 await admin.page.goto(`${base}/super-admin/fulfillment/reviews/${taxID}`);
 await admin.page.getByLabel('Review note',{exact:true}).fill('Verified isolated tax and return classification.');
 await admin.page.getByLabel(/I verified/).check();
 await save(admin.page,admin.page.getByRole('button',{name:'Approve verified details',exact:true}),`/fulfillment/reviews/${taxID}`);
 await seller.page.reload();await expect(seller.page.getByText('Approved',{exact:true}).first()).toBeVisible();
 await seller.context.close();await admin.context.close();
});

test('Company setup review is role restricted',async({browser})=>{
 for(const role of ['customer','seller','admin']){const a=await actor(browser,role);const response=await a.page.request.get(`${base}/super-admin/fulfillment/reviews`);expect(response.status()).toBe(403);await a.context.close();}
});

async function save(page,button,actionSuffix='/fulfillment/action') {
 const navigation=page.waitForEvent('framenavigated',{predicate:frame=>frame===page.mainFrame()});
 const response=page.waitForResponse(r=>r.url().endsWith(actionSuffix)&&r.request().method()==='POST');
 await button.click();
 const dialog=page.getByRole('dialog');
 if(await dialog.isVisible()) await dialog.locator('[data-confirm-accept]').click();
 expect((await response).status()).toBe(303);await navigation;await page.waitForLoadState('networkidle');
}

test('Two sellers fulfill independent packages after a partial refund; all roles retain the same item facts',async({browser})=>{
 test.setTimeout(180000);
 expect(fixture.MixedOrder).toBeTruthy();
 const customer=await actor(browser,'customer',390);
 await customer.page.goto(`${base}/orders/${fixture.MixedOrder}`);
 await expect(customer.page.getByText('Payment: Partially refunded',{exact:true})).toBeVisible();
 await expect(customer.page.getByRole('rowheader',{name:/Second seller private product/})).toBeVisible();
 await expect(customer.page.locator('.commerce-financial')).toContainText('₹10.00');
 for(const role of ['seller','second_seller']){
  const seller=await actor(browser,role,390);
  await seller.page.goto(`${base}/seller-admin/orders/${fixture.MixedOrder}?tab=overview`);
  await expect(seller.page.locator('.commerce-panel table tbody tr')).toHaveCount(1);
  if(role==='seller') await expect(seller.page.getByRole('rowheader',{name:/Second seller private product/})).toHaveCount(0);
  await seller.page.getByRole('navigation',{name:'Order sections'}).getByRole('link',{name:/^Packages/}).click();
  const accept=seller.page.getByRole('button',{name:'Accept order',exact:true});
  if(await accept.count()) await save(seller.page,accept);
  const start=seller.page.getByRole('button',{name:'Start packing',exact:true});
  if(await start.count()) await save(seller.page,start);
  const pack=seller.page.locator('form').filter({has:seller.page.getByRole('button',{name:'Save package',exact:true})});
  await pack.locator('input[name^="item_"]').fill('2');
  for(const [label,value] of [['Weight (grams)','500'],['Length (cm)','15'],['Width (cm)','12'],['Height (cm)','8']]) await pack.getByLabel(label,{exact:true}).fill(value);
  await save(seller.page,pack.getByRole('button',{name:'Save package',exact:true}));
  await seller.page.getByLabel(/I confirm the package contents/).check();
  await seller.page.getByLabel('Pre-dispatch quality-check evidence',{exact:true}).fill('Isolated fixture pre-dispatch inspection confirms packaging and contents.');
  await save(seller.page,seller.page.getByRole('button',{name:'Ready to ship',exact:true}));
  await expect(seller.page.getByRole('link',{name:'Download carrier label',exact:true})).toHaveCount(0);
  const handover=seller.page.locator('form').filter({has:seller.page.getByRole('button',{name:'Record courier handover',exact:true})});
  await handover.getByLabel('Actual carrier',{exact:true}).fill('Isolated fixture carrier');
  await handover.getByLabel('Actual tracking number',{exact:true}).fill(`ISOLATED-${role}`);
  await handover.getByLabel('Courier handover / delivery evidence').fill('Isolated browser test handover; no external booking or real delivery.');
  await save(seller.page,handover.getByRole('button',{name:'Record courier handover',exact:true}));
  await audit(seller.page);await seller.context.close();
 }
 const admin=await actor(browser,'super_admin');
 await admin.page.goto(`${base}/super-admin/orders/${fixture.MixedOrder}?tab=packages`);
 await expect(admin.page.getByText('ISOLATED-seller',{exact:false}).first()).toBeVisible();
 for(let i=0;i<2;i++){
  const delivery=admin.page.locator('form').filter({has:admin.page.getByRole('button',{name:'Confirm verified delivery',exact:true})}).first();
  await delivery.getByLabel('Courier handover / delivery evidence').fill('Isolated simulated delivery evidence; not a production delivery.');
  await save(admin.page,delivery.getByRole('button',{name:'Confirm verified delivery',exact:true}));
 }
 await expect(admin.page.getByText('Order: Completed',{exact:true})).toBeVisible();
 await customer.page.goto(`${base}/orders/${fixture.MixedOrder}?tab=packages`);
 await expect(customer.page.locator('.commerce-tabs')).toContainText('Packages (2)');
 await expect(customer.page.getByText('Order: Completed',{exact:true})).toBeVisible();
 await customer.page.getByRole('link',{name:'Documents',exact:true}).click();
 const invoices=customer.page.getByRole('link',{name:/^Download invoice/});await expect(invoices).toHaveCount(2);
 for(const invoice of await invoices.all()){const response=await customer.page.request.get(new URL(await invoice.getAttribute('href'),base).href);expect(response.status()).toBe(200);expect((await response.body()).subarray(0,4).toString()).toBe('%PDF');}
 await customer.page.goto(`${base}/returns`);await expect(customer.page.getByText(fixture.MixedOrder,{exact:true})).toBeVisible();
 await customer.context.close();await admin.context.close();
});

test('Historical orders keep recorded delivery progress and their customer payment receipt',async({browser})=>{
 expect(fixture.LegacyOrder).toBeTruthy();
 for(const role of ['legacy_customer','legacy_seller','super_admin']){
  const a=await actor(browser,role,390);
  await a.page.goto(`${base}${a.root}/orders/${fixture.LegacyOrder}?tab=packages`);
  await expect(a.page.getByText('Payment: Captured',{exact:true})).toBeVisible();
  await expect(a.page.getByRole('heading',{name:'Historical delivery progress',exact:true})).toBeVisible();
  await expect(a.page.getByText(/HISTORICAL-SAVED-TRACK/)).toBeVisible();
  await expect(a.page.getByRole('button',{name:'Accept order',exact:true})).toHaveCount(0);
  await audit(a.page);
  if(role==='legacy_customer'){
   await a.page.getByRole('navigation',{name:'Order sections'}).getByRole('link',{name:'Documents',exact:true}).click();
   const receipt=a.page.locator('.commerce-workspace').getByRole('link',{name:'Download payment receipt',exact:true}).last();await expect(receipt).toBeVisible();
   const response=await a.page.request.get(new URL(await receipt.getAttribute('href'),base).href);expect(response.status()).toBe(200);expect((await response.body()).subarray(0,4).toString()).toBe('%PDF');
  }
  await a.context.close();
 }
});

test('Seller parcel default is previewed before it is applied',async({browser})=>{
 const seller=await actor(browser,'seller',390);
 try {
  await seller.page.goto(`${base}/seller-admin/fulfillment?tab=setup`);
  await seller.page.getByRole('link',{name:'Manage parcel profiles'}).press('Enter');
  await expect(seller.page.getByRole('heading',{name:'Parcel profiles',exact:true})).toBeVisible();
  const name='Isolated parcel profile '+Date.now();
  await seller.page.getByLabel('Profile name').fill(name);
  await seller.page.getByLabel('Packed weight (g)',{exact:true}).fill('600');
  await seller.page.getByLabel('Length (cm)',{exact:true}).fill('25');
  await seller.page.getByLabel('Width (cm)',{exact:true}).fill('20');
  await seller.page.getByLabel('Height (cm)',{exact:true}).fill('8');
  await seller.page.getByRole('button',{name:'Create profile'}).press('Enter');
  const select=seller.page.getByRole('combobox',{name:'Seller default',exact:true});
  const [profile]=await select.selectOption({label:name+' · 600 g · 25 × 20 × 8 cm'});
  await seller.page.getByRole('button',{name:'Preview default change'}).press('Enter');
  await expect(seller.page.getByRole('heading',{name:'Review the change'})).toBeFocused();
  await expect(seller.page.getByText(name,{exact:false})).toBeVisible();
  await seller.page.getByRole('button',{name:'Apply change'}).press('Enter');
  await expect(select).toHaveValue(profile);
  expect(await seller.page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 } finally {
  await seller.context.close();
 }
});
