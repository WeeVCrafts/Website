const { chromium, firefox, webkit, request } = require('@playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');

const target = process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18080';
if (target !== 'http://127.0.0.1:18080') throw new Error('UI verification is restricted to the dedicated QA preview.');
const {output} = require('./ui-ux-qa-paths.cjs');
const routes = {
  customer: ['/', '/products', '/products/blue-tote', '/categories', '/categories/arts', '/makers', '/brands', '/deals', '/search?q=tote', '/cart', '/checkout', '/wishlist', '/account', '/account/profile', '/account/addresses', '/account/notifications', '/account/security', '/account/sessions', '/account/privacy', '/account/reviews', '/account/support', '/orders', '/tracking', '/returns', '/payments', '/payments/success', '/payments/pending', '/payments/failed', '/contact', '/faq', '/shipping', '/size-guide', '/terms', '/privacy', '/about', '/our-story', '/sustainability', '/press', '/careers'],
  seller: ['/seller-admin', '/seller-admin/products', '/seller-admin/products/new', '/seller-admin/inventory', '/seller-admin/orders', '/seller-admin/returns', '/seller-admin/earnings', '/seller-admin/analytics', '/seller-admin/settings', '/seller-admin/onboarding', '/seller-admin/support', '/seller-admin/terms'],
  admin: ['','dashboard','products','products/new','orders','inventory','support','sellers','seller-approvals','seller-suspensions','returns','finance','brands','customers','category-management','commission-rules','reviews','marketing','analytics','payments','failed-payments','refunds','settlements','finance-reconciliation'].map(path=>'/admin'+(path?'/'+path:'')),
  super: ['','dashboard','products','products/new','orders','inventory','support','sellers','seller-approvals','seller-suspensions','returns','finance','brands','customers','category-management','commission-rules','reviews','marketing','analytics','payments','failed-payments','refunds','settlements','finance-reconciliation','users','roles','security','settings','audit-logs','security-events','system-health','search-indexing-health','worker-queue-health'].map(path=>'/super-admin'+(path?'/'+path:''))
};
const accounts = {customer:'customer01',seller:'owner01',admin:'marketplace01',super:'super01'};
routes.guest=['/login','/register','/forgot-password','/reset-password','/verify-email'];
for (const roleRoutes of Object.values(routes)) roleRoutes.push('/notifications');
routes.customer.push('/privacy/center');
const report = {target, browsers:{}, coverage:'Dedicated demo fixtures only; no physical-phone or screen-reader certification.'};
async function main() {
  await fs.mkdir(output,{recursive:true});
  const states={};
  states.guest={cookies:[],origins:[]};
  for (const [role,account] of Object.entries(accounts)) {
    const api=await request.newContext({baseURL:target});
    const response=await api.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}});
    assert(response.ok(),role+' login '+response.status()); states[role]=await api.storageState(); await api.dispose();
  }
  for (const [engine,type] of Object.entries({chromium,firefox,webkit}).filter(([name])=>(process.env.WEEVCRAFTS_UI_ENGINES||'chromium,firefox,webkit').split(',').includes(name))) {
    const browser=await type.launch(); const results=[];report.browsers[engine]=results;
    async function check(name,work) {
      try {await work();results.push({name,pass:true});console.log(engine+' PASS '+name);}
      catch(error){results.push({name,pass:false,error:error.message});console.log(engine+' FAIL '+name+' '+error.message.slice(0,250));}
    }
    const context=await browser.newContext({baseURL:target,storageState:states.customer,viewport:{width:390,height:844},reducedMotion:'reduce'});
    const page=await context.newPage();page.setDefaultTimeout(8000);
    const errors=[];page.on('pageerror',error=>errors.push(error.message));
    await check('manual hero: controls, inert slides, no autoplay',async()=>{
      await page.goto('/');await page.locator('[data-hero-next]').click();
      assert.equal(await page.locator('[data-hero-count]').textContent(),'2 / 3');
      assert.equal(await page.locator('[data-hero-slide]:not([hidden])').count(),1);
      assert.equal(await page.locator('[data-hero-slide][inert]').count(),2);
      assert.equal(await page.locator('[data-hero-slide] img').evaluateAll(images=>images.filter(image=>image.hasAttribute('data-src')).length),1);
      await page.locator('[data-hero-next]').focus();await page.keyboard.press('Enter');
      assert.equal(await page.locator('[data-hero-count]').textContent(),'3 / 3');
    });
    await check('mobile filter: closed inert, continuous selection, Escape focus',async()=>{
      await page.goto('/products');assert(await page.locator('[data-filter-panel]').evaluate(element=>element.inert));
      const filterBox=await page.locator('[data-filter-open]').boundingBox();
      const sortBox=await page.getByRole('combobox',{name:'Sort by',exact:true}).boundingBox();
      assert(Math.abs(filterBox.y+filterBox.height/2-sortBox.y-sortBox.height/2)<2,'390px toolbar controls must share one row');
      assert.equal(await page.locator('.product-card img').first().getAttribute('loading'),'eager');
      assert.equal(await page.locator('.product-card img').first().getAttribute('fetchpriority'),'high');
      await page.locator('[data-filter-open]').click();
      await page.locator('[data-filter-panel] input[name="category"][value="crafts"]').check();
      await page.waitForURL(/category=crafts/);
      await page.waitForFunction(()=>document.querySelector('[data-filter-panel]')?.classList.contains('is-open'));
      assert(await page.locator('[data-filter-panel]').evaluate(element=>element.contains(document.activeElement)));
      await page.locator('[data-filter-panel] input[name="availability"]').check();
      await page.waitForURL(/availability=in-stock/);
      await page.waitForFunction(()=>document.querySelector('[data-filter-panel]')?.classList.contains('is-open'));
      assert(await page.locator('[data-filter-panel] input[name="category"][value="crafts"]').isChecked());
      await page.keyboard.press('Escape');
      assert(await page.locator('[data-filter-open]').evaluate(element=>element===document.activeElement));
      assert(await page.locator('[data-filter-panel]').evaluate(element=>element.inert));
    });
    await check('failed filter preserves query and offers in-sheet retry',async()=>{
      await page.goto('/products');await page.locator('[data-filter-open]').click();
      await page.route('**/products?**',route=>route.fulfill({status:503,contentType:'text/plain',body:'QA temporary unavailable'}));
      await page.locator('[data-filter-panel] input[name="category"][value="arts"]').check();
      await page.locator('[data-filter-panel] [data-listing-feedback].is-error').waitFor();
      assert(await page.locator('[data-filter-panel] input[value="arts"]').isChecked());
      await page.unroute('**/products?**');await page.locator('[data-filter-panel] [data-listing-feedback] button').click();
      await page.waitForURL(/category=arts/);await page.keyboard.press('Escape');
    });
    await check('local facet search: no-match and clear',async()=>{
      await page.goto('/products');await page.locator('[data-filter-open]').click();
      let facetRequests=0;
      const countFacetRequest=request=>{if(request.headers()['hx-request']&&new URL(request.url()).pathname==='/products')facetRequests++;};
      page.on('request',countFacetRequest);
      await page.locator('#facet-category').fill('zzzz-no-category');
      await page.locator('.facet-search').first().locator('.facet-empty:not([hidden])').waitFor();
      await page.locator('[data-filter-close]').first().focus();
      await page.getByRole('button',{name:'Clear categories search'}).click();
      assert.equal(await page.locator('#facet-category').inputValue(),'');await page.keyboard.press('Escape');
      page.off('request',countFacetRequest);assert.equal(facetRequests,0,'Local facet search/clear must not submit filters');
    });
    await check('gallery lightbox and keyboard tabs',async()=>{
      await page.goto('/products/blue-tote');await page.locator('[data-gallery-zoom]').click();
      await page.locator('dialog.media-lightbox').waitFor();await page.keyboard.press('Escape');
      assert(await page.locator('[data-gallery-zoom]').evaluate(element=>element===document.activeElement));
      await page.locator('#product-tab-description').focus();await page.keyboard.press('End');
      assert.equal(await page.locator('#product-tab-reviews').getAttribute('aria-selected'),'true');
      await page.keyboard.press('Home');assert.equal(await page.locator('#product-tab-description').getAttribute('aria-selected'),'true');
      assert(await page.locator('[data-mobile-price]').count());
    });
    await check('real options: selected price/SKU, unavailable controls and missing media',async()=>{
      await page.goto('/products/blue-tote');const select=page.locator('#detail-variant');
      await select.selectOption(await select.locator('option').filter({hasText:'Extra large · unavailable'}).getAttribute('value'));
      assert(await page.locator('.mobile-buy-bar button').first().isDisabled());
      assert.equal(await page.locator('[data-mobile-price]').textContent(),await select.locator('option:checked').getAttribute('data-price'));
      await select.selectOption(await select.locator('option').filter({hasText:'Large · natural cotton'}).getAttribute('value'));assert.equal(await page.locator('.mobile-buy-bar button').first().isDisabled(),false);
      assert.equal(await page.locator('[data-selected-variant-sku]').textContent(),'UI-QA-TOTE-LARGE');
      await page.goto('/products/ui-qa-missing-media');await page.waitForFunction(()=>document.querySelector('[data-gallery-media]').src.includes('product-unavailable.svg'));
      assert(await page.locator('.mobile-buy-bar button').first().isDisabled());
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2),false);
    });
    await check('keyboard policy focus is clear of sticky bars',async()=>{
      await page.goto('/products/blue-tote');await page.keyboard.press('Tab');
      await page.locator('.product-policy-links a').last().focus();
      assert(await page.locator('.product-policy-links a').last().evaluate(element=>{
        const r=element.getBoundingClientRect(),h=document.querySelector('.site-header').getBoundingClientRect(),b=document.querySelector('.mobile-buy-bar').getBoundingClientRect();return r.top>=h.bottom&&r.bottom<=(b.height?b.top:innerHeight);
      }));
    });
    await check('search clear and IME submission guard',async()=>{
      await page.goto('/products');await page.getByRole('button',{name:'Search',exact:true}).click();await page.locator('#global-search').fill('tote');
      await page.getByRole('button',{name:'Clear product search'}).click();assert.equal(await page.locator('#global-search').inputValue(),'');
      const before=page.url();await page.locator('#global-search').evaluate(input=>{
        input.dispatchEvent(new CompositionEvent('compositionstart',{bubbles:true}));input.value='布';
        input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',isComposing:true,bubbles:true,cancelable:true}));
      });assert.equal(page.url(),before);
      await page.locator('#global-search').evaluate(input=>input.dispatchEvent(new CompositionEvent('compositionend',{bubbles:true})));
    });
    await check('checkout: inline error, retained address, step focus and Back',async()=>{
      await page.goto('/checkout');const name=page.locator('[name="recipient_name"]');
      await name.fill('');await page.getByRole('button',{name:'Continue to delivery'}).click();
      assert.equal(await name.getAttribute('aria-invalid'),'true');assert(await name.evaluate(element=>element===document.activeElement));
      await name.fill('UI QA Customer');await page.getByRole('button',{name:'Continue to delivery'}).click();
      if (!(await page.locator('.checkout-card[data-step="2"]').isVisible())) {
        await page.locator('[name="phone"]').fill('9876543210');await page.getByRole('button',{name:'Continue to delivery'}).click();
      }
      await page.locator('.checkout-card[data-step="2"]').waitFor({state:'visible'});
      assert(await page.locator('.checkout-card[data-step="2"] h2').evaluate(element=>element===document.activeElement));
      await page.getByRole('button',{name:'Back to address'}).click();assert.equal(await name.inputValue(),'UI QA Customer');
    });
    await check('no uncaught storefront errors',async()=>assert.deepEqual(errors,[]));
    await context.close();
    for (const role of ['seller','admin','super']) {
      const roleContext=await browser.newContext({baseURL:target,storageState:states[role],viewport:{width:390,height:844}});
      const rolePage=await roleContext.newPage();rolePage.setDefaultTimeout(8000);
      await check(role+' navigation inert, trap, close and desktop restoration',async()=>{
        await rolePage.goto(routes[role][0]);const kind=role==='seller'?'seller':'admin';
        const panel=rolePage.locator('#'+kind+'-sidebar');
        assert(await panel.evaluate(element=>element.inert));
        await rolePage.locator('.'+kind+'-menu-button').click();await panel.locator('[aria-label="Close '+kind+' navigation"]').waitFor();
        await rolePage.waitForFunction(id=>document.getElementById(id).getAttribute('aria-modal')==='true',kind+'-sidebar');
        await rolePage.keyboard.press('Shift+Tab');assert(await panel.evaluate(element=>element.contains(document.activeElement)));
        await rolePage.keyboard.press('Escape');await rolePage.waitForFunction(id=>document.getElementById(id).inert,kind+'-sidebar');
        await rolePage.waitForFunction(selector=>document.querySelector(selector)===document.activeElement,'.'+kind+'-menu-button');
        await rolePage.setViewportSize({width:1440,height:1000});await rolePage.waitForFunction(id=>!document.getElementById(id).inert,kind+'-sidebar');
      });await roleContext.close();
    }
    if(engine==='chromium') {
      const sellerContext=await browser.newContext({baseURL:target,storageState:states.seller,viewport:{width:390,height:844}});const editor=await sellerContext.newPage();editor.setDefaultTimeout(8000);
      await check('seller rejected save preserves fields and unsaved navigation is cancel-safe',async()=>{
        await editor.goto('/seller-admin/products/new');const form=editor.locator('[data-product-builder]');
        await form.locator('[name="name"]').fill('UI QA retained draft name');await form.locator('[name="sku"]').fill('UI-QA-FORM-NOT-SAVED');
        await form.locator('[name="category_slug"]').selectOption({index:1});await form.locator('[name="price_rupees"]').fill('100');
        await form.locator('[name="description"]').fill('Isolated UI failure test; never saved.');await form.locator('[name="initial_stock"]').fill('1');
        await editor.route('**/seller-admin/products',route=>route.request().method()==='POST'?route.fulfill({status:503,contentType:'text/html',body:'<main><p>QA connection unavailable. Try again later.</p></main>'}):route.continue());
        await form.getByRole('button',{name:'Save product draft'}).click();await form.locator('.ui-form-error').waitFor();
        assert.equal(await form.locator('[name="name"]').inputValue(),'UI QA retained draft name');
        await form.getByRole('link',{name:'Cancel',exact:true}).click();await editor.locator('dialog.ui-confirm').waitFor();
        await editor.getByRole('button',{name:'Keep editing',exact:true}).click();assert.equal(await form.locator('[name="name"]').inputValue(),'UI QA retained draft name');
      });await sellerContext.close();
      await check('current role boundaries remain unchanged',async()=>{
        const restricted=await request.newContext({baseURL:target,storageState:states.admin});
        for(const route of ['/super-admin','/admin/users','/admin/security'])assert.equal((await restricted.get(route)).status(),403,route);
        await restricted.dispose();const seller=await request.newContext({baseURL:target,storageState:states.seller});
        assert.equal((await seller.get('/admin')).status(),403);assert.equal((await seller.get('/seller-admin/team')).status(),404);await seller.dispose();
      });
    }
    if (engine==='chromium') for(const [role,roleRoutes] of Object.entries(routes)) {
      const auditContext=await browser.newContext({baseURL:target,storageState:states[role]});const audit=await auditContext.newPage();
      await check(role+' route / reflow / axe matrix',async()=>{
        const matrix=[];
        const screenRoutes=[...roleRoutes];
        for(const route of screenRoutes) {
          await audit.setViewportSize({width:390,height:844});const response=await audit.goto(route,{waitUntil:'networkidle'});
          if(response.status()!==200){matrix.push({route,status:response.status()});continue;}
          const widths=[];
          for(const width of [320,360,390,430,768,1024,1440]) {
            await audit.setViewportSize({width,height:width===1440?1000:844});
            widths.push({width,overflow:await audit.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2)});
          }
          await audit.setViewportSize({width:390,height:844});await audit.addScriptTag({path:require.resolve('axe-core/axe.min.js')});
          // Audit the detail decision after deliberate scroll; check focus
          // obstruction separately above rather than accepting covered targets.
          if(route==='/products/blue-tote') await audit.locator('.product-policy-links').scrollIntoViewIfNeeded();
          const violations=await audit.evaluate(async()=> (await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations.map(v=>({id:v.id,impact:v.impact,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));
          matrix.push({route,status:response.status(),widths,violations});
          // One owned record per detail-screen family. Do not follow mutation,
          // export/download, tracking-provider or retired-role destinations.
          const families=role==='customer'?['/brands/','/makers/','/orders/','/returns/']:role==='seller'?['/seller-admin/products/','/seller-admin/orders/']:['/'+(role==='super'?'super-admin':'admin')+'/products/','/'+(role==='super'?'super-admin':'admin')+'/support?ticket_id='];
          const links=await audit.locator('a[href]').evaluateAll(anchors=>anchors.map(anchor=>anchor.getAttribute('href')));
          for(const family of families) {
            if(screenRoutes.some(existing=>existing.startsWith(family)&&!existing.endsWith('/new')))continue;
            const next=links.find(href=>href.startsWith(family)&&!href.includes('/label')&&!href.includes('/receipt')&&!href.includes('/invoice')&&!href.includes('/cancel')&&(!family.includes('/products/')||href.endsWith('/edit')));
            if(next)screenRoutes.push(next);
          }
          console.log('matrix '+role+' '+route+' '+(violations.length?'axe='+violations.map(v=>v.id).join(','):'OK'));
          await audit.screenshot({path:path.join(output,role+'-'+route.replace(/\W+/g,'-')+'-390.png')});
        }
        report[role+'Matrix']=matrix;
        assert.equal(matrix.filter(item=>item.status!==200||item.widths?.some(w=>w.overflow)||item.violations?.length).length,0,JSON.stringify(matrix.filter(item=>item.status!==200||item.widths?.some(w=>w.overflow)||item.violations?.length).map(item=>({route:item.route,status:item.status,overflow:item.widths?.filter(w=>w.overflow),violations:item.violations?.map(v=>v.id)}))));
      });await auditContext.close();
    }
    await browser.close();
    await fs.writeFile(path.join(output,'verification.json'),JSON.stringify(report,null,2));
  }
  process.exitCode=Object.values(report.browsers).flat().some(result=>!result.pass)?1:0;
}
main().catch(async error=>{console.error(error);await fs.writeFile(path.join(output,'verification.json'),JSON.stringify(report,null,2));process.exitCode=1;});
