const {test, expect} = require('@playwright/test');
const path = require('node:path');
const os = require('node:os');

// Run with customer-layout.config.js. This fixture renders
// production templates/assets; API stubs below never write marketplace data.
const baseURL = 'http://127.0.0.1:18086';
test.use({baseURL});

async function ready(page, route) {
  let quantity = 1;
  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url());
    let data = [];
    if (url.pathname.endsWith('/auth/session')) data = {user_id:42};
    if (url.pathname.endsWith('/cart')) data = {id:1, items:quantity ? [{product_slug:'layout-mug',variant_id:101,quantity,unit_price_cents:9999900,available:10,variant_name:'Standard'}] : []};
    if (url.pathname.endsWith('/products/layout-mug')) data = {id:1,slug:'layout-mug',name:'Hand painted ceramic coffee mug with traditional floral patterns',seller:{name:'Independent Indian Ceramics Workshop'},variants:[{id:101,price_cents:9999900,inventory_quantity:10}],image_url:'/assets/images/customer/ceramic-mugs.webp'};
    if (url.pathname.endsWith('/checkout/quote')) {
      if (url.searchParams.get('coupon_code') === 'INVALID') return route.fulfill({status:422,json:{error:{message:'Coupon is not available.'}}});
      data = {subtotal_cents:9999900,shipping_cents:9900,discount_cents:10000,total_cents:9999800,policy_revision:1};
    }
    await route.fulfill({json:{data}});
  });
  await page.route('**/ui/**', async route => {
    if (!/\/ui\/(cart|wishlist)\//.test(new URL(route.request().url()).pathname)) return route.fulfill({json:{data:[]}});
    const values = new URLSearchParams(route.request().postData());
    expect(values.get('variant_id')).toBe('101');
    quantity = values.has('remove') || values.get('source') === 'cart' ? 0 : quantity + Number(values.get('delta'));
    await page.context().addCookies([{name:'layout_quantity',value:String(quantity),url:baseURL}]);
    await route.fulfill({status:200,body:''});
  });
  await page.goto(route);
  await page.waitForFunction(() => window.weevcraftsBackend && window.WVUI);
  await page.evaluate(() => document.fonts.ready);
}

for (const width of [320,390,720,1440]) {
  test(`cart and checkout align at ${width}px with long names and large totals`, async ({page}) => {
    await page.setViewportSize({width,height:1000});
    await ready(page, '/cart');
    await expect(page.locator('.cart-line-actions > .text-button')).toHaveCount(2);
    if (width <= 720) {
      const seller = await page.locator('.cart-seller-head > div').boundingBox();
      const delivery = await page.locator('.cart-seller-head .delivery').boundingBox();
      expect(delivery.y).toBeGreaterThanOrEqual(seller.y + seller.height);
      const quantity = await page.locator('.cart-line-actions .quantity-control').boundingBox();
      const remove = await page.locator('.cart-line-actions > .text-button').first().boundingBox();
      expect(remove.y).toBeGreaterThanOrEqual(quantity.y + quantity.height);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({path:path.join(os.tmpdir(),`weev-customer-cart-${width}.png`),fullPage:true});
    await page.goto('/checkout');
    const coupon = page.locator('#checkout-coupon-code');
    const input = await coupon.boundingBox();
    const button = await page.locator('[data-preview-quote]').boundingBox();
    expect(button.y).toBeGreaterThanOrEqual(input.y + input.height);
    for (const item of await page.locator('.summary-items small').all()) await expect(item).toHaveCSS('text-align','left');
    await page.getByRole('button',{name:'Use account details',exact:true}).press('Enter');
    await expect(page.locator('[data-checkout-prefill-status]')).toHaveText('Account details added. Add your delivery address to continue.');
    await expect(page.locator('[name=line1]')).toBeFocused();
    await expect(page.locator('#ui-toast')).toBeEmpty();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({path:path.join(os.tmpdir(),`weev-customer-checkout-${width}.png`),fullPage:true});
    await coupon.fill('VALID');
    await page.locator('[data-preview-quote]').click();
    await expect(page.locator('.checkout-summary [data-quote-discount]')).toHaveText('INR 100.00');
    await coupon.fill('INVALID');
    await page.locator('[data-preview-quote]').click();
    await expect(page.locator('#ui-toast')).toContainText('Coupon is not available');
    for (const [name,value] of Object.entries({line1:'12 Craft Lane',city:'Pune',state:'Maharashtra',postal_code:'411001'})) await page.locator(`[name="${name}"]`).fill(value);
    await page.getByRole('button',{name:'Continue to delivery',exact:true}).click();
    await expect(page.locator('.checkout-card[data-step="2"]')).toBeVisible();
    await page.getByRole('button',{name:'Back to address',exact:true}).click();
    await expect(page.locator('[name=line1]')).toHaveValue('12 Craft Lane');
  });
}

test('cart controls retain variant selection after reflow and refresh', async ({page}) => {
  await page.setViewportSize({width:390,height:844});
  await ready(page, '/cart');
  await page.getByRole('button',{name:/Increase .* quantity/}).click();
  await expect(page.locator('.cart-layout output')).toHaveText('2');
  await page.getByRole('button',{name:/Decrease .* quantity/}).click();
  await expect(page.locator('.cart-layout output')).toHaveText('1');
  await page.locator('.cart-layout').getByRole('button',{name:'Move to wishlist'}).click();
  await expect(page.locator('.cart-layout .empty-state')).toBeVisible();
  await page.context().clearCookies();
  await page.reload();
  await page.locator('.cart-layout').getByRole('button',{name:'Remove',exact:true}).click();
  await expect(page.locator('.cart-layout .empty-state')).toBeVisible();
});
