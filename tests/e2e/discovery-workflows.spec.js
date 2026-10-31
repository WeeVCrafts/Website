const {test,expect}=require('@playwright/test');
const {execFileSync}=require('node:child_process');
const {createHmac}=require('node:crypto');
const path=require('node:path');
const os=require('node:os');
const baseURL='http://127.0.0.1:18083';
test.use({baseURL});
test.skip(process.env.WEEVCRAFTS_DISCOVERY_QA!=='true','Requires isolated discovery QA database, app and loopback gateway fixture.');
const password='WeeVCrafts-Demo-2026!';
function sql(query){return execFileSync('docker',['exec','weev-discovery-qa-20261031','psql','-U','discoveryqa','-d','weev_discovery_qa2','-v','ON_ERROR_STOP=1','-tAc',query],{encoding:'utf8'}).trim();}
async function login(page,email){expect((await page.request.post('/api/v1/auth/login',{data:{email,password}})).ok()).toBe(true);}
async function ready(page,url){await page.goto(url);await page.waitForFunction(()=>window.WVUI);await page.evaluate(()=>document.fonts.ready);}

test('search, featured navigation and simplified product/stock entry',async({page})=>{
 test.setTimeout(180000);
 const name='Discovery browser '+Date.now();
 await login(page,'demo.owner01@weevcrafts.local');
 await ready(page,'/seller-admin/products/new');
 const form=page.locator('[data-product-simple]');
 await expect(form.getByRole('button',{name:'Save product draft',exact:true})).toBeVisible();
 await expect(form.locator('[data-product-step="pricing"]')).toBeVisible();
 await expect(form.locator('[data-product-step="photos"]')).toBeVisible();
 await form.locator('[name=name]').fill(name);
 await form.locator('[name=description]').fill('A synthetic product for isolated browser verification.');
 await form.locator('[name=category_slug]').selectOption('home');
 await form.locator('[name=price_rupees]').fill('499.50');
 await form.locator('[name=initial_stock]').fill('3');
 await form.locator('[name=search_tags]').fill('Discoverytag, coffee cup, DISCOVERYTAG');
 await form.getByText('Use an existing image URL',{exact:true}).click();
 await form.locator('[name=image_url]').fill('/assets/images/customer/ceramic-mugs.webp');
 await form.getByRole('button',{name:'Save product draft',exact:true}).click();
 await expect(page).toHaveURL(/\/seller-admin\/products\/\d+\/edit\?/);
 const id=sql(`SELECT id FROM products WHERE name='${name}'`);
 expect(id).toMatch(/^\d+$/);
 expect(sql(`SELECT status||'|'||array_to_string(search_tags,',') FROM products WHERE id=${id}`)).toBe('draft|discoverytag,coffee cup');
 sql(`UPDATE products SET status='approved' WHERE id=${id}`);
 await login(page,'demo.super01@weevcrafts.local');
 await ready(page,`/super-admin/products/${id}/edit`);
 await page.locator('[name=featured_pinned]').check();
 await page.locator('[name=featured_rank]').fill('0');
 await page.getByRole('button',{name:'Save product',exact:true}).click();
 await expect(page).toHaveURL(/\/super-admin\/products\?/);
 expect(sql(`SELECT featured_pinned FROM products WHERE id=${id}`)).toBe('t');
 const sku=sql(`SELECT sku FROM product_variants WHERE product_id=${id} ORDER BY id LIMIT 1`);
 await ready(page,'/super-admin/inventory?q='+encodeURIComponent(sku));
 await page.getByText('Adjust stock',{exact:true}).click();
 const stock=page.locator('[data-stock-adjustment]');
 await stock.locator('[name=quantity]').fill('2');
 await stock.locator('[name=reason]').fill('QA stock count');
 await expect(stock.locator('[data-stock-preview]')).toContainText('after change: 5');
 await stock.getByRole('button',{name:'Save stock change'}).click();
 await expect(page).toHaveURL(/notice=Stock/);
 expect(sql(`SELECT available_quantity FROM inventory_stock s JOIN product_variants v ON v.id=s.variant_id WHERE v.product_id=${id}`)).toBe('5');
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:1000});
  await ready(page,'/search?q=discoverytag');
  await expect(page.locator('.product-card').filter({hasText:name})).toBeVisible();
  await page.locator('#global-search').fill('cofee cup');
  await page.locator('#global-search').press('Enter');
  await expect(page.locator('.product-card').filter({hasText:name})).toBeVisible();
  await ready(page,'/featured-products');
  await expect(page.locator('.product-card').first()).toContainText(name);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await page.screenshot({path:path.join(os.tmpdir(),`weev-featured-${width}.png`),fullPage:true});
 }
});

test('checkout callback stays pending until signed capture; receipt stays private',async({page,browser})=>{
 test.setTimeout(120000);
 await login(page,'demo.customer08@weevcrafts.local');
 await page.addInitScript(()=>{window.Razorpay=class{constructor(options){window.qaPaymentOptions=options;}on(){}open(){window.qaPaymentOpened=true;}};});
 await ready(page,'/products/wooden-box');
 await page.locator('.detail-actions').getByRole('button',{name:'Add to cart',exact:true}).click();
 await expect(page).toHaveURL(/\/cart$/);
 await ready(page,'/checkout');
 for(const [name,value] of Object.entries({recipient_name:'Discovery customer',phone:'9876543210',line1:'12 Test Lane',city:'Pune',state:'Maharashtra',postal_code:'411001'}))await page.locator(`#checkout-address-form [name=${name}]`).fill(value);
 for(const label of ['Continue to delivery','Continue to payment','Continue to review'])await page.getByRole('button',{name:label,exact:true}).click();
 await page.getByRole('button',{name:'Place order',exact:true}).click();
 await page.waitForFunction(()=>window.qaPaymentOpened);
 const options=await page.evaluate(()=>({order_id:window.qaPaymentOptions.order_id,amount:window.qaPaymentOptions.amount,currency:window.qaPaymentOptions.currency}));
 const payment='pay_discovery_'+Date.now();
 const signature=createHmac('sha256','qa-fixture-only').update(options.order_id+'|'+payment).digest('hex');
 await page.evaluate(data=>window.qaPaymentOptions.handler(data),{razorpay_order_id:options.order_id,razorpay_payment_id:payment,razorpay_signature:signature});
 await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation/);
 const number=decodeURIComponent(new URL(page.url()).pathname.split('/')[2]);
 expect(sql(`SELECT status FROM orders WHERE order_number='${number}'`)).toBe('pending_payment');
 const body=JSON.stringify({entity:'event',event:'payment.captured',payload:{payment:{entity:{id:payment,order_id:options.order_id,amount:options.amount,currency:options.currency,status:'captured'}}}});
 const headers={'Content-Type':'application/json','X-Razorpay-Signature':createHmac('sha256','qa-webhook-only').update(body).digest('hex'),'X-Razorpay-Event-Id':payment};
 expect((await page.request.post('/webhooks/razorpay',{data:body+' ',headers})).status()).toBe(401);
 expect((await page.request.post('/webhooks/razorpay',{data:body,headers})).ok()).toBe(true);
 const replay=await page.request.post('/webhooks/razorpay',{data:body,headers});expect(replay.ok()).toBe(true);
 expect(sql(`SELECT status FROM orders WHERE order_number='${number}'`)).toBe('paid');
 const receipt=await page.request.get(`/orders/${encodeURIComponent(number)}/invoice?format=pdf`);
 expect(receipt.ok()).toBe(true);expect(receipt.headers()['content-type']).toBe('application/pdf');expect(receipt.headers()['cache-control']).toContain('no-store');
 const other=await browser.newContext({baseURL});
 expect((await other.request.get(`/orders/${encodeURIComponent(number)}/invoice?format=pdf`,{maxRedirects:0})).status()).not.toBe(200);await other.close();
});
