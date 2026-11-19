const {test,expect}=require('@playwright/test');
const path=require('node:path');
const fs=require('node:fs');
const {output}=require('../../scripts/ui-ux-qa-paths.cjs');
fs.mkdirSync(output,{recursive:true});
const widths=[320,390,768,800,801,1024,1440,1920];
const credentials={email:'demo.customer01@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'};

async function checkActions(page) {
  const actions=page.locator('.customer-shopping-header .header-actions>a,.customer-shopping-header .header-actions>.profile-disclosure>.profile-toggle');
  await expect(actions).toHaveCount(3);
  const defects=await actions.evaluateAll(elements=>elements.flatMap(element=>{
    const errors=[],r=element.getBoundingClientRect(),style=getComputedStyle(element);
    const label=element.querySelector('.header-action-label,.account-label'),icon=element.querySelector('.material-symbols-outlined,.header-avatar');
    if(r.width<48||r.height<48)errors.push('action target smaller than 48px');
    if(!label||getComputedStyle(label).display==='none'||getComputedStyle(label).fontSize!=='14px')errors.push('label missing or inconsistent');
    if(style.flexDirection!=='column')errors.push('action is not stacked');
    if(icon&&label&&icon.getBoundingClientRect().bottom>label.getBoundingClientRect().top+1)errors.push('icon overlaps label');
    if(r.left<0||r.right>innerWidth+1)errors.push('action outside viewport');
    return errors;
  }));
  expect(defects).toEqual([]);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
}

test('guest shopping actions are uniform across customer and auth screens',async({page,browserName})=>{
  test.setTimeout(120000);
  await page.goto('/products');
  const productPath=await page.locator('a[href^="/products/"]').first().getAttribute('href');
  expect(productPath).toMatch(/^\/products\/[^/]+$/);
  for(const route of ['/','/products','/categories',productPath,'/makers','/search?q=art','/featured-products','/shipping','/cart','/login','/register','/forgot-password']) {
    await page.goto(route);await page.evaluate(()=>document.fonts.ready);
    for(const width of widths) {
      await page.setViewportSize({width,height:900});await checkActions(page);
      await expect(page.getByRole('link',{name:'Sign in',exact:true}).first()).toBeVisible();
      const header=page.locator('.customer-shopping-header');
      if (width>=1200) {
        await expect(header.locator('.homepage-delivery')).toBeVisible();
        await expect(header.locator('.storefront-nav .homepage-offers')).toBeVisible();
      }
      await expect(header.locator('.storefront-nav a[href="/featured-products"]')).toHaveCount(0);
      if(width<=800) {
        const search=await page.locator('.search-shell').boundingBox(),actions=await page.locator('.header-actions').boundingBox();
        expect(search.y).toBeGreaterThanOrEqual(actions.y+actions.height);
      }
      if(browserName==='chromium'&&[320,1440].includes(width)&&['/','/products','/login'].includes(route))await page.screenshot({path:path.join(output,`uniform-header-${route==='/'?'home':route.slice(1)}-${width}.png`)});
    }
  }
});

test('signed-in customers keep three actions and notification access inside Account',async({page})=>{
  test.setTimeout(120000);
  if(process.env.MARKETPLACE_UI_FIXTURE_FILE) {
    const baseURL=test.info().project.use.baseURL;
    expect(new URL(baseURL).hostname).toBe('127.0.0.1');
    const fixture=JSON.parse(fs.readFileSync(process.env.MARKETPLACE_UI_FIXTURE_FILE,'utf8'));
    await page.context().addCookies([{name:'wecratfs_session',value:fixture.Roles.customer.Token,url:baseURL,httpOnly:true}]);
  } else {
    expect((await page.request.post('/api/v1/auth/login',{data:credentials})).ok()).toBe(true);
  }
  for(const route of ['/','/products','/account','/orders','/wishlist','/cart']) {
    await page.goto(route);await page.evaluate(()=>document.fonts.ready);
    for(const width of widths) {
      await page.setViewportSize({width,height:900});
      for(const value of ['0','12','123']) {
        await page.locator('#wishlist-count em,#cart-count em').evaluateAll((elements,value)=>elements.forEach(e=>e.textContent=value),value);
        await checkActions(page);
      }
      const toggle=page.getByRole('button',{name:'Account menu',exact:true});
      await toggle.press('Enter');await expect(page.locator('#profile-dropdown')).toBeVisible();
      await expect(page.locator('#profile-dropdown [data-notification-bell]')).toBeVisible();
      await expect(page.locator('#profile-dropdown').getByRole('button',{name:'Sign out'})).toBeVisible();
      await page.keyboard.press('Escape');await expect(toggle).toBeFocused();
    }
  }
});

test('customer password login defaults home and preserves safe continuation',async({browser,baseURL})=>{
  test.setTimeout(120000);
  for(const javaScriptEnabled of [true,false])for(const next of ['', '/cart', 'https://evil.test']) {
    const context=await browser.newContext({baseURL,javaScriptEnabled});const page=await context.newPage();
    await page.goto('/login'+(next?'?next='+encodeURIComponent(next):''));
    const form=page.locator('.gallery-form form[data-backend-auth="login"]');
    await form.locator('[name=email]').fill(credentials.email);await form.locator('[name=password]').fill(credentials.password);
    await form.locator('button[type=submit]').click();
    await expect(page).toHaveURL(new URL(next==='/cart'?'/cart':'/',baseURL).href);
    await context.close();
  }
});

test('mocked Google exchange and account linking honor homepage and task destinations',async({browser,baseURL})=>{
  test.setTimeout(90000);
  for(const scenario of [{next:'',link:false},{next:'/cart',link:false},{next:'',link:true}]) {
    const context=await browser.newContext({baseURL});const page=await context.newPage();let exchanges=0;
    await page.route('**/api/v1/firebase/config',route=>route.fulfill({json:{enabled:true,config:{projectId:'isolated-test'}}}));
    await page.route('https://www.gstatic.com/firebasejs/13.0.0/*.js',route=>route.fulfill({contentType:'text/javascript',headers:{'Access-Control-Allow-Origin':'*'},body:route.request().url().endsWith('firebase-app.js')?'export function initializeApp(){return {}}':`
      export const inMemoryPersistence={};export function getAuth(){return {}};export async function setPersistence(){};
      export class GoogleAuthProvider {setCustomParameters(){}}
      export async function signInWithPopup(){return {user:{emailVerified:true,getIdToken:async()=>"isolated-mock-proof"}}};
    `}));
    await page.route('**/api/v1/auth/firebase/session',route=>{
      exchanges++;const body=route.request().postDataJSON();expect(body.next).toBe(scenario.next);
      if(scenario.link&&exchanges===1)return route.fulfill({status:409,json:{error:{code:'LINK_REQUIRED',message:'Confirm the existing account password.'}}});
      if(scenario.link)expect(body.link_password).toBe('isolated-link-proof');
      // Missing redirect also exercises the browser's homepage fallback.
      return route.fulfill({json:{csrf_token:'isolated-csrf',...(scenario.next?{redirect_to:scenario.next}:{})}});
    });
    await page.goto('/login'+(scenario.next?'?next='+encodeURIComponent(scenario.next):''));
    const form=page.locator('.gallery-form');await expect(form.locator('[data-google-signin]')).toBeEnabled();
    await expect(form.locator('[data-google-username]')).toBeHidden();await form.locator('[data-google-signin]').click();
    if(scenario.link) {
      const link=form.locator('[data-firebase-link]');await expect(link).toBeVisible();
      await link.locator('input').fill('isolated-link-proof');await link.locator('button[type=submit]').click();
    }
    await expect(page).toHaveURL(new URL(scenario.next||'/',baseURL).href);
    expect(exchanges).toBe(scenario.link?2:1);await context.close();
  }
});
