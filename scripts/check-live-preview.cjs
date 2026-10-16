// Read-only acceptance against the actual local app. Never inserts fixtures.
const {chromium}=require('@playwright/test');
const fs=require('node:fs/promises');
const path=require('node:path');
const os=require('node:os');
const assert=require('node:assert/strict');
const base=process.env.WEEVCRAFTS_BASE_URL||'http://localhost:8090';
const destination=new URL(base);
if(!['localhost','127.0.0.1'].includes(destination.hostname))throw Error('Local preview only');
const output=process.env.WEEVCRAFTS_UI_ARTIFACTS||path.join(os.tmpdir(),'weevcrafts-live-preview-20261015');
(async()=>{
 await fs.mkdir(output,{recursive:true});
 const browser=await chromium.launch();
 const report={base,fixtureWrites:0,screens:[],errors:[],axe:[]};
 try{
  const context=await browser.newContext({baseURL:base,reducedMotion:'reduce'});
  const page=await context.newPage();
  page.on('pageerror',error=>report.errors.push(error.message));
  for(const width of [1680,1448,1440,1024,390,320]){
   await page.setViewportSize({width,height:width===1680?936:width>800?1086:844});
   for(const [name,route] of [['home','/'],['home-living','/products?category=home']]){
    const response=await page.goto(route);assert(response.ok(),route);
    await page.evaluate(()=>document.fonts.ready);
    await page.waitForFunction(()=>!!document.querySelector('.site-header')?._x_dataStack);
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${route} overflow ${width}`);
    assert(await page.locator('.wordmark-mark').first().evaluate(image=>image.complete&&image.naturalWidth>0),'Supplied logo failed to load');
    assert.equal(await page.locator('.header-main .location,.artisan-link,[data-hero-autoplay]').count(),0,'Removed controls must not remain in the document');
    assert.equal(await page.locator('.wordmark strong').first().textContent(),'WeevCrafts');
    if(route==='/') {
     assert.equal(await page.title(),'WeevCrafts | Handmade Indian Arts, Crafts & Sarees');
     const heroAction = width > 800
      ? page.locator('.desktop-home-intro [data-hero-slide]:not([hidden]) .primary-button')
      : page.locator('.mobile-home-copy .primary-button');
     assert.equal(await heroAction.evaluate(element=>getComputedStyle(element).backgroundColor),'rgb(18, 63, 50)');
     if(width>800) assert(await page.locator('.wordmark-mark').first().evaluate(element=>element.getBoundingClientRect().width>=48),'Desktop logo must remain prominent');
     await page.screenshot({path:path.join(output,`home-${width}-viewport.png`)});
    }
    await page.screenshot({path:path.join(output,`${name}-${width}.png`),fullPage:true});
    report.screens.push({route,width});
   }
  }
  await page.setViewportSize({width:390,height:844});
  await page.goto('/products?category=home');
  await page.locator('[data-filter-open]').click();
  await page.locator('[data-filter-panel] input[name="availability"]').check();
  await page.waitForURL(/availability=in-stock/);
  assert(await page.locator('[data-filter-panel]').evaluate(element=>element.classList.contains('is-open')));
  await page.keyboard.press('Escape');
  assert(await page.locator('[data-filter-open]').evaluate(element=>element===document.activeElement));
  report.filters='live URL update, preserved open sheet and restored focus';
  for(const route of ['/','/products','/categories','/makers','/brands','/deals','/search?q=handmade','/cart','/wishlist','/checkout','/account','/orders','/returns','/payments','/notifications','/contact','/faq','/shipping','/terms','/privacy','/about','/our-story','/sustainability','/press','/careers','/login','/register','/forgot-password']){
   await page.goto(route);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),route+' narrow overflow');
   assert(!page.url().includes(':18080'),'Isolated preview must not be used');
  }
  for(const route of ['/','/products?category=home','/login','/contact']){
   await page.goto(route);await page.addScriptTag({path:require.resolve('axe-core')});
   const violations=await page.evaluate(async()=> (await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})));
   report.axe.push({route,violations});assert.equal(violations.length,0,JSON.stringify({route,violations}));
  }
  await page.setViewportSize({width:1440,height:1000});
  await page.goto('/');
  assert.equal(await page.locator('[data-hero-slide]').count(),5);
  for(let index=0;index<5;index++){
   await page.locator(`[data-hero-select="${index}"]`).press('Enter');
   assert.equal(await page.locator('[data-hero-count]').textContent(),`${index+1} / 5`);
   const image=page.locator('[data-hero-slide]:not([hidden]) img');
   await image.evaluate(image=>image.decode());
  }
  assert.equal(report.errors.length,0,report.errors.join('\n'));
  report.pass=true;console.log(JSON.stringify(report,null,2));
 }finally{
  await fs.writeFile(path.join(output,'verification.json'),JSON.stringify(report,null,2));
  await browser.close();
 }
})().catch(error=>{console.error(error);process.exitCode=1});
