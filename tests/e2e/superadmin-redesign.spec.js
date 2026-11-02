const {test,expect}=require('@playwright/test');
const path=require('node:path');
const fs=require('node:fs');
const axePath=require.resolve('axe-core/axe.min.js');
const evidence=path.resolve(__dirname,'../../superadmin-screens/verified');
const widths=[320,360,390,720,900,1024,1440];
const routes=['','orders','support','sellers','products','inventory','returns','finance','marketing','analytics','users','roles','security','settings','payments','failed-payments','refunds','settlements','commission-rules','finance-reconciliation','audit-logs','system-health','search-indexing-health','worker-queue-health','products/new','category-management','brands','customers','reviews'];

test.beforeAll(()=>{
 if(process.env.WEEVCRAFTS_QA_DATABASE!=='seller_workflows_20261017') throw new Error('Use the isolated seller_workflows_20261017 preview on port 18080.');
 fs.mkdirSync(evidence,{recursive:true});
});
async function login(page,account='super01'){
 const response=await page.request.post('/api/v1/auth/login',{data:{email:`demo.${account}@weevcrafts.local`,password:'WeeVCrafts-Demo-2026!'}});
 expect(response.ok()).toBeTruthy();
}
async function fits(page,route,width){
 const overflow=await page.evaluate(()=>({viewport:innerWidth,document:document.documentElement.scrollWidth}));
 expect(overflow.document,`${route} at ${width}px overflows`).toBeLessThanOrEqual(overflow.viewport+1);
 const bad=await page.locator('.admin-content :is(button,.admin-button,.support-button)').evaluateAll(nodes=>nodes.filter(el=>el.checkVisibility()&&getComputedStyle(el).whiteSpace==='nowrap'&&el.scrollWidth>el.clientWidth+3).map(el=>el.textContent.trim()));
 expect(bad,`${route}: clipped action text`).toEqual([]);
}

for(const group of [routes.slice(0,10),routes.slice(10,20),routes.slice(20)]){
 test(`responsive routes ${group[0]||'dashboard'} through ${group.at(-1)}`,async({page})=>{
  await login(page);
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  for(const route of group){
   for(const width of widths){
    await page.setViewportSize({width,height:1000});
    const response=await page.goto('/super-admin'+(route?'/'+route:''));
    expect(response.status(),route).toBe(200);
    await expect(page.locator('body')).toHaveClass(/superadmin-body/);
    await expect(page.locator('[role=alert]')).toHaveCount(0);
    await fits(page,route,width);
    if((width===1440||width===390)&&routes.indexOf(route)<14){await page.screenshot({path:path.join(evidence,`${route||'dashboard'}-${width}.png`),fullPage:route===''&&width===1440});}
   }
   if(['','orders','support','sellers','products','inventory','returns','finance','analytics','users'].includes(route)){
    await page.setViewportSize({width:1536,height:1024});await fits(page,route,1536);await page.screenshot({path:path.join(evidence,`${route||'dashboard'}-1536.png`)});
   }
  }
  expect(errors).toEqual([]);
 });
}

test('one User Management entry, persistent icon rail and mobile focus',async({page})=>{
 await login(page);await page.setViewportSize({width:1440,height:1000});await page.goto('/super-admin/users');
 const nav=page.locator('#admin-sidebar nav');
 await expect(nav.getByRole('link',{name:'User Management',exact:true})).toHaveCount(1);
 for(const name of ['Users','Roles','Security','Settings']) await expect(nav.getByRole('link',{name,exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'Collapse sidebar',exact:true}).click();
 expect((await page.locator('#admin-sidebar').boundingBox()).width).toBe(64);
 await page.reload();expect((await page.locator('#admin-sidebar').boundingBox()).width).toBe(64);
 await page.locator('#admin-sidebar').getByRole('link',{name:'User Management'}).focus();
 await expect(page.locator('#admin-sidebar .admin-nav-tooltip').filter({hasText:'User Management'})).toBeVisible();
 await page.setViewportSize({width:390,height:844});
 const menu=page.getByRole('button',{name:'Open admin navigation',exact:true});await menu.click();
 await expect(menu).toHaveAttribute('aria-expanded','true');
 await expect(page.locator('.admin-main')).toHaveAttribute('inert');
 const first=page.locator('#admin-sidebar button').first();await expect(first).toBeFocused();
 await page.keyboard.press('Shift+Tab');await expect(page.locator('#admin-sidebar a').last()).toBeFocused();
 await page.keyboard.press('Escape');await expect(menu).toBeFocused();await expect(menu).toHaveAttribute('aria-expanded','false');
 await page.setViewportSize({width:1440,height:1000});await page.getByRole('button',{name:'Expand sidebar'}).click();
 for(const [route,name] of [['users','Accounts'],['roles','Roles'],['security','Security'],['settings','Settings']]){
  await page.goto('/super-admin/'+route);await expect(nav.getByRole('link',{name:'User Management'})).toHaveAttribute('aria-current','page');
  await expect(page.getByRole('navigation',{name:'User Management sections'}).getByRole('link',{name,exact:true})).toHaveAttribute('aria-current','page');
 }
});

test('account charts, filters, CSV and existing account dialog work',async({page})=>{
 await login(page);await page.goto('/super-admin/users');
 await expect(page.getByRole('heading',{name:'Users by role'})).toBeVisible();
 await page.locator('#insight-range').selectOption('7');await page.getByRole('form',{name:'Chart date range'}).getByRole('button',{name:'Apply',exact:true}).click();
 await expect(page).toHaveURL(/range=7/);await page.getByText('View chart data',{exact:true}).click();
 await expect(page.getByRole('region',{name:'User growth data'}).locator('tbody tr')).toHaveCount(7);
 const manage=page.locator('.user-management-actions-cell button').first();
 if(await manage.count()){await manage.click();await expect(page.getByRole('dialog')).toBeVisible();await page.keyboard.press('Escape');await expect(manage).toBeFocused();}
 const filters=page.locator('.user-management-filters');await filters.locator('[name=q]').fill('no-matching-account-qa');await filters.getByRole('button',{name:'Apply filters'}).click();
 await expect(page.getByText('No matching user accounts.')).toBeVisible();
 const csv=await page.request.get(await page.getByRole('link',{name:'Export CSV'}).getAttribute('href'));expect(csv.status()).toBe(200);expect(await csv.text()).toContain('Account ID,Display name');
 await page.goto('/super-admin/inventory?stock=out_of_stock');await expect(page.locator('[name=stock]')).toHaveValue('out_of_stock');
 await page.goto('/super-admin/returns?status=approved&range=7');await expect(page.locator('.superadmin-queue-filters [name=status]')).toHaveValue('approved');
 const returnsCSV=await page.request.get('/super-admin/returns/export?status=approved&range=7');expect(returnsCSV.status()).toBe(200);
 const reportCSV=await page.request.get('/super-admin/analytics/export?range=7');expect(reportCSV.status()).toBe(200);
});

test('detail forms, empty states and narrow account actions remain usable',async({page})=>{
 await login(page);await page.setViewportSize({width:390,height:1000});
 await page.goto('/super-admin/products');
 const edit=page.getByRole('link',{name:'Edit',exact:true}).first();
 if(await edit.count()){
  await edit.click();await expect(page.getByRole('button',{name:'Save product',exact:true})).toBeVisible();
  for(const width of widths){await page.setViewportSize({width,height:1000});await fits(page,'product edit',width);}
 }
 await page.setViewportSize({width:390,height:1000});await page.goto('/super-admin/users');
 const actions=page.locator('.user-management-actions-cell').first();await actions.scrollIntoViewIfNeeded();
 await page.screenshot({path:path.join(evidence,'accounts-actions-390.png')});
 const manage=actions.locator('button').first();
 if(await manage.count()) {await manage.click();await expect(page.getByRole('dialog')).toBeVisible();await fits(page,'account dialog',390);await page.screenshot({path:path.join(evidence,'account-dialog-390.png')});await page.keyboard.press('Escape');}
 await page.locator('.user-create-disclosure summary').click();await expect(page.locator('.user-create-popover')).toBeVisible();await fits(page,'create account',390);await page.locator('.user-create-disclosure summary').click();
 for(const route of ['orders','support','products','inventory','returns']){
  await page.goto('/super-admin/'+route+'?q=no-matching-reference-qa');await fits(page,route+' empty',390);
  await page.screenshot({path:path.join(evidence,route+'-empty-390.png')});
 }
 await page.goto('/super-admin/finance?range=7&q=no-matching-reference-qa');await page.getByRole('navigation',{name:'Finance sections'}).getByRole('link',{name:'Refunds',exact:true}).click();await expect(page).toHaveURL(/range=7/);await expect(page).toHaveURL(/q=no-matching-reference-qa/);
});

test('keyboard, accessibility, reflow and other role boundaries',async({page})=>{
 await login(page);
 for(const route of ['users','inventory','returns','finance','support','settings']){
  await page.goto('/super-admin/'+route);await page.addScriptTag({path:axePath});
  const findings=await page.evaluate(async()=>{const r=await axe.run(document.querySelector('.admin-main'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}});return r.violations.filter(v=>['critical','serious'].includes(v.impact)).map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}));});
  expect(findings,route+' accessibility').toEqual([]);
 }
 await page.setViewportSize({width:720,height:500});await page.goto('/super-admin/users');await page.evaluate(()=>document.body.style.zoom='2');await fits(page,'200% zoom reflow',720);
 for(const [account,route] of [['customer01','/account'],['owner01','/seller-admin'],['marketplace01','/admin']]){
  await page.context().clearCookies();await login(page,account);await page.goto(route);await expect(page.locator('body')).not.toHaveClass(/superadmin-body/);
  expect((await page.request.get('/super-admin/users',{maxRedirects:0})).status()).toBe(403);
  expect((await page.request.get('/super-admin/analytics/export',{maxRedirects:0})).status()).toBe(403);
 }
});
