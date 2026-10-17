const {test,expect}=require('@playwright/test');
const path=require('node:path');
const os=require('node:os');
test.use({baseURL:'https://weevcrafts.com',trace:'off',screenshot:'off'});

// Explicit opt-in, public GETs only. All mutation suites keep their local guards.
test.describe('Published storefront deployment smoke',()=>{
 test.skip(process.env.WEEVCRAFTS_LIVE_SMOKE!=='1','Enable explicitly after deployment.');
 test('brand discovery metadata and sitemap resolve on the canonical domain',async({request})=>{
  test.setTimeout(120000);
  const home=await request.get('/');expect(home.ok()).toBeTruthy();const html=await home.text();
  expect(html).toContain('<title>WeevCrafts | Handmade Indian Arts, Crafts &amp; Sarees</title>');
  expect(html).toContain('rel="canonical" href="https://weevcrafts.com/"');
  expect(html).toContain('name="robots" content="index,follow"');
  const schema=JSON.parse(html.match(/<script[^>]*id="website-schema"[^>]*>([\s\S]*?)<\/script>/)[1]);
  const website=schema['@graph'].find(n=>n['@type']==='WebSite'),organization=schema['@graph'].find(n=>n['@type']==='Organization');
  expect(website.name).toBe('WeevCrafts');expect(website.alternateName).toContain('weevcrafts');expect(new Set(website.alternateName).size).toBe(website.alternateName.length);expect(website.publisher['@id']).toBe(organization['@id']);
  const robots=await request.get('/robots.txt');expect(robots.ok()).toBeTruthy();expect(await robots.text()).toContain('Sitemap: https://weevcrafts.com/sitemap.xml');
  const sitemap=await request.get('/sitemap.xml');expect(sitemap.ok()).toBeTruthy();const xml=await sitemap.text();
  expect(xml).toContain('xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"');
  const locations=[...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map(m=>m[1].replaceAll('&amp;','&'));
  expect(locations.length).toBeGreaterThan(9);expect(locations.some(u=>u.includes('/makers/'))).toBe(true);
  for(const location of locations){expect(location.startsWith('https://weevcrafts.com/')).toBe(true);const response=await request.get(location);expect(response.status(),location).toBe(200);}
 });
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
