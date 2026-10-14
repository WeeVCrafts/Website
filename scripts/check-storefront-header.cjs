const { chromium, firefox, webkit, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const baseURL = (process.env.WEEVCRAFTS_BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
const screenshots = process.env.WEEVCRAFTS_HEADER_SCREENSHOTS;
const selectedBrowsers = (process.env.WEEVCRAFTS_HEADER_BROWSERS || 'chromium,firefox,webkit').split(',');
if (screenshots) fs.mkdirSync(screenshots, {recursive:true});

(async () => {
 for (const [name, type] of Object.entries({chromium, firefox, webkit})) {
  if (!selectedBrowsers.includes(name)) continue;
  const browser = await type.launch();
  for (const role of ['guest', 'customer']) {
   const page = await browser.newPage({viewport:{width:1440,height:900}});
   const errors = [];
   page.on('pageerror', err => errors.push(err.message));
   if (role === 'customer') {
    const response = await page.request.post(baseURL + '/api/v1/auth/login', {data:{email:process.env.WEEVCRAFTS_CUSTOMER_EMAIL || 'demo.customer01@weevcrafts.local',password:process.env.WEEVCRAFTS_DEMO_PASSWORD || 'WeeVCrafts-Demo-2026!'}});
    expect(response.ok()).toBeTruthy();
   }
   await page.goto(baseURL + '/products', {waitUntil:'domcontentloaded'});
   await page.waitForFunction(()=>!!document.querySelector('.site-header')._x_dataStack);
   await page.waitForFunction(()=>document.fonts.status === 'loaded', null, {timeout:30000});
   if (role === 'guest') {
    // Current shopping policy requires sign-in. Do not test profile/drawer
    // activation behind the mandatory account dialog as if it were interactive.
    await expect(page.locator('#guest-login-dialog')).toBeVisible();
    for (const width of [320,390,560,768,820,960,1024,1280,1440]) {
     await page.setViewportSize({width,height:900});
     expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
     await expect(page.locator('[data-login-continue]')).toBeFocused();
    }
    await page.locator('[data-login-continue]').press('Enter');
    await expect(page).toHaveURL(/\/login\?next=/);
    expect(errors).toEqual([]);
    console.log(`${name} guest: required account dialog and sign-in destination passed at 9 widths`);
    await page.close();continue;
   }
   await page.evaluate(()=>window.scrollTo(0,Math.min(500,document.body.scrollHeight)));
   const stickyState=await page.locator('.site-header').evaluate(el=>({position:getComputedStyle(el).position,top:Math.round(el.getBoundingClientRect().top)}));
   expect(stickyState.position,`${name} ${role} header position`).toBe('sticky');
   expect(stickyState.top,`${name} ${role} header top after scroll`).toBe(0);
   await page.evaluate(()=>window.scrollTo(0,0));
   for (const width of [320,390,560,768,820,960,1024,1280,1440]) {
    if (process.env.WEEVCRAFTS_HEADER_VERBOSE) console.log(`${name} ${role}: checking ${width}px`);
    await page.setViewportSize({width,height:900});
    const profileToggle = page.getByRole('button', {name:'Open profile menu',exact:true});
    await expect(page.locator('#profile-dropdown')).not.toBeVisible();
    await profileToggle.press('Enter');
    await expect(profileToggle).toHaveAttribute('aria-expanded','true');
    await expect(page.locator('#profile-dropdown')).toBeVisible();
    await expect(page.locator('#profile-dropdown a').first(), `${name} ${role} ${width}px profile focus`).toBeFocused();
    if(width>700) await expect(page.locator('.header-actions > #wishlist-count')).toBeVisible();
    else await expect(page.locator('.header-actions > #wishlist-count')).not.toBeVisible();
    await expect(page.locator('.header-actions > #cart-count')).toBeVisible();
    for (const value of ['0','12','123','—']) {
     await page.locator('#wishlist-count em, #cart-count em').evaluateAll((els,value) => els.forEach(el=>el.textContent=value),value);
     const defects = await page.evaluate(() => {
      const issues=[];
      if(document.documentElement.scrollWidth>innerWidth+1) issues.push('page overflows');
      const parts=[...document.querySelectorAll('.header-main > .wordmark,.header-main .search-shell,.profile-toggle,.menu-toggle,.header-main .location,.header-actions > a')].filter(el=>el.getClientRects().length);
      const brand=document.querySelector('.header-main > .wordmark').getBoundingClientRect();
      const header=document.querySelector('.header-main').getBoundingClientRect();
      const menu=document.querySelector('.menu-toggle').getBoundingClientRect();
      const search=document.querySelector('.search-shell').getBoundingClientRect();
      const profile=document.querySelector('.profile-toggle').getBoundingClientRect();
      const location=document.querySelector('.header-main .location').getBoundingClientRect();
      const panel=document.querySelector('.profile-panel').getBoundingClientRect();
      if(header.height>76)issues.push('header is not one compact row');
      if(search.right>profile.left)issues.push('search is not before profile');
      if(Math.abs((search.top+search.bottom)/2-(profile.top+profile.bottom)/2)>2)issues.push('search and profile are not aligned');
      if(innerWidth>1100){
       if(location.right>search.left)issues.push('delivery location is not left of search');
       if(Math.abs((location.top+location.bottom)/2-(search.top+search.bottom)/2)>2)issues.push('delivery location is not in search row');
      }
      if(panel.left<0||panel.right>innerWidth+1||panel.bottom>innerHeight+1)issues.push('profile panel overflows viewport');
      if(document.querySelector('.desktop-navigation'))issues.push('inline shop navigation remains');
      for(let i=0;i<parts.length;i++)for(let j=i+1;j<parts.length;j++){
       const a=parts[i].getBoundingClientRect(),b=parts[j].getBoundingClientRect();
       if(Math.min(a.right,b.right)-Math.max(a.left,b.left)>1&&Math.min(a.bottom,b.bottom)-Math.max(a.top,b.top)>1)issues.push(parts[i].className+' overlaps '+parts[j].className);
      }
      document.querySelectorAll('.header-actions em').forEach(el=>{
       if(!el.getClientRects().length)return;
       const s=getComputedStyle(el),r=el.getBoundingClientRect();
       if(parseFloat(s.lineHeight)>r.height)issues.push('counter text is clipped');
       const range=document.createRange();range.selectNodeContents(el);const t=range.getBoundingClientRect();
       if(t.left<r.left+1||t.right>r.right-1||t.top<r.top+1||t.bottom>r.bottom-1)issues.push('counter glyphs overflow');
      });
      document.querySelectorAll('.header-actions > a,.profile-toggle,.profile-panel a,.profile-logout button,.menu-toggle,.header-main .location').forEach(el=>{
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
    await page.keyboard.press('Escape');
    await expect(page.locator('#profile-dropdown')).not.toBeVisible();
    await expect(profileToggle).toBeFocused();
    await profileToggle.press('Enter');
    const searchInput = page.getByRole('combobox', {name:'Search products',exact:true});
    if(width<=700) await page.locator('.global-search button[type=submit]').click();
    await searchInput.focus();
    await expect(searchInput).toBeVisible();
    const searchBounds = await searchInput.boundingBox();
    await page.mouse.click(searchBounds.x + searchBounds.width / 2, searchBounds.y + searchBounds.height / 2);
    await expect(page.locator('#profile-dropdown')).not.toBeVisible();
   }
   const menu = page.getByRole('dialog',{name:'Explore the store'});
   const toggle=page.locator('.menu-toggle');
   for (const width of [390,1440]) {
    await page.setViewportSize({width,height:900});
    await toggle.press('Enter');
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
    await toggle.press('Enter');
    await page.mouse.click(await page.evaluate(()=>document.documentElement.clientWidth-5),300);
    await expect(menu).not.toBeVisible();
    await expect(toggle).toBeFocused();
   }
   await page.emulateMedia({reducedMotion:'reduce'});
   await toggle.press('Enter');
   expect(await menu.evaluate(el=>getComputedStyle(el).animationName)).toBe('none');
   await menu.getByRole('button',{name:'Close navigation',exact:true}).press('Enter');
   await expect(toggle).toBeFocused();
   await toggle.press('Enter');
   await menu.getByRole('link',{name:'Browse categories',exact:true}).press('Enter');
   await expect(page).toHaveURL(baseURL+'/categories');
   await expect(page.locator('#mobile-menu')).not.toBeVisible();
   await page.goto(baseURL+'/products', {waitUntil:'domcontentloaded'});
   if(screenshots&&name==='chromium'&&role==='customer')for(const width of [390,1440]){
    await page.setViewportSize({width,height:900});
    await page.locator('#wishlist-count em').evaluate(el=>el.textContent='12');
    await page.locator('#cart-count em').evaluate(el=>el.textContent='3');
    await page.locator('.site-header').screenshot({path:path.join(screenshots,`header-updated-${width}.png`)});
   }
   expect(errors,`${name} ${role} page errors`).toEqual([]);
   console.log(`${name} ${role}: 9 widths, single fixed toolbar, aligned search/profile, direct shopping counters/profile dismissal, modal keyboard/backdrop/navigation/reduced motion passed`);
   await page.close();
  }
  await browser.close();
 }
})().catch(err=>{console.error(err);process.exit(1)});
