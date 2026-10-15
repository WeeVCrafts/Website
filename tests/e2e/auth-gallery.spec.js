const { test, expect } = require('@playwright/test');

// This suite creates sessions only against the dedicated UI QA application.
test.use({ trace: 'off', screenshot: 'off' });
test.beforeEach(async ({ baseURL }) => {
  test.skip(baseURL !== 'http://127.0.0.1:18080', 'Requires isolated UI QA services.');
});

async function openPrompt(page) {
  await page.goto('/products');
  await page.locator('#cart-count').click();
  const dialog = page.locator('#guest-login-dialog');
  await expect(dialog).toBeVisible();
  return dialog;
}

test('inline password sign-in preserves its destination and never performs the shopping action', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const dialog = await openPrompt(page);
  let shoppingWrites = 0;
  page.on('request', request => {
    if (request.method() === 'POST' && /\/api\/v1\/(cart|wishlist)/.test(request.url())) shoppingWrites++;
  });
  const form = dialog.locator('[data-backend-auth=login]');
  await form.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(dialog.getByLabel('Username or email', { exact: true })).toBeFocused();
  await dialog.getByLabel('Username or email', { exact: true }).fill('demo.customer01@weevcrafts.local');
  await dialog.getByLabel('Password', { exact: true }).fill('WeeVCrafts-Demo-2026!');
  await form.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).toHaveURL(/\/cart$/);
  await expect(page.locator('form[action="/logout"]').first()).toBeAttached();
  expect(shoppingWrites).toBe(0);
});

test('centred popup traps focus, resets dismissed secrets, and scrolls on short screens', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const dialog = await openPrompt(page);
  await expect(dialog.getByRole('button', { name: 'Close sign-in prompt' })).toBeFocused();
  expect(await dialog.locator('.gallery-credentials [type="submit"]').evaluate(element => parseFloat(getComputedStyle(element).fontSize))).toBeGreaterThanOrEqual(16);
  expect(await dialog.locator('.auth-field > label').first().evaluate(element => parseFloat(getComputedStyle(element).fontSize))).toBeGreaterThanOrEqual(14);
  const box = await dialog.boundingBox();
  expect(box.x).toBeGreaterThan(10);
  expect(box.y).toBeGreaterThan(10);
  expect(Math.abs(box.y + box.height / 2 - 422)).toBeLessThan(2);
  await page.keyboard.press('Shift+Tab');
  await expect(dialog.getByRole('button', { name: 'Keep browsing' })).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(dialog.getByRole('button', { name: 'Close sign-in prompt' })).toBeFocused();
  await dialog.getByLabel('Username or email', { exact: true }).fill('dismissed@example.test');
  await dialog.getByLabel('Password', { exact: true }).fill('Do-Not-Retain-This!');
  await dialog.getByRole('button', { name: 'Show password', exact: true }).click();
  await page.keyboard.press('Escape');
  await expect(page.locator('#cart-count')).toBeFocused();
  await page.locator('#cart-count').click();
  await expect(dialog.getByLabel('Password', { exact: true })).toHaveValue('');
  await expect(dialog.getByLabel('Password', { exact: true })).toHaveAttribute('type', 'password');
  await expect(dialog.getByLabel('Username or email', { exact: true })).toHaveValue('');
  for (const [width, height] of [[320, 568], [390, 400], [768, 600]]) {
    await page.setViewportSize({ width, height });
    const metrics = await dialog.evaluate(element => ({ width: element.clientWidth, scroll: element.scrollWidth, bottom: element.getBoundingClientRect().bottom }));
    expect(metrics.scroll).toBeLessThanOrEqual(metrics.width + 1);
    expect(metrics.bottom).toBeLessThanOrEqual(height);
    await dialog.getByLabel('Password', { exact: true }).focus();
    await expect(dialog.getByLabel('Password', { exact: true })).toBeInViewport();
    await dialog.getByRole('button', { name: 'Keep browsing' }).focus();
    await expect(dialog.getByRole('button', { name: 'Keep browsing' })).toBeInViewport();
  }
  await dialog.getByRole('button', { name: 'Keep browsing' }).click();
  await page.locator('#cart-count').click();
  await page.mouse.click(2, 2);
  await expect(dialog).not.toBeVisible();
  await expect(page.locator('#cart-count')).toBeFocused();
});

async function mockGoogle(page, options = {}) {
  const exchanges = [];
  let sdkRequests = 0;
  await page.route('**/cart', route => route.request().method() === 'GET' ? route.fulfill({ contentType: 'text/html', body: '<!doctype html><title>Isolated return destination</title><main>QA cart destination</main>' }) : route.continue());
  await page.route('**/api/v1/firebase/config', route => route.fulfill({ json: { enabled: true, config: { apiKey: 'qa-public-key', projectId: 'isolated-qa', authDomain: 'isolated-qa.invalid' } } }));
  await page.route('https://www.gstatic.com/firebasejs/13.0.0/firebase-app.js', route => {
    sdkRequests++;
    return route.fulfill({ contentType: 'text/javascript', body: 'export const initializeApp=()=>({});' });
  });
  await page.route('https://www.gstatic.com/firebasejs/13.0.0/firebase-auth.js', route => route.fulfill({
    contentType: 'text/javascript',
    body: `export const getAuth=()=>({}); export const inMemoryPersistence={}; export const setPersistence=async()=>{};
      export class GoogleAuthProvider {setCustomParameters(){}}
      export const signInWithPopup=async()=>({user:{emailVerified:true,getIdToken:async()=>"isolated-qa-token"}});`
  }));
  await page.route('**/api/v1/auth/firebase/session', async route => {
    const body = route.request().postDataJSON();
    exchanges.push(body);
    if (options.link && !body.link_password) return route.fulfill({ status: 409, json: { error: { code: 'LINK_REQUIRED', message: 'Confirm your existing password.' } } });
    return route.fulfill({ json: { redirect_to: body.next || '/products', csrf_token: 'qa-only-csrf' } });
  });
  return { exchanges, sdkRequests: () => sdkRequests };
}

test('Google consent and return path work in page and popup without a live provider account', async ({ page }) => {
  const mock = await mockGoogle(page);
  await page.goto('/products');
  expect(mock.sdkRequests()).toBe(0);
  await page.locator('#cart-count').click();
  const dialog = page.locator('#guest-login-dialog');
  const google = dialog.locator('[data-google-signin]');
  await expect(google).toBeEnabled();
  await google.click();
  await expect(dialog.locator('[data-firebase-status]').first()).toContainText('Accept the privacy notice');
  await expect(dialog.locator('[data-google-terms]')).toBeFocused();
  expect(mock.exchanges).toHaveLength(0);
  // Consent is fictional fixture state; all identity exchange requests are intercepted.
  await dialog.locator('[data-google-terms]').check();
  await google.click();
  await expect(page).toHaveURL(/\/cart$/);
  expect(mock.exchanges[0].next).toBe('/cart');
  expect(mock.exchanges[0].accepted_terms).toBe(true);
  await page.goto('/login?next=%2Fproducts%3Fcategory%3Dcrafts');
  await expect(page.locator('[data-google-signin]')).toBeEnabled();
  await page.locator('[data-google-terms]').check();
  await page.locator('[data-google-signin]').click();
  await expect(page).toHaveURL(/\/products\?category=crafts$/);
  expect(mock.exchanges.at(-1).next).toBe('/products?category=crafts');
});

test('Google linking proof stays in the active popup and is cleared on dismissal', async ({ page }) => {
  const mock = await mockGoogle(page, { link: true });
  const dialog = await openPrompt(page);
  await expect(dialog.locator('[data-google-signin]')).toBeEnabled();
  await dialog.locator('[data-google-terms]').check();
  await dialog.locator('[data-google-signin]').click();
  const link = dialog.locator('[data-firebase-link]');
  await expect(link).toBeVisible();
  await expect(link.getByLabel('Current WeeVCrafts password', { exact: true })).toBeFocused();
  await link.getByLabel('Current WeeVCrafts password', { exact: true }).fill('qa-existing-proof');
  await page.keyboard.press('Escape');
  await page.locator('#cart-count').click();
  await expect(link).toBeHidden();
  await expect(dialog.locator('[data-google-terms]')).not.toBeChecked();
  await dialog.locator('[data-google-terms]').check();
  await dialog.locator('[data-google-signin]').click();
  await link.getByLabel('Current WeeVCrafts password', { exact: true }).fill('qa-existing-proof');
  await link.getByRole('button', { name: 'Connect account securely' }).click();
  await expect(page).toHaveURL(/\/cart$/);
  expect(mock.exchanges.at(-1).link_password).toBe('qa-existing-proof');
  expect(mock.exchanges.at(-1).next).toBe('/cart');
});
