const { test, expect } = require('@playwright/test');
const fs = require('node:fs');

const fixtureFile = process.env.SELLER_SHIPPING_UI_FIXTURE;
test.skip(!fixtureFile, 'Requires the disposable seller catalog fixture.');

test('seller finds and saves packed measurements on the product editor', async ({ page, baseURL }, testInfo) => {
  expect(new URL(baseURL).hostname).toBe('127.0.0.1');
  const fixture = JSON.parse(fs.readFileSync(fixtureFile, 'utf8'));
  await page.context().addCookies([{ name: 'wecratfs_session', value: fixture.Roles.seller.Token, url: baseURL, httpOnly: true }]);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/seller-admin/products');
  await page.getByRole('link', { name: 'Fixture Product', exact: true }).click();
  const productID = new URL(page.url()).pathname.split('/').at(-2);

  const shippingLink = page.getByRole('link', { name: 'Edit shipping measurements', exact: true }).first();
  await expect(shippingLink).toBeVisible();
  await shippingLink.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/#product-variants$/);
  const form = page.locator('form.product-variant-edit').filter({ has: page.getByRole('button', { name: 'Save variant', exact: true }) }).first();
  await expect(form.getByRole('group', { name: 'Shipping measurements' })).toBeVisible();
  const measurements = [['Packed weight (grams)', '740'], ['Packed length (cm)', '31'], ['Packed width (cm)', '18'], ['Packed height (cm)', '6']];
  const saveButton = form.getByRole('button', { name: 'Save variant', exact: true });
  const variantResponse = () => page.waitForResponse(response => response.url().endsWith('/seller-admin/products/variants/update') && response.request().method() === 'POST');
  for (const [label] of measurements) {
    await form.getByRole('spinbutton', { name: label, exact: true }).fill('');
  }
  await form.getByRole('spinbutton', { name: 'Packed weight (grams)', exact: true }).fill('740');
  const invalid = variantResponse();
  await saveButton.click();
  expect((await invalid).status()).toBe(422);
  await expect(form.getByRole('alert')).toContainText('all three dimensions');
  await expect(form.getByRole('spinbutton', { name: 'Packed weight (grams)', exact: true })).toHaveValue('740');
  for (const [label, value] of measurements) {
    await form.getByRole('spinbutton', { name: label, exact: true }).fill(value);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await form.getByRole('group', { name: 'Shipping measurements' }).screenshot({ path: testInfo.outputPath('packed-measurements-mobile.png') });
  const save = variantResponse();
  await saveButton.click();
  expect((await save).status()).toBe(303);
  await page.waitForURL(/notice=Product\+variant\+saved/);

  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.reload();
  await expect(form.getByRole('spinbutton', { name: 'Packed weight (grams)', exact: true })).toHaveValue('740');
  await expect(form.getByRole('spinbutton', { name: 'Packed length (cm)', exact: true })).toHaveValue('31');
  await expect(form.getByRole('spinbutton', { name: 'Packed width (cm)', exact: true })).toHaveValue('18');
  await expect(form.getByRole('spinbutton', { name: 'Packed height (cm)', exact: true })).toHaveValue('6');
  await expect(form.locator('input[name="cost_price_cents"]')).toHaveValue('731');
  await expect(page.locator('input[name="dimensions"]')).not.toHaveValue('31×18×6');

  await page.context().addCookies([{ name: 'wecratfs_session', value: fixture.Roles.super_admin.Token, url: baseURL, httpOnly: true }]);
  await page.goto(`/super-admin/products/${productID}/edit`);
  await page.getByRole('link', { name: 'Edit shipping measurements', exact: true }).click();
  await expect(page).toHaveURL(/#product-variants$/);
  await expect(form.getByRole('spinbutton', { name: 'Packed weight (grams)', exact: true })).toHaveValue('740');
});
