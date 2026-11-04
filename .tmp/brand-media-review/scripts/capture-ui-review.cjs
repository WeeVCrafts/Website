const { chromium, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

// Local demo capture only: no catalogue, order or payment mutations.
async function main() {
  const output = path.resolve(process.argv[2] || 'tmp/ui-review');
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL: 'http://127.0.0.1:8080' });
  const page = await context.newPage();
  const measurements = [];
  try {
    await page.goto('/', { waitUntil: 'networkidle' });
    for (const width of [320, 390, 601, 700, 701, 720, 721, 768, 960, 1536]) {
      await page.setViewportSize({ width, height: 1024 });
      await page.evaluate(() => document.fonts.ready);
      const layout = await page.evaluate(() => ({
        width: innerWidth,
        header: document.querySelector('.site-header').getBoundingClientRect().height,
        productTop: document.querySelector('.product-grid').getBoundingClientRect().top,
        overflow: document.documentElement.scrollWidth > innerWidth + 2,
        clipped: [...document.querySelectorAll('.home-rail .category-card')].some(card => {
          const action = card.querySelector('b');
          return action.getClientRects().length && action.getBoundingClientRect().bottom > card.getBoundingClientRect().bottom + 1;
        }),
      }));
      measurements.push(layout);
      expect(layout.overflow || layout.clipped, `Homepage at ${width}px`).toBe(false);
      expect(layout.productTop, `Product discovery at ${width}px`).toBeLessThan(1024);
    }
    await page.screenshot({ path: path.join(output, 'home-desktop-final.png') });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: path.join(output, 'home-mobile-final.png') });
    await page.goto('/login');
    await page.locator('#page input[name="email"]').fill('demo.owner01@weevcrafts.local');
    await page.locator('#page input[name="password"]').fill(process.env.WEEVCRAFTS_ROLE_TEST_PASSWORD || 'WeeVCrafts-Demo-2026!');
    await page.getByRole('button', { name: /^Sign in$/i }).click();
    await page.waitForURL(url => url.pathname === '/seller-admin');
    await page.setViewportSize({ width: 1505, height: 1045 });
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({ path: path.join(output, 'seller-desktop-final.png') });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/seller-admin/products/new', { waitUntil: 'networkidle' });
    const price = page.getByRole('spinbutton', { name: 'Unit cost (INR, optional)', exact: true });
    await price.scrollIntoViewIfNeeded();
    await price.focus();
    await expect(price).toHaveCSS('font-size', '16px');
    await page.screenshot({ path: path.join(output, 'editor-mobile-final.png') });
    await fs.writeFile(path.join(output, 'final-layout-measurements.json'), JSON.stringify(measurements, null, 2));
    console.log(`Saved four native viewport screenshots to ${output}`);
    console.log(JSON.stringify(measurements));
  } finally {
    await browser.close();
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
