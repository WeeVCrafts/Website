const {test,expect}=require('@playwright/test');
const {execFileSync}=require('node:child_process');
const path=require('node:path');
const photograph=path.resolve('web/assets/images/brand/weevcrafts-logo.png');
const axe=require.resolve('axe-core/axe.min.js');
function sql(query){
 if(process.env.WEEVCRAFTS_QA_DATABASE!=='category_publication_20261102')throw Error('Requires dedicated category QA database.');
 return execFileSync('docker',['exec','weevcrafts-storefront-qa-db','psql','-v','ON_ERROR_STOP=1','-U','storefrontqa','-d','category_publication_20261102','-tAc',query],{encoding:'utf8',windowsHide:true}).trim();
}
function storageWrite(allow){
 sql('SELECT 1');
 const app=JSON.parse(execFileSync('docker',['inspect','weevcrafts-app-1'],{encoding:'utf8',windowsHide:true}))[0];
 const key=app.Config.Env.find(v=>v.startsWith('S3_ACCESS_KEY=')).slice('S3_ACCESS_KEY='.length);
 execFileSync('docker',['exec','weevcrafts-garage-1','/garage','bucket',allow?'allow':'deny','--write','--key',key,'weevcrafts-category-publication-qa'],{stdio:'pipe',windowsHide:true});
}
async function login(page,role='super01'){expect((await page.request.post('/api/v1/auth/login',{data:{email:`demo.${role}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}})).ok()).toBeTruthy();}
async function ready(page){await page.waitForFunction(()=>window.WVUI);await page.evaluate(()=>document.fonts.ready);}
async function open(page,query='new=1'){await page.goto('/super-admin/category-management?'+query);await ready(page);return page.getByRole('dialog');}
async function token(page){return page.locator('form[action="/logout"] [name=csrf_token]').first().inputValue();}
test.beforeAll(()=>{sql('SELECT 1');});

test('create, replace and hide publish immediately with persistent real photographs',async({page},info)=>{
 await login(page);await page.goto('/super-admin/products');await expect(page.getByRole('link',{name:'Manage categories',exact:true})).toBeVisible();
 await page.getByRole('link',{name:'Add category',exact:true}).click();await ready(page);let dialog=page.getByRole('dialog',{name:'Add category',exact:true});
 const name=`Craft category ${info.project.name} ${Date.now()}`;
 await dialog.getByLabel('Category name',{exact:true}).fill(name);const slug=await dialog.locator('[name=id]').inputValue();expect(slug).toMatch(/^craft-category-/);
 await dialog.locator('[name=image_file]').setInputFiles(photograph);await expect(dialog.locator('[data-category-preview]')).toBeVisible();
 await dialog.getByLabel('Audit reason').fill('Verify immediate category publication and real image storage');
 storageWrite(false);
 try {await dialog.getByRole('button',{name:'Save category',exact:true}).click();await expect(dialog.locator('.ui-form-error')).toContainText('could not be stored');}
 finally {storageWrite(true);}
 expect(await dialog.locator('[name=image_file]').evaluate(el=>el.files.length)).toBe(1);await expect(dialog.getByLabel('Category name',{exact:true})).toHaveValue(name);
 await dialog.getByRole('button',{name:'Save category',exact:true}).click();await expect(page).toHaveURL(/notice=Category/);await expect(page.getByRole('dialog')).toHaveCount(0);
 const oldURL=sql(`SELECT image_url FROM categories WHERE slug='${slug}'`);expect(oldURL).toMatch(/^\/media\/storefront\/item-/);
 const home=await page.request.get('/');expect(await home.text()).toContain(`data-storefront-category="${slug}"`);
 expect(await (await page.request.get('/categories')).text()).toContain('category='+slug);expect(await (await page.request.get('/products')).text()).toContain('value="'+slug+'"');
 const bytes=await page.request.get(oldURL+'?w=480');expect(bytes.status()).toBe(200);expect(bytes.headers()['content-type']).toBe('image/jpeg');
 expect((await page.request.get('/categories/'+slug)).status()).toBe(200);
 dialog=await open(page,'edit='+slug);await expect(dialog.locator('[name=id]')).toHaveAttribute('readonly','');
 await dialog.getByLabel('Category name',{exact:true}).fill(name+' updated');await dialog.locator('[name=image_file]').setInputFiles(photograph);await dialog.getByLabel('Audit reason').fill('Replace category photograph');
 await dialog.getByRole('button',{name:'Save category',exact:true}).click();await expect(page).toHaveURL(/notice=Category/);
 const nextURL=sql(`SELECT image_url FROM categories WHERE slug='${slug}'`);expect(nextURL).not.toBe(oldURL);expect((await page.request.get(oldURL)).status()).toBe(200);expect((await page.request.get(nextURL)).status()).toBe(200);
 dialog=await open(page,'edit='+slug);await dialog.getByLabel('Show in customer discovery').uncheck();await dialog.getByLabel('Audit reason').fill('Hide without removing product assignments');await dialog.getByRole('button',{name:'Save category',exact:true}).click();await expect(page).toHaveURL(/notice=Category/);
 expect(await (await page.request.get('/')).text()).not.toContain(`data-storefront-category="${slug}"`);expect((await page.request.get('/categories/'+slug)).status()).toBe(200);
 expect(await (await page.request.get('/categories')).text()).not.toContain('category='+slug);expect(await (await page.request.get('/products')).text()).not.toContain('value="'+slug+'"');
 await page.goto('/super-admin/products/new');await expect(page.locator('select[name=category_slug]')).toContainText(name+' updated');
});

test('permissions, CSRF, stale edits, invalid media and failed publications preserve saved data',async({page,browser})=>{
 await login(page);let dialog=await open(page);const csrf=await token(page),version=await dialog.locator('[name=expected_release]').inputValue();
 const before=sql('SELECT count(*) FROM storefront_media');
 const fields={csrf_token:csrf,id:'arts',name:'Must not replace Arts',display_order:'1000',show_in_navigation:'true',reason:'Verify rejected mutations preserve published records',expected_release:version};
 expect((await page.request.post('/super-admin/category-management/update',{form:{...fields,csrf_token:'wrong'}})).status()).toBe(403);
 expect((await page.request.post('/super-admin/category-management/update',{form:{...fields,expected_release:'999999999'}})).status()).toBe(409);
 expect((await page.request.post('/admin/category-management/update',{form:{...fields,expected_release:'999999999'}})).status()).toBe(409);
 const invalid=await page.request.post('/super-admin/category-management/create',{multipart:{...fields,id:'invalid-image-qa',image_file:{name:'fake.png',mimeType:'image/png',buffer:Buffer.from('not an image')}}});expect(invalid.status()).toBe(422);
 const fs=require('node:fs');
 const oversized=await page.request.post('/super-admin/category-management/create',{multipart:{...fields,id:'oversized-image-qa',image_file:{name:'large.png',mimeType:'image/png',buffer:Buffer.concat([fs.readFileSync(photograph),Buffer.alloc((8<<20)+1-fs.statSync(photograph).size)])}}});expect(oversized.status()).toBe(422);
 const duplicate=await page.request.post('/super-admin/category-management/create',{multipart:{...fields,image_file:{name:'real.png',mimeType:'image/png',buffer:fs.readFileSync(photograph)}}});expect(duplicate.status()).toBe(409);
 expect(sql('SELECT count(*) FROM storefront_media')).toBe(before);expect(sql("SELECT name FROM categories WHERE slug='arts'")).not.toBe(fields.name);
 storageWrite(false);
 try { const outage=await page.request.post('/super-admin/category-management/create',{multipart:{...fields,id:'storage-outage-'+Date.now(),image_file:{name:'real.png',mimeType:'image/png',buffer:fs.readFileSync(photograph)}}});expect(outage.status()).toBe(503);expect(await outage.text()).toContain('could not be stored'); }
 finally {storageWrite(true);}
 expect(sql('SELECT count(*) FROM storefront_media')).toBe(before);

 const context=await browser.newContext({baseURL:'http://127.0.0.1:18087'}),admin=await context.newPage();await login(admin,'marketplace01');await admin.goto('/admin/category-management');const adminToken=await token(admin);
 expect((await admin.request.post('/super-admin/category-management/update',{form:{...fields,csrf_token:adminToken}})).status()).toBe(403);
 expect((await admin.request.post('/admin/category-management/update',{form:{...fields,csrf_token:adminToken}})).status()).toBe(409);await context.close();
 expect((await page.request.post('/super-admin/storefront/new',{form:{csrf_token:csrf}})).status()).toBe(410);
 dialog=await open(page,'edit=arts');await dialog.getByLabel('Category name',{exact:true}).fill('Unsaved retained name');await dialog.getByLabel('Audit reason').fill('Check stale form recovery');
 await dialog.locator('[name=expected_release]').evaluate(el=>el.value='999999999');await dialog.getByRole('button',{name:'Save category',exact:true}).click();await expect(dialog.locator('.ui-form-error')).toContainText('storefront changed');await expect(dialog.getByLabel('Category name',{exact:true})).toHaveValue('Unsaved retained name');await expect(dialog.getByRole('link',{name:'Reload latest categories'})).toBeVisible();
});

test('responsive category editor keeps focus, unsaved values and readable actions',async({page},info)=>{
 await login(page);const dialog=await open(page);await dialog.getByLabel('Category name',{exact:true}).fill('A long artisan category name for woven home furnishings and handmade gifts');
 await dialog.locator('[name=image_file]').setInputFiles(photograph);await expect(dialog.locator('[data-category-preview]')).toBeVisible();
 for(const width of [320,390,768,1024,1440]){await page.setViewportSize({width,height:950});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();await page.screenshot({path:test.info().outputPath(`category-${width}.png`)});}
 await page.setViewportSize({width:1280,height:950});await page.evaluate(()=>document.documentElement.style.zoom='2');expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();await page.evaluate(()=>document.documentElement.style.zoom='');
 await dialog.getByRole('button',{name:'Cancel',exact:true}).focus();await page.keyboard.press('Tab');expect(await page.evaluate(()=>!!document.activeElement.closest('dialog'))).toBeTruthy();
 await page.keyboard.press('Escape');await expect(page.locator('.ui-confirm')).toBeVisible();await page.locator('.ui-confirm').getByRole('button',{name:'Keep editing',exact:true}).click();await expect(dialog.getByLabel('Category name',{exact:true})).toHaveValue(/A long artisan/);
 await page.addScriptTag({path:axe});const violations=await page.evaluate(async()=>{const result=await axe.run(document.querySelector('[data-category-editor]'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}});return result.violations.map(v=>v.id);});expect(violations).toEqual([]);
 await page.keyboard.press('Escape');await page.locator('.ui-confirm').getByRole('button',{name:'Discard changes',exact:true}).click();await expect(dialog).not.toBeVisible();await expect(page.locator('[data-category-search]')).toBeFocused();
});
