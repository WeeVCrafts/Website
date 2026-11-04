const {test, expect} = require('@playwright/test');
const path = require('node:path');
const os = require('node:os');

test('preferred brand, homepage introduction and metadata remain consistent', async ({page, baseURL}, testInfo) => {
  if (baseURL !== 'http://127.0.0.1:18088') throw Error('Loopback QA only');
  for (const width of [320, 390, 768, 1024, 1536]) {
    await page.setViewportSize({width, height: 1024});
    await page.goto('/');
    await expect(page).toHaveTitle(/^Weevcrafts \|/);
    await expect(page.locator('meta[property="og:site_name"]')).toHaveAttribute('content', 'Weevcrafts');
    await expect(page.locator('#homepage-introduction-title')).toHaveText('Discover Weevcrafts — Handmade Crafts from India');
    await expect(page.locator('.homepage-introduction')).toHaveCount(1);
    const positions = await page.evaluate(() => {
      const hero = document.querySelector('[data-hero]');
      const intro = document.querySelector('.homepage-introduction');
      const categories = document.querySelector('[data-category-rail]');
      return {hero: hero.getBoundingClientRect().toJSON(), intro: intro.getBoundingClientRect().toJSON(), categories: categories.getBoundingClientRect().toJSON(), overflow: document.documentElement.scrollWidth > innerWidth + 1};
    });
    expect(positions.overflow).toBe(false);
    if (width <= 768) {
      const firstCard = page.locator('.home-category-editorial > .product-card').first();
      if (await firstCard.count()) await expect(firstCard).toHaveCSS('display', 'flex');
    }
    expect(positions.intro.y).toBeGreaterThanOrEqual(positions.hero.bottom - 1);
    expect(positions.categories.y).toBeGreaterThan(positions.intro.y);
    if (width >= 1024) expect(positions.hero.height).toBe(420);
    const schema = await page.locator('script[type="application/ld+json"]').allTextContents();
    const nodes = schema.flatMap(text => { const v = JSON.parse(text); return v['@graph'] || [v]; });
    for (const type of ['Organization', 'WebSite']) {
      expect(nodes.some(n => n['@type'] === type && n.name === 'Weevcrafts')).toBe(true);
    }
    expect(await page.locator('body').innerText()).not.toMatch(/WeevCrafts|WeeVCrafts|WeeVcrafts/);
    if (width === 390 || width === 1536) await page.screenshot({path: path.join(os.tmpdir(), `weev-brand-${testInfo.project.name}-${width}.png`), fullPage: true});
  }
  for (const route of ['/our-story', '/about', '/shipping', '/login', '/products/blue-tote']) {
    await page.goto(route);
    expect(await page.title()).toContain('Weevcrafts');
    expect(await page.locator('body').innerText()).not.toMatch(/WeevCrafts|WeeVCrafts|WeeVcrafts/);
  }
});
