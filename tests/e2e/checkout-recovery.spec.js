const{test,expect,request}=require('@playwright/test');
const{execFileSync}=require('node:child_process');const path=require('node:path');const os=require('node:os');
const database='homepage_reference_20261103';const slug='payment-recovery-browser-qa';
function sql(query){if(process.env.WEEVCRAFTS_QA_DATABASE!==database)throw Error('Isolated QA database required');return execFileSync('docker',['exec','weevcrafts-storefront-qa-db','psql','-v','ON_ERROR_STOP=1','-U','storefrontqa','-d',database,'-tAc',query],{encoding:'utf8',windowsHide:true}).trim();}
async function login(page,admin=false){const response=await page.request.post('/api/v1/auth/login',{data:{email:admin?'demo.super01@weevcrafts.local':'demo.customer08@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}});expect(response.ok()).toBe(true);return(await response.json()).csrf_token;}
async function checkoutPage(page){await page.goto('/products/'+slug);await page.waitForFunction(()=>window.weevcraftsBackend);const intent=await page.evaluate(async slug=>window.weevcraftsBackend.buyNow(slug,1,null,crypto.randomUUID()),slug);await page.goto(intent.checkout_url);const form=page.locator('#checkout-address-form');for(const[name,value]of Object.entries({recipient_name:'Payment recovery QA',phone:'9876543210',line1:'18 QA Craft Lane',line2:'',city:'Pune',state:'Maharashtra',postal_code:'411001'}))await form.locator(`[name=${name}]`).fill(value);return form;}
async function review(page){await page.locator('[data-checkout-review]').press('Enter');await expect(page.locator('[data-step="2"] h2')).toBeFocused({timeout:30000});}
async function pendingOrder(page){await checkoutPage(page);await review(page);let creations=0;await page.route('**/api/v1/checkout',async route=>{creations++;await route.continue();});await page.route('**/api/v1/payments/demo-capture/*',route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":{"message":"Isolated QA provider outage"}}'}),{times:1});await page.locator('[data-backend-action="checkout"]').evaluate(b=>{b.click();b.click();});await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation/);expect(creations).toBe(1);return decodeURIComponent(new URL(page.url()).pathname.split('/')[2]);}
test.beforeAll(async({baseURL})=>{
 if(baseURL!=='http://127.0.0.1:18088')throw Error('Loopback only');
 sql(`WITH p AS(INSERT INTO products(slug,brand_slug,category_slug,seller_id,name,description,price_cents,status,image_url) SELECT '${slug}',(SELECT slug FROM brands ORDER BY slug LIMIT 1),(SELECT slug FROM categories ORDER BY slug LIMIT 1),id,'Checkout QA handmade product with a deliberately long descriptive name','Isolated checkout acceptance fixture',100000,'approved','/assets/images/customer/ceramic-mugs.webp' FROM sellers WHERE status='active' ORDER BY id LIMIT 1 ON CONFLICT(slug) DO NOTHING RETURNING id) SELECT count(*) FROM p`);
 sql(`INSERT INTO product_variants(product_id,sku,name,price_cents,status) SELECT id,'PAYMENT-RECOVERY-BROWSER-QA','Standard',100000,'active' FROM products WHERE slug='${slug}' ON CONFLICT(sku) DO NOTHING`);
 sql(`INSERT INTO inventory_stock(variant_id,available_quantity) SELECT id,100 FROM product_variants WHERE sku='PAYMENT-RECOVERY-BROWSER-QA' ON CONFLICT(variant_id) DO NOTHING`);
});

test('two-step checkout retains address, validates delivery early and reflows',async({page},testInfo)=>{
 await login(page);const form=await checkoutPage(page);
 for(const width of[320,390,768,1024,1536]){
  await page.setViewportSize({width,height:1000});await expect(page.locator('.steps [role=listitem]')).toHaveCount(2);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await review(page);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await page.screenshot({path:path.join(os.tmpdir(),`weev-checkout-two-step-${testInfo.project.name}-${width}.png`),fullPage:true});
  await page.getByRole('button',{name:'Edit address',exact:true}).press('Enter');await expect(form.locator('[name=line1]')).toHaveValue('18 QA Craft Lane');
 }
 await page.setViewportSize({width:768,height:1000});await page.evaluate(()=>document.documentElement.style.zoom='2');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);await page.evaluate(()=>document.documentElement.style.zoom='');
 const invalid=await page.request.get('/api/v1/checkout/quote?postal_code=123');expect(invalid.status()).toBe(422);expect((await invalid.json()).error.code).toBe('PIN_INVALID');
 await page.route('**/api/v1/checkout/quote?**',route=>route.fulfill({status:422,contentType:'application/json',body:'{"error":{"code":"PIN_UNSERVICEABLE","message":"Delivery is not currently available to this PIN code."}}'}),{times:1});
 await page.locator('[data-checkout-review]').click();await expect(page.locator('[data-checkout-address-feedback]')).toContainText('Delivery is not currently available');await expect(form.locator('[name=line1]')).toHaveValue('18 QA Craft Lane');await expect(form).toBeVisible();
 const before=Number(sql("SELECT count(*) FROM addresses WHERE user_id=(SELECT id FROM users WHERE email='demo.customer08@weevcrafts.local') AND line1='18 QA Craft Lane'"));
 await page.locator('[data-checkout-save-address]').click();await expect(page.locator('[data-checkout-address-feedback]')).toContainText('Address saved');await page.locator('[data-checkout-save-address]').click();await expect(page.locator('[data-checkout-address-feedback]')).toContainText('Address saved');
 const after=Number(sql("SELECT count(*) FROM addresses WHERE user_id=(SELECT id FROM users WHERE email='demo.customer08@weevcrafts.local') AND line1='18 QA Craft Lane'"));expect(after).toBe(before||1);
 await page.setViewportSize({width:390,height:900});await review(page);await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});const violations=await page.evaluate(async()=> (await axe.run(document.querySelector('.checkout-layout'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations);expect(violations.map(v=>({id:v.id,targets:v.nodes.map(n=>n.target)}))).toEqual([]);
});

test('pending confirmation refresh and continuation reuse one payable order',async({page})=>{
 const csrf=await login(page);const number=await pendingOrder(page);await expect(page.locator('[data-confirmation-content]')).toContainText('Payment pending');
 await page.route('**/ui/orders/status?**',route=>route.fulfill({status:503,body:'QA unavailable'}));await page.locator('[data-check-payment-status]').click();await expect(page.locator('[data-payment-refresh-feedback]')).toContainText('last saved status');await page.unroute('**/ui/orders/status?**');
 expect((await page.request.post('/dev/payments/'+number+'/fail',{headers:{'X-CSRF-Token':csrf}})).ok()).toBe(true);
 expect(sql(`SELECT status FROM orders WHERE order_number='${number}'`)).toBe('pending_payment');
 await page.locator('[data-retry-payment]').click();await expect(page).toHaveURL(/\/confirmation/);await expect(page.locator('[data-confirmation-content]')).toContainText('Payment completed');expect(sql(`SELECT count(*) FROM orders WHERE order_number='${number}'`)).toBe('1');
 await page.reload();await expect(page.locator('[data-retry-payment]')).toHaveCount(0);
});

test('late capture is held and finance actions require role, CSRF and explicit reason',async({page,browser},testInfo)=>{
 const csrf=await login(page);const number=await pendingOrder(page);
 sql(`UPDATE inventory_reservations SET expires_at=NOW()-INTERVAL '1 minute' WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`);
 expect((await page.request.post('/dev/payments/'+number+'/capture',{headers:{'X-CSRF-Token':csrf}})).ok()).toBe(true);
 await page.reload();await expect(page.getByRole('heading',{name:'Payment received — review in progress'})).toBeVisible();await expect(page.locator('[data-retry-payment]')).toHaveCount(0);
 const id=sql(`SELECT id FROM payment_review_cases WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`);
 expect((await page.request.post('/super-admin/payment-reviews/approve',{form:{review_id:id,reason:'Not authorized',csrf_token:csrf}})).status()).toBe(403);
 const context=await browser.newContext({baseURL:'http://127.0.0.1:18088'});const admin=await context.newPage();try{
  const token=await login(admin,true);expect((await admin.request.post('/super-admin/payment-reviews/approve',{form:{review_id:id,reason:'Missing CSRF'}})).status()).toBe(403);
  await admin.goto('/super-admin/payment-reviews');
  for(const width of[320,390,768,1024,1536]){await admin.setViewportSize({width,height:1000});expect(await admin.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);}
  await admin.screenshot({path:path.join(os.tmpdir(),`weev-finance-review-${testInfo.project.name}.png`),fullPage:true});
  const approval=admin.locator('form[action$="/approve"]').filter({has:admin.locator('input[name=review_id][value="'+id+'"]')});
  await approval.locator('textarea').fill('QA keyboard confirmation check');const trigger=approval.getByRole('button');await trigger.click();await expect(admin.getByRole('dialog')).toBeVisible();await admin.keyboard.press('Escape');await expect(admin.getByRole('dialog')).toHaveCount(0);await expect(trigger).toBeFocused();
  await approval.locator('textarea').fill('QA finance revalidated current stock');await trigger.click();await admin.getByRole('dialog').getByRole('button',{name:'Revalidate stock & fulfil'}).click();await expect(admin.locator('.admin-notice')).toContainText('Decision saved',{timeout:30000});
  expect(sql(`SELECT status FROM payment_review_cases WHERE id=${id}`)).toBe('approved');
  await page.reload();await expect(page.locator('[data-confirmation-content]')).toContainText('Payment completed');
 }finally{await context.close();}
});

test('finance can authorize a captured-payment refund without creating a return',async({page,browser})=>{
 const csrf=await login(page);const number=await pendingOrder(page);sql(`UPDATE inventory_reservations SET expires_at=NOW()-INTERVAL '1 minute' WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`);
 expect((await page.request.post('/dev/payments/'+number+'/capture',{headers:{'X-CSRF-Token':csrf}})).ok()).toBe(true);
 const id=sql(`SELECT id FROM payment_review_cases WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`);
 const context=await browser.newContext({baseURL:'http://127.0.0.1:18088'});const admin=await context.newPage();try{
  const token=await login(admin,true);const form={review_id:id,reason:'QA explicitly authorizes refund of late capture',csrf_token:token};
  await admin.goto('/super-admin/payment-reviews');const refund=admin.locator('form[action$="/refund"]').filter({has:admin.locator('input[name=review_id][value="'+id+'"]')});await refund.locator('textarea').fill(form.reason);await refund.getByRole('button',{name:'Authorize refund',exact:true}).click();await admin.getByRole('dialog').getByRole('button',{name:'Authorize refund',exact:true}).click();await expect(admin.locator('.admin-notice')).toContainText('Decision saved',{timeout:30000});
  expect((await admin.request.post('/super-admin/payment-reviews/refund',{form})).ok()).toBe(true);
  expect(sql(`SELECT status FROM payment_review_cases WHERE id=${id}`)).toBe('refunded');expect(sql(`SELECT count(*) FROM return_requests WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`)).toBe('0');
  await page.reload();await expect(page.locator('[data-confirmation-content]')).toContainText('Payment refunded');await expect(page.locator('[data-retry-payment]')).toHaveCount(0);
 }finally{await context.close();}
});

test('a verified failed refund needs a fresh authorization bound to its old refund ID',async({page,browser})=>{
 const csrf=await login(page);const number=await pendingOrder(page);sql(`UPDATE inventory_reservations SET expires_at=NOW()-INTERVAL '1 minute' WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`);
 expect((await page.request.post('/dev/payments/'+number+'/capture',{headers:{'X-CSRF-Token':csrf}})).ok()).toBe(true);
 const id=sql(`SELECT id FROM payment_review_cases WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`);
 // A provider-confirmed failure fixture, confined to the disposable local store.
 sql(`UPDATE payment_review_cases SET status='refund_pending',refund_submission_state='failed',provider_refund_id='qa_failed_'||id::text,resolved_by=(SELECT id FROM users WHERE email='demo.super01@weevcrafts.local'),resolution_reason='QA fixture: provider confirmed failure' WHERE id=${id}`);
 const context=await browser.newContext({baseURL:'http://127.0.0.1:18088'});const admin=await context.newPage();try{
  const token=await login(admin,true);const oldID='qa_failed_'+id;
  expect((await admin.request.post('/super-admin/payment-reviews/refund-retry',{form:{review_id:id,reason:'Missing expected provider refund',csrf_token:token}})).status()).toBe(409);
  await admin.goto('/super-admin/payment-reviews');const form=admin.locator('form[action$="/refund-retry"]').filter({has:admin.locator('input[name=review_id][value="'+id+'"]')});
  await expect(form.locator('[name=expected_refund_id]')).toHaveValue(oldID);await form.locator('textarea').fill('Explicit new QA authorization after verified failure');await form.getByRole('button',{name:'Authorize new refund attempt'}).click();
  const dialog=admin.getByRole('dialog');await expect(dialog).toBeVisible();await dialog.getByRole('button',{name:'Authorize new refund attempt'}).click();await expect(admin.locator('.admin-notice')).toContainText('Decision saved',{timeout:30000});
  expect(sql(`SELECT status||':'||refund_attempt::text FROM payment_review_cases WHERE id=${id}`)).toBe('refunded:2');
  expect((await admin.request.post('/super-admin/payment-reviews/refund-retry',{form:{review_id:id,reason:'Replay old authorization',expected_refund_id:oldID,csrf_token:token}})).status()).toBe(409);
  expect(sql(`SELECT refund_attempt FROM payment_review_cases WHERE id=${id}`)).toBe('2');
 }finally{await context.close();}
});
