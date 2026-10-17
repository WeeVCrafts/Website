const {test, expect, request} = require('@playwright/test');
const path = require('node:path');
const os = require('node:os');

const base = process.env.WEEVCRAFTS_BASE_URL || 'http://127.0.0.1:18080';
test.use({baseURL:base,viewport:{width:390,height:844}});
let session;

test.beforeAll(async () => {
  if (new URL(base).hostname !== '127.0.0.1' || new URL(base).port !== '18080') throw new Error('Shopping regression tests require the isolated local QA app on port 18080.');
  const api = await request.newContext({baseURL:base});
  const response = await api.post('/api/v1/auth/login', {data:{email:'demo.customer08@weevcrafts.local',password:'WeeVCrafts-Demo-2026!'}});
  expect(response.status()).toBe(200);
  session = await api.storageState();
  await api.dispose();
});

async function ready(page, authenticated = true, route = '/products') {
  if (authenticated) await page.context().addCookies(session.cookies);
  await page.emulateMedia({reducedMotion:'reduce'});
  await page.goto(route);
  await page.waitForFunction(() => window.weevcraftsBackend);
  await page.evaluate(() => document.fonts.ready);
}

async function firstHeart(page) {
  const heart = page.locator('.product-card .heart').first();
  await expect(heart).toBeVisible();
  return {heart, slug:await heart.getAttribute('data-product-slug')};
}

test('hero dots and swipe work without arrows; category controls follow overflow and keyboard', async ({page, browserName}) => {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await ready(page, false, '/');
  for (const width of [390,768,1440]) {
    await page.setViewportSize({width,height:900});
    await expect(page.locator('[data-hero-prev],[data-hero-next]')).toHaveCount(0);
    const rail = page.locator('[data-category-track]').first();
    const next = page.getByRole('button',{name:'Next categories',exact:true}).first();
    const previous = page.getByRole('button',{name:'Previous categories',exact:true}).first();
    const overflow = await rail.evaluate(el => el.scrollWidth > el.parentElement.clientWidth + 2);
    if (overflow) {
      await expect(next).toBeVisible();
      await rail.evaluate(el => el.scrollLeft = 0);
      await expect(previous).toBeDisabled();
      await next.press('Enter');
      await expect.poll(() => rail.evaluate(el => el.scrollLeft)).toBeGreaterThan(0);
      await rail.evaluate(el => el.scrollLeft = el.scrollWidth);
      await expect(next).toBeDisabled();
      await expect(previous).toBeEnabled();
    } else {
      await expect(next).toBeHidden();
      await expect(previous).toBeHidden();
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)).toBe(false);
  }
  const track = page.locator('[data-category-track]').first();
  await track.evaluate(el => {for(let i=0;i<8;i++){const item=el.firstElementChild.cloneNode(true);item.dataset.qaExtra='true';el.append(item);}});
  await expect(page.getByRole('button',{name:'Next categories',exact:true}).first()).toBeVisible();
  await track.evaluate(el => el.querySelectorAll('[data-qa-extra]').forEach(item => item.remove()));
  await expect(page.getByRole('button',{name:'Next categories',exact:true}).first()).toBeHidden();
  await page.locator('[data-hero-select="1"]').press('Enter');
  await expect(page.locator('[data-hero-count]')).toHaveText('2 / 5');
  await expect(page.locator('[data-hero-slide]:not([hidden])')).toHaveCount(1);
  const hero = page.locator('[data-hero]');
  await hero.dispatchEvent('touchstart',{touches:[{identifier:1,clientX:280,clientY:250}]});
  await hero.dispatchEvent('touchend',{changedTouches:[{identifier:1,clientX:100,clientY:255}]});
  await expect(page.locator('[data-hero-count]')).toHaveText('3 / 5');
  await expect(hero).toHaveAttribute('data-playing','false');
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:path.join(os.tmpdir(),`weev-shopping-mobile-${browserName}.png`)});
  expect(errors).toEqual([]);
});

test('count failures are quiet, independent, bounded and do not disable shopping', async ({page}) => {
  let cartReads = 0, wishlistReads = 0, productReads = 0;
  await page.route('**/api/v1/cart', route => {cartReads++;return route.fulfill({status:503,json:{error:{message:'Temporary cart outage'}}});});
  await page.route('**/api/v1/wishlist', route => {wishlistReads++;return route.fulfill({json:{data:[]}});});
  page.on('request', req => {if (new URL(req.url()).pathname.startsWith('/api/v1/products/')) productReads++;});
  await ready(page);
  const {heart} = await firstHeart(page);
  const initialCount = await page.locator('#cart-count em').textContent();
  await expect.poll(() => cartReads).toBe(2);
  await expect(page.locator('#cart-count em')).toHaveText(initialCount);
  await expect(page.locator('#wishlist-count em')).toHaveText('0');
  expect(wishlistReads).toBe(1);
  expect(productReads).toBe(0);
  expect(await page.evaluate(() => !!window.weevcraftsBackendError)).toBe(false);
  await expect(page.locator('#ui-toast')).not.toContainText('could not be refreshed');
  await expect(heart).toBeEnabled();
  await page.route('**/api/v1/wishlist/add', route => route.fulfill({status:201,json:{status:'added'}}));
  await page.route('**/api/v1/wishlist', route => route.fulfill({status:503,json:{error:{message:'Count unavailable'}}}));
  await heart.click();
  await expect(heart).toHaveAttribute('aria-pressed','true');
  await expect(heart).toBeEnabled();
  await expect(page.locator('#ui-toast')).toContainText('Saved to your wishlist');
});

test('pending hearts stay visible, synchronize duplicates and reject rapid taps; failed writes preserve state', async ({page}) => {
  let saved = false, writes = 0, release;
  let slug = '';
  const gate = new Promise(resolve => release = resolve);
  await page.route('**/api/v1/wishlist', route => route.fulfill({json:{data:saved ? [{slug}] : []}}));
  await page.route('**/api/v1/wishlist/add', async route => {writes++;await gate;saved=true;await route.fulfill({status:201,json:{status:'added'}});});
  await ready(page);
  const result = await firstHeart(page);slug = result.slug;
  const heart = result.heart;
  await expect(heart).toHaveAttribute('aria-pressed','false');
  await heart.evaluate(button => {
    const article=button.closest('article');article.after(article.cloneNode(true));
    document.dispatchEvent(new CustomEvent('htmx:afterSettle'));
  });
  const peers = page.locator('.heart').filter({has:page.locator('svg')}).and(page.locator(`[data-product-slug="${slug}"]`));
  const before = await heart.boundingBox();
  try {
    await heart.evaluate(button => {button.click();button.click();button.click();});
    await expect.poll(() => writes).toBe(1);
    for (const peer of await peers.all()) {
      await expect(peer).toBeDisabled();
      await expect(peer).toHaveAttribute('aria-busy','true');
      await expect(peer.locator('svg')).toBeVisible();
      await expect(peer).toHaveAttribute('aria-pressed','false');
    }
    const pending = await heart.boundingBox();
    expect(pending.width).toBe(before.width);expect(pending.height).toBe(before.height);
  } finally {release();}
  for (const peer of await peers.all()) {
    await expect(peer).toHaveAttribute('aria-pressed','true');
    await expect(peer).toBeEnabled();
    expect(await peer.locator('svg').evaluate(el => getComputedStyle(el).fill)).not.toBe('none');
  }
  await page.route('**/api/v1/wishlist/*', route => route.request().method()==='DELETE'
    ? route.fulfill({status:503,json:{error:{message:'Please try removing this item again.'}}}) : route.continue());
  await heart.click();
  await expect(page.locator('#ui-toast')).toContainText('try removing');
  await expect(heart).toHaveAttribute('aria-pressed','true');
  await expect(heart).toBeEnabled();
  expect(writes).toBe(1);
  await page.unroute('**/api/v1/wishlist/*');
  await page.route('**/api/v1/wishlist/*', route => {
    if(route.request().method() !== 'DELETE') return route.continue();
    saved=false;return route.fulfill({json:{status:'removed'}});
  });
  await heart.click();await expect(heart).toHaveAttribute('aria-pressed','false');await expect(heart).toBeEnabled();
  await page.route('**/api/v1/wishlist/add', route => route.fulfill({status:503,json:{error:{message:'Save unavailable. Please try again.'}}}));
  await heart.click();await expect(page.locator('#ui-toast')).toContainText('Save unavailable');
  await expect(heart).toHaveAttribute('aria-pressed','false');await expect(heart).toBeEnabled();
});

test('a delayed count response cannot undo a confirmed save or a replacement card', async ({page}) => {
  let reads = 0, release, slug = '';
  const gate = new Promise(resolve => release = resolve);
  await page.route('**/api/v1/wishlist', async route => {
    if (++reads === 1) {await gate;await route.fulfill({json:{data:[]}});}
    else await route.fulfill({status:503,json:{error:{message:'Temporary count failure'}}});
  });
  await page.route('**/api/v1/wishlist/add', route => route.fulfill({status:201,json:{status:'added'}}));
  await ready(page);
  const result = await firstHeart(page);slug=result.slug;
  const heart = result.heart;
  try {
    await expect.poll(() => reads).toBe(1);
    // Choose an unsaved product even if this QA customer's server wishlist is populated.
    await heart.evaluate(button => {button.setAttribute('aria-pressed','false');button.dataset.backendAction='wishlist-add';});
    await heart.click();
    await expect(heart).toHaveAttribute('aria-pressed','true');
    await expect(heart).toBeEnabled();
  } finally {release();}
  await expect.poll(() => reads).toBeGreaterThanOrEqual(2);
  await heart.evaluate(button => {
    const card=button.closest('article');const replacement=card.cloneNode(true);
    const heart=replacement.querySelector('.heart');heart.classList.remove('is-active');heart.setAttribute('aria-pressed','false');
    card.replaceWith(replacement);document.dispatchEvent(new CustomEvent('htmx:afterSettle'));
  });
  await expect(page.locator(`.heart[data-product-slug="${slug}"]`).first()).toHaveAttribute('aria-pressed','true');
  await expect(page.locator('#ui-toast')).toContainText('Saved to your wishlist');
});

test('session expiry opens the existing sign-in prompt without changing the heart', async ({page}) => {
  await ready(page);
  const {heart} = await firstHeart(page);
  const previous = await heart.getAttribute('aria-pressed');
  await page.route('**/api/v1/auth/session', route => route.fulfill({status:401,json:{error:{message:'Sign in'}}}));
  await heart.click();
  await expect(page.locator('#guest-login-dialog')).toBeVisible();
  await expect(heart).toHaveAttribute('aria-pressed',previous);
  await page.locator('#guest-login-dialog [aria-label="Close sign-in prompt"]').click();
  await expect(heart).toBeEnabled();
});

test('Back/Forward restoration renews cancelled reads and reconciles saved state', async ({page}) => {
  let reads=0, slug='';
  await page.route('**/api/v1/wishlist', route => route.fulfill({json:{data:++reads > 1 ? [{slug}] : []}}));
  await ready(page);
  const result=await firstHeart(page);slug=result.slug;
  await expect.poll(() => reads).toBe(1);
  await expect(result.heart).toHaveAttribute('aria-pressed','false');
  await page.evaluate(() => {
    window.weevcraftsBackend.readController.abort();
    window.dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}));
  });
  await expect.poll(() => reads).toBe(2);
  await expect(result.heart).toHaveAttribute('aria-pressed','true');
  await expect(page.locator('#wishlist-count em')).toHaveText('1');
});

test('confirmed wishlist persistence, detail state, removal focus and empty state use the real API', async ({page}) => {
  const api = await request.newContext({baseURL:base});
  const registered = await api.post('/api/v1/auth/register',{data:{email:`wishlist.qa.${Date.now()}.${Math.random().toString(36).slice(2)}@example.invalid`,password:'Wishlist-QA-2026!',display_name:'Wishlist QA',phone:'+919876543210',accepted_terms:true}});
  expect(registered.status()).toBe(201);
  const state = await api.storageState();await page.context().addCookies(state.cookies);
  await ready(page, false);
  const {heart,slug} = await firstHeart(page);
  await heart.click();await expect(heart).toHaveAttribute('aria-pressed','true');await expect(heart).toBeEnabled();
  await page.reload();await page.waitForFunction(() => window.weevcraftsBackend);
  await expect(page.locator(`.heart[data-product-slug="${slug}"]`).first()).toHaveAttribute('aria-pressed','true');
  await page.goto(`/products/${slug}`);await page.waitForFunction(() => window.weevcraftsBackend);
  await expect(page.locator('.gallery-heart')).toHaveAttribute('aria-pressed','true');
  await page.goto('/wishlist');await page.waitForFunction(() => window.weevcraftsBackend);
  const remove = page.locator('#wishlist-grid .heart').first();
  await remove.focus();await remove.press('Enter');
  await expect(page.locator('#wishlist-grid .saved-card')).toHaveCount(0);
  await expect(page.locator('#wishlist-grid .empty-state a')).toBeFocused();
  await expect(page.locator('#wishlist-count em')).toHaveText('0');
  const wishlist = await api.get('/api/v1/wishlist');expect((await wishlist.json()).data).toEqual([]);
  await api.dispose();
});

test('category links remain usable without JavaScript and scrolling controls stay hidden', async ({browser}) => {
  const context = await browser.newContext({baseURL:base,javaScriptEnabled:false,viewport:{width:390,height:844}});
  try {
    const page=await context.newPage();await page.goto('/');
    await expect(page.locator('[data-hero-prev],[data-hero-next]')).toHaveCount(0);
    for(const button of await page.locator('[data-category-direction]').all()) await expect(button).toBeHidden();
    const category=page.locator('[data-category-track] a').first();
    const href=await category.getAttribute('href');await category.click();
    await expect(page).toHaveURL(new URL(href,base).href);
  } finally {await context.close();}
});
