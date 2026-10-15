const {test,expect}=require('@playwright/test');

test('hero controls reserve space and secondary media waits for selection',async({page})=>{
  await page.setViewportSize({width:390,height:844});
  let release;
  const scriptsReady=new Promise(resolve=>{release=resolve;});
  await page.route('**/assets/js/*.js*',async route=>{await scriptsReady;await route.continue();});
  const images=[];
  page.on('request',request=>{if(request.resourceType()==='image')images.push(request.url());});
  try {
    await page.goto('/',{waitUntil:'commit'});
    const controls=page.locator('[data-hero-controls]');
    await controls.waitFor({state:'attached'});
    await page.evaluate(()=>document.fonts.ready);
    const before=await controls.boundingBox();
    expect(before.height).toBeGreaterThanOrEqual(64);
    expect(await controls.isVisible()).toBe(false);
    expect(await page.locator('[data-hero-slide] img').nth(1).getAttribute('src')).toBeNull();
    expect(await page.locator('[data-hero-slide] img').nth(1).getAttribute('srcset')).toBeNull();
    release();
    await expect(controls).toBeVisible();
    const after=await controls.boundingBox();
    expect(after.height).toBe(before.height);
    expect(Math.abs(after.y-before.y)).toBeLessThan(1);
    expect(images.some(url=>new URL(url).pathname.includes('textiles-banner'))).toBe(false);
    await page.emulateMedia({reducedMotion:'reduce'});
    await page.locator('[data-hero-next]').press('Enter');
    await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
    const second=page.locator('[data-hero-slide] img').nth(1);
    await expect.poll(()=>second.evaluate(image=>image.complete&&image.naturalWidth>0&&image.currentSrc)).toBeTruthy();
    expect(await second.evaluate(image=>new URL(image.currentSrc).pathname)).toMatch(/textiles-banner-(320|640|960)\.webp$/);
    expect(await page.locator('[data-hero-slide]').nth(1).evaluate(slide=>slide.getAnimations().length)).toBe(0);
  } finally {release();}
});

test('delayed product media keeps its frame at supported viewport widths',async({page})=>{
  for(const width of [320,390,768,1024,1440]) {
    await page.setViewportSize({width,height:900});
    let release;
    const imageReady=new Promise(resolve=>{release=resolve;});
    await page.route('**/assets/images/customer/**',async route=>{await imageReady;await route.continue();});
    try {
      await page.goto('/products/blue-tote',{waitUntil:'domcontentloaded'});
      await page.evaluate(()=>document.fonts.ready);
      const frame=page.locator('.gallery-main');
      const before=await frame.boundingBox();
      const image=page.locator('[data-gallery-media]').first();
      await expect(image).toHaveAttribute('fetchpriority','high');
      await expect(image).toHaveAttribute('loading','eager');
      release();
      await expect.poll(()=>image.evaluate(image=>image.complete&&image.naturalWidth>0)).toBe(true);
      const after=await frame.boundingBox();
      expect(Math.abs(after.height-before.height)).toBeLessThan(1);
      expect(Math.abs(after.width-after.height)).toBeLessThan(1);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2)).toBe(false);
      if(width===1440) {
        expect(await image.evaluate(image=>new URL(image.currentSrc).pathname)).toContain('-640.webp');
        await page.locator('[data-gallery-zoom]').press('Enter');
        await expect(page.locator('.media-lightbox img')).toHaveAttribute('src',await image.evaluate(image=>image.src));
        await page.keyboard.press('Escape');
        await expect(page.locator('[data-gallery-zoom]')).toBeFocused();
      }
    } finally {release();await page.unroute('**/assets/images/customer/**');}
  }
});

test('filter fragment preserves surrounding content, keyboard focus and native history',async({page})=>{
  await page.setViewportSize({width:390,height:844});
  await page.goto('/products');
  await page.locator('[data-filter-open]').press('Enter');
  await page.locator('#facet-category').fill('art');
  await page.locator('#facet-category').press('Tab');
  await page.evaluate(()=>{
    window.preservedBanner=document.querySelector('.domestic-delivery-banner');
    window.preservedHeader=document.querySelector('.site-header');
  });
  const checkbox=page.locator('[data-filter-panel] input[name="category"][value="arts"]');
  const responsePromise=page.waitForResponse(response=>new URL(response.url()).searchParams.get('category')==='arts');
  await checkbox.check();
  const response=await responsePromise;
  const html=await response.text();
  expect(response.request().headers()['hx-target']).toBe('listing-content');
  expect(html).toContain('id="listing-content"');
  expect(html).not.toContain('<script');
  expect(html).not.toContain('class="site-header');
  await expect(page).toHaveURL(/category=arts/);
  await expect(page.locator('[data-filter-panel]')).toHaveClass(/is-open/);
  await expect(page.locator('#facet-category')).toHaveValue('art');
  await expect(checkbox).toBeFocused();
  expect(await page.evaluate(()=>window.preservedBanner===document.querySelector('.domestic-delivery-banner')&&window.preservedHeader===document.querySelector('.site-header'))).toBe(true);
  await page.keyboard.press('Escape');
  await expect(page.locator('[data-filter-open]')).toBeFocused();
  await page.goBack();
  await expect(page).toHaveURL(/\/products$/);
  await expect(page.locator('#listing-filters')).toBeVisible();
  await page.goForward();
  await expect(page).toHaveURL(/category=arts/);
  await expect(page.locator('[data-filter-panel] input[name="category"][value="arts"]')).toBeChecked();
});
