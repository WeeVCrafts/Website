const {test,expect}=require('@playwright/test');
const path=require('node:path');
const fs=require('node:fs/promises');
const os=require('node:os');
const base=process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18080';
const output=path.join(os.tmpdir(),'weevcrafts-catalog-20261016');
test.use({baseURL:base});
test.beforeAll(async()=>{
 if(new URL(base).hostname!=='127.0.0.1'||new URL(base).port!=='18080')throw new Error('Catalog QA requires the isolated app at 127.0.0.1:18080.');
 await fs.mkdir(output,{recursive:true});
});
async function ready(page){await page.evaluate(()=>document.fonts.ready);await page.waitForFunction(()=>document.querySelector('[data-hero]')?.dataset.playing!==undefined);}
async function login(page,account='marketplace01'){
 const response=await page.request.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}});expect(response.ok()).toBeTruthy();
}

test('autoplay advances after four seconds at every supported width without playback buttons',async({page})=>{
 test.setTimeout(90000);
 const instant=new Date();
 await page.clock.install({time:instant});
 await page.clock.pauseAt(new Date(instant.getTime()+1000));
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:1000});
  await page.goto('/');await ready(page);await page.mouse.move(0,0);
  await expect(page.locator('[data-hero-autoplay]')).toHaveCount(0);
  await expect(page.locator('[data-hero]')).toHaveAttribute('data-playing','true');
  await page.clock.runFor(4100);
  await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
  await expect(page.locator('[data-hero-slide]:not([hidden])')).toHaveCount(1);
  await expect(page.locator('[data-hero-slide][inert]')).toHaveCount(4);
  if(width===390){
   for(const slide of [3,4,5,1]){
    await page.clock.runFor(4100);
    await expect(page.locator('[data-hero-count]')).toHaveText(`${slide} / 5`);
   }
   await page.locator('.home-gifts-band').scrollIntoViewIfNeeded();
   await expect(page.locator('[data-hero]')).toHaveAttribute('data-playing','false');
   await page.clock.runFor(8200);
   await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
  }
  expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
 }
});

test('swipe, keyboard focus, hover, reduced motion and failed images keep the current slide usable',async({page})=>{
 await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});
 await page.goto('/');await ready(page);
 const hero=page.locator('[data-hero]');
 await hero.dispatchEvent('touchstart',{touches:[{identifier:1,clientX:280,clientY:250}]});
 await hero.dispatchEvent('touchend',{changedTouches:[{identifier:1,clientX:100,clientY:255}]});
 await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
 await expect(hero).toHaveAttribute('data-playing','false');
 await page.locator('[data-hero-prev]').press('Enter');
 await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
 await page.route('**/*textiles-banner*',route=>route.abort());
 await page.reload();await ready(page);
 await page.locator('[data-hero-next]').press('Enter');
 await expect(page.locator('[data-hero-status]')).toContainText('could not load');
 await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
 await page.unroute('**/*textiles-banner*');
 await page.emulateMedia({reducedMotion:'no-preference'});await page.mouse.move(0,0);
 await page.locator('#global-search').click();
 await expect(hero).toHaveAttribute('data-playing','true');
 await hero.hover();await expect(hero).toHaveAttribute('data-playing','false');
 await page.mouse.move(0,0);await expect(hero).toHaveAttribute('data-playing','true');
 await page.locator('[data-hero-prev]').press('Tab');
 await expect(hero).toHaveAttribute('data-playing','false');
});

test('four catalog sections preserve responsive cards and category navigation',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:1000});await page.goto('/');await ready(page);
  for(const title of ['Featured from our makers','New arrivals','Shop by category','Gifts under ₹1,500'])await expect(page.getByRole('heading',{name:title,exact:true})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
  await expect(page.locator('.home-arrivals-track .product-card')).toHaveCount(8);
  await page.screenshot({path:path.join(output,`home-${width}.png`),fullPage:true});
 }
 const categoryLink=page.locator('.home-category-selectors a[href="/?home_category=home#home-by-category"]');
  const categoryName=await categoryLink.textContent();await categoryLink.click();
 await expect(page).toHaveURL(/home_category=home/);
 await expect(page.locator('.home-category-selectors [aria-current=true]')).toHaveText(categoryName);
 const rail=page.locator('[data-rail-track]');await page.getByRole('button',{name:'Next new arrivals'}).click();await expect.poll(()=>rail.evaluate(el=>el.scrollLeft)).toBeGreaterThan(0);
});

test('admin saves category media, order and membership; other roles cannot manage taxonomy',async({page,browser})=>{
 test.setTimeout(120000);await login(page);await page.goto('/admin/category-management');
 const name=`QA Catalog ${Date.now()}`,slug=name.toLowerCase().replaceAll(' ','-');
 await page.locator('summary').filter({hasText:'Create category'}).press('Enter');
 const create=page.locator('form[action="/admin/category-management/create"]');
 await create.getByLabel('Category name',{exact:true}).press('Tab');
 await expect(create.getByLabel('URL slug',{exact:true})).toBeFocused();
 await create.getByLabel('Category name',{exact:true}).fill(name);await create.getByLabel('URL slug',{exact:true}).fill(slug);await create.getByLabel('Display order').fill('1');
 await create.getByLabel('Audit reason').fill('Verify local category administration');
 await create.locator('[name=image_file]').setInputFiles({name:'invalid.png',mimeType:'image/png',buffer:Buffer.from('This is not a real photograph')});
 await create.getByRole('button',{name:'Create category'}).click();
 await expect(create.locator('.ui-form-error')).toContainText('valid JPEG');
 await expect(create.getByLabel('Category name',{exact:true})).toHaveValue(name);
 await create.locator('[name=image_file]').setInputFiles(path.resolve('web/assets/images/customer/ceramic-mugs.webp'));
 await create.getByLabel('Audit reason').fill('Verify local category administration');
 await create.getByRole('button',{name:'Create category'}).click();
 await expect(page).toHaveURL(/notice=/);
 let record=page.locator('.category-management-record').filter({has:page.getByRole('heading',{name,exact:true})});
 await expect(record).toBeVisible();const image=record.locator('img');await expect(image).toHaveAttribute('src',/\/media\/categories\//);expect((await page.request.get(await image.getAttribute('src'))).ok()).toBeTruthy();
 await record.getByRole('link',{name:'Manage products'}).click();
 const assignments=page.locator('#category-products');const selectedProduct=assignments.locator('input[name=product_id]').first();const selectedID=await selectedProduct.getAttribute('value');await selectedProduct.check();await assignments.getByLabel('Audit reason').fill('Cross list the selected product');
 await assignments.getByRole('button',{name:'Add selected products'}).click();await expect(page).toHaveURL(/notice=/);
 const listing=await page.request.get(`/products?category=${slug}`);expect(listing.ok()).toBeTruthy();expect(await listing.text()).toContain('product-card');
 await page.goto('/');await ready(page);await expect(page.locator('.discovery-categories a').first()).toHaveText(name);
 await page.goto(`/admin/products/${selectedID}/edit`);
 let productEditor=page.locator('form.admin-product-editor-form').first();
 const primary=productEditor.locator('[name=category_slug]');const originalPrimary=await primary.inputValue();const originalRank=await productEditor.locator('[name=featured_rank]').inputValue();
 await primary.selectOption(slug);await productEditor.locator(`[name=additional_category][value="${originalPrimary}"]`).uncheck();await productEditor.locator('[name=additional_category][value=gifts]').check();await productEditor.locator('[name=featured_rank]').fill('3');
 await productEditor.getByRole('button',{name:'Save product',exact:true}).click();await expect(page).toHaveURL(/\/admin\/products\?notice=/);
 await page.goto('/admin/category-management');
 const protectedToken=await page.locator('input[name=csrf_token]').first().inputValue();
 expect((await page.request.post('/admin/category-management/assign-products',{form:{csrf_token:protectedToken,id:slug,product_id:selectedID,intent:'remove',reason:'Verify primary membership protection'}})).status()).toBe(409);
 await page.goto(`/admin/products/${selectedID}/edit`);productEditor=page.locator('form.admin-product-editor-form').first();
 await expect(productEditor.locator('[name=category_slug]')).toHaveValue(slug);await expect(productEditor.locator('[name=additional_category][value=gifts]')).toBeChecked();await expect(productEditor.locator('[name=featured_rank]')).toHaveValue('3');
 await productEditor.locator('[name=category_slug]').selectOption(originalPrimary);await productEditor.locator(`[name=additional_category][value="${slug}"]`).check();await productEditor.locator('[name=featured_rank]').fill(originalRank);await productEditor.getByRole('button',{name:'Save product',exact:true}).click();await expect(page).toHaveURL(/\/admin\/products\?notice=/);

 await page.goto('/admin/category-management');record=page.locator('.category-management-record').filter({has:page.getByRole('heading',{name,exact:true})});
 const previousImage=await record.locator('img').getAttribute('src');
 await record.getByText('Edit category',{exact:true}).press('Enter');const edit=record.locator('form[action$="/update"]');await edit.locator('[name=show_in_navigation]').uncheck();await edit.locator('[name=image_file]').setInputFiles(path.resolve('web/assets/images/customer/wooden-box.webp'));await edit.getByLabel('Audit reason').fill('Replace category image and hide discovery link');await edit.getByRole('button',{name:'Save category'}).click();await expect(page).toHaveURL(/notice=/);await page.waitForLoadState('domcontentloaded');
 const replacementImage=await page.locator('.category-management-record').filter({has:page.getByRole('heading',{name,exact:true})}).locator('img').getAttribute('src');
 expect(replacementImage).not.toBe(previousImage);expect((await page.request.get(replacementImage)).status()).toBe(200);expect((await page.request.get(previousImage)).status()).toBe(404);
 await page.goto('/');await ready(page);await expect(page.locator('.discovery-categories').getByRole('link',{name,exact:true})).toHaveCount(0);
 expect((await page.request.get(`/products?category=${slug}`)).ok()).toBeTruthy();
 for(const account of ['owner01','customer08']){
  const context=await browser.newContext({baseURL:base});const other=await context.newPage();await login(other,account);const response=await other.request.get('/admin/category-management');expect(response.status()).toBe(403);await context.close();
 }
 await page.goto('/admin/category-management');await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(output,'admin-categories-390.png'),fullPage:true});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
 const csrf=await page.locator('input[name=csrf_token]').first().inputValue();
 expect((await page.request.post('/admin/category-management/assign-products',{form:{csrf_token:csrf,id:slug,product_id:selectedID,intent:'remove',reason:'Remove isolated QA assignment'}})).ok()).toBeTruthy();
 expect((await page.request.post('/admin/category-management/delete',{form:{csrf_token:csrf,id:slug,reason:'Remove isolated QA category'}})).ok()).toBeTruthy();
});
