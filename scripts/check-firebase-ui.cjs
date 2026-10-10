const {chromium,firefox,webkit,expect}=require('@playwright/test');
const fs=require('node:fs');
const base=(process.env.WEEVCRAFTS_BASE_URL||'http://localhost:8080').replace(/\/$/,'');
(async()=>{
 for(const [name,type]of Object.entries({chromium,firefox,webkit})){
  const browser=await type.launch();const page=await browser.newPage({viewport:{width:390,height:844}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base+'/login');
  const google=page.locator('[data-firebase-google-panel]');await expect(google).toBeVisible({timeout:30000});
  await expect(page.locator('[data-firebase-link]')).toBeHidden();
  if(process.env.WEEVCRAFTS_FIREBASE_SCREENSHOTS){
   fs.mkdirSync(process.env.WEEVCRAFTS_FIREBASE_SCREENSHOTS,{recursive:true});
   await page.screenshot({path:process.env.WEEVCRAFTS_FIREBASE_SCREENSHOTS+'/'+name+'-login-mobile.png',fullPage:true});
   await page.setViewportSize({width:1440,height:900});
   await page.screenshot({path:process.env.WEEVCRAFTS_FIREBASE_SCREENSHOTS+'/'+name+'-login.png',fullPage:true});
   await page.setViewportSize({width:390,height:844});
  }
  await google.getByRole('button',{name:'Continue with Google'}).click();
  await expect(google.getByRole('status')).toContainText('Accept the privacy notice');
  await page.locator('form[data-backend-auth=login] input[name=email]').fill('not-a-customer@example.invalid');
  await page.locator('form[data-backend-auth=login] input[name=password]').fill('invalid-fixture-password');
  await page.locator('form[data-backend-auth=login] button[type=submit]').click();
  await expect(page.locator('form[data-backend-auth=login]')).toContainText('Sign-in could not be completed',{timeout:30000});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
  const sdkState=await page.evaluate(()=>({local:Object.keys(localStorage).filter(k=>k.startsWith('firebase:auth')),session:Object.keys(sessionStorage).filter(k=>k.startsWith('firebase:auth'))}));
  expect(sdkState.local).toEqual([]);expect(sdkState.session).toEqual([]);
  await page.setViewportSize({width:1440,height:900});
  const response=await page.request.post(base+'/api/v1/auth/login',{data:{email:process.env.WEEVCRAFTS_CUSTOMER_EMAIL||'demo.customer01@weevcrafts.local',password:process.env.WEEVCRAFTS_DEMO_PASSWORD||'WeeVCrafts-Demo-2026!'}});expect(response.ok()).toBeTruthy();
  await page.goto(base+'/account/notifications');await expect(page.locator('[data-firebase-push-panel]')).toBeVisible({timeout:30000});await expect(page.locator('[data-push-enable]')).toBeVisible();await expect(page.locator('[data-push-test]')).toBeHidden();
  await page.goto(base+'/account/privacy');await expect(page.locator('[data-firebase-diagnostics]')).toBeVisible({timeout:30000});
  const perfRequests=[];page.on('request',r=>{if(/firebaselogging|firebaseremoteconfig|firebaseinstallations/.test(r.url()))perfRequests.push(true)});await page.reload();await expect(page.locator('[data-firebase-diagnostics]')).toBeVisible({timeout:30000});expect(perfRequests.length).toBe(0);
  expect(errors).toEqual([]);console.log(name+': Google controls, terms guard, login errors, memory persistence, notification consent and diagnostic opt-in passed');await browser.close();
 }
})().catch(error=>{console.error('Firebase UI verification failed: '+error.message);process.exit(1)});
