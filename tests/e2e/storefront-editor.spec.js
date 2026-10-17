const {test,expect}=require('@playwright/test');
const path=require('node:path');
const fs=require('node:fs/promises');
const os=require('node:os');
const base=process.env.WEEVCRAFTS_BASE_URL||'http://127.0.0.1:18080';
test.use({baseURL:base});
test.describe.configure({mode:'serial'});
test.beforeAll(()=>{if(new URL(base).hostname!=='127.0.0.1'||!['18080','18085'].includes(new URL(base).port)||(new URL(base).port==='18085'&&process.env.WEEVCRAFTS_QA_DATABASE!=='storefront_guided_20261017'))throw new Error('Storefront mutations require the disposable local QA app.');});
async function login(page,account){const response=await page.request.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}});expect(response.ok()).toBeTruthy();}
async function saved(page){await expect(page.locator('.admin-notice[role=status]')).toContainText(/saved|Submitted|Published|requested/i);}
async function change(page,name,keyboard=false){const before=Number(await page.locator('input[name=version]').first().inputValue());const button=page.getByRole('button',{name,exact:true}).last();if(keyboard)await button.press('Enter');else await button.click();await expect(page.locator('.sf-draft-state')).toContainText(`Version ${before+1}`);}

async function tab(page,name){const ids={'Homepage':'homepage','Hero slides':'hero','Categories':'categories','Theme':'theme','Navigation':'navigation','Media':'media','Page templates':'templates','Content pages':'pages','Store settings':'settings','Promotions':'promotions','Publishing':'publishing'};const url=new URL(page.url());await page.goto(url.pathname+'?'+new URLSearchParams({draft:url.searchParams.get('draft'),tab:ids[name]}));}

test('Admin draft, private preview, Super Admin review, atomic publication and restoration',async({browser})=>{
 test.setTimeout(180000);
 const adminContext=await browser.newContext({baseURL:base});const superContext=await browser.newContext({baseURL:base});const publicContext=await browser.newContext({baseURL:base});
 const admin=await adminContext.newPage(),superadmin=await superContext.newPage(),customer=await publicContext.newPage();
 await login(admin,'marketplace01');await login(superadmin,'super01');
 await customer.goto('/');const original=await customer.locator('[data-hero-slide] h1').first().textContent();const revision=(await customer.request.get('/')).headers()['x-storefront-revision'];
 await admin.goto('/admin/storefront');await admin.getByRole('button',{name:'New draft from published design'}).click();await expect(admin).toHaveURL(/draft=\d+/);const draft=new URL(admin.url()).searchParams.get('draft');
 await tab(admin,'Hero slides');await admin.locator('.sf-items a').first().click();await admin.getByLabel('Heading',{exact:true}).fill('QA draft collection');await change(admin,'Save draft');
 await tab(admin,'Categories');await admin.locator('.sf-items a').filter({hasText:'Arts'}).first().click();await admin.getByLabel('Display order',{exact:true}).fill('999');await change(admin,'Save draft',true);
 await customer.reload();await expect(customer.locator('[data-hero-slide] h1').first()).toHaveText(original);
 const preview=await adminContext.newPage();const previewAPI=[];preview.on("request",request=>{if(/\/api\/v1\/(cart|wishlist|checkout)/.test(request.url()))previewAPI.push(request.url())});const previewResponse=await preview.goto(`/storefront/preview/${draft}`);expect(previewResponse.headers()['x-robots-tag']).toContain('noindex');expect(previewAPI).toEqual([]);await expect(preview.locator('[data-hero-slide] h1').first()).toHaveText('QA draft collection');
 expect(await preview.locator('meta[name=robots]').getAttribute('content')).toContain('noindex');expect((await customer.request.get(`/storefront/preview/${draft}`)).status()).toBe(401);expect((await admin.request.post(`/storefront/preview/${draft}`)).status()).toBe(405);
 await tab(admin,'Publishing');await expect(admin.getByRole('button',{name:'Publish reviewed changes'})).toHaveCount(0);await change(admin,'Submit for review');
 await superadmin.goto(`/super-admin/storefront?draft=${draft}&tab=publishing`);await superadmin.getByLabel('Publication / review note').fill('Review heading before publication');await change(superadmin,'Request changes');await expect(superadmin.locator('.sf-draft-state')).toContainText('draft');
 await admin.reload();await change(admin,'Submit for review');await superadmin.reload();await expect(superadmin.locator('.sf-draft-state')).toContainText('Submitted for review');await superadmin.getByLabel('Publication / review note').fill('Reviewed isolated homepage acceptance');await change(superadmin,'Publish reviewed changes');
 await customer.reload();await expect(customer.locator('[data-hero-slide] h1').first()).toHaveText('QA draft collection');await expect(customer.locator('.discovery-categories a').first()).not.toHaveText('Arts');
 const releaseRow=superadmin.locator('.sf-release-list li').filter({has:superadmin.getByText(`Release ${revision}`,{exact:true})});await releaseRow.getByRole('button',{name:'Restore as draft'}).click();await superadmin.getByLabel('Publication / review note').fill('Restore accepted storefront after isolated QA');await change(superadmin,'Publish reviewed changes');await customer.reload();await expect(customer.locator('[data-hero-slide] h1').first()).toHaveText(original);
 await adminContext.close();await superContext.close();await publicContext.close();
});

test('stale edits retain proposed values; editor and preview fit supported widths',async({page,context})=>{
 test.setTimeout(120000);await login(page,'super01');await page.goto('/super-admin/storefront');await page.getByRole('button',{name:'New draft from published design'}).click();await expect(page).toHaveURL(/draft=/);await tab(page,'Hero slides');await page.locator('.sf-items a').first().click();
 const other=await context.newPage();await other.goto(page.url());await page.getByLabel('Heading',{exact:true}).fill('First concurrent edit');await change(page,'Save draft');
 await other.getByLabel('Heading',{exact:true}).fill('Preserved stale proposal');await other.locator('[data-editor-form-save]').click();await expect(other.locator('.ui-form-error')).toContainText('changed');await expect(other.getByLabel('Heading',{exact:true})).toHaveValue('Preserved stale proposal');await expect(other.getByRole('link',{name:'Compare with the latest saved version'})).toBeVisible();
 const output=path.join(os.tmpdir(),'weevcrafts-storefront-editor');await fs.mkdir(output,{recursive:true});
 for(const width of [320,390,768,1440]){await page.setViewportSize({width,height:1000});await page.goto(new URL(page.url()).pathname+new URL(page.url()).search);expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);const formWidth=await page.getByLabel('Heading',{exact:true}).evaluate(el=>({field:el.getBoundingClientRect().width,panel:el.closest('.sf-settings').getBoundingClientRect().width}));expect(formWidth.field/formWidth.panel).toBeGreaterThan(.7);if(width<1100){await page.getByRole('tab',{name:'Edit',exact:true}).press('ArrowRight');await expect(page.getByRole('tab',{name:'Preview',exact:true})).toHaveAttribute('aria-selected','true');await expect(page.locator('[data-storefront-preview]')).toBeVisible();await page.getByRole('tab',{name:'Preview',exact:true}).press('ArrowLeft');await expect(page.getByLabel('Heading',{exact:true})).toBeVisible()}await page.screenshot({path:path.join(output,`editor-${width}.png`),fullPage:true});}
 await page.setViewportSize({width:1440,height:1000});
});

test('media uploads validate files and stay private until publication',async({page,browser})=>{
 test.setTimeout(180000);await login(page,'super01');const originalRevision=(await page.request.get('/')).headers()['x-storefront-revision'];await page.goto('/super-admin/storefront');await page.getByRole('button',{name:'New draft from published design'}).click();await expect(page).toHaveURL(/draft=/);await tab(page,'Media');
 const form=page.locator('form[action$="/storefront/upload"]');await form.getByLabel('Image description',{exact:true}).fill('QA ceramic photograph');await form.locator('[name=image_file]').setInputFiles({name:'invalid.png',mimeType:'image/png',buffer:Buffer.from('invalid image')});await form.getByRole('button',{name:'Upload photograph'}).click();await expect(form.locator('.ui-form-error')).toBeVisible();
 await form.locator('[name=image_file]').setInputFiles(path.resolve('web/assets/images/customer/ceramic-mugs.webp'));await form.getByRole('button',{name:'Upload photograph'}).click();await expect(page.locator('.admin-notice')).toContainText('Image uploaded',{timeout:40000});const media=await page.locator('.sf-media-grid input').first().inputValue();expect((await page.request.get(media+'?w=480')).ok()).toBeTruthy();
 const anonymous=await browser.newContext({baseURL:base});expect((await anonymous.request.get(media)).status()).toBe(404);
 await tab(page,'Hero slides');await page.locator('.sf-items a').first().click();await page.locator('.sf-settings .sf-image-control details').first().evaluate(el=>el.open=true);await page.getByLabel('Photograph URL',{exact:true}).fill(media);await change(page,'Save draft');await tab(page,'Publishing');await page.getByLabel('Publication / review note').fill('Reviewed uploaded photograph and responsive variants');await change(page,'Publish reviewed changes');expect((await anonymous.request.get(media+'?w=480')).ok()).toBeTruthy();
 await page.locator('.sf-release-list li').filter({has:page.getByText(`Release ${originalRevision}`,{exact:true})}).getByRole('button',{name:'Restore as draft'}).click();await page.getByLabel('Publication / review note').fill('Restore accepted photograph after isolated media acceptance');await change(page,'Publish reviewed changes');expect((await anonymous.request.get(media)).ok()).toBeTruthy();await anonymous.close();
});

test('templates, content pages and shopping settings share private drafts and publication',async({page,browser})=>{
 test.setTimeout(180000);
 await login(page,'super01');
 const originalRevision=(await page.request.get('/')).headers()['x-storefront-revision'];
 await page.goto('/super-admin/storefront');await page.getByRole('button',{name:'New draft from published design'}).click();await expect(page).toHaveURL(/draft=/);
 const draft=new URL(page.url()).searchParams.get('draft');
 const customer=await browser.newContext({baseURL:base});const publicPage=await customer.newPage();
 await tab(page,'Page templates');await page.locator('.sf-items a').filter({hasText:'search · Default'}).click();
 await page.getByLabel('Page heading / SEO title',{exact:true}).fill('Find a handmade favourite');await change(page,'Save draft');
 await publicPage.goto('/products');await expect(publicPage.getByRole('heading',{name:'Find a handmade favourite',exact:true})).toHaveCount(0);
 const preview=await page.context().newPage();await preview.goto(`/storefront/preview/${draft}?path=%2Fproducts`);await expect(preview.getByRole('heading',{name:'Find a handmade favourite',exact:true})).toBeVisible();await expect(preview.locator('.product-card').first()).toBeVisible();
 await tab(page,'Content pages');await page.getByText('Add content page',{exact:true}).click();
 const addPage=page.locator('form').filter({has:page.getByRole('button',{name:'Add page',exact:true})});await addPage.getByLabel('Page title',{exact:true}).fill('QA craft guide');await addPage.getByLabel('URL slug',{exact:true}).fill('qa-craft-guide');await change(page,'Add page');
 await page.getByLabel('Page content',{exact:true}).fill('A private guide to **handmade** craft.');await change(page,'Save draft');
 expect((await customer.request.get('/pages/qa-craft-guide')).status()).toBe(404);await preview.goto(`/storefront/preview/${draft}?path=%2Fpages%2Fqa-craft-guide`);await expect(preview.getByRole('heading',{name:'QA craft guide',exact:true,level:1})).toBeVisible();await expect(preview.locator('strong').filter({hasText:'handmade'})).toBeVisible();
 await tab(page,'Store settings');await page.getByLabel('Shipping fee (₹ INR)',{exact:true}).fill('123.45');await page.getByLabel('Wishlist',{exact:true}).uncheck();await change(page,'Save draft');
 await page.getByLabel('Example subtotal (₹ INR)',{exact:true}).fill('1000');await page.getByRole('button',{name:'Calculate example',exact:true}).click();await expect(page.locator('output')).toContainText('123.45');
 expect((await customer.request.get('/wishlist')).status()).not.toBe(403);
 const csrf=await page.locator('input[name=csrf_token]').first().inputValue();const version=await page.locator('input[name=version]').first().inputValue();
 expect((await page.request.post('/super-admin/storefront/submit',{form:{draft,version,csrf_token:'invalid'}})).status()).toBe(403);
 await tab(page,'Publishing');await page.getByLabel('Publication / review note').fill('Reviewed template, content and policy acceptance');await change(page,'Publish reviewed changes');
 await publicPage.goto('/products');await expect(publicPage.getByRole('heading',{name:'Find a handmade favourite',exact:true})).toBeVisible();await publicPage.goto('/pages/qa-craft-guide');await expect(publicPage.getByRole('heading',{name:'QA craft guide',exact:true,level:1})).toBeVisible();
 expect((await customer.request.get('/wishlist')).status()).toBe(403);expect((await customer.request.post('/api/v1/wishlist',{data:{}})).status()).toBe(403);
 await page.locator('.sf-release-list li').filter({has:page.getByText(`Release ${originalRevision}`,{exact:true})}).getByRole('button',{name:'Restore as draft'}).click();await page.getByLabel('Publication / review note').fill('Restore accepted design and shopping policy after isolated QA');await change(page,'Publish reviewed changes');
 expect((await customer.request.get('/pages/qa-craft-guide')).status()).toBe(404);expect((await customer.request.get('/wishlist')).status()).not.toBe(403);
 await publicPage.goto('/');await publicPage.evaluate(()=>document.fonts.ready);await expect(publicPage.locator('.header-main .wordmark strong')).toHaveText('WeevCrafts');expect(await publicPage.locator('.header-main .wordmark strong').evaluate(el=>getComputedStyle(el).fontFamily)).toContain('Cormorant Garamond');await publicPage.screenshot({path:path.join(os.tmpdir(),'weevcrafts-storefront-editor','home-brand-1440.png')});
 await customer.close();
});
