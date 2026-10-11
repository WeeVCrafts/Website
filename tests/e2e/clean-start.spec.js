const { test, expect, request } = require('@playwright/test');
const password = 'WeeVCrafts-Local-2026!';
test.beforeAll(() => {
 const host=new URL(process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:8080').hostname;
 if(!['localhost','127.0.0.1','[::1]'].includes(host)) throw new Error('Clean-start verification requires a local application.');
});
const roles = [
  {email:'customer@weevcrafts.local',path:'/account',header:'.site-header',logout:'.header-logout'},
  {email:'seller@weevcrafts.local',path:'/seller-admin',header:'.seller-header',logout:'.seller-header-logout'},
  {email:'superadmin@weevcrafts.local',path:'/super-admin/users',header:'.admin-header',logout:'.admin-logout'},
];
async function signIn(page, email) {
 await page.goto('/login');
 await page.locator('[data-backend-auth="login"] [name="email"]').fill(email);
 await page.locator('[data-backend-auth="login"] [name="password"]').fill(password);
 await page.locator('[data-backend-auth="login"] [type="submit"]').click();
 await expect(page).not.toHaveURL(/\/login(?:\?|$)/);
}
for (const role of roles) {
 test(`${role.email}: header logout works at desktop and narrow widths`, async ({page}) => {
  await signIn(page,role.email);
  await page.goto(role.path);
  for (const width of [1440,960,720,390,320]) {
   await page.setViewportSize({width,height:900});
   const button=page.locator(role.logout+' button');
   await expect(button).toBeVisible();
   const box=await button.boundingBox();
   expect(box.width).toBeGreaterThanOrEqual(44);
   expect(box.height).toBeGreaterThanOrEqual(44);
   expect(box.x+box.width).toBeLessThanOrEqual(width+1);
   expect(box.y).toBeLessThan(100);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
   await button.focus();await expect(button).toBeFocused();
   if(process.env.WEEVCRAFTS_REVIEW_DIR && [1440,390].includes(width)) {
    await page.screenshot({path:require('node:path').join(process.env.WEEVCRAFTS_REVIEW_DIR,role.email.split('@')[0]+'-'+width+'.png')});
   }

  }
  if(role.email.startsWith('seller')) {
   const denied=await page.request.get('/super-admin/users');expect(denied.status()).toBe(403);
  }
  if(role.email.startsWith('superadmin')) {
   await expect(page.locator('.role-assignment-card')).toHaveCount(3);
   const owner=page.locator('.role-assignment-card').filter({hasText:'seller@weevcrafts.local'});
   await owner.locator('summary').click();
   await expect(owner.locator('[name="role"]')).toHaveValue('seller_owner');
   const own=page.locator('.role-assignment-card').filter({hasText:'superadmin@weevcrafts.local'});
   expect(await own.locator('.role-assignment-form,.role-delete-form').count()).toBe(0);
  }
  await page.locator(role.logout+' button').press('Enter');
  await page.goto(role.path);
  await expect(page).toHaveURL(/\/login/);
 });
}
test('Super Admin account creation, search, edits, role changes and deletion persist', async ({page}) => {
 await signIn(page,'superadmin@weevcrafts.local');await page.goto('/super-admin/users');
 await page.locator('.account-create > summary').click();
 const create=page.locator('.role-user-create-form');
 await create.locator('[name="display_name"]').fill('QA Clean Account');
 await create.locator('[name="email"]').fill('qa.clean@example.test');
 await create.locator('[name="password"]').fill('Temporary-Clean-2026!');
 await create.getByRole('button',{name:'Show password'}).click();
 await expect(create.locator('[name="password"]')).toHaveAttribute('type','text');
 await create.getByRole('button',{name:'Hide password'}).click();
 await create.locator('[name="reason"]').fill('Verify account creation');
 await create.getByRole('button',{name:'Create account',exact:true}).click();
 await expect(page).toHaveURL(/\/super-admin\/users\?notice=/);
 await page.locator('.role-assignment-search [name="q"]').fill('qa.clean');
 await page.locator('.role-assignment-search [type="submit"]').click();
 await expect(page.locator('.role-assignment-card')).toHaveCount(1);
 await page.locator('.account-controls summary').click();
 const edit=page.locator('.role-user-edit-form');
 await edit.locator('[name="display_name"]').fill('QA Updated Account');
 await edit.locator('[name="status"]').selectOption('suspended');
 await edit.locator('[name="password"]').fill('Updated-Clean-2026!');
 await edit.locator('[name="reason"]').fill('Verify persisted account edit');
 await edit.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(page).toHaveURL(/notice=Account\+updated/);
 let account=page.locator('.role-assignment-card').filter({hasText:'qa.clean@example.test'});
 await expect(account.locator('.role-assignment-user strong')).toHaveText('QA Updated Account');
 await account.locator('summary').click();
 await expect(account.locator('.ui-status')).toHaveText('Suspended');
 const reactivate=account.locator('.role-user-edit-form');
 await reactivate.locator('[name="status"]').selectOption('active');
 await reactivate.locator('[name="reason"]').fill('Restore temporary account access');
 await reactivate.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(account.locator('.ui-status')).toHaveText('Active');
 const loginCheck=await request.newContext({baseURL:new URL(page.url()).origin});
 try {
  const signedIn=await loginCheck.post('/api/v1/auth/login',{data:{email:'qa.clean@example.test',password:'Updated-Clean-2026!'}});
  expect(signedIn.ok()).toBeTruthy();
 } finally { await loginCheck.dispose(); }
 await account.locator('summary').click();
 const role=account.locator('.role-assignment-form');
 await role.locator('[name="role"]').selectOption('support_agent');
 await role.locator('[name="reason"]').fill('Verify scoped role change');
 await role.getByRole('button',{name:'Save role change'}).click();
 await expect(page).toHaveURL(/notice=Role\+updated/);
 account=page.locator('.role-assignment-card').filter({hasText:'qa.clean@example.test'});
 await expect(account.locator('.role-badges')).toContainText('Support agent');
 await account.locator('summary').click();
 const remove=account.locator('.role-delete-form');
 await remove.locator('[name="reason"]').fill('Remove temporary verification account');
 await remove.getByRole('button',{name:'Delete account'}).click();
 await expect(page.locator('.ui-confirm')).toBeVisible();
 await page.locator('[data-confirm-accept]').click();
 await expect(page).toHaveURL(/notice=User\+account\+deleted/);
 await expect(page.locator('.role-assignment-card').filter({hasText:'qa.clean@example.test'})).toHaveCount(0);
 await expect(page.locator('.role-assignment-card .ui-status').filter({hasText:'Deleted'})).toHaveCount(1);
 const denied=await page.request.post('/super-admin/users/create',{form:{email:'rejected@example.test',display_name:'Rejected Account',password,role:'customer',reason:'Missing CSRF check'}});
 expect(denied.status()).toBe(403);
});
