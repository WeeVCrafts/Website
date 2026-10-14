// Read-only checks and intercepted failures against dedicated UI QA only.
const {chromium, request} = require('@playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const target = 'http://127.0.0.1:18080';
const {output} = require('./ui-ux-qa-paths.cjs');
async function main() {
  await fs.mkdir(output,{recursive:true});
  const states = {};
  for (const [role, account] of Object.entries({customer:'customer01',seller:'owner01',admin:'marketplace01',super:'super01'})) {
    const api = await request.newContext({baseURL:target});
    assert((await api.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}})).ok());
    states[role] = await api.storageState(); await api.dispose();
  }
  const browser = await chromium.launch(); const checks = [];
  async function check(name, work) {
    try {const evidence=await work();checks.push({name,pass:true,evidence});console.log('PASS '+name);}
    catch(error) {checks.push({name,pass:false,error:error.message});console.log('FAIL '+name+' '+error.message);}
  }
  const context = await browser.newContext({baseURL:target,storageState:states.customer,viewport:{width:390,height:844}});
  const page = await context.newPage(); page.setDefaultTimeout(12000);
  const overflow = () => page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2);
  try {
    await check('search no matches, error, retained query, keyboard selection', async()=>{
      await page.goto('/products'); const input=page.locator('#global-search'); await input.focus();
      await input.fill('zzzz-ui-no-piece');await page.locator('.search-suggestion-empty').waitFor();
      assert.equal(await input.inputValue(),'zzzz-ui-no-piece');
      await page.route('**/ui/search-suggest?**',route=>route.fulfill({status:503,body:'QA unavailable'}));
      await input.fill('tote'); await page.locator('[data-search-feedback]').waitFor();
      await page.waitForFunction(()=>document.querySelector('[data-search-feedback]').textContent.includes('could not be loaded'));
      assert.equal(await input.inputValue(),'tote'); await page.unroute('**/ui/search-suggest?**');
      await input.fill('pottery');await page.locator('#search-suggestions [role=option]').first().waitFor();
      await input.press('ArrowDown');assert(await input.getAttribute('aria-activedescendant'));
      await input.press('Escape');assert.equal(await input.getAttribute('aria-expanded'),'false');
    });
    await check('text-spacing override, enlarged text, landscape and 400% reflow equivalent',async()=>{
      const samples=[];
      for(const route of ['/products','/products/blue-tote','/cart','/checkout','/account/profile']) {
        await page.setViewportSize({width:640,height:500});await page.goto(route);
        await page.addStyleTag({content:'*:not(.material-symbols-outlined){line-height:1.5!important;letter-spacing:.12em!important;word-spacing:.16em!important}p{margin-bottom:2em!important}'});
        assert.equal(await overflow(),false,route+' text spacing / 640px reflow');
        await page.setViewportSize({width:844,height:390}); assert.equal(await overflow(),false,route+' landscape');
        await page.setViewportSize({width:1440,height:900});
        await page.evaluate(()=>{
          const entries=Array.from(document.querySelectorAll('body *')).map(element=>[element,getComputedStyle(element).fontSize]);
          for(const [element,size] of entries)if(!element.classList.contains('material-symbols-outlined'))element.style.fontSize=(parseFloat(size)*2)+'px';
        });
        assert.equal(await overflow(),false,route+' text-only enlargement');samples.push(route);
      }
      return {samples,method:'CSS text-only 200% and 640px reflow equivalent to 400% of a 2560px desktop; not physical browser zoom certification'};
    });
    await check('short viewport: filter internal scrolling, reachable field and focus',async()=>{
      await page.setViewportSize({width:390,height:360});await page.goto('/products');await page.locator('[data-filter-open]').click();
      assert(await page.locator('[data-filter-panel]').evaluate(element=>element.scrollHeight>element.clientHeight));
      await page.keyboard.press('Escape');assert(await page.locator('[data-filter-open]').evaluate(element=>element===document.activeElement));
      await page.goto('/checkout');await page.keyboard.press('Tab');await page.locator('[name="postal_code"]').focus();
      await page.waitForFunction(()=>document.querySelector('[name="postal_code"]').getBoundingClientRect().bottom<=innerHeight);
      return '390x360 browser viewport; real on-screen keyboard remains a device check';
    });
    await check('forced colors and reduced motion retain active controls',async()=>{
      await page.setViewportSize({width:390,height:844});await page.emulateMedia({forcedColors:'active',reducedMotion:'reduce'});await page.goto('/');
      await page.locator('[data-hero-next]').press('Enter');assert.equal(await page.locator('[data-hero-count]').textContent(),'2 / 3');
      assert.equal(await page.locator('[data-hero-next]').evaluate(element=>getComputedStyle(element).transitionDuration),'0s');
      await page.goto('/products');await page.locator('[data-filter-open]').press('Enter');await page.keyboard.press('Escape');
      await page.emulateMedia({forcedColors:'none',reducedMotion:'no-preference'});
    });
    await check('no-JS first story, native filtering and direct links',async()=>{
      const plain=await browser.newContext({baseURL:target,storageState:states.customer,javaScriptEnabled:false,viewport:{width:390,height:844}});
      try {const view=await plain.newPage();await view.goto('/');assert(await view.locator('[data-hero-slide]').first().isVisible());
        await view.goto('/products');assert(await view.locator('[data-filter-panel]').isVisible());
        await view.locator('input[name="category"][value="arts"]').check();await view.getByRole('button',{name:'Apply filters',exact:true}).click();
        await view.waitForURL(/category=arts/);assert.equal(await view.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2),false);
      }finally {await plain.close();}
    });
    await check('multi-media gallery: button, keyboard, swipe alternative and deferred video',async()=>{
      // Browser-only presentation fixture. No fake media is saved to a product.
      const mediaContext=await browser.newContext({baseURL:target,storageState:states.customer,viewport:{width:390,height:844}});
      try {const gallery=await mediaContext.newPage();let videoRequests=0,videoUnavailable=true;
        await gallery.route('**/__ui-qa/video.mp4',route=>{videoRequests++;return route.fulfill(videoUnavailable?{status:503,body:'QA media unavailable'}:{path:'tests/e2e/fixtures/flower.mp4',contentType:'video/mp4'});});
        await gallery.addInitScript(()=>document.addEventListener('DOMContentLoaded',()=>{
          const root=document.querySelector('[data-ui-gallery]');if(!root)return;
          for(const type of ['img','video']){
            const button=document.createElement('button');button.type='button';button.dataset.gallerySelect='';button.textContent='QA '+type;root.querySelector('.thumbs').append(button);
            const media=document.createElement(type);media.dataset.galleryMedia='';media.hidden=true;
            if(type==='img'){media.dataset.src='/assets/images/customer/blue-tote-480.webp';media.alt='QA alternate image';media.width=480;media.height=480;}
            else{media.controls=true;media.preload='none';media.setAttribute('aria-label','QA product video');const source=document.createElement('source');source.dataset.src='/__ui-qa/video.mp4';source.type='video/mp4';media.append(source);}
            root.querySelector('.gallery-main').append(media);
          }
        },{once:true}));
        await gallery.goto('/products/blue-tote');assert.equal(videoRequests,0);
        await gallery.locator('[data-gallery-next]').click();assert.equal(await gallery.locator('[data-gallery-count]').textContent(),'2 / 3');
        await gallery.locator('[data-gallery-select]').nth(1).press('End');await gallery.locator('video').waitFor({state:'visible'});
        await gallery.getByRole('button',{name:'Retry video',exact:true}).waitFor();assert(videoRequests>0);
        assert(await gallery.locator('[data-gallery-zoom]').isHidden());assert(await gallery.locator('video').evaluate(video=>video.paused&&video.controls&&!video.autoplay));
        assert.match(await gallery.locator('.gallery-video-feedback').textContent(),/Video could not load/);
        videoUnavailable=false;
        await gallery.getByRole('button',{name:'Retry video',exact:true}).click();
        await gallery.waitForFunction(()=>document.querySelector('video').readyState>=1&&!document.querySelector('video').dataset.failed);
        await gallery.locator('[data-gallery-prev]').click();assert(await gallery.locator('video').evaluate(video=>video.hidden&&video.paused));
        await gallery.locator('.gallery-main').evaluate(frame=>{frame.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,pointerType:'touch',isPrimary:true,pointerId:9,clientX:220,clientY:100}));frame.dispatchEvent(new PointerEvent('pointerup',{bubbles:true,pointerType:'touch',isPrimary:true,pointerId:9,clientX:300,clientY:102}));});
        assert.equal(await gallery.locator('[data-gallery-count]').textContent(),'1 / 3');
        return {videoRequests,fixture:'browser-only media plus maintained local video test file'};
      }finally {await mediaContext.close();}
    });
    await check('seller file preview, invalid type, failure retains retry selection',async()=>{
      const seller=await browser.newContext({baseURL:target,storageState:states.seller,viewport:{width:390,height:844}});
      try {const editor=await seller.newPage();await editor.goto('/seller-admin/products');
        const edit=editor.locator('a[href*="/seller-admin/products/"][href$="/edit"]').first();await edit.click();
        const file=editor.locator('[data-media-file]');await file.setInputFiles({name:'invalid.txt',mimeType:'text/plain',buffer:Buffer.from('QA')});
        await editor.locator('[data-media-upload]').click();assert.match(await editor.locator('[data-media-status]').textContent(),/JPEG/);
        await file.setInputFiles('web/assets/images/customer/blue-tote.webp');
        assert.match(await editor.locator('[data-media-selection] figcaption').textContent(),/blue-tote.webp.*MiB/);
        let posts=0;await editor.route('**/api/v1/seller/products/media-url',route=>{posts++;return route.fulfill({status:503,contentType:'application/json',body:'{"message":"QA unavailable"}'});});
        await editor.locator('[data-media-upload]').click();await editor.waitForFunction(()=>document.querySelector('[data-media-status]').textContent.includes('could not be prepared'));
        assert.equal(posts,1);assert(await editor.locator('[data-media-upload]').isEnabled());assert(await file.isEnabled());
        assert.equal(await file.evaluate(input=>input.files[0].name),'blue-tote.webp');
        return 'No real storage upload or product mutation; prepare failure intercepted';
      }finally {await seller.close();}
    });
    await check('current role fields, permission summary and audit table keep native semantics',async()=>{
      const admin=await browser.newContext({baseURL:target,storageState:states.super,viewport:{width:390,height:844}});
      try {const roles=await admin.newPage();await roles.goto('/super-admin/users');
        const values=await roles.locator('.role-assignment-form select[name="role"]').first().locator('option').evaluateAll(options=>options.map(option=>option.value));
        assert.deepEqual(values,['customer','seller','admin','super_admin']);
        await roles.goto('/super-admin/roles');await roles.getByText('Role permissions',{exact:true}).click();
        const matrix=roles.locator('.ui-table-region[aria-label="Role and permission matrix"]');
        assert.equal(await matrix.locator('th[scope="col"]').count(),7);assert.equal(await matrix.locator('th[scope="row"]').count(),4);
        assert.equal(await matrix.locator('td:not([data-label])').count(),0);
        await roles.goto('/super-admin/security');const table=roles.locator('.ui-table-region[aria-label="Audit queue"]');
        assert(await table.locator('th[scope="col"]').count()>0);
        assert.equal(await table.locator('td:not([data-label])').count(),0);
        await roles.screenshot({path:output+'/role-permissions-390.png'});
      }finally {await admin.close();}
    });
    await check('owned seller order detail, print actions, reflow and full font fallback',async()=>{
      const seller=await browser.newContext({baseURL:target,storageState:states.seller});
      try {const detail=await seller.newPage();assert.equal((await detail.goto('/seller-admin/orders/WCDEMO1001')).status(),200);
        assert.equal(await detail.locator('[data-print-kind]').count(),3);
        assert(await detail.locator('.ui-table-region[aria-label="Seller order items"] th[scope="col"]').count()>0);
        for(const width of [320,360,390,430,768,1024,1440]) {await detail.setViewportSize({width,height:844});assert.equal(await detail.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2),false);}
        await detail.setViewportSize({width:390,height:844});await detail.addScriptTag({path:require.resolve('axe-core/axe.min.js')});
        const violations=await detail.evaluate(async()=> (await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations.map(item=>item.id));assert.deepEqual(violations,[]);
        await detail.evaluate(()=>document.fonts.load('400 16px "Inter"','ΩЖ'));
        const resources=await detail.evaluate(()=>performance.getEntriesByType('resource').filter(item=>item.name.includes('/fonts/')).map(item=>item.name));
        assert(resources.some(url=>url.includes('inter-latin.woff2')));assert(resources.some(url=>url.includes('inter-variable.woff2')));
        assert.equal(new Set(resources).size,resources.length,'Font preload/CSS URLs must not duplicate');
        await detail.screenshot({path:output+'/seller-order-detail-390.png'});
        return {widths:7,print:'controls present; native print dialog not driven',font:'Latin fast path plus full supported-character fallback'};
      }finally {await seller.close();}
    });
    await check('notification read/unread text, scoped action and foreign-owner rejection',async()=>{
      const inbox=await browser.newContext({baseURL:target,viewport:{width:390,height:844}});
      const other=await browser.newContext({baseURL:target});
      try {
        assert((await inbox.request.post('/api/v1/auth/login',{data:{email:'demo.customer05@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}})).ok());
        const view=await inbox.newPage();await view.goto('/notifications');
        const item=view.locator('.notification-card').filter({has:view.getByRole('heading',{name:'UI QA unread state',exact:true})});
        assert.equal(await item.locator('.notification-unread').textContent(),'Unread');
        assert(await view.locator('.notification-read').count()>0);
        const id=await item.locator('[name="notification_id"]').inputValue();
        const login=await other.request.post('/api/v1/auth/login',{data:{email:'demo.customer08@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}});assert(login.ok());
        const foreign=await other.newPage();await foreign.goto('/notifications');
        const csrf=await foreign.locator('form[action="/logout"] [name="csrf_token"]').first().inputValue();
        // The current server uses 404 to avoid disclosing another owner's row.
        assert.equal((await other.request.post('/notifications/read',{form:{notification_id:id,csrf_token:csrf},maxRedirects:0})).status(),404);
        await item.getByRole('button',{name:'Mark as read',exact:true}).click();await view.waitForURL('**/notifications');
        await view.locator('.notification-card').filter({has:view.getByRole('heading',{name:'UI QA unread state',exact:true})}).locator('.notification-read').waitFor();
      }finally {await inbox.close();await other.close();}
    });
    await check('failed purchase retains option/quantity and clears local busy state',async()=>{
      await page.setViewportSize({width:390,height:844});await page.goto('/products/blue-tote');
      const select=page.locator('#detail-variant');const id=await select.locator('option').filter({hasText:'Large · natural cotton'}).getAttribute('value');
      await select.selectOption(id);await page.locator('.quantity-row .quantity-control button').last().click();
      let requests=0;await page.route('**/api/v1/cart/add',async route=>{requests++;await new Promise(resolve=>setTimeout(resolve,250));await route.fulfill({status:409,contentType:'application/json',body:'{"error":{"code":"INSUFFICIENT_STOCK","message":"UI QA stock changed. Review this item."}}'});});
      const add=page.locator('.detail-actions').getByRole('button',{name:'Add to cart',exact:true});await add.click();
      await page.waitForFunction(()=>Array.from(document.querySelectorAll('.is-error')).some(element=>element.textContent.includes('UI QA stock changed')));
      assert.equal(await select.inputValue(),id);assert.equal(await page.locator('#detail-quantity').inputValue(),'2');assert(await add.isEnabled());assert.equal(requests,1);
      await page.unroute('**/api/v1/cart/add');
    });
    await fs.writeFile(output+'/recovery-verification.json',JSON.stringify({target,checks,limitations:['No physical phones, browser zoom or screen reader','Configured QA storage unavailable; recovery tested with intercepted responses']},null,2));
  }finally {await context.close();await browser.close();}
  process.exitCode=checks.some(check=>!check.pass)?1:0;
}
main().catch(error=>{console.error(error);process.exitCode=1;});
