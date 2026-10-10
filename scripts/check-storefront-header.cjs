const { chromium, firefox, webkit, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const baseURL = (process.env.WEEVCRAFTS_BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
const screenshots = process.env.WEEVCRAFTS_HEADER_SCREENSHOTS;
if (screenshots) fs.mkdirSync(screenshots, {recursive:true});

(async () => {
 for (const [name, type] of Object.entries({chromium, firefox, webkit})) {
  const browser = await type.launch();
  for (const role of ['guest', 'customer']) {
   const page = await browser.newPage({viewport:{width:1440,height:900}});
   const errors = [];
   page.on('pageerror', err => errors.push(err.message));
   if (role === 'customer') {
    const response = await page.request.post(baseURL + '/api/v1/auth/login', {data:{email:process.env.WEEVCRAFTS_CUSTOMER_EMAIL || 'demo.customer01@weevcrafts.local',password:process.env.WEEVCRAFTS_DEMO_PASSWORD || 'WeeVCrafts-Demo-2026!'}});
    expect(response.ok()).toBeTruthy();
   }
   await page.goto(baseURL + '/products');
   await page.waitForFunction(()=>!!document.querySelector('.site-header')._x_dataStack);
   await page.evaluate(()=>document.fonts.ready);
   for (const width of [320,390,560,768,820,960,1024,1280,1440]) {
    await page.setViewportSize({width,height:900});
    for (const value of ['0','12','123','—']) {
     await page.locator('#wishlist-count em, #cart-count em').evaluateAll((els,value) => els.forEach(el=>el.textContent=value),value);
     const defects = await page.evaluate(() => {
      const issues=[];
      if(document.documentElement.scrollWidth>innerWidth+1) issues.push('page overflows');
      const parts=[...document.querySelectorAll('.header-main > .wordmark,.header-main .search-shell,.header-main .header-actions,.menu-toggle,.header-main .location')];
      for(let i=0;i<parts.length;i++)for(let j=i+1;j<parts.length;j++){
       const a=parts[i].getBoundingClientRect(),b=parts[j].getBoundingClientRect();
       if(Math.min(a.right,b.right)-Math.max(a.left,b.left)>1&&Math.min(a.bottom,b.bottom)-Math.max(a.top,b.top)>1)issues.push(parts[i].className+' overlaps '+parts[j].className);
      }
      document.querySelectorAll('.header-actions em').forEach(el=>{
       if(!el.getClientRects().length)return;
       const s=getComputedStyle(el),r=el.getBoundingClientRect();
       if(parseFloat(s.lineHeight)>r.height-4)issues.push('counter text is clipped');
       const range=document.createRange();range.selectNodeContents(el);const t=range.getBoundingClientRect();
       if(t.left<r.left+1||t.right>r.right-1||t.top<r.top+1||t.bottom>r.bottom-1)issues.push('counter glyphs overflow');
      });
      document.querySelectorAll('.header-actions > a,.header-logout button,.menu-toggle,.header-main .location').forEach(el=>{
       const r=el.getBoundingClientRect();if(!el.getClientRects().length)return;if(r.width<44||r.height<44)issues.push('touch target too small');
      });
      document.querySelectorAll('.desktop-navigation a').forEach(el=>{
       if(parseFloat(getComputedStyle(el).fontSize)<15)issues.push('desktop navigation text too small');
      });
      document.querySelectorAll('.menu-toggle,.header-main .location small').forEach(el=>{
       if(parseFloat(getComputedStyle(el).fontSize)<14)issues.push('header text too small');
      });
      return issues;
     });
     expect(defects,`${name} ${role} ${width}px count=${value}`).toEqual([]);
    }
   }
   const menu = page.getByRole('dialog',{name:'Explore the store'});
   const toggle=page.locator('.menu-toggle');
   for (const width of [390,1440]) {
    await page.setViewportSize({width,height:900});
    await toggle.click();
    await expect(menu).toBeVisible();
    expect(await menu.locator('a').evaluateAll(els=>els.every(el=>parseFloat(getComputedStyle(el).fontSize)>=16))).toBeTruthy();
    await expect(toggle).toHaveAttribute('aria-expanded','true');
    await expect(menu.getByRole('button',{name:'Close navigation',exact:true})).toBeFocused();
    await page.keyboard.press('Shift+Tab');
    expect(await page.evaluate(()=>!!document.activeElement.closest('#mobile-menu'))).toBeTruthy();
    await page.keyboard.press('Tab');
    await expect(menu.getByRole('button',{name:'Close navigation',exact:true})).toBeFocused();
    expect(await page.evaluate(()=>getComputedStyle(document.documentElement).overflow)).toBe('hidden');
    expect(await menu.evaluate(el=>el.scrollTop)).toBe(0);
    if (screenshots && name==='chromium' && role==='customer') await page.screenshot({path:path.join(screenshots,`header-menu-${width}.png`),animations:'disabled'});
    await page.keyboard.press('Escape');
    await expect(menu).not.toBeVisible();
    await expect(toggle).toBeFocused();
    await toggle.click();
    await page.mouse.click(await page.evaluate(()=>document.documentElement.clientWidth-5),300);
    await expect(menu).not.toBeVisible();
    await expect(toggle).toBeFocused();
   }
   await page.emulateMedia({reducedMotion:'reduce'});
   await toggle.click();
   expect(await menu.evaluate(el=>getComputedStyle(el).animationName)).toBe('none');
   await menu.getByRole('button',{name:'Close navigation',exact:true}).click();
   await expect(toggle).toBeFocused();
   await toggle.click();
   await menu.getByRole('link',{name:'Browse categories',exact:true}).click();
   await expect(page).toHaveURL(baseURL+'/categories');
   await expect(page.locator('#mobile-menu')).not.toBeVisible();
   await page.goto(baseURL+'/products');
   if(screenshots&&name==='chromium'&&role==='customer')for(const width of [390,1440]){
    await page.setViewportSize({width,height:900});
    await page.locator('#wishlist-count em').evaluate(el=>el.textContent='12');
    await page.locator('#cart-count em').evaluate(el=>el.textContent='3');
    await page.locator('.site-header').screenshot({path:path.join(screenshots,`header-updated-${width}.png`)});
   }
   expect(errors,`${name} ${role} page errors`).toEqual([]);
   console.log(`${name} ${role}: 9 widths, 0/12/123/unavailable counts, modal keyboard/backdrop/navigation/reduced motion passed`);
   await page.close();
  }
  await browser.close();
 }
})().catch(err=>{console.error(err);process.exit(1)});
