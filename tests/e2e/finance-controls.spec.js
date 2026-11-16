const {test, expect} = require('@playwright/test');
const os = require('node:os');
const path = require('node:path');
test.use({baseURL:'http://127.0.0.1:18087'});

for (const width of [390,1440]) {
  test(`manual finance controls remain usable by keyboard at ${width}px`, async ({page})=>{
    await page.setViewportSize({width,height:1000});
    await page.goto('/admin/finance');
    await page.waitForLoadState('networkidle');
    await expect(page.getByText('No button sends money.',{exact:false})).toBeVisible();
    await expect(page.getByRole('button',{name:'Approve recipient and batch',exact:true})).toHaveCount(0);
    const summary=page.locator('summary').filter({hasText:'Prepare recipient for approval'});
    await summary.focus();await page.keyboard.press('Enter');
    const account=page.getByLabel('Account holder',{exact:true});await expect(account).toBeVisible();
    await page.keyboard.press('Tab');await expect(account).toBeFocused();
    await account.fill('Verified seller account holder');
    await page.getByLabel('Bank name',{exact:true}).fill('Seller nominated bank');
    await expect(page.getByLabel('Private recipient evidence',{exact:true})).toHaveAttribute('required','');
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.screenshot({path:path.join(os.tmpdir(),`weev-finance-prepare-${width}.png`),fullPage:true});
    await page.goto('/admin/finance?actor=approver');
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('button',{name:'Save recipient version',exact:true})).toHaveCount(0);
    const approval=page.getByRole('checkbox',{name:'I independently verified the seller, bank recipient and payable amount.'});
    await approval.focus();await page.keyboard.press('Space');await expect(approval).toBeChecked();
    await page.goto('/admin/finance?stage=approved');
    await page.locator('summary').filter({hasText:'Record completed bank transfer'}).press('Enter');
    await expect(page.getByLabel('Transferred amount in paise',{exact:true})).toHaveValue('900000');
    await expect(page.getByLabel('Private transfer evidence',{exact:true})).toHaveAttribute('required','');
    await page.goto('/admin/finance?actor=approver&stage=proof');
    await expect(page.getByRole('button',{name:'Confirm transfer paid',exact:true})).toBeVisible();
    await expect(page.getByRole('button',{name:'Cancel batch before transfer',exact:true})).toHaveCount(0);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.screenshot({path:path.join(os.tmpdir(),`weev-finance-proof-${width}.png`),fullPage:true});
    await page.goto('/admin/finance?stage=legacy');
    await expect(page.getByText('Historical paid record: evidence requires reconciliation',{exact:true})).toBeVisible();
    await expect(page.getByRole('button',{name:'Confirm transfer paid',exact:true})).toHaveCount(0);
  });
}
test("daily reconciliation is usable on a phone and links to the review queue", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("http://127.0.0.1:18087/admin/finance-reconciliation");
  await expect(page.getByRole("heading", { name: "Daily reconciliation" })).toBeVisible();
  await expect(page.getByLabel("Report date (IST)")).toHaveValue("2026-11-16");
  await page.getByLabel("Report date (IST)").focus();
  const showReport = page.getByRole("button", { name: "Show report" });
  for (let step = 0; step < 5 && !(await showReport.evaluate(el => el === document.activeElement)); step++) {
    await page.keyboard.press("Tab");
  }
  await expect(showReport).toBeFocused();
  await expect(page.getByRole("link", { name: "WC-FIXTURE" })).toHaveAttribute("href", "/admin/payment-reviews");
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(392);
});
