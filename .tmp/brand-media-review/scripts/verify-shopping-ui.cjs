const { chromium, firefox, webkit }=require('@playwright/test');
const assert=require('node:assert/strict');const fs=require('node:fs/promises');
const target='http://127.0.0.1:18080';
const {output}=require('./ui-ux-qa-paths.cjs');
async function main(){
 await fs.mkdir(output,{recursive:true});
 const results=[];
 for(const [engine,type]of Object.entries({chromium,firefox,webkit}).filter(([name])=>(process.env.WEEVCRAFTS_UI_ENGINES||'chromium,firefox,webkit').split(',').includes(name))){
  const browser=await type.launch();const context=await browser.newContext({baseURL:target,viewport:{width:390,height:844}});
  const page=await context.newPage();page.setDefaultTimeout(15000);
  const started=Date.now();
  try{
   assert((await context.request.post('/api/v1/auth/login',{data:{email:'demo.customer08@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}})).ok());
   await page.goto('/products/blue-tote');await page.waitForFunction(()=>window.weevcraftsBackend);
   const select=page.locator('#detail-variant');const variant=await select.locator('option').filter({hasText:'Large · natural cotton'}).getAttribute('value');
   // Cleanup only this script's QA option through the existing cart operation.
   await page.evaluate(id=>window.weevcraftsBackend.removeCart('blue-tote',id),variant);
   await select.selectOption(variant);
   let additions=0;
   await page.route('**/api/v1/cart/add',async route=>{additions++;await new Promise(resolve=>setTimeout(resolve,250));await route.continue();});
   await page.locator('.mobile-buy-bar button').first().click();
   await page.locator('#cart-drawer[open]').waitFor();assert.equal(additions,1);
   assert.match(await page.locator('[data-cart-drawer-items]').textContent(),/Large · natural cotton/);
   await page.keyboard.press('Escape');assert.equal(await page.locator('#cart-drawer').getAttribute('open'),null);
   await page.goto('/cart');await page.waitForFunction(()=>window.weevcraftsBackend);
   const row=()=>page.locator('#cart-layout .cart-line').filter({hasText:'Large · natural cotton'});
   assert.equal((await row().locator('output').textContent()).trim(),'1');
   let changes=0;
   await page.route('**/ui/cart/blue-tote',async route=>{if(route.request().method()==='POST'){changes++;await new Promise(resolve=>setTimeout(resolve,250));}await route.continue();});
   await row().locator('.quantity-control').evaluate(control=>{control.querySelectorAll('button')[1].click();control.querySelectorAll('button')[0].click();});
   await page.waitForFunction(()=>Array.from(document.querySelectorAll('#cart-layout .cart-line')).find(row=>row.textContent.includes('Large · natural cotton'))?.querySelector('output').textContent.trim()==='2');
   assert.equal(changes,1);assert(await row().evaluate(element=>element.contains(document.activeElement)));
   const response=await context.request.get('/cart');const serverTotal=(await response.text()).match(/Total amount <strong>([^<]+)<\/strong>/)[1];
   assert.equal(await page.locator('.cart-summary h3 strong').textContent(),serverTotal);
   await row().getByRole('button',{name:'Remove',exact:true}).click();await row().waitFor({state:'detached'});
   assert(await page.evaluate(()=>!!document.activeElement.closest('.cart-items')));
   await page.goto('/products/blue-tote');await page.waitForFunction(()=>window.weevcraftsBackend);
   const heart=page.locator('.gallery-heart');const initial=await heart.getAttribute('aria-pressed');
   await heart.click();await page.waitForFunction(previous=>document.querySelector('.gallery-heart').getAttribute('aria-pressed')!==previous,initial);
   const after=await heart.getAttribute('aria-pressed');
   if(after==='false'){await heart.click();await page.waitForFunction(()=>document.querySelector('.gallery-heart').getAttribute('aria-pressed')==='true');}
   await page.goto('/wishlist');assert(await page.getByRole('link',{name:'Indigo Everyday Tote',exact:true}).isVisible());
   await page.getByRole('link',{name:'Indigo Everyday Tote',exact:true}).click();await page.waitForURL(/\/products\/blue-tote$/);
   if(initial==='false'){await heart.click();await page.waitForFunction(()=>document.querySelector('.gallery-heart').getAttribute('aria-pressed')==='false');}
   await page.goto('/products?category=textiles&sort=price-asc');const catalogue=page.url();
   await page.getByRole('link',{name:'Indigo Everyday Tote',exact:true}).click();await page.waitForURL(/\/products\/blue-tote$/);await page.goBack();
   assert.equal(page.url(),catalogue);assert.equal(await page.locator('#listing-sort').inputValue(),'price-asc');
   await page.goto('/products');await page.locator('[data-filter-open]').click();
   const home=page.locator('[data-filter-panel] input[name="category"][value="home"]');await home.check();
   await page.waitForFunction(()=>document.querySelector('[data-filter-panel]')?.classList.contains('is-open')&&location.search.includes('category='));
   await page.locator('[data-filter-panel] input[name="price"][value="1000-2500"]').check();await page.waitForURL(/price=1000-2500/);await page.keyboard.press('Escape');
   await page.locator('.product-card').first().waitFor();
   const prices=await page.locator('.product-card .price strong').allTextContents();assert(prices.length>0);assert(prices.every(price=>{const amount=Number(price.replace(/[^0-9.]/g,''));return amount>=1000&&amount<=2500;}));
   results.push({engine,pass:true,durationSeconds:(Date.now()-started)/1000,timing:'automated task run, not a participant-study time',checks:'chosen option, cart drawer/Escape, serialized row changes, authoritative total, removal focus, confirmed Wishlist retrieval, native Back/query, home piece within budget'});
   console.log(engine+' shopping interactions PASS');
  }catch(error){
   const state=await page.evaluate(()=>({url:location.pathname,dialogs:Array.from(document.querySelectorAll('dialog[open]')).map(element=>element.id),heartDisabled:document.querySelector('.gallery-heart')?.disabled})).catch(()=>null);
   await page.screenshot({path:output+'/shopping-failed-'+engine+'.png'}).catch(()=>{});
   results.push({engine,pass:false,error:error.message,state});console.log(engine+' FAIL '+error.message);
  }
  finally{await context.close();await browser.close();}
 }
 await fs.writeFile(output+'/shopping-verification.json',JSON.stringify(results,null,2));process.exitCode=results.some(result=>!result.pass)?1:0;
}
main().catch(error=>{console.error(error);process.exitCode=1;});
