const {test,expect}=require('@playwright/test');
const fs=require('node:fs/promises');
const path=require('node:path');
const os=require('node:os');
const captures=process.env.WEEVCRAFTS_HOME_CAPTURES||path.join(os.tmpdir(),'weevcrafts-homepage-captures');
test.beforeAll(async({baseURL})=>{
 if(baseURL!=='http://127.0.0.1:18088'||process.env.WEEVCRAFTS_QA_DATABASE!=='homepage_reference_20261103')throw new Error('Homepage checks require the dedicated loopback QA database.');
 await fs.mkdir(captures,{recursive:true});
});
async function ready(page,fonts=true){
 if(fonts)await page.evaluate(()=>document.fonts.ready);
 await expect(page.locator('[data-hero]')).toHaveAttribute('data-playing',/true|false/);
 await expect.poll(()=>page.locator('[data-hero-slide] img').first().evaluate(img=>img.complete&&img.naturalWidth>0)).toBe(true);
}
async function nextReady(page,index){await expect.poll(()=>page.locator('[data-hero-slide] img').nth(index).evaluate(img=>img.complete&&img.naturalWidth>0)).toBe(true);}
async function noOverflow(page){const sizes=await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth}));expect(sizes.scrollWidth,`Document overflow at ${sizes.width}px`).toBeLessThanOrEqual(sizes.width+1);}

test('three-second autoplay cycles all five slides and pauses safely',async({page})=>{
 const time=new Date();await page.clock.install({time});await page.clock.pauseAt(new Date(time.getTime()+1000));
 await page.setViewportSize({width:1536,height:1024});await page.goto('/');await ready(page);await page.mouse.move(0,0);
 await nextReady(page,1);
 await expect(page.locator('[data-hero-select]')).toHaveCount(5);
 await page.clock.runFor(2999);await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
 await page.clock.runFor(1);await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
 for(const index of [2,3,4,0]){
  await nextReady(page,index);await page.clock.runFor(3000);await expect(page.locator('[data-hero-count]')).toHaveText(`${index+1} / 5`);
 }
 await page.clock.runFor(500);await expect(page.locator('[data-hero-slide]:not([hidden])')).toHaveCount(1);await expect(page.locator('[data-hero-slide][inert]')).toHaveCount(4);
 const hero=page.locator('[data-hero]');
 await hero.hover();await expect(hero).toHaveAttribute('data-playing','false');await page.clock.runFor(6000);await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
 await page.mouse.move(0,0);await expect(hero).toHaveAttribute('data-playing','true');
 await page.locator('[data-hero-select="0"]').focus();await page.keyboard.press('Tab');await expect(hero).toHaveAttribute('data-playing','false');await page.clock.runFor(6000);
 await page.locator('#global-search').focus();await expect(hero).toHaveAttribute('data-playing','true');
 await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));});
 await expect(hero).toHaveAttribute('data-playing','false');await page.clock.runFor(6000);await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
 await page.evaluate(()=>{delete document.hidden;document.dispatchEvent(new Event('visibilitychange'));});await expect(hero).toHaveAttribute('data-playing','true');
 await page.locator('footer.site-footer').scrollIntoViewIfNeeded();await expect(hero).toHaveAttribute('data-playing','false');await page.clock.runFor(6000);await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
 await hero.scrollIntoViewIfNeeded();await page.mouse.move(0,0);await expect(hero).toHaveAttribute('data-playing','true');await page.clock.runFor(2999);await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');await page.clock.runFor(1);await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
});

test('manual arrows, indicators, swipe, reset and reduced motion remain usable',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await page.setViewportSize({width:390,height:844});await page.goto('/');await ready(page);
 const count=page.locator('[data-hero-count]'),hero=page.locator('[data-hero]');
 await page.getByRole('button',{name:'Previous collection',exact:true}).press('Enter');await expect(count).toHaveText('5 / 5');
 await page.getByRole('button',{name:'Next collection',exact:true}).press('Enter');await expect(count).toHaveText('1 / 5');
 await page.locator('[data-hero-select="2"]').press('Enter');await expect(count).toHaveText('3 / 5');
 await hero.evaluate(el=>{for(const [type,key,x,y] of [['touchstart','touches',290,250],['touchend','changedTouches',90,255]]){const event=new Event(type,{bubbles:true});Object.defineProperty(event,key,{value:[{identifier:1,clientX:x,clientY:y}]});el.dispatchEvent(event);}});await expect(count).toHaveText('4 / 5');
 await expect(hero).toHaveAttribute('data-playing','false');expect(await page.locator('[data-hero-slide]').nth(3).evaluate(el=>el.getAnimations().length)).toBe(0);
 const time=new Date();await page.clock.install({time});await page.clock.pauseAt(new Date(time.getTime()+1000));await page.clock.runFor(9000);await expect(count).toHaveText('4 / 5');
 await page.locator('[data-hero-select="0"]').click();await expect(count).toHaveText('1 / 5');await nextReady(page,1);await page.locator('#global-search').focus();await page.mouse.move(0,0);await page.emulateMedia({reducedMotion:'no-preference'});await expect(hero).toHaveAttribute('data-playing','true');
 await page.clock.runFor(2000);await page.getByRole('button',{name:'Next collection',exact:true}).click();await expect(count).toHaveText('2 / 5');await nextReady(page,2);await page.locator('#global-search').focus();await page.mouse.move(0,0);
 await page.clock.runFor(2999);await expect(count).toHaveText('2 / 5');await page.clock.runFor(1);await expect(count).toHaveText('3 / 5');
});

test('slow and failed photographs retain the usable slide',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});let release;const pending=new Promise(resolve=>{release=resolve;});
 await page.route('**/*textiles-banner*',async route=>{await pending;await route.abort();});
 try{
  await page.goto('/',{waitUntil:'domcontentloaded'});await ready(page,false);
  await page.locator('[data-hero-select="1"]').click();await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');await expect(page.locator('[data-hero-slide]').first()).toBeVisible();
  release();await expect(page.locator('[data-hero-status]')).toContainText('could not load');await expect(page.locator('[data-hero-count]')).toHaveText('1 / 5');
  await page.locator('[data-hero-select="2"]').click();await expect(page.locator('[data-hero-count]')).toHaveText('3 / 5');
 }finally{release();}
});

test('reference composition, responsive controls, long labels and keyboard drawer',async({page},testInfo)=>{
 await page.emulateMedia({reducedMotion:'reduce'});
 for(const width of [320,390,768,1024,1536]){
  await page.setViewportSize({width,height:1024});await page.goto('/');await ready(page);await noOverflow(page);
  expect(await page.locator('.section-head .text-link').evaluateAll(links=>links.every(link=>link.scrollWidth<=link.clientWidth+1))).toBe(true);
  await expect(page.locator('.homepage-utility')).toBeVisible();await expect(page.locator('.trust-strip')).toHaveCount(1);await expect(page.locator('#section-featured .product-card')).toHaveCount(6);
  expect(await page.locator('.category-rail').evaluate(el=>el.compareDocumentPosition(document.querySelector('.trust-strip'))&Node.DOCUMENT_POSITION_FOLLOWING)).toBeTruthy();
  expect(await page.locator('.trust-strip').evaluate(el=>el.compareDocumentPosition(document.querySelector('#section-featured'))&Node.DOCUMENT_POSITION_FOLLOWING)).toBeTruthy();
  await expect(page.getByRole('link',{name:'Sign in',exact:true}).first()).toBeVisible();
  for(const control of ['[data-hero-prev]','[data-hero-next]','[data-hero-select="0"]']){
   const box=await page.locator(control).boundingBox();expect(box.width).toBeGreaterThanOrEqual(44);expect(box.height).toBeGreaterThanOrEqual(44);
  }
  await page.screenshot({path:path.join(captures,`${testInfo.project.name}-home-${width}.png`),fullPage:width!==1536});
  await page.locator('.discovery-category').first().evaluate(el=>{el.lastChild.textContent='Arts and extraordinarily long handmade collection names';});await noOverflow(page);
 }
 // 1536px at 200% browser reflow corresponds to a 768 CSS-pixel layout.
 await page.setViewportSize({width:768,height:512});await page.goto('/');await ready(page);await noOverflow(page);
 await page.getByRole('button',{name:'Open navigation menu'}).press('Enter');await expect(page.locator('#mobile-menu')).toBeVisible();await page.keyboard.press('Escape');await expect(page.locator('#mobile-menu')).not.toBeVisible();await expect(page.getByRole('button',{name:'Open navigation menu'})).toBeFocused();
 await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});const violations=await page.evaluate(async()=> (await axe.run(document.querySelector('.home-page'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations);expect(violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}))).toEqual([]);
});

test('homepage search navigates the document and other storefront headers stay unchanged',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await page.goto('/');await ready(page);
 await expect(page.locator('.global-search')).not.toHaveAttribute('hx-get');await expect(page.locator('.homepage-delivery')).toHaveAttribute('href','/shipping');await expect(page.locator('.homepage-offers')).toHaveAttribute('href','/deals');
 await page.locator('#global-search').fill('ceramic');await page.locator('.global-search button[type=submit]').click();await expect(page).toHaveURL(/\/search\?q=ceramic/);await expect(page.locator('body')).not.toHaveClass(/homepage-reference/);await expect(page.locator('.homepage-utility')).toHaveCount(0);await expect(page.locator('.global-search')).toHaveAttribute('hx-get','/search');
 await page.goBack();await expect(page.locator('body')).toHaveClass(/homepage-reference/);
 await page.goto('/shipping');await expect(page.locator('.homepage-utility')).toHaveCount(0);await expect(page.locator('link[href*="homepage-reference"]')).toHaveCount(0);await expect(page.locator('.trust-strip')).toHaveCount(1);
});

test('authenticated header retains real cart, wishlist and account actions',async({page})=>{
 const response=await page.request.post('/api/v1/auth/login',{data:{email:'demo.customer08@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}});expect(response.ok()).toBe(true);
 await page.emulateMedia({reducedMotion:'reduce'});
 for(const width of [320,390,768,1024,1536]){
  await page.setViewportSize({width,height:1024});await page.goto('/');await ready(page);await noOverflow(page);
  await expect(page.getByRole('link',{name:'Sign in',exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'Open profile menu'}).press('Enter');await expect(page.locator('#profile-dropdown')).toBeVisible();await page.keyboard.press('Escape');await expect(page.getByRole('button',{name:'Open profile menu'})).toBeFocused();
  await expect(page.locator('#cart-count')).toHaveAttribute('aria-label',/^Cart, \d+ items$/);
  if(width>800)await expect(page.locator('#wishlist-count')).toBeVisible();
 }
});
