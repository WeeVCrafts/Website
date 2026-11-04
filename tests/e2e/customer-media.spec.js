const {test,expect,request}=require('@playwright/test');
const {execFileSync}=require('node:child_process');const fs=require('node:fs');const sharp=require('sharp');const path=require('node:path');const os=require('node:os');
const database='homepage_reference_20261103';let productID;
function guard(){if(process.env.WEEVCRAFTS_QA_DATABASE!==database)throw Error('Customer media tests require the isolated homepage QA database.');}
function sql(query){guard();return execFileSync('docker',['exec','weevcrafts-storefront-qa-db','psql','-v','ON_ERROR_STOP=1','-U','storefrontqa','-d',database,'-tAc',query],{encoding:'utf8',windowsHide:true}).trim();}
async function upload(api,csrf,type,data){const headers={'X-CSRF-Token':csrf};const prepared=await api.post('/api/v1/seller/products/media-url',{headers,data:{product_id:productID,content_type:type,byte_size:data.length}});expect(prepared.ok()).toBe(true);const {data:media}=await prepared.json();const stored=await api.put(media.upload_url,{headers:{...headers,'Content-Type':type},data});expect(stored.ok()).toBe(true);const done=await api.post('/api/v1/seller/products/media-finalize',{headers,data:{media_id:media.id}});expect(done.ok()).toBe(true);return media.id;}
test.beforeAll(async({baseURL})=>{
 guard();if(baseURL!=='http://127.0.0.1:18088')throw Error('Loopback only.');
 productID=Number(sql("SELECT id FROM products WHERE slug='blue-tote'"));
 const api=await request.newContext({baseURL});try{
  const login=await api.post('/api/v1/auth/login',{data:{email:'demo.super01@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}});expect(login.ok()).toBe(true);const session=await login.json();const csrf=session.csrf_token;
  // Keep setup repeatable without importing the owner's local image packs.
  if(!Number(sql(`SELECT count(*) FROM product_media WHERE product_id=${productID} AND content_type='video/mp4' AND status='ready'`))){
   await upload(api,csrf,'video/mp4',fs.readFileSync(path.join(__dirname,'fixtures/customer-media/preview.mp4')));
   const image=await sharp('web/assets/images/customer/blue-tote.webp').png().toBuffer();
   for(let i=0;i<4;i++)await upload(api,csrf,'image/png',image);
  }
 }finally{await api.dispose();}
});

test('uploaded photographs are resized, ranged video remains playable and errors never cache',async({page})=>{
 await page.goto('/products/blue-tote');
 const image=page.locator('[data-gallery-media]').first();await image.evaluate(i=>i.decode());
 const src=await image.getAttribute('src');expect(src).toMatch(/\/media\/products\/\d+\?v=2/);
 const originalURL=new URL(src,'http://127.0.0.1:18088');originalURL.searchParams.delete('w');const smallURL=new URL(originalURL);smallURL.searchParams.set('w','320');
 const original=await page.request.get(originalURL.href);const small=await page.request.get(smallURL.href);expect(small.status()).toBe(200);
 const originalBytes=await original.body(),smallBytes=await small.body();expect(smallBytes.length).toBeLessThan(originalBytes.length/2);expect((await sharp(smallBytes).metadata()).width).toBe(320);
 const video=page.locator('[data-video-thumbnail]').first();await video.scrollIntoViewIfNeeded();
 const supported=await video.evaluate(v=>!!v.canPlayType(v.dataset.contentType));
 if(supported)await expect.poll(()=>video.evaluate(v=>v.readyState>=2||v.dataset.previewUnavailable==='true'),{timeout:30000}).toBe(true);
 else{await expect(video).toHaveAttribute('data-preview-unavailable','true');await video.locator('..').locator('img').evaluate(i=>i.decode());}
 if(await video.getAttribute('data-preview-unavailable'))await video.locator('..').locator('img').evaluate(i=>i.decode());
 const videoURL=await page.locator('[data-gallery-media] source').first().getAttribute('data-src');const range=await page.request.get(videoURL,{headers:{Range:'bytes=0-1'}});expect(range.status()).toBe(206);expect((await range.body()).length).toBe(2);expect(range.headers()['content-range']).toMatch(/^bytes 0-1\//);
 const head=await page.request.head(videoURL);expect(head.status()).toBe(200);expect((await head.body()).length).toBe(0);
 await page.locator('[data-gallery-select]').filter({has:page.locator('[data-video-thumbnail]')}).click();await expect(page.locator('[data-gallery-media]:visible')).toHaveJSProperty('tagName','VIDEO');
 await expect(page.locator('[data-gallery-zoom]')).toBeHidden();
 if(!supported)await expect(page.locator('.gallery-video-feedback')).toContainText('cannot play this video format');
 await page.locator('[data-gallery-select]').first().press('Enter');await expect(image).toBeVisible();
 const missing=await page.request.get('/media/products/999999999?w=320');expect(missing.status()).toBe(404);expect(missing.headers()['cache-control']).toBe('no-store');
});

test('product details handle many thumbnails, long names and narrow reflow',async({page},testInfo)=>{
 for(const width of [320,390,768,1024,1536]){
  await page.setViewportSize({width,height:900});await page.goto('/products/blue-tote');await page.evaluate(()=>document.fonts.ready);
  await page.locator('.detail-copy h1').evaluate(el=>el.textContent='An extraordinarily long handmade product name for the customer reflow check');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  const buttons=page.locator('[data-gallery-select]');expect(await buttons.count()).toBeGreaterThanOrEqual(5);
  if(width<=720){const boxes=await buttons.evaluateAll(items=>items.map(x=>x.getBoundingClientRect().toJSON()));expect(boxes[0].y).toBeCloseTo(boxes[1].y,0);expect(boxes[1].x).toBeGreaterThan(boxes[0].x);}
  if(width<=720)await expect(page.locator('.detail-actions button').first()).toHaveCSS('white-space','nowrap');
  await buttons.first().press('End');await expect(buttons.last()).toHaveAttribute('aria-pressed','true');await buttons.last().press('Home');
  await page.locator('[data-gallery-zoom]').press('Enter');await expect(page.locator('.media-lightbox')).toBeVisible();await page.keyboard.press('Escape');await expect(page.locator('[data-gallery-zoom]')).toBeFocused();
  await page.screenshot({path:path.join(os.tmpdir(),`weev-product-repaired-${testInfo.project.name}-${width}.png`),fullPage:true});
  if(width===390){await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});const violations=await page.evaluate(async()=> (await axe.run(document.querySelector('.product-detail'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations);expect(violations.map(v=>({id:v.id,targets:v.nodes.map(n=>n.target)}))).toEqual([]);}
 }
});

for(const signedIn of [false,true])test(`customer routes reflow before and after sign-in: ${signedIn}`,async({page,context})=>{
 test.setTimeout(900000);
 if(signedIn)expect((await page.request.post('/api/v1/auth/login',{data:{email:'demo.customer08@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}})).ok()).toBe(true);
 const number=sql("SELECT o.order_number FROM orders o JOIN users u ON u.id=o.user_id WHERE u.email='demo.customer08@weevcrafts.local' ORDER BY o.id DESC LIMIT 1");
 const routes=['/','/products','/products/blue-tote','/categories','/makers','/brands','/search?q=craft','/deals','/featured-products','/cart','/wishlist','/login','/register','/forgot-password','/reset-password','/verify-email','/shipping','/faq','/contact','/size-guide','/terms','/privacy','/about','/our-story','/sustainability','/press','/careers','/pages/returns','/account','/account/profile','/account/addresses','/account/notifications','/account/security','/account/sessions','/account/privacy','/account/reviews','/account/support','/orders','/tracking','/returns','/payments','/notifications','/checkout'];
 if(number)routes.push('/orders/'+number,'/orders/'+number+'/confirmation','/orders/'+number+'/tracking');
 for(const width of [320,390,768,1024,1536]){
  // Release the previous viewport's graphics surfaces on long Windows runs.
  await page.close();page=await context.newPage();
  await page.setViewportSize({width,height:900});
  for(const route of routes){
   const response=await page.goto(route,{waitUntil:'domcontentloaded'});expect(response.status(),route).toBeLessThan(500);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${route} at ${width}`).toBe(true);
  }
 }
});
