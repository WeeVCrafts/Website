const { expect, test } = require('@playwright/test');

const productPath = '/products/layout-gallery';

test.use({ hasTouch: true, reducedMotion: 'reduce' });
test.describe.configure({ timeout: 120_000 });

async function expectPageReflow(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
}

async function swipeLeft(session, x, y) {
  await session.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] });
  for (const distance of [25, 65, 105, 145]) {
    await session.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: x - distance, y }] });
  }
  await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

test('responsive storefront gallery uses a bounded rail and keeps the single-photo state', async ({ page }) => {
  await page.goto(productPath, { waitUntil: 'domcontentloaded' });
  const rail = page.locator('.gallery > .thumbs');
  const buttons = rail.locator('[data-gallery-select]');
  await expect(buttons).toHaveCount(11);
  for (const width of [320, 390, 720, 721, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(rail).toHaveCSS('flex-direction', width <= 720 ? 'row' : 'column');
    const first = await buttons.first().boundingBox();
    const second = await buttons.nth(1).boundingBox();
    if (width <= 720) {
      expect(Math.abs(first.y - second.y)).toBeLessThan(1);
      expect(second.x).toBeGreaterThan(first.x);
      expect(first.width).toBeGreaterThanOrEqual(44);
      expect(first.height).toBeGreaterThanOrEqual(44);
      expect(await rail.evaluate(el => el.scrollWidth > el.clientWidth)).toBe(true);
    } else {
      expect(second.y).toBeGreaterThan(first.y);
      expect(Math.abs(first.x - second.x)).toBeLessThan(1);
    }
    await expectPageReflow(page);
  }
  await page.setViewportSize({ width: 320, height: 844 });
  await page.goto('/products/layout-mug', { waitUntil: 'domcontentloaded' });
  await expect(page.locator('[data-gallery-select]')).toHaveCount(1);
  await expect(page.locator('[data-gallery-controls]')).toBeHidden();
  await expect(page.locator('[data-gallery-zoom]')).toBeEnabled();
});

test('responsive storefront gallery reveals selected media and uses photo-only Zoom', async ({ page, browserName }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(productPath, { waitUntil: 'domcontentloaded' });
  const rail = page.locator('.gallery > .thumbs');
  const first = rail.getByRole('button', { name: 'Show image 1 of 11', exact: true });
  const scale = rail.getByRole('button', { name: /show scale and dimensions guide/i });
  const zoom = page.locator('[data-gallery-zoom]');
  const status = page.locator('[data-gallery-status]');
  const purchaseBar = page.locator('.mobile-buy-bar');
  await expect(purchaseBar.getByRole('button', { name: 'Add to cart' })).toBeInViewport();
  await expect(purchaseBar.getByRole('button', { name: 'Buy now' })).toBeInViewport();

  if (browserName === 'chromium') {
    const session = await page.context().newCDPSession(page);
    const frame = page.locator('.gallery-main');
    await frame.scrollIntoViewIfNeeded();
    const frameBox = await frame.boundingBox();
    await swipeLeft(session, frameBox.x + frameBox.width - 52, frameBox.y + frameBox.height / 2);
    await expect(page.locator('[data-gallery-count]')).toHaveText('2 / 11');
    await rail.scrollIntoViewIfNeeded();
    const railBox = await rail.boundingBox();
    await swipeLeft(session, railBox.x + railBox.width - 52, railBox.y + railBox.height / 2);
    await expect.poll(() => rail.evaluate(el => el.scrollLeft)).toBeGreaterThan(10);
    await session.detach();
  }

  await first.press('End');
  await expect(scale).toBeFocused();
  await expect(scale).toBeInViewport({ ratio: 1 });
  await expect(scale).toHaveAttribute('aria-pressed', 'true');
  await expect(status).toHaveText('Showing scale and dimensions guide 11 of 11');
  await expect(zoom).toBeHidden();
  await expect.poll(() => rail.evaluate(el => el.scrollLeft)).toBeGreaterThan(10);

  await scale.press('ArrowLeft');
  const video = rail.getByRole('button', { name: /show video 10 of 11/i });
  await expect(video).toBeFocused();
  await expect(page.locator('video[data-gallery-media]')).toBeVisible();
  await expect(zoom).toBeHidden();
  await page.locator('[data-gallery-next]').tap();
  await expect(scale).toHaveAttribute('aria-pressed', 'true');

  await scale.press('Home');
  await expect(first).toBeFocused();
  await expect(first).toBeInViewport({ ratio: 1 });
  await expect.poll(() => rail.evaluate(el => el.scrollLeft)).toBeLessThanOrEqual(1);
  await expect(zoom).toBeEnabled();
  await zoom.press('Enter');
  await expect(page.locator('.media-lightbox img')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(zoom).toBeFocused();
  await expectPageReflow(page);
});

test('responsive storefront hero keeps the photograph clear and leaves controls usable', async ({ page }) => {
  await page.goto('/', { waitUntil: 'domcontentloaded' });
  const hero = page.locator('[data-hero]');
  const slide = hero.locator('[data-hero-slide]').first();
  const copy = slide.locator('.hero-copy');
  await expect(hero).toHaveAttribute('data-playing', 'false');
  for (const width of [320, 390, 599, 600, 800, 801, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(copy).toHaveCSS('background-image', 'none');
    const overlay = await slide.evaluate(el => {
      const layer = getComputedStyle(el, '::before');
      return { image: layer.backgroundImage, inset: layer.inset, pointerEvents: layer.pointerEvents };
    });
    if (width <= 599) {
      expect(overlay.image).toContain('linear-gradient(');
      expect(overlay.image).toContain('to right');
      expect(overlay.image).toMatch(/255, 253, 248.*90%/);
      expect(overlay.inset).toBe('0px');
      expect(overlay.pointerEvents).toBe('none');
      await expect(copy.locator('h1')).toHaveCSS('color', 'rgb(18, 63, 50)');
      await expect(copy.locator('h1 em')).toHaveCSS('color', 'rgb(157, 64, 49)');
      const slideBox = await slide.boundingBox();
      const controlsBox = await hero.locator('[data-hero-controls]').boundingBox();
      expect(controlsBox.y).toBeGreaterThanOrEqual(slideBox.y + slideBox.height - 1);
      for (const link of await copy.getByRole('link').all()) await link.click({ trial: true });
    } else {
      expect(overlay.image).toBe('none');
    }
    await expectPageReflow(page);
  }

  await page.setViewportSize({ width: 390, height: 844 });
  const selectors = hero.locator('[data-hero-select]');
  await expect(selectors).toHaveCount(5);
  for (let index = 0; index < 5; index++) {
    await selectors.nth(index).tap();
    await expect(selectors.nth(index)).toHaveAttribute('aria-pressed', 'true');
    const activeSlide = hero.locator('[data-hero-slide]').nth(index);
    await expect(activeSlide).toBeVisible();
    await expect(activeSlide.locator('.hero-actions a').first()).toBeInViewport();
    for (const link of await activeSlide.locator('.hero-actions a').all()) await link.click({ trial: true });
    expect(await activeSlide.evaluate(el => el.getAnimations().length)).toBe(0);
  }
  await hero.getByRole('button', { name: 'Next collection' }).press('Enter');
  await expect(selectors.first()).toHaveAttribute('aria-pressed', 'true');
  await page.route('**/products', route => route.fulfill({ body: 'Fixture catalog' }));
  await hero.getByRole('link', { name: 'Shop collections' }).tap();
  await expect(page).toHaveURL(/\/products$/);
});
