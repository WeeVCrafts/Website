const {test,expect}=require('@playwright/test');
const {execFileSync}=require('node:child_process');
const path=require('node:path');
const os=require('node:os');
const base=process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18080';
test.use({baseURL:base});
test.describe.configure({mode:'default'});
function sql(query){
 if(process.env.WEEVCRAFTS_QA_DATABASE!=='seller_workflows_20261017'||new URL(base).hostname!=='127.0.0.1'||new URL(base).port!=='18080')throw new Error('Seller mutations require the dedicated seller_workflows_20261017 database and local port 18080.');
 return execFileSync('docker',['exec','weevcrafts-storefront-qa-db','psql','-v','ON_ERROR_STOP=1','-U','storefrontqa','-d','seller_workflows_20261017','-tAc',query],{encoding:'utf8'}).trim();
}
async function login(page,account='owner01'){
 const response=await page.request.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}});expect(response.ok()).toBeTruthy();
}
const stockID=()=>sql("SELECT id FROM product_variants WHERE sku='SELLER-WORKFLOW-A'");
test.beforeAll(()=>{
 sql(`INSERT INTO categories(slug,name) VALUES('seller-workflow','Seller workflow QA') ON CONFLICT DO NOTHING;
 INSERT INTO brands(slug,name) VALUES('seller-workflow','Seller workflow QA') ON CONFLICT DO NOTHING;
 INSERT INTO products(slug,brand_slug,category_slug,name,description,price_cents,seller_id,status)
 SELECT 'seller-workflow-stock','seller-workflow','seller-workflow','Seller workflow stock','Isolated seller workflow fixture',19900,id,'approved' FROM sellers WHERE owner_user_id=(SELECT id FROM users WHERE email='demo.owner01@weevcrafts.local') ON CONFLICT(slug) DO NOTHING;
 INSERT INTO product_variants(product_id,sku,name,price_cents,status) SELECT p.id,v.sku,v.name,19900,v.status FROM products p CROSS JOIN (VALUES('SELLER-WORKFLOW-A','Standard','active'),('SELLER-WORKFLOW-B','Blue','active'),('SELLER-WORKFLOW-ARCHIVED','Retired','archived')) v(sku,name,status) WHERE p.slug='seller-workflow-stock' ON CONFLICT(sku) DO NOTHING;
 INSERT INTO inventory_stock(variant_id,available_quantity,reserved_quantity) SELECT id,CASE WHEN sku='SELLER-WORKFLOW-B' THEN 0 ELSE 20 END,CASE WHEN sku='SELLER-WORKFLOW-A' THEN 3 ELSE 0 END FROM product_variants WHERE sku LIKE 'SELLER-WORKFLOW-%' ON CONFLICT(variant_id) DO UPDATE SET available_quantity=EXCLUDED.available_quantity,reserved_quantity=EXCLUDED.reserved_quantity;
 INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) SELECT 'WC-SELLER-WORKFLOW-'||n,(SELECT id FROM users WHERE email='demo.customer01@weevcrafts.local'),'paid',19900,19900,'{"recipient_name":"QA customer","line1":"QA address","city":"Hyderabad","state":"Telangana","postal_code":"500001","country_code":"IN"}','seller-workflow',n::text FROM generate_series(1,55) n ON CONFLICT DO NOTHING;
 INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) SELECT o.id,v.id,'Seller workflow stock',v.sku,19900,1,19900 FROM orders o CROSS JOIN product_variants v WHERE o.order_number LIKE 'WC-SELLER-WORKFLOW-%' AND v.sku='SELLER-WORKFLOW-A' AND NOT EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id);
 INSERT INTO payment_attempts(order_id,provider,status,amount_cents) SELECT id,'qa','captured',19900 FROM orders WHERE order_number LIKE 'WC-SELLER-WORKFLOW-%' AND NOT EXISTS(SELECT 1 FROM payment_attempts WHERE order_id=orders.id);
 UPDATE orders SET status='paid' WHERE order_number LIKE 'WC-SELLER-WORKFLOW-%';
 UPDATE seller_fulfillments SET status='pending',carrier='',tracking_number='',last_note='',revision=revision+1 WHERE order_id IN(SELECT id FROM orders WHERE order_number LIKE 'WC-SELLER-WORKFLOW-%');`);
});

test('guided product entry validates steps, retains edits and creates one reviewable draft',async({page})=>{
 await login(page);await page.goto('/seller-admin/products/new');
 const form=page.locator('[data-product-builder]');
 await page.getByRole('button',{name:'Next',exact:true}).click();await expect(form.locator('[name=name]')).toBeFocused();
 const suffix=Date.now();await form.locator('[name=name]').fill('Workflow listing '+suffix);
 await form.locator('[name=category_slug]').selectOption('seller-workflow');await form.locator('[name=sku]').fill('WORKFLOW-'+suffix);
 await form.locator('[name=description]').fill('A complete product description for isolated QA.');
 await form.getByRole('link',{name:'Cancel',exact:true}).click();await expect(page.getByRole('dialog')).toBeVisible();await page.getByRole('button',{name:'Keep editing',exact:true}).click();await expect(form.locator('[name=name]')).toHaveValue('Workflow listing '+suffix);
 await page.getByRole('button',{name:'Next',exact:true}).click();await expect(form.locator('[name=price_rupees]')).toBeVisible();
 await form.locator('[name=price_rupees]').fill('199.25');await form.locator('[name=initial_stock]').fill('7');
 await form.locator('[name=compare_at_rupees]').fill('100');
 await page.getByRole('button',{name:'Next',exact:true}).click();await page.getByRole('button',{name:'Back',exact:true}).click();await expect(form.locator('[name=price_rupees]')).toHaveValue('199.25');
 await page.getByRole('button',{name:'Next',exact:true}).click();await page.getByRole('button',{name:'Next',exact:true}).click();
 await expect(form.locator('[data-product-review]')).toContainText('199.25');await page.getByRole('button',{name:'Edit details',exact:true}).click();await expect(form.locator('[name=name]')).toHaveValue('Workflow listing '+suffix);
 await page.getByRole('button',{name:'4. Review',exact:true}).click();
 await page.getByRole('button',{name:'Save product draft',exact:true}).click();
 await expect(form.locator('[data-product-step=pricing]')).toBeVisible();await expect(form.locator('[name=compare_at_rupees]')).toBeFocused();await expect(form.locator('[name=name]')).toHaveValue('Workflow listing '+suffix);
 await form.locator('[name=compare_at_rupees]').fill('299.25');await page.getByRole('button',{name:'4. Review',exact:true}).click();
 await page.getByRole('button',{name:'Save product draft',exact:true}).click({clickCount:2});await expect(page).toHaveURL(/\/products\/\d+\/edit\?step=review/);
 expect(sql("SELECT count(*) FROM product_variants WHERE sku='WORKFLOW-"+suffix+"'")).toBe('1');
 await expect(page.locator('[data-review-submit]')).toBeVisible();
 await page.getByRole('button',{name:'Edit details',exact:true}).click();await page.getByLabel('Title',{exact:true}).fill('Edited workflow listing '+suffix);await expect(page.locator('[data-review-submit] button')).toBeDisabled();
 await page.getByRole('button',{name:'4. Review',exact:true}).click();await page.getByRole('button',{name:'Save product draft',exact:true}).click();await expect(page.locator('.seller-notice')).toContainText('saved');
 await page.locator('[data-review-submit] button').click();await expect(page).toHaveURL(/notice=Product\+submitted/);
});

test('server rejects invalid native product submissions with retained values; no-JS form works',async({browser})=>{
 const context=await browser.newContext({baseURL:base,javaScriptEnabled:false});const page=await context.newPage();await login(page);await page.goto('/seller-admin/products/new');
 await page.getByLabel('Title',{exact:true}).fill('Native retained listing');await page.locator('[name=category_slug]').selectOption('seller-workflow');await page.locator('[name=sku]').fill('NATIVE-'+Date.now());await page.locator('[name=description]').fill('Native product draft with retained server values.');
 await page.getByLabel('Price (INR)',{exact:true}).fill('199.25');await page.getByLabel('Compare-at price (INR, optional)',{exact:true}).fill('1');
 await page.getByRole('button',{name:'Save product draft',exact:true}).click();await expect(page.locator('.seller-form-error')).toBeVisible();await expect(page.getByLabel('Title',{exact:true})).toHaveValue('Native retained listing');await expect(page.getByLabel('Compare-at price (INR, optional)',{exact:true})).toHaveValue('1');
 await page.getByLabel('Compare-at price (INR, optional)',{exact:true}).fill('299');await page.getByRole('button',{name:'Save product draft',exact:true}).click();await expect(page).toHaveURL(/\/products\/\d+\/edit/);await context.close();
});

test('cover upload failure keeps a saved draft and retry targets its existing ID',async({page})=>{
 await login(page);await page.goto('/seller-admin/products/new');const token=await page.locator('[name=csrf_token]').first().inputValue();
 const sku='COVER-FAIL-'+Date.now();
 const response=await page.request.post('/seller-admin/products',{multipart:{csrf_token:token,name:'Saved cover retry',description:'Isolated draft whose cover storage is unavailable.',category_slug:'seller-workflow',category_name:'Seller workflow QA',sku,brand_slug:'seller-workflow',brand_name:'Seller workflow QA',price_rupees:'199',initial_stock:'0',image_file:{name:'cover.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==','base64')}}});
 expect(response.ok()).toBeTruthy();expect(response.url()).toMatch(/\/products\/\d+\/edit\?step=photos/);
 await page.goto(response.url());await expect(page.locator('.seller-notice')).toContainText(/saved.*upload failed/i);expect(sql("SELECT count(*) FROM product_variants WHERE sku='"+sku+"'")).toBe('1');
 await expect(page.locator('[data-product-builder]')).toHaveAttribute('action',/\/products\/\d+\/edit/);
});

test('stock direction, committed quantities, reservations, history and filters remain consistent',async({page})=>{
 await login(page);const id=stockID();await page.goto('/seller-admin/inventory?q=SELLER-WORKFLOW-A&variant='+id+'#stock-'+id);
 const row=page.locator('#stock-'+id), form=row.locator('[data-stock-adjustment]');
 await form.locator('[name=quantity]').fill('4');await form.getByLabel('Reason',{exact:true}).fill('Isolated replenishment');await expect(form.locator('[data-stock-preview]')).toContainText('24');
 await form.getByRole('button',{name:'Save stock change'}).click();await expect(page.locator('.seller-notice')).toContainText('saved');await expect(row.locator('[data-label=Available]')).toHaveText('24');await expect(row.locator('[data-label=Reserved]')).toHaveText('3');await expect(page.locator('#inventory-history')).toContainText('Isolated replenishment');
 await form.locator('[name=adjustment_type]').selectOption('remove');await form.locator('[name=quantity]').fill('25');await form.getByLabel('Reason',{exact:true}).fill('Too many units');await form.getByRole('button',{name:'Save stock change'}).click();await expect(form.locator('[aria-invalid=true]')).toBeVisible();
 await form.locator('[name=quantity]').fill('4');await form.getByRole('button',{name:'Save stock change'}).click();await expect(row.locator('[data-label=Available]')).toHaveText('20');
 await page.goto('/seller-admin/inventory?q=SELLER-WORKFLOW&stock=out');await expect(page.locator('[data-label=SKU]')).toHaveText('SELLER-WORKFLOW-B');await expect(page.getByText('SELLER-WORKFLOW-ARCHIVED',{exact:true})).toHaveCount(0);
});

test('one paginated order queue, explicit next steps, tracking validation and revision conflicts',async({page,context})=>{
 await login(page);await page.goto('/seller-admin/orders?q=WC-SELLER-WORKFLOW');await expect(page.locator('[data-seller-orders-region] tbody tr')).toHaveCount(50);await expect(page.locator('[data-seller-orders-region]')).toContainText('55 matching orders');
 await page.locator('[data-seller-orders-region]').getByRole('link',{name:'Next page',exact:true}).click();await expect(page).toHaveURL(/page=2/);await expect(page.locator('[data-seller-orders-region] tbody tr')).toHaveCount(5);
 await page.locator('[data-seller-orders-region]').getByRole('link',{name:'Open order',exact:true}).first().click();const detailURL=page.url();await expect(page.getByRole('link',{name:'Back to orders'})).toHaveAttribute('href',/page=2/);await expect(page.locator('.seller-nav').getByRole('link',{name:'Returns',exact:true})).toBeVisible();
 const second=await context.newPage();await second.goto(detailURL);await second.locator('[name=note]').fill('Unsaved fulfilment note');
 await page.getByRole('button',{name:'Start processing',exact:true}).click();await expect(page.getByRole('button',{name:'Mark as packed',exact:true})).toBeVisible();
 await expect(second.locator('[data-order-conflict]')).toBeVisible({timeout:20000});await expect(second.locator('[name=note]')).toHaveValue('Unsaved fulfilment note');
 await second.getByRole('button',{name:'Start processing',exact:true}).click();await expect(second.locator('.ui-form-error')).toContainText('another session');await expect(second.locator('[name=note]')).toHaveValue('Unsaved fulfilment note');await second.close();
 await page.getByRole('button',{name:'Mark as packed',exact:true}).click();await expect(page.getByRole('button',{name:'Mark as shipped',exact:true})).toBeVisible();await page.getByRole('button',{name:'Mark as shipped',exact:true}).click();await expect(page.locator('[name=carrier]')).toHaveAttribute('aria-invalid','true');
 await page.locator('[name=carrier]').selectOption('Other');await expect(page.locator('[name=custom_carrier]')).toBeVisible();await page.locator('[name=custom_carrier]').fill('QA courier');await page.locator('[name=tracking_number]').fill('QA-TRACK-123');await page.getByRole('button',{name:'Mark as shipped',exact:true}).click();await expect(page.getByRole('button',{name:'Mark ready to deliver',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Mark ready to deliver',exact:true}).click();await page.getByRole('button',{name:'Mark as delivered',exact:true}).click();await expect(page.locator('[data-seller-order-detail]')).toContainText('Delivered');await expect(page.locator('[data-fulfillment-form]')).toHaveCount(0);
});

test('inventory mutations reject invalid access and concurrent additions keep reserved units',async({page,browser})=>{
 await login(page);const id=stockID();await page.goto('/seller-admin/inventory');const token=await page.locator('[name=csrf_token]').first().inputValue();
 const before=Number(sql('SELECT available_quantity FROM inventory_stock WHERE variant_id='+id));
 const form={csrf_token:token,variant_id:id,adjustment_type:'add',quantity:'1',reason:'Concurrent fixture restock'};
 const responses=await Promise.all([page.request.post('/seller-admin/inventory/adjust',{form,maxRedirects:0}),page.request.post('/seller-admin/inventory/adjust',{form,maxRedirects:0})]);responses.forEach(response=>expect(response.status()).toBe(303));
 expect(sql('SELECT available_quantity FROM inventory_stock WHERE variant_id='+id)).toBe(String(before+2));expect(sql('SELECT reserved_quantity FROM inventory_stock WHERE variant_id='+id)).toBe('3');
 expect((await page.request.post('/seller-admin/inventory/adjust',{form:{...form,adjustment_type:'remove',quantity:'100000'}})).status()).toBe(422);
 expect((await page.request.post('/seller-admin/inventory/adjust',{form:{...form,csrf_token:'invalid'}})).status()).toBe(403);
 const legacy=await page.request.post('/seller-admin/inventory/adjust',{form:{csrf_token:token,variant_id:id,delta:'1',reason:'Legacy adjustment fixture'},maxRedirects:0});expect(legacy.status()).toBe(303);
 const otherContext=await browser.newContext({baseURL:base});const other=await otherContext.newPage();await login(other,'owner02');await other.goto('/seller-admin/inventory?variant='+id);const otherToken=await other.locator('[name=csrf_token]').first().inputValue();await expect(other.locator('#inventory-history')).not.toContainText('Concurrent fixture restock');
 expect((await other.request.post('/seller-admin/inventory/adjust',{form:{...form,csrf_token:otherToken}})).status()).toBe(403);
 await otherContext.close();
});

test('seller pages retain keyboard navigation and narrow layout',async({page})=>{
 await login(page);const errors=[];page.on('pageerror',error=>errors.push(error.message));
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});
  for(const route of ['/seller-admin','/seller-admin/products/new','/seller-admin/inventory','/seller-admin/orders','/seller-admin/earnings','/seller-admin/analytics','/seller-admin/returns','/seller-admin/support','/seller-admin/settings']){
   await page.goto(route);expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),route).toBe(false);
  }
  await page.goto('/seller-admin/products/new');await page.screenshot({path:path.join(os.tmpdir(),`seller-workflow-${width}.png`),fullPage:true});
  const category=page.locator('[data-product-builder] [name=category_slug]');await category.focus();await category.press('Home');await category.press('ArrowDown');await category.press('Enter');await expect(category).not.toHaveValue('');
  if(width===390){await page.getByRole('button',{name:'Open seller navigation',exact:true}).click();await expect(page.locator('#seller-sidebar')).toHaveAttribute('aria-modal','true');await page.keyboard.press('Escape');await expect(page.getByRole('button',{name:'Open seller navigation',exact:true})).toBeFocused();}
  await page.locator('[data-product-builder]').getByRole('link',{name:'Cancel',exact:true}).click();await page.getByRole('button',{name:'Discard changes',exact:true}).click();await expect(page).toHaveURL(/\/seller-admin\/products$/);
 }
 expect(errors).toEqual([]);
});

test('homepage overlaps slides for 450ms, handles rapid input and respects reduced motion',async({page})=>{
 await page.goto('/');
 // Capture the intermediate DOM in the browser, even when the test runner is
 // delayed beyond the short visual transition by another process.
 await page.evaluate(()=>{
  window.heroFadeEvidence=[];
  new MutationObserver(()=>{
   const visible=Array.from(document.querySelectorAll('[data-hero-slide]:not([hidden])'));
   if(visible.length===2){const boxes=visible.map(slide=>slide.getBoundingClientRect());window.heroFadeEvidence.push({interactive:visible.filter(slide=>!slide.inert).length,durations:visible.flatMap(slide=>slide.getAnimations().map(animation=>animation.effect.getTiming().duration)),overlap:boxes[0].x===boxes[1].x&&boxes[0].y===boxes[1].y&&boxes[0].width===boxes[1].width});}
  }).observe(document.querySelector('[data-hero]'),{subtree:true,attributes:true,attributeFilter:['hidden','inert']});
 });
 await page.locator('[data-hero-select="1"]').click();await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
 const evidence=await page.evaluate(()=>window.heroFadeEvidence);expect(evidence).toContainEqual({interactive:1,durations:[450,450],overlap:true});
 await page.locator('[data-hero-select="2"]').click();await page.locator('[data-hero-select="3"]').click();await expect(page.locator('[data-hero-count]')).toHaveText('4 / 5');await expect(page.locator('[data-hero-slide]:not([hidden])')).toHaveCount(1);
 await page.emulateMedia({reducedMotion:'reduce'});await page.locator('[data-hero-select="2"]').click();await expect(page.locator('[data-hero-count]')).toHaveText('3 / 5');await expect(page.locator('[data-hero-slide]:not([hidden])')).toHaveCount(1);await expect(page.locator('[data-hero]')).toHaveAttribute('data-playing','false');
 await expect(page.locator('[data-hero-autoplay]')).toHaveCount(0);
});
