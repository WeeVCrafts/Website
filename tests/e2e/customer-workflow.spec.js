const {test,expect}=require('@playwright/test');
const {execFileSync}=require('node:child_process');
const path=require('node:path');
const database=process.env.WEEVCRAFTS_QA_DATABASE;
const isolated=database==='weev_customer_workflow_20261011' && new URL(process.env.WEEVCRAFTS_BASE_URL||'http://localhost:8080').port==='18081';
const password='WeeVCrafts-Local-2026!';
const slug='customer-workflow-ceramic-mugs';
function sql(query){
 if(!isolated)throw new Error('Customer workflow fixtures require the disposable 18081 deployment.');
 return execFileSync('docker',['compose','exec','-T','postgres','psql','-v','ON_ERROR_STOP=1','-U','wecratfs','-d',database,'-tAc',query],{encoding:'utf8'}).trim();
}
async function login(page,role='customer'){
 const result=await page.request.post('/api/v1/auth/login',{data:{email:`${role}@weevcrafts.local`,password}});
 expect(result.ok()).toBeTruthy();
 await page.goto(role==='seller'?'/seller-admin':'/account');
}
async function addPieces(page,count=1){
 await page.goto('/products/'+slug);
 const select=page.locator('#detail-variant');
 const options=await select.locator('option').evaluateAll(options=>options.map(option=>option.value));
 for(let i=0;i<count;i++){
  await select.selectOption(options[i]);
  await page.locator('.detail-actions').getByRole('button',{name:'Add to cart',exact:true}).click();
  await expect(page.locator('#cart-count em')).toHaveText(String(i+1));
  await page.getByRole('button',{name:'Close shopping bag',exact:true}).click();
  await expect(page.locator('#cart-drawer')).not.toBeVisible();
 }
}
async function addressAndReview(page){
 await page.goto('/checkout');
 if(await page.locator('[data-checkout-address-select]').count())await page.locator('[data-checkout-address-select]').selectOption('');
 const form=page.locator('#checkout-address-form');
 for(const [name,value] of Object.entries({recipient_name:'Workflow Customer',phone:'9876543210',line1:'12 Craft Lane',city:'Pune',state:'Maharashtra',postal_code:'411001'}))await form.locator(`[name="${name}"]`).fill(value);
 await page.getByRole('button',{name:'Continue to delivery'}).click();
 await page.getByRole('button',{name:'Continue to payment'}).click();
 await page.getByRole('button',{name:'Continue to review'}).click();
}
async function placeMockOrder(page,result){
 // Exercise non-success outcomes through the isolated payment API; checkout's
 // normal user flow completes the mock payment automatically.
 if(result!=='capture')await page.route('**/dev/payments/*/capture',async route=>{
  const endpoint=route.request().url().replace(/capture$/,result);
  const response=await page.request.post(endpoint,{headers:route.request().headers()});
  await route.fulfill({response});
 },{times:1});
 await page.getByRole('button',{name:'Place order',exact:true}).click();
 await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation(?:\?|$)/,{timeout:30000});
 const number=decodeURIComponent(new URL(page.url()).pathname.split('/')[2]);
 await page.getByRole('link',{name:'View order details',exact:true}).click();
 return number;
}
async function emailFor(request,subject,number){
 let found;
 await expect.poll(async()=>{
  const list=await (await request.get('http://127.0.0.1:8025/api/v1/messages?limit=200')).json();
  for(const message of list.messages||[]){
   if(message.Subject!==subject||!(message.To||[]).some(to=>to.Address==='customer@weevcrafts.local'))continue;
   const detail=await (await request.get(`http://127.0.0.1:8025/api/v1/message/${message.ID}`)).json();
   if((detail.Text||'').includes(number)){found=detail;return true;}
  }
  return false;
 },{timeout:80000,intervals:[1000,3000,5000]}).toBeTruthy();
 expect(found.Text).toContain(`http://127.0.0.1:18081/orders/${number}/tracking`);
 return found;
}

test.describe.serial('Professional customer workflow in an isolated local store',()=>{
 test.use({actionTimeout:20000});
 test.skip(!isolated,'Requires the disposable workflow database and local 18081 app.');
 test('Empty catalogue and cart provide honest next actions',async({page})=>{
  await page.goto('/products');
  await expect(page.getByRole('heading',{name:'Our collection is on its way'})).toBeVisible();
  await expect(page.locator('nav.pagination')).toHaveCount(0);
  await page.goto('/cart');
  await expect(page.getByRole('heading',{name:'Your cart is empty'})).toBeVisible();
  await expect(page.getByText('You might also like',{exact:true})).toHaveCount(0);
  await page.goto('/checkout');await expect(page).toHaveURL(/\/login\?next=/);
  sql(`WITH p AS (INSERT INTO products(slug,brand_slug,category_slug,seller_id,name,description,price_cents,status,image_url) SELECT '${slug}',(SELECT slug FROM brands LIMIT 1),(SELECT slug FROM categories LIMIT 1),id,'Workflow ceramic mugs','Material: Ceramic\\nCare: Hand wash',89900,'approved','/assets/images/customer/ceramic-mugs.webp' FROM sellers LIMIT 1 RETURNING id),v AS (INSERT INTO product_variants(product_id,sku,name,price_cents,status) SELECT p.id,'WORKFLOW-'||x.name,x.name,89900,'active' FROM p CROSS JOIN (VALUES('Blue pair'),('Sand pair'))x(name) RETURNING id) INSERT INTO inventory_stock(variant_id,available_quantity) SELECT id,100 FROM v`);
 });
 test('Mobile account and shopping shortcuts are accessible with keyboard and reduced motion',async({page})=>{
  await login(page);await page.setViewportSize({width:390,height:844});
  await expect.poll(()=>page.locator('[data-account-navigation]').getAttribute('open')).toBeNull();
  expect((await page.locator('.account-content').boundingBox()).y).toBeLessThan(550);
  const summary=page.locator('[data-account-navigation] summary');
  await summary.press('Enter');await expect(page.getByRole('navigation',{name:'Account sections'})).toBeVisible();
  await summary.press('Enter');await expect(page.getByRole('navigation',{name:'Account sections'})).not.toBeVisible();
  for(const width of [320,390,720,960,1440]){
   await page.setViewportSize({width,height:900});
   for(const id of (width>700?['cart-count','wishlist-count']:['cart-count'])){const link=page.locator('.header-actions > #'+id);await expect(link).toBeVisible();const b=await link.boundingBox();expect(b.width).toBeGreaterThanOrEqual(44);expect(b.height).toBeGreaterThanOrEqual(44);}
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
  }
  await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});
  await expect(page.locator('.header-logout button')).toBeVisible();
  if(process.env.WEEVCRAFTS_REVIEW_DIR)await page.screenshot({path:path.join(process.env.WEEVCRAFTS_REVIEW_DIR,'customer-account-after.png')});
  await page.goto('/products?q=unmatched-workflow-product');await expect(page.getByRole('heading',{name:'No matching pieces'})).toBeVisible();
 });
 test('Checkout preserves corrections, shows one maker, confirms payment and mails tracking',async({page,request,browser})=>{
  test.setTimeout(180000);await login(page);await addPieces(page,2);await addressAndReview(page);
  await page.getByRole('button',{name:'Edit address',exact:true}).click();
  await expect(page.locator('[name="line1"]')).toHaveValue('12 Craft Lane');
  await page.locator('[name="line1"]').fill('14 Craft Lane');await page.getByRole('button',{name:'Continue to delivery'}).click();
  await expect(page.locator('[data-step="2"]')).toContainText('2 items from 1 maker');
  await page.getByRole('button',{name:'Back to address'}).press('Enter');await expect(page.locator('[data-step="1"] h2')).toBeFocused();
  await page.getByRole('button',{name:'Continue to delivery'}).click();await page.getByRole('button',{name:'Continue to payment'}).click();await page.getByRole('button',{name:'Continue to review'}).click();
  await expect(page.locator('[data-checkout-address-summary]')).toContainText('14 Craft Lane');
  if(process.env.WEEVCRAFTS_REVIEW_DIR)await page.screenshot({path:path.join(process.env.WEEVCRAFTS_REVIEW_DIR,'customer-checkout-review.png')});
  await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();await expect(page.getByRole('button',{name:'Edit address',exact:true})).toBeVisible();
  if(process.env.WEEVCRAFTS_REVIEW_DIR)await page.screenshot({path:path.join(process.env.WEEVCRAFTS_REVIEW_DIR,'customer-checkout-mobile.png')});await page.setViewportSize({width:1440,height:1000});
  const number=await placeMockOrder(page,'capture');await expect(page.locator('.order-detail-status')).toContainText('Confirmed');
  await emailFor(request,'Your Weevcrafts order has been received',number);await emailFor(request,'Your Weevcrafts order is confirmed',number);
  expect(sql(`SELECT COUNT(*) FROM outbox_events WHERE event_type='OrderPaid' AND aggregate_id='${number}'`)).toBe('1');
  const seller=await browser.newPage();await login(seller,'seller');await seller.goto('/seller-admin/orders/'+number);
  for(const status of ['processing','shipped','delivered']){
   await seller.getByRole('button',{name:'Simulate next delivery update'}).click();
   await expect(seller).toHaveURL(/\/seller-admin\/orders(?:\?|$)/);
   await expect.poll(()=>sql(`SELECT status FROM seller_fulfillments WHERE order_id=(SELECT id FROM orders WHERE order_number='${number}')`)).toBe(status);
   await seller.goto('/seller-admin/orders/'+number);
   await expect(seller.locator('[data-mock-delivery]')).toHaveCount(status==='delivered'?0:1);
  }
  await seller.close();await page.goto('/orders/'+number+'/tracking');await expect(page.getByRole('heading',{name:'Track your order',exact:true})).toBeVisible();
  await expect(page.locator('.shipment-timeline .done')).toHaveCount(4);await expect(page.locator('.shipment-card')).toContainText('DEMO-'+number);
  if(process.env.WEEVCRAFTS_REVIEW_DIR)await page.screenshot({path:path.join(process.env.WEEVCRAFTS_REVIEW_DIR,'customer-tracking-after.png')});
  await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();if(process.env.WEEVCRAFTS_REVIEW_DIR)await page.screenshot({path:path.join(process.env.WEEVCRAFTS_REVIEW_DIR,'customer-tracking-mobile.png')});
  await emailFor(request,'Your Weevcrafts order has shipped',number);
 });
 test('Pending mock payment resumes the saved order; failure is clearly labelled',async({page})=>{
  await login(page);await addPieces(page);await addressAndReview(page);const pending=await placeMockOrder(page,'pending');
  await page.getByRole('button',{name:'Continue payment',exact:true}).click();
  await expect(page).toHaveURL(new RegExp('/orders/'+pending+'/confirmation'));
  await page.getByRole('link',{name:'View order details',exact:true}).click();await expect(page.locator('.order-detail-status')).toContainText('Confirmed');
  await addPieces(page);await addressAndReview(page);await placeMockOrder(page,'fail');await expect(page.locator('.order-detail-status')).toContainText(/failed|not completed/i);
  await expect(page.getByRole('button',{name:'Continue payment',exact:true})).toHaveCount(0);
 });
 test('Delivered purchases support verified reviews, returns and a linked support request',async({page})=>{
  await login(page);const number=sql("SELECT order_number FROM orders WHERE status='delivered' LIMIT 1");expect(number).not.toBe('');
  await page.goto('/account/reviews');const review=page.locator('form[action="/account/reviews"]').first();
  await review.getByLabel('Rating').selectOption('5');await review.getByLabel('Review title').fill('Thoughtful craftsmanship');await review.getByLabel('Your review').fill('The mugs arrived safely and the handmade finish is beautiful.');
  await review.getByRole('button',{name:'Submit verified review'}).click();await expect(page).toHaveURL(/notice=Review/);await expect(page.locator('.review-list')).toContainText('pending');
  await page.goto('/orders/'+number);await page.getByLabel('Reason for return',{exact:true}).fill('The size does not suit our shelves. Please help with the return.');await page.getByRole('button',{name:'Submit return request'}).click();await expect(page.locator('.order-return-card')).toContainText('Return request');
  await page.goto('/returns');await expect(page.locator('.returns-main')).toContainText(number);
  await page.goto('/contact');const support=page.locator('[data-customer-support-create]');await support.getByLabel('Subject',{exact:true}).fill('Workflow delivery and return question');await support.getByLabel('Order number (optional)').fill(number);await support.getByLabel('Message',{exact:true}).fill('Please help me follow the return for my delivered ceramic mugs.');await support.getByRole('button',{name:'Send support request'}).click();
  await expect.poll(()=>sql("SELECT COUNT(*) FROM support_tickets WHERE subject='Workflow delivery and return question'")).toBe('1');
  await page.goto('/account/support');await expect(page.locator('.account-content')).toContainText('Workflow delivery and return question');
 });
 test('Password change rejects bad credentials and CSRF, then invalidates earlier sessions',async({page,browser})=>{
  await login(page);const other=await browser.newPage();await login(other);await page.goto('/account/security');
  await expect(page.locator('[data-customer-password]')).toBeVisible();await expect(page.locator('[data-customer-password] .password-visibility')).toHaveCount(3);await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();if(process.env.WEEVCRAFTS_REVIEW_DIR)await page.screenshot({path:path.join(process.env.WEEVCRAFTS_REVIEW_DIR,'customer-security-mobile.png')});
  const csrf=await page.locator('[name="csrf_token"]').first().inputValue();
  const submit=(current,token=csrf)=>page.request.post('/account/password',{headers:{Accept:'application/json'},form:{csrf_token:token,current_password:current,new_password:'Workflow-Changed-2026!',confirm_password:'Workflow-Changed-2026!'}});
  expect((await submit(password,'invalid')).status()).toBe(403);expect((await submit('wrong-current-password')).status()).toBe(422);
  expect((await submit(password)).status()).toBe(200);await other.goto('/account');await expect(other).toHaveURL(/\/login/);await other.close();
  expect((await page.request.post('/api/v1/auth/login',{data:{email:'customer@weevcrafts.local',password}})).status()).not.toBe(200);
  expect((await page.request.post('/api/v1/auth/login',{data:{email:'customer@weevcrafts.local',password:'Workflow-Changed-2026!'}})).ok()).toBeTruthy();
 });
});
