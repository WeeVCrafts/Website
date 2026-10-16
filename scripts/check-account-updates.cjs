const {chromium,expect:playwrightExpect}=require('@playwright/test');
const expect=playwrightExpect.configure({timeout:30000});
const {execFileSync}=require('node:child_process');
const fs=require('node:fs');
const baseURL=process.env.WEEVCRAFTS_BASE_URL;
const database=process.env.WEEVCRAFTS_QA_DATABASE;
if(database!=='weev_updates_20261012'||new URL(baseURL).port!=='18882')throw new Error('Requires the isolated updates QA deployment.');
const evidence=process.env.WEEVCRAFTS_REVIEW_DIR||'test-results/account-updates';
fs.mkdirSync(evidence,{recursive:true});
function sql(query){return execFileSync('docker',['exec','weevcrafts-postgres-1','psql','-v','ON_ERROR_STOP=1','-U','wecratfs','-d',database,'-Atc',query],{encoding:'utf8'}).trim();}
async function login(browser,username,password,route){
 const page=await browser.newPage({viewport:{width:1440,height:900}});
 page.on('pageerror',error=>{throw error;});
 const result=await page.request.post(baseURL+'/api/v1/auth/login',{data:{email:username,password}});
 expect(result.ok(),username+' login').toBeTruthy();await page.goto(baseURL+route);return page;
}
async function csrf(page){return page.locator('[name="csrf_token"]').first().inputValue();}
async function checkout(page,fail=false){
 await page.goto(baseURL+'/products/qa-notification-mug');
 await page.locator('.detail-actions').getByRole('button',{name:'Add to cart',exact:true}).click();
 await expect(page.locator('#cart-count em')).not.toHaveText('0',{timeout:30000});
 if(await page.locator('#cart-drawer').isVisible()) await page.getByRole('button',{name:'Close shopping bag',exact:true}).click();
 await page.goto(baseURL+'/checkout');
 const select=page.locator('[data-checkout-address-select]');if(await select.count())await select.selectOption('');
 const form=page.locator('#checkout-address-form');
 for(const [name,value] of Object.entries({recipient_name:'QA Customer',phone:'9876543210',line1:'12 Craft Lane',city:'Pune',state:'Maharashtra',postal_code:'411001'}))await form.locator(`[name="${name}"]`).fill(value);
 await page.getByRole('button',{name:'Continue to delivery'}).click();
 await page.getByRole('button',{name:'Continue to payment'}).click();
 await page.getByRole('button',{name:'Continue to review'}).click();
 if(fail)await page.route('**/dev/payments/*/capture',route=>route.fulfill({status:503,contentType:'application/json',body:'{"message":"Temporary payment outage"}'}),{times:1});
 await page.getByRole('button',{name:'Place order',exact:true}).click();
 await expect(page).toHaveURL(/\/orders\/[^/]+\/confirmation(?:\?|$)/,{timeout:30000});
 const number=decodeURIComponent(new URL(page.url()).pathname.split('/')[2]);
 if(!fail){
  await expect(page.getByRole('heading',{name:'Order placed successfully!'})).toBeVisible();
  await expect(page.locator('.confirmation-summary')).toContainText('Payment completed');
  await expect(page.locator('.confirmation-summary')).toContainText('No money has been charged');
  expect(sql(`SELECT status FROM orders WHERE order_number='${number}'`)).toBe('paid');
  await expect(page.locator('.confirmation-progress li.done')).toHaveCount(2);
 }else await expect(page.locator('.confirmation-summary')).toContainText('Payment pending');
 await page.reload();await expect(page.locator('.confirmation-summary')).toContainText(number);
 return number;
}
(async()=>{
 const browser=await chromium.launch();
 const customer=await login(browser,'customer',process.env.WEEVCRAFTS_QA_CUSTOMER_PASSWORD,'/account');
 const seller=await login(browser,'seller',process.env.WEEVCRAFTS_QA_SELLER_PASSWORD,'/seller-admin');
 const admin=await login(browser,'admin',process.env.WEEVCRAFTS_QA_ADMIN_PASSWORD,'/admin');
 const superadmin=await login(browser,'superadmin',process.env.WEEVCRAFTS_QA_SUPERADMIN_PASSWORD,'/super-admin');
 for(const [name,page,sidebar] of [['customer',customer,'.account-sidebar'],['seller',seller,'.seller-sidebar'],['admin',admin,'.admin-sidebar'],['superadmin',superadmin,'.admin-sidebar']]){
  for(const width of [1440,390,320]){
   await page.setViewportSize({width,height:900});await page.emulateMedia({reducedMotion:'reduce'});
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),name+' '+width+' overflow').toBeTruthy();
   await page.screenshot({path:`${evidence}/${name}-${width}.png`,animations:'disabled'});
   if(width===1440)await expect(page.locator(sidebar+' nav a').first()).toBeVisible();
   if(name==='admin'||name==='superadmin') expect((await page.locator('.admin-header').boundingBox()).height).toBeLessThanOrEqual(76);
   if(width===390 && name!=='customer'){
    const toggle=page.locator(name==='seller'?'.seller-menu-button':'.admin-menu-button');
    await toggle.press('Enter');await expect(page.locator(sidebar)).toHaveClass(/is-open/);
    await page.keyboard.press('Escape');await expect(toggle).toBeFocused();
    if(name!=='seller'){await page.locator('.admin-search button').click();await expect(page.locator('#admin-search')).toBeFocused();await page.keyboard.press('Escape');await expect(toggle).toBeFocused();}
   }
  }
 }
 await customer.setViewportSize({width:1440,height:1000});
 const number=await checkout(customer);
 await customer.screenshot({path:`${evidence}/confirmation-desktop.png`,animations:'disabled'});
 await customer.setViewportSize({width:390,height:844});
 expect(await customer.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
 await customer.screenshot({path:`${evidence}/confirmation-mobile.png`,animations:'disabled'});
 const outsider=await admin.request.get(baseURL+'/orders/'+number+'/confirmation',{maxRedirects:0});expect(outsider.status()).not.toBe(200);
 const pending=await checkout(customer,true);
 await customer.getByRole('link',{name:'View order details',exact:true}).click();
 await customer.getByRole('button',{name:'Continue payment',exact:true}).click();
 await expect(customer).toHaveURL(new RegExp('/orders/'+pending+'/confirmation(?:\\?|$)'));
 expect(sql(`SELECT status FROM orders WHERE order_number='${pending}'`)).toBe('paid');
 await seller.goto(baseURL+'/seller-admin/support');
 const marker='Notification QA '+Date.now();
 const request=seller.locator('form[action="/seller-admin/support/create"]');
 await request.locator('[name="subject"]').fill(marker);
 await request.locator('[name="message"]').fill('Please help with the seller account workflow.');
 await request.getByRole('button',{name:'Send request'}).click();
 await expect(seller).toHaveURL(/notice=/);
 const ticket=sql(`SELECT id FROM support_tickets WHERE subject='${marker}'`);expect(ticket).toMatch(/^\d+$/);
 await superadmin.goto(baseURL+'/super-admin/support?ticket_id='+ticket);
 const reply=superadmin.locator('form[action$="/cases/reply"]');
 await reply.locator('[name="message"]').fill('Your seller request has been reviewed. Please reply with the next details.');
 await reply.getByRole('button',{name:/Send/}).click();
 await expect(superadmin).toHaveURL(new RegExp('support/cases\\?ticket_id='+ticket),{timeout:30000});
 await seller.goto(baseURL+'/notifications');
 const notice=seller.locator('.notification-card').filter({hasText:'New support reply'}).first();
 await expect(notice).toContainText('Unread');
 const id=await notice.locator('[name="notification_id"]').inputValue();
 const denied=await customer.request.post(baseURL+'/notifications/read',{form:{csrf_token:await csrf(customer),notification_id:id},maxRedirects:0});expect(denied.status()).toBe(404);
 const forged=await seller.request.post(baseURL+'/notifications/read',{form:{csrf_token:'forged',notification_id:id},maxRedirects:0});expect(forged.status()).toBe(403);
 await notice.getByRole('button',{name:'View update'}).click();
 await expect(seller).toHaveURL(new RegExp('ticket_id='+ticket));
 await expect(seller.locator('#ticket-'+ticket)).toContainText('Your seller request has been reviewed');
 expect(sql(`SELECT read_at IS NOT NULL FROM notifications WHERE id=${id}`)).toBe('t');
 const sellerReply=seller.locator('#ticket-'+ticket+' form[action$="/reply"]');
 await sellerReply.locator('[name="message"]').fill('Thank you. Here are the requested details for company support.');
 await sellerReply.getByRole('button',{name:'Send reply'}).click();
 await expect(seller).toHaveURL(/notice=/,{timeout:30000});
 await admin.goto(baseURL+'/notifications');
 await expect(admin.locator('.notification-card').filter({hasText:'New support reply'}).first()).toBeVisible();
 await customer.goto(baseURL+'/notifications');await expect(customer.locator('.notification-list')).not.toContainText(marker);
 await expect.poll(async()=>{
  const list=await (await customer.request.get('http://127.0.0.1:8025/api/v1/messages?limit=100')).json();
  for(const message of list.messages||[]){
   if(message.Subject!=='Your WeevCrafts order is confirmed')continue;
   const detail=await (await customer.request.get('http://127.0.0.1:8025/api/v1/message/'+message.ID)).json();
   if((detail.Text||'').includes(number)&&detail.Text.includes(baseURL+'/orders/'+number+'/tracking'))return true;
  }return false;
 },{timeout:80000,intervals:[1000,3000,5000]}).toBeTruthy();
 console.log('All four sidebars, automatic mock payment, durable confirmation, retry, support reply notification, ownership/CSRF and transactional email checks passed.');
 await browser.close();
})().catch(err=>{console.error(err);process.exit(1)});
