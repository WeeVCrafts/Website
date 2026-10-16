const {test,expect}=require('@playwright/test');
const path=require('node:path');
const os=require('node:os');

// Explicit opt-in, public GETs only. All mutation suites keep their local guards.
test.describe('Published storefront deployment smoke',()=>{
 test.skip(process.env.WEEVCRAFTS_LIVE_SMOKE!=='1','Enable explicitly after deployment.');
 test.use({baseURL:'https://weevcrafts.com',trace:'off',screenshot:'off'});
 test('published identity, responsive layout and public policy content',async({page})=>{
  test.setTimeout(120000);
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:1000});const response=await page.goto('/');expect(response.ok()).toBeTruthy();expect(Number(response.headers()['x-storefront-revision'])).toBeGreaterThan(0);
   await page.evaluate(()=>document.fonts.ready);await expect(page.locator('.header-main .wordmark strong')).toHaveText('WeevCrafts');
   expect(await page.locator('.header-main .wordmark strong').evaluate(el=>getComputedStyle(el).fontFamily)).toContain('Cormorant Garamond');expect(await page.locator('body').evaluate(el=>getComputedStyle(el).fontFamily)).toContain('DM Sans');
   expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
   await page.screenshot({path:path.join(os.tmpdir(),`weevcrafts-production-${width}.png`)});
  }
  const policy=await page.goto('/pages/returns');expect(policy.ok()).toBeTruthy();await expect(page.getByRole('heading',{level:1,name:'Returns and refunds',exact:true})).toBeVisible();
 });
});
