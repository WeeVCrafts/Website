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
  expect(await page.locator('body').evaluate(el=>getComputedStyle(el).fontFamily)).toContain('DM Sans');
  expect(await page.locator('[data-hero-slide]:not([hidden]) h1').evaluate(el=>parseFloat(getComputedStyle(el).fontSize))).toBeGreaterThan(width>800?38:20);
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
 const categoryLink=page.locator('.home-category-selectors a[href="/?home_category=home#section-by-category"]');
  const categoryName=await categoryLink.textContent();await categoryLink.click();
 await expect(page).toHaveURL(/home_category=home/);
 await expect(page.locator('.home-category-selectors [aria-current=true]')).toHaveText(categoryName);
 const rail=page.locator('[data-rail-track]');await page.getByRole('button',{name:'Next New arrivals'}).click();await expect.poll(()=>rail.evaluate(el=>el.scrollLeft)).toBeGreaterThan(0);
});

test('legacy taxonomy routes require publication and catalog membership remains available',async({page})=>{
 await login(page);await page.goto('/admin/category-management?category=gifts');
 await expect(page.getByRole('link',{name:'Manage category presentation in Storefront'})).toBeVisible();
 const csrf=await page.locator('[name=csrf_token]').first().inputValue();
 expect((await page.request.post('/admin/category-management/update',{form:{csrf_token:csrf,id:'gifts',name:'Bypass'}})).status()).toBe(409);
 const assignments=page.locator('#category-products');await expect(assignments).toBeVisible();
 const product=assignments.locator('[name=product_id]:not(:checked)').first();const id=await product.inputValue();await product.check();await assignments.getByLabel('Audit reason').fill('Verify catalog cross-listing without changing presentation');
 await assignments.getByRole('button',{name:'Add selected products'}).click();await expect(page).toHaveURL(/notice=/);
 const token=await page.locator('[name=csrf_token]').first().inputValue();
 expect((await page.request.post('/admin/category-management/assign-products',{form:{csrf_token:token,id:'gifts',product_id:id,intent:'remove',reason:'Restore isolated QA membership'}})).ok()).toBeTruthy();
});
