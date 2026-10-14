const {chromium,firefox,webkit,expect}=require('@playwright/test');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {output}=require('./ui-ux-qa-paths.cjs');
const target=process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18080';
if(target!=='http://127.0.0.1:18080')throw new Error('Auth checks require the isolated UI QA preview.');
const report={target,browsers:{},scope:'Public discovery and existing protected actions; demo fixtures/intercepted failures only.'};
async function main(){
 await fs.mkdir(output,{recursive:true});
 for(const [engine,type]of Object.entries({chromium,firefox,webkit})){
  const browser=await type.launch();const context=await browser.newContext({baseURL:target,viewport:{width:390,height:844}});
  const page=await context.newPage();page.setDefaultTimeout(15000);page.setDefaultNavigationTimeout(30000);
  const errors=[];page.on('pageerror',error=>errors.push(error.message));const checks=[];report.browsers[engine]=checks;
  async function check(name,work){try{await work();checks.push({name,pass:true});console.log(engine+' PASS '+name);}catch(error){checks.push({name,pass:false,error:error.message});console.log(engine+' FAIL '+name+' '+error.message.slice(0,240));await page.screenshot({path:path.join(output,'auth-failed-'+engine+'.png')}).catch(()=>{});}finally{await page.unroute('**/api/v1/auth/login').catch(()=>{});await page.unroute('**/api/v1/auth/register').catch(()=>{});}}
  await check('guest discovery stays public; filters and native navigation work',async()=>{
   for(const route of ['/','/products','/products/blue-tote','/categories','/makers','/brands','/search?q=tote','/deals']){
    assert.equal((await page.goto(route,{waitUntil:'networkidle'})).status(),200);
    assert.equal(await page.locator('#guest-login-dialog[open]').count(),0);
    assert.equal(new URL(page.url()).pathname,new URL(route,target).pathname);
   }
   await page.goto('/products');await page.locator('[data-filter-open]').click();
   await page.locator('[data-filter-panel] input[name=category][value=crafts]').check();await page.waitForURL(/category=crafts/);
   await page.waitForFunction(()=>document.querySelector('[data-filter-panel]')?.classList.contains('is-open')&&!document.querySelector('#listing-results')?.hasAttribute('aria-busy'));
   await page.keyboard.press('Escape');await expect(page.locator('[data-filter-panel]')).not.toBeVisible();await page.locator('.product-card h3 a[href^="/products/"]').first().click();
   assert.match(new URL(page.url()).pathname,/^\/products\/.+/);assert.equal(await page.locator('#guest-login-dialog[open]').count(),0);
  });
  await check('protected action asks only on intent; close/Escape restore focus without a write',async()=>{
   await page.goto('/products/blue-tote');let writes=0;
   const count=request=>{if(request.method()==='POST'&&/\/cart\/|\/wishlist\//.test(request.url()))writes++;};page.on('request',count);
   const buy=page.locator('.mobile-buy-bar button').first();await buy.click();
   await expect(page.locator('#guest-login-dialog')).toBeVisible();
   await expect(page.getByRole('button',{name:'Close sign-in prompt'})).toBeFocused();
   await page.keyboard.press('Escape');await expect(page.locator('#guest-login-dialog')).not.toBeVisible();await expect(buy).toBeFocused();
   await buy.click();await page.getByRole('button',{name:'Keep browsing'}).click();await expect(buy).toBeFocused();assert.equal(writes,0);page.off('request',count);
   await buy.click();await page.locator('[data-login-continue]').click();
   await page.waitForURL(/\/login\?next=/);assert.equal(new URL(page.url()).searchParams.get('next'),'/products/blue-tote');
   for(const protectedRoute of ['/account','/orders','/checkout','/wishlist']){
    const response=await context.request.get(protectedRoute,{maxRedirects:0});assert.equal(response.status(),303);
   }
   assert.equal((await context.request.get('/api/v1/orders')).status(),401);
  });
  await check('login/signup navigation keeps return path; account exit never gates browsing',async()=>{
   await page.goto('/login?next=%2Fproducts%2Fblue-tote%3Fcategory%3Dtextiles');
   await page.getByRole('navigation',{name:'Account access'}).getByRole('link',{name:'Create account',exact:true}).click();
   assert.equal(new URL(page.url()).searchParams.get('next'),'/products/blue-tote?category=textiles');
   assert.equal(await page.locator('[name=next]').inputValue(),'/products/blue-tote?category=textiles');
   await page.getByRole('navigation',{name:'Account access'}).getByRole('link',{name:'Sign in',exact:true}).click();
   assert.equal(new URL(page.url()).searchParams.get('next'),'/products/blue-tote?category=textiles');
   await page.getByRole('link',{name:'Continue browsing',exact:true}).click();await page.waitForURL(target+'/');assert.equal(await page.locator('#guest-login-dialog[open]').count(),0);
  });
  await check('auth forms: labelled errors, reveal, pending/duplicate guard and failed-input recovery',async()=>{
   await page.goto('/login');const form=page.locator('[data-backend-auth=login]');let attempts=0;
   await page.route('**/api/v1/auth/login',async route=>{attempts++;await new Promise(resolve=>setTimeout(resolve,300));await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'QA_UNAVAILABLE',message:'Sign-in is temporarily unavailable. Please try again.'}})});});
   await form.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.locator('#login-email')).toBeFocused();
   await expect(page.locator('#login-email')).toHaveAttribute('aria-invalid','true');assert.equal(attempts,0);
   await page.getByLabel('Username or email',{exact:true}).fill('demo.customer01@weevcrafts.local');
   await page.getByLabel('Password',{exact:true}).fill('WeeVCrafts-Demo-2026!');
   await page.getByRole('button',{name:'Show password',exact:true}).click();assert.equal(await page.locator('#login-password').getAttribute('type'),'text');
   await page.getByRole('button',{name:'Hide password',exact:true}).click();assert.equal(await page.locator('#login-password').getAttribute('type'),'password');
   const button=form.getByRole('button',{name:'Sign in',exact:true});const before=await button.boundingBox();
   await button.evaluate(element=>{element.click();element.click();});await expect(button).toBeDisabled();await expect(button).toHaveAttribute('aria-busy','true');
   const pending=await button.boundingBox();assert.equal(pending.width,before.width);assert.equal(pending.height,before.height);
   await expect(form.locator('[data-backend-form-message]')).toContainText('temporarily unavailable');await expect(button).toBeEnabled();assert.equal(attempts,1);
   assert.equal(await page.locator('#login-email').inputValue(),'demo.customer01@weevcrafts.local');assert.equal(await page.locator('#login-password').inputValue(),'WeeVCrafts-Demo-2026!');await page.unroute('**/api/v1/auth/login');
   await page.goto('/register');const register=page.locator('[data-backend-auth=register]');let registrations=0;
   await page.route('**/api/v1/auth/register',route=>{registrations++;return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'QA_UNAVAILABLE',message:'Account creation is unavailable. Please retry.'}})});});
   await register.getByRole('button',{name:'Create account',exact:true}).click();await expect(page.locator('#register-name')).toBeFocused();assert.equal(registrations,0);
   await page.getByLabel('Full name',{exact:true}).fill('UI Auth Review');await page.getByLabel('Email address',{exact:true}).fill('auth-review@weevcrafts.local');
   await page.getByLabel('Phone number',{exact:true}).fill('9876543210');await page.getByLabel('Password',{exact:true}).fill('WeeVCrafts-Demo-2026!');
   await register.getByRole('button',{name:'Create account',exact:true}).click();await expect(register.locator('[name=consent]')).toHaveAttribute('aria-invalid','true');assert.equal(registrations,0);
   await register.locator('[name=consent]').check();await register.getByRole('button',{name:'Create account',exact:true}).click();
   await expect(register.locator('[data-backend-form-message]')).toContainText('Please retry');assert.equal(registrations,1);assert.equal(await page.locator('#register-name').inputValue(),'UI Auth Review');assert(await register.locator('[name=consent]').isChecked());
   await page.unroute('**/api/v1/auth/register');
  });
  await check('auth viewport/keyboard/accessibility matrix and unavailable-provider fallback',async()=>{
   for(const route of ['/login','/register','/forgot-password','/reset-password']){
    await page.goto(route,{waitUntil:'networkidle'});await page.evaluate(()=>document.fonts.ready);
    for(const width of [320,360,390,430,768,1024,1440]){await page.setViewportSize({width,height:844});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2),false,route+' '+width);}
    await page.setViewportSize({width:390,height:844});
    if(route==='/login'||route==='/register'){
     assert.equal(await page.locator('.site-footer').count(),0);
     assert.equal(await page.locator('[data-backend-auth] input[name=password]').getAttribute('autocomplete'),route==='/login'?'current-password':'new-password');
     await expect(page.locator('[data-google-signin]')).toBeHidden();await expect(page.locator('[data-firebase-config-note]')).toContainText('Google sign-in is unavailable');
     await page.locator('[data-backend-auth] input:not([type=hidden])').first().focus();await page.keyboard.press('Tab');assert(await page.evaluate(()=>document.activeElement!==document.body));
    }
    if(engine==='chromium'){
     await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});
     const violations=await page.evaluate(async()=> (await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations.map(v=>v.id));assert.deepEqual(violations,[],route);
     if(route==='/login'||route==='/register'){for(const width of [390,1440]){await page.setViewportSize({width,height:900});await page.screenshot({path:path.join(output,'auth-after-'+route.slice(1)+'-'+width+'.png'),fullPage:true});}}
    }
   }
  });
  await check('existing customer sign-in returns to selected public product',async()=>{
   await page.goto('/login?next=%2Fproducts%2Fblue-tote');await page.locator('#login-email').fill('demo.customer01@weevcrafts.local');await page.locator('#login-password').fill('WeeVCrafts-Demo-2026!');
   await page.locator('[data-backend-auth=login]').getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(target+'/products/blue-tote');await expect(page.locator('form[action="/logout"]').first()).toBeAttached();
  });
  await check('no uncaught errors',async()=>assert.deepEqual(errors,[]));
  await context.close();
  if(engine==='chromium'){
   const signup=await browser.newContext({baseURL:target,viewport:{width:390,height:844}});const create=await signup.newPage();
   await check('native signup succeeds on one isolated demo account and keeps return path',async()=>{
    await create.goto('/register?next=%2Fproducts%2Fblue-tote');
    await create.getByLabel('Full name',{exact:true}).fill('UI Auth Review');
    await create.getByLabel('Email address',{exact:true}).fill('ui-auth-review-'+Date.now()+'@weevcrafts.local');
    await create.getByLabel('Phone number',{exact:true}).fill('9876543210');
    await create.getByLabel('Password',{exact:true}).fill('WeeVCrafts-Demo-2026!');
    await create.locator('[name=consent]').check();
    await create.locator('[data-backend-auth=register]').getByRole('button',{name:'Create account',exact:true}).click();
    await create.waitForURL(target+'/products/blue-tote',{timeout:30000});
    await expect(create.locator('form[action="/logout"]').first()).toBeAttached();
   });await signup.close();
   const plain=await browser.newContext({baseURL:target,javaScriptEnabled:false,viewport:{width:390,height:844}});const nojs=await plain.newPage();
   await check('no-JS discovery and native account forms remain available',async()=>{
    assert.equal((await nojs.goto('/products')).status(),200);await nojs.locator('.product-card a[href^="/products/"]').first().click();assert.match(new URL(nojs.url()).pathname,/^\/products\/.+/);
    await nojs.goto('/login');await expect(nojs.locator('[data-backend-auth=login]')).toBeVisible();await expect(nojs.locator('[data-google-signin]')).toBeHidden();await nojs.getByRole('link',{name:'Continue browsing',exact:true}).click();await nojs.waitForURL(target+'/');
   });await plain.close();
  }
  await browser.close();await fs.writeFile(path.join(output,'auth-discovery-verification.json'),JSON.stringify(report,null,2));
 }
 process.exitCode=Object.values(report.browsers).flat().some(c=>!c.pass)?1:0;
}
main().catch(async error=>{console.error(error.message);await fs.writeFile(path.join(output,'auth-discovery-verification.json'),JSON.stringify({...report,fatal:error.message},null,2));process.exitCode=1;});
