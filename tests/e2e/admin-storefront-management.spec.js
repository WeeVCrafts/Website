const {test,expect}=require('@playwright/test');
const path=require('node:path');
const os=require('node:os');
const fs=require('node:fs/promises');
const base=process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18085';
const output=path.join(os.tmpdir(),'weev-admin-guided-review');
test.use({baseURL:base,trace:'off'});
test.beforeAll(async()=>{
 const url=new URL(base);
 if(url.hostname!=='127.0.0.1'||url.port!=='18085'||process.env.WEEVCRAFTS_QA_DATABASE!=='storefront_guided_20261017')throw new Error('Requires the isolated storefront QA application.');
 await fs.mkdir(output,{recursive:true});
});
async function login(page,role='super01'){
 expect((await page.request.post('/api/v1/auth/login',{data:{email:`demo.${role}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}})).ok()).toBeTruthy();
}
async function ready(page){await page.waitForFunction(()=>window.WVUI);}
async function draft(page){await page.goto('/super-admin/storefront');await ready(page);await page.getByRole('button',{name:'New draft from published design'}).click();await expect(page).toHaveURL(/draft=\d+/);return new URL(page.url()).searchParams.get('draft');}
async function open(page,id,tab,section=''){await page.goto('/super-admin/storefront?'+new URLSearchParams({draft:id,tab,section}));await ready(page);}
async function token(page){return page.locator('form[action="/logout"] [name=csrf_token]').first().inputValue();}

test('Users uses a focused modal and preserves filters and rejected values',async({page})=>{
 const email=`admin.modal.${Date.now()}@example.invalid`;
 expect((await page.request.post('/api/v1/auth/register',{data:{email,password:'Modal-QA-2026-safe!',display_name:'Modal QA customer',phone:'+919876543210',accepted_terms:true}})).ok()).toBeTruthy();
 await login(page);await page.goto('/super-admin/users?'+new URLSearchParams({q:email,status:'active',role:'customer'}));await ready(page);
 const manage=page.getByRole('button',{name:/Manage account for/});await manage.click();
 const dialog=page.getByRole('dialog',{name:'Edit account'});await expect(dialog).toBeVisible();
 await dialog.getByRole('button',{name:'Save account',exact:true}).focus();await page.keyboard.press('Tab');expect(await page.evaluate(()=>document.activeElement.closest('dialog')?.open)).toBeTruthy();
 await dialog.getByLabel('Display name').fill('Updated modal customer');
 await dialog.locator('input[name=email]').fill('demo.super01@weevcrafts.local');
 await dialog.getByLabel('Reason',{exact:true}).fill('Verify rejected duplicate account email');
 await dialog.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(dialog.locator('.ui-form-error')).toContainText(/Email|email/);
 await expect(dialog.getByLabel('Display name')).toHaveValue('Updated modal customer');
 await dialog.locator('input[name=email]').fill(email);
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:900});
  expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();
  await page.screenshot({path:path.join(output,`account-${width}.png`)});
 }
 await dialog.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(page).toHaveURL(new RegExp('q='+encodeURIComponent(email)));await expect(page).toHaveURL(/status=active/);
 await expect(page.locator('.user-account-cell').getByText('Updated modal customer',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:/Manage account for/}).click();await page.keyboard.press('Escape');await expect(dialog).not.toBeVisible();await expect(page.getByRole('button',{name:/Manage account for/})).toBeFocused();
 await page.goto('/super-admin/security');expect(await page.locator('form[action$="/users/update"],form[action$="/roles/update"],form[action$="/users/create"]').count()).toBe(0);
 await expect(page.getByRole('link',{name:'Open Users',exact:true})).toBeVisible();
 await page.goto('/super-admin/roles?q='+encodeURIComponent(email));expect(await page.locator('form[action$="/users/update"]').count()).toBe(0);expect(await page.locator('form[action$="/roles/update"]').count()).toBeGreaterThan(0);
});

test('Draft deletion confirms identity and enforces CSRF, roles, version and state',async({page,browser})=>{
 await login(page);const id=await draft(page);const csrf=await token(page);const live=(await page.request.get('/')).headers()['x-storefront-revision'];
 const action='/super-admin/storefront/delete-draft';
 expect((await page.request.post(action,{form:{draft:id,version:'1'}})).status()).toBe(403);
 expect((await page.request.post(action,{form:{draft:id,version:'999',csrf_token:csrf}})).status()).toBe(409);
 const context=await browser.newContext({baseURL:base});const admin=await context.newPage();await login(admin,'marketplace01');await admin.goto('/admin/storefront');
 expect((await admin.request.post(action,{form:{draft:id,version:'1',csrf_token:await token(admin)}})).status()).toBe(403);await context.close();
 await page.goto('/super-admin/storefront');const row=page.locator('.sf-draft-list li').filter({has:page.getByText(`Draft ${id}`,{exact:true})});await row.getByRole('button',{name:/Delete draft/}).click();
 const confirm=page.locator('.ui-confirm');await expect(confirm).toContainText(`Delete draft ${id} permanently`);await expect(confirm.getByRole('button',{name:'Cancel',exact:true})).toBeFocused();await confirm.getByRole('button',{name:'Cancel',exact:true}).click();await expect(row).toBeVisible();
 await row.getByRole('button',{name:/Delete draft/}).click();await page.locator('.ui-confirm').getByRole('button',{name:/Delete draft/}).click();await expect(row).toHaveCount(0);
 expect((await page.request.get('/')).headers()['x-storefront-revision']).toBe(live);
 expect((await page.request.post(action,{form:{draft:id,version:'1',csrf_token:await token(page)}})).status()).toBe(404);
 const submitted=await draft(page);await open(page,submitted,'publishing');await page.getByRole('button',{name:'Submit for review',exact:true}).click();await expect(page.locator('.sf-draft-state')).toContainText('Submitted');
 expect((await page.request.post(action,{form:{draft:submitted,version:'2',csrf_token:await token(page)}})).status()).toBe(409);
});

test('Categories use searchable thumbnails, image uploads and popup forms',async({page})=>{
 await login(page);const id=await draft(page);await open(page,id,'categories');await page.getByRole('button',{name:'Add category',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'Add category',exact:true});const slug='guided-popup-'+Date.now();
 await dialog.getByLabel('Category name',{exact:true}).fill('Guided popup category');await dialog.getByLabel('Permanent URL slug',{exact:true}).fill(slug);await dialog.getByLabel('Description',{exact:true}).fill('Photograph and category presentation saved together.');
 await dialog.getByRole('button',{name:'Choose photograph',exact:true}).click();const media=page.getByRole('dialog',{name:'Choose a photograph',exact:true});await media.getByText('Upload a new photograph',{exact:true}).click();
 await media.getByLabel('Photograph file').setInputFiles(path.resolve('web/assets/images/brand/weevcrafts-logo.png'));await media.getByLabel('Image description',{exact:true}).fill('Isolated category upload verification');
 await page.route('**/super-admin/storefront/upload',route=>route.fulfill({status:503,contentType:'text/plain',body:'Upload temporarily unavailable. Retry this photograph.'}),{times:1});
 await media.getByRole('button',{name:'Upload and use photograph',exact:true}).click();await expect(media.locator('[data-upload-status]')).toContainText('temporarily unavailable');expect(await media.getByLabel('Photograph file').evaluate(input=>input.files.length)).toBe(1);
 await page.route('**/super-admin/storefront/upload',async route=>{await new Promise(resolve=>setTimeout(resolve,500));await route.continue();},{times:1});
 await media.getByRole('button',{name:'Upload and use photograph',exact:true}).click();await expect(media.getByRole('progressbar')).toBeVisible();await expect(media).not.toBeVisible({timeout:30000});await expect(dialog.locator('.sf-image-control>img')).toBeVisible();
 await dialog.getByRole('button',{name:'Add category to draft',exact:true}).click();await expect(dialog).not.toBeVisible();
 const search=page.getByRole('searchbox',{name:'Search categories',exact:true});await search.fill('no-category-matches-this');await expect(page.locator('[data-category-empty]')).toBeVisible();await page.getByRole('button',{name:'Clear search',exact:true}).click();await search.fill('Guided popup category');
 await page.locator('.sf-category-list a').filter({hasText:'Guided popup category'}).click();const edit=page.getByRole('dialog',{name:'Edit category',exact:true});await expect(edit).toBeVisible();await expect(edit.locator('input[readonly]')).toHaveValue(slug);await expect(edit.locator('.sf-image-control>img')).toBeVisible();
 await edit.getByLabel('Category name',{exact:true}).fill('Unsaved category name');await page.keyboard.press('Escape');await expect(page.locator('.ui-confirm')).toBeVisible();await page.locator('.ui-confirm').getByRole('button',{name:'Keep editing',exact:true}).click();await expect(edit.getByLabel('Category name',{exact:true})).toHaveValue('Unsaved category name');
 for(const width of [320,390,768,1440]){await page.setViewportSize({width,height:900});expect(await edit.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();await page.screenshot({path:path.join(output,`category-${width}.png`)});}
});

test('Homepage rows, product order and slide controls retain the published storefront',async({page})=>{
 await login(page);const id=await draft(page);const live=(await page.request.get('/')).headers()['x-storefront-revision'];await open(page,id,'homepage');
 await expect(page.getByRole('link',{name:'Categories',exact:true})).toBeVisible();await expect(page.getByText('More tools',{exact:true})).toBeVisible();
 await page.getByText('More tools',{exact:true}).click();await page.getByRole('link',{name:'Media library',exact:true}).click();await expect(page.getByRole('heading',{name:'Media library',exact:true})).toBeVisible();await open(page,id,'homepage');
 await page.locator('.sf-add select').selectOption('curated');await page.getByRole('button',{name:'Add row',exact:true}).click();await expect(page.locator('.sf-product-picker')).toBeVisible();
 const choices=page.locator('.sf-product-picker input[name=product_id]');await choices.nth(0).check();await choices.nth(1).check();await expect(page.locator('[data-product-selection-count]')).toContainText('2 of 24');
 const first=await choices.nth(0).inputValue(),second=await choices.nth(1).inputValue();await page.locator('.sf-product-picker label').filter({has:page.locator(`input[value="${second}"]`)}).getByRole('button',{name:/Move up/}).click();await page.locator('[data-editor-form-save]').click();await expect(page.locator('.sf-draft-state')).toContainText('Saved draft');expect(await page.locator('.sf-product-picker input:checked').first().inputValue()).toBe(second);
 await page.getByLabel('Search catalog',{exact:true}).fill('ceramic');await page.getByRole('button',{name:'Find products',exact:true}).click();await expect(page).toHaveURL(/product_q=ceramic/);await expect(page.locator('.sf-product-picker input:checked')).toHaveCount(2);expect(await page.locator('.sf-product-picker input:checked').first().inputValue()).toBe(second);
 await open(page,id,'hero');await page.locator('.sf-items a').first().click();const slide=page.locator('.sf-visual-row').first();await expect(slide.locator('img')).toBeVisible();await slide.getByRole('button',{name:'Hide',exact:true}).click();await expect(page.locator('.sf-visual-row').first()).toContainText('Hidden');await page.locator('.sf-visual-row').first().getByRole('button',{name:'Show',exact:true}).click();await expect(page.locator('.sf-visual-row').first()).toContainText('Visible');
 for(const width of [320,390,768,1440]){await page.setViewportSize({width,height:900});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();await page.screenshot({path:path.join(output,`editor-${width}.png`)});}
 expect((await page.request.get('/')).headers()['x-storefront-revision']).toBe(live);
});


test('Seller profile links and bookmarks resolve without exposing customer account mutations',async({page})=>{
 await login(page,'owner01');
 await page.goto('/seller-admin');await ready(page);await expect(page.locator('.seller-user')).toHaveAttribute('href','/seller-admin/settings');await page.locator('.seller-user').click();await expect(page.getByRole('heading',{name:'Your public maker story',exact:true})).toBeVisible();
 for(const route of ['/seller-admin/profile','/seller-admin/account','/account','/account/profile']){const response=await page.goto(route);expect(response.status()).toBe(200);await expect(page).toHaveURL(/\/seller-admin\/settings$/);}
 await page.goto('/');await page.getByRole('button',{name:'Open profile menu',exact:true}).click();await expect(page.locator('#profile-dropdown').getByRole('link',{name:'Account & profile',exact:true})).toHaveAttribute('href','/seller-admin/settings');
 expect((await page.request.post('/account/profile',{form:{name:'Not a customer profile',phone:'+919876543210',csrf_token:await token(page)}})).status()).toBe(403);
 await page.goto('/seller-admin/settings');const form=page.locator('form[action="/seller-admin/settings"]');await form.getByLabel('Reason for change').fill('QA verifies seller profile save on own account');await form.getByRole('button',{name:'Save maker story',exact:true}).click();await expect(page).toHaveURL(/notice=Public/);await expect(page.getByRole('heading',{name:'Your public maker story',exact:true})).toBeVisible();
 for(const width of [390,1440]){await page.setViewportSize({width,height:900});await page.screenshot({path:path.join(output,`seller-profile-${width}.png`)});}
});
