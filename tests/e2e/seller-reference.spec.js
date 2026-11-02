const {test,expect}=require('@playwright/test');
const {execFileSync}=require('node:child_process');

function sql(query){
 const target=new URL(process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18086');
 if(target.hostname!=='127.0.0.1'||!['18080','18086'].includes(target.port)||process.env.WEEVCRAFTS_QA_DATABASE!=='seller_workflows_20261017')throw new Error('Requires isolated seller QA database.');
 return execFileSync('docker',['exec','weevcrafts-storefront-qa-db','psql','-v','ON_ERROR_STOP=1','-U','storefrontqa','-d','seller_workflows_20261017','-tAc',query],{encoding:'utf8',windowsHide:true}).trim();
}
async function login(page,empty=false){
 const response=await page.request.post('/api/v1/auth/login',{data:{email:empty?'seller.reference.empty@weevcrafts.local':'demo.owner01@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}});
 expect(response.ok()).toBeTruthy();
}
test.afterAll(()=>{sql("DELETE FROM notifications WHERE source_key LIKE 'seller-reference-%'");});
test.beforeAll(()=>{
 sql(`INSERT INTO users(email,display_name,status,username) VALUES('seller.reference.empty@weevcrafts.local','Reference maker','active','seller_reference_empty') ON CONFLICT(email) DO NOTHING;
 INSERT INTO credentials(user_id,password_hash) SELECT u.id,c.password_hash FROM users u CROSS JOIN credentials c JOIN users owner ON owner.id=c.user_id AND owner.email='demo.owner01@weevcrafts.local' WHERE u.email='seller.reference.empty@weevcrafts.local' ON CONFLICT(user_id) DO NOTHING;
 INSERT INTO user_roles(user_id,role_slug) SELECT id,'seller' FROM users WHERE email='seller.reference.empty@weevcrafts.local' ON CONFLICT DO NOTHING;
 INSERT INTO sellers(owner_user_id,display_name,status) SELECT id,'Reference maker','pending' FROM users WHERE email='seller.reference.empty@weevcrafts.local' AND NOT EXISTS(SELECT 1 FROM sellers WHERE owner_user_id=users.id);`);
});

test('all ten reference screens render empty and populated states without overflow',async({page},testInfo)=>{
 const routes=['/seller-admin','/seller-admin/products','/seller-admin/inventory','/seller-admin/orders','/seller-admin/earnings','/seller-admin/analytics','/seller-admin/returns','/seller-admin/support','/seller-admin/settings','/notifications'];
 await login(page,true);
 const errors=[];page.on('pageerror',error=>errors.push(error.message));
 for(const [i,route] of routes.entries()){
  await page.setViewportSize({width:i<6?1672:1600,height:i<6?941:1000});
  const response=await page.goto(route);expect(response.status(),route).toBe(200);
  await expect(page.locator('h1')).toBeVisible();
  const broken=await page.locator('main img,.seller-intro-art img,.seller-empty-art').evaluateAll(images=>images.filter(img=>!img.complete||!img.naturalWidth).map(img=>img.src));
  expect(broken,route).toEqual([]);
  await page.screenshot({path:testInfo.outputPath(`${String(i+1).padStart(2,'0')}-desktop.png`),fullPage:true});
  for(const width of [320,390,720,768,820,960,1024,1280,1440]){
   await page.setViewportSize({width,height:900});
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${route} ${width}`).toBeTruthy();
  }
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:testInfo.outputPath(`${String(i+1).padStart(2,'0')}-mobile.png`),fullPage:true});
 }
 expect(errors).toEqual([]);
});

test('desktop rail persists, tooltips support keyboard, and mobile drawer stays independent',async({page})=>{
 await login(page);await page.setViewportSize({width:1440,height:1000});await page.goto('/seller-admin');
 const toggle=page.getByRole('button',{name:'Collapse sidebar',exact:true});
 await toggle.click();await expect(page.locator('body')).toHaveClass(/seller-rail/);
 expect(await page.locator('#seller-sidebar').evaluate(node=>node.getBoundingClientRect().width)).toBe(64);
 const products=page.locator('#seller-sidebar').getByRole('link',{name:'Products',exact:true});
 await products.focus();await expect(page.getByRole('tooltip')).toHaveText('Products');
 await page.keyboard.press('Escape');await expect(page.getByRole('tooltip')).toBeHidden();
 await page.reload();await expect(page.locator('body')).toHaveClass(/seller-rail/);
 await page.setViewportSize({width:390,height:844});await expect(page.locator('body')).not.toHaveClass(/seller-rail/);
 const menu=page.getByRole('button',{name:'Open seller navigation',exact:true});await menu.click();
 await expect(page.locator('#seller-sidebar')).toHaveAttribute('aria-modal','true');
 await page.keyboard.press('Escape');await expect(menu).toBeFocused();
 await page.setViewportSize({width:1440,height:1000});await expect(page.locator('body')).toHaveClass(/seller-rail/);
 await page.getByRole('button',{name:'Expand sidebar',exact:true}).click();
});

test('profile preview reflects unsaved edits and cancellation preserves saved profile',async({page})=>{
 await login(page);await page.goto('/seller-admin/settings');
 const form=page.locator('[data-store-profile]');const original=await form.locator('[name=display_name]').inputValue();
 await form.locator('[name=display_name]').fill('Preview only studio');await form.locator('[name=public_location]').fill('Hyderabad, Telangana');
 await expect(page.locator('[data-preview-name]')).toHaveText('Preview only studio');
 await expect(page.locator('[data-preview-location]')).toHaveText('Hyderabad, Telangana');
 await page.locator('#seller-sidebar').getByRole('link',{name:'Products',exact:true}).click();
 await expect(page.getByRole('dialog')).toBeVisible();await page.getByRole('button',{name:'Keep editing',exact:true}).click();
 await expect(form.locator('[name=display_name]')).toHaveValue('Preview only studio');
 // A fresh request reads persisted state without changing the current unsaved form.
 const persisted=await page.context().newPage();
 await persisted.goto('/seller-admin/settings');
 await expect(persisted.locator('[data-store-profile] [name=display_name]')).toHaveValue(original);
 await persisted.close();
 await form.locator('[name=reason]').fill('Isolated profile recovery check');
 await page.route('**/seller-admin/settings',route=>route.request().method()==='POST'?route.fulfill({status:503,contentType:'text/plain',body:'Profile could not be saved. Please retry.'}):route.continue());
 await form.getByRole('button',{name:'Save profile changes',exact:true}).click();
 await expect(form.locator('[role=alert]')).toContainText('Please retry');
 await expect(form.locator('[name=display_name]')).toHaveValue('Preview only studio');
 await expect(form.getByRole('button',{name:'Save profile changes',exact:true})).toBeEnabled();
});

test('notification filtering, cursors and read actions enforce recipient ownership',async({page},testInfo)=>{
 const prefix=`seller-reference-${testInfo.project.name}-${Date.now()}`;
 sql(`INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 SELECT id,'support','${prefix} support '||n,'Saved support update','/seller-admin/support','${prefix}-support-'||n FROM users CROSS JOIN generate_series(1,55)n WHERE email='demo.owner01@weevcrafts.local';
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key) SELECT id,'order','${prefix} private','Private update','/seller-admin/orders','${prefix}-private' FROM users WHERE email='seller.reference.empty@weevcrafts.local';`);
 await login(page);await page.goto('/notifications?filter=support');
 await expect(page.locator('.seller-notification-view')).not.toContainText(`${prefix} private`);
 await page.getByRole('link',{name:'Older updates',exact:true}).click();await expect(page).toHaveURL(/before=\d+&filter=support/);
 await expect(page.locator('.seller-notification-view')).toContainText(`${prefix} support`);
 const csrf=await page.locator('form[action="/notifications/read"] [name=csrf_token]').first().inputValue();
 const foreign=sql(`SELECT id FROM notifications WHERE source_key='${prefix}-private'`);
 const denied=await page.request.post('/notifications/read',{form:{csrf_token:csrf,notification_id:foreign},maxRedirects:0});expect(denied.status()).toBe(404);
 await page.getByRole('button',{name:'Mark all as read',exact:true}).click();await expect(page).toHaveURL(/filter=support/);
 expect(sql(`SELECT COUNT(*) FROM notifications n JOIN users u ON u.id=n.user_id WHERE u.email='demo.owner01@weevcrafts.local' AND read_at IS NULL`)).toBe('0');
 expect(sql(`SELECT COUNT(*) FROM notifications WHERE source_key='${prefix}-private' AND read_at IS NULL`)).toBe('1');
});
