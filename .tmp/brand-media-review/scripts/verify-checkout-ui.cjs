const {chromium}=require('@playwright/test');const assert=require('node:assert/strict');const fs=require('node:fs/promises');
const {output}=require('./ui-ux-qa-paths.cjs');
async function main(){
 await fs.mkdir(output,{recursive:true});
 const browser=await chromium.launch();const context=await browser.newContext({baseURL:'http://127.0.0.1:18080',viewport:{width:390,height:844}});const page=await context.newPage();
 let creations=0;const errors=[];page.on('pageerror',error=>errors.push(error.message));
 try{
  assert((await context.request.post('/api/v1/auth/login',{data:{email:'demo.customer07@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}})).ok());
  await page.goto('/products/blue-pottery');await page.waitForFunction(()=>window.weevcraftsBackend);
  await page.locator('.mobile-buy-bar button').last().click();await page.waitForURL(/\/checkout$/);
  const fields={recipient_name:'UI QA Customer',phone:'9876543210',line1:'10 Isolated QA Street',city:'Bengaluru',state:'Karnataka',postal_code:'560034'};
  for(const [name,value]of Object.entries(fields))await page.locator('#checkout-address-form [name="'+name+'"]').fill(value);
  await page.getByRole('button',{name:'Continue to delivery'}).click();await page.getByRole('button',{name:'Continue to payment'}).click();
  assert.match(await page.locator('.checkout-card[data-step="3"]').textContent(),/local|simulat/i);
  await page.getByRole('button',{name:'Continue to review'}).click();
  await page.route('**/api/v1/checkout',async route=>{if(route.request().method()==='POST')creations++;await route.continue();});
  await page.locator('[data-backend-action="checkout"]').evaluate(button=>{button.click();button.click();});
  await page.waitForURL(/\/orders\/[^/]+\/confirmation$/, {timeout:30000});assert.equal(creations,1);
  assert(await page.locator('.order-confirmation').isVisible());const url=page.url();const confirmation=await page.locator('.order-confirmation').textContent();
  assert.match(confirmation,/simulat|no (real )?money|no payment/i);
  await page.reload();assert.equal(page.url(),url);assert(await page.locator('.order-confirmation').isVisible());
  const link=page.getByRole('link',{name:'View order',exact:true});await link.click();await page.waitForURL(/\/orders\/[^/]+$/);
  // Fail only the local simulated capture. The saved order must remain pending,
  // and retry must reuse it rather than creating another order.
  await page.goto('/products/blue-pottery');await page.locator('.mobile-buy-bar button').last().click();await page.waitForURL(/\/checkout$/);
  for(const [name,value]of Object.entries(fields))await page.locator('#checkout-address-form [name="'+name+'"]').fill(value);
  await page.getByRole('button',{name:'Continue to delivery'}).click();await page.getByRole('button',{name:'Continue to payment'}).click();await page.getByRole('button',{name:'Continue to review'}).click();
  await page.route('**/api/v1/payments/demo-capture/*',route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":{"message":"QA simulated provider unavailable"}}'}));
  await page.locator('[data-backend-action="checkout"]').click();await page.waitForURL(/\/orders\/[^/]+\/confirmation$/);
  const pendingURL=page.url();assert.match(await page.locator('.order-confirmation').textContent(),/Payment pending/);assert.equal(creations,2);
  await page.getByRole('link',{name:'View order',exact:true}).click();await page.waitForURL(/\/orders\/[^/]+$/);
  await page.unroute('**/api/v1/payments/demo-capture/*');await page.locator('[data-retry-payment]').click();
  await page.waitForURL(pendingURL);assert.equal(creations,2);assert.match(await page.locator('.order-confirmation').textContent(),/Payment completed/i);
  assert.deepEqual(errors,[]);
  const report={pass:true,orderURL:url,recoveredOrderURL:pendingURL,creations,errors,checks:'local payment only; duplicate activation, retained address, authoritative confirmation, refresh, failed capture stays pending and retry reuses saved order'};
  await fs.writeFile(output+'/checkout-verification.json',JSON.stringify(report,null,2));console.log(report);
 }finally{await browser.close();}
}
main().catch(error=>{console.error(error);process.exitCode=1;});
