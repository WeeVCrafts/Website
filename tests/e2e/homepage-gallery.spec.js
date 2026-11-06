const {test,expect}=require('@playwright/test');
const path=require('node:path');
const {output}=require('../../scripts/ui-ux-qa-paths.cjs');
require('node:fs').mkdirSync(output,{recursive:true});

async function galleryFixture(page,count=7) {
  await page.addInitScript(count=>document.addEventListener('DOMContentLoaded',()=>{
    const gallery=document.querySelector('[data-ui-gallery]');if(!gallery)return;
    gallery.querySelectorAll('[data-gallery-media]').forEach((media,i)=>{if(i)media.remove();});
    gallery.querySelectorAll('[data-gallery-select]').forEach((button,i)=>{if(i)button.remove();});
    for(let i=1;i<count;i++) {
      const button=gallery.querySelector('[data-gallery-select]').cloneNode(true);button.setAttribute('aria-pressed','false');
      gallery.querySelector('.thumbs').append(button);
      const media=document.createElement(i===count-1?'video':'img');media.dataset.galleryMedia='';media.hidden=true;
      if(media.tagName==='IMG') {media.dataset.src='/__gallery/image-'+i+'.webp';media.width=800;media.height=800;media.alt='QA photo '+i;}
      else {media.controls=true;media.preload='none';const source=document.createElement('source');source.dataset.src='/__gallery/video.mp4';source.type='video/mp4';media.append(source);}
      gallery.querySelector('.gallery-main').append(media);
    }
  }),count);
  await page.route('**/__gallery/image-*.webp',route=>route.fulfill({path:'web/assets/images/customer/saree-maroon.webp',contentType:'image/webp'}));
  // Match the application's range delivery; WebKit requests byte ranges for MP4.
  const videoBytes=require('node:fs').readFileSync('tests/e2e/fixtures/flower.mp4');
  await page.route('**/__gallery/video.mp4',route=>{
    const range=/^bytes=(\d+)-(\d*)$/.exec(route.request().headers().range||'');
    if(range) {
      const start=Number(range[1]),end=Math.min(Number(range[2]||videoBytes.length-1),videoBytes.length-1);
      return route.fulfill({status:206,contentType:'video/mp4',headers:{'Accept-Ranges':'bytes','Content-Range':`bytes ${start}-${end}/${videoBytes.length}`},body:videoBytes.subarray(start,end+1)});
    }
    return route.fulfill({body:videoBytes,contentType:'video/mp4',headers:{'Accept-Ranges':'bytes'}});
  });
}

test('homepage ordering, trust placement and responsive utility alignment',async({page})=>{
  await page.goto('/');
  const headings=await page.locator('main h2').allTextContents();
  expect(headings.findIndex(text=>/New arrivals/i.test(text))).toBeLessThan(headings.findIndex(text=>/Featured from our makers/i.test(text)));
  await expect(page.locator('.trust-strip')).toHaveCount(1);
  expect(await page.locator('.trust-strip').evaluate(trust=>{
    const collections=document.querySelector('.home-collections');return !!(collections.compareDocumentPosition(trust)&Node.DOCUMENT_POSITION_FOLLOWING);
  })).toBe(true);
  for(const width of [320,390,720,721,768,800,801,1024,1100,1101,1440,1920]) {
    await page.setViewportSize({width,height:900});
    const shipping=page.locator('.homepage-shipping'),support=page.locator('.homepage-support'),slogan=page.locator('.homepage-slogan');
    await expect(shipping).toBeVisible();await expect(support).toBeVisible();
    const left=await shipping.boundingBox(),right=await support.boundingBox();
    expect(left.x+left.width).toBeLessThanOrEqual(right.x);
    if(width>1100) {await expect(slogan).toBeVisible();const middle=await slogan.boundingBox(),bar=await page.locator('.homepage-utility').boundingBox();expect(Math.abs(middle.x+middle.width/2-(bar.x+bar.width/2))).toBeLessThan(2);}
    else await expect(slogan).toBeHidden();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
    if([390,1440].includes(width))await page.screenshot({path:path.join(output,`homepage-gallery-home-${width}.png`)});
  }
});

test('seven gallery items keep counter and controls beneath photo across breakpoints',async({page,browserName})=>{
  await galleryFixture(page);
  await page.goto('/products/blue-tote');
  await page.locator('[data-gallery-next]').click();
  await expect(page.locator('[data-gallery-count]')).toHaveText('2 / 7');
  for(const width of [320,390,720,721,768,800,801,844,1024,1440,1920]) {
    await page.setViewportSize({width,height:width===844?390:900});
    const frame=await page.locator('.gallery-main').boundingBox(),controls=await page.locator('.gallery-controls').boundingBox();
    const count=page.locator('[data-gallery-count]');
    expect(Math.abs(frame.width-frame.height)).toBeLessThan(2);
    expect(controls.y).toBeGreaterThanOrEqual(frame.y+frame.height);
    if(width>720)expect(Math.abs(controls.x-frame.x)).toBeLessThan(2);
    expect(await count.evaluate(element=>element.scrollWidth<=element.clientWidth&&element.getBoundingClientRect().height<30)).toBe(true);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
    if([390,1440].includes(width))await page.screenshot({path:path.join(output,`homepage-gallery-product-${width}.png`)});
  }
  await page.locator('[data-gallery-select]').nth(1).press('End');
  await expect(page.locator('[data-gallery-count]')).toHaveText('7 / 7');
  await expect(page.locator('[data-gallery-zoom]')).toBeHidden();
  if(browserName==='webkit') {
    // Windows WebKit may advertise MP4 without an available system decoder.
    await expect.poll(()=>page.locator('[data-gallery-media]').last().evaluate(video=>video.readyState>=2||video.dataset.failed==='true'||video.dataset.unsupported==='true')).toBe(true);
    const ready=await page.locator('[data-gallery-media]').last().evaluate(video=>video.readyState>=2);
    if(!ready)await expect(page.locator('.gallery-video-feedback')).toContainText(/could not load|cannot play/);
  } else await expect.poll(()=>page.locator('[data-gallery-media]').last().evaluate(video=>video.readyState>=2)).toBe(true);
  await page.locator('[data-gallery-select]').last().press('Home');
  await expect(page.locator('[data-gallery-count]')).toHaveText('1 / 7');
  await page.locator('.gallery-main').evaluate(frame=>{
    frame.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true,pointerType:'touch',isPrimary:true,pointerId:9,clientX:220,clientY:100}));
    frame.dispatchEvent(new PointerEvent('pointerup',{bubbles:true,pointerType:'touch',isPrimary:true,pointerId:9,clientX:100,clientY:102}));
  });
  await expect(page.locator('[data-gallery-count]')).toHaveText('2 / 7');
});

test('photo loading, failure, retry, stale events and zoom feedback',async({page})=>{
  await galleryFixture(page,12);
  let release;
  const pending=new Promise(resolve=>{release=resolve;});
  let fail=true;
  await page.route('**/__gallery/image-1.webp',async route=>{await pending;await route.fulfill(fail?{status:503,body:'unavailable'}:{path:'web/assets/images/customer/saree-maroon.webp',contentType:'image/webp'});});
  try {
    await page.goto('/products/blue-tote');
    await page.locator('[data-gallery-next]').click();
    await expect(page.locator('.gallery-main')).toHaveAttribute('aria-busy','true');
    await expect(page.locator('[data-gallery-status]')).toHaveText('Loading product image…');
    await expect(page.locator('[data-gallery-zoom]')).toBeDisabled();
    await page.locator('[data-gallery-next]').click();
    await expect(page.locator('.gallery-main')).toHaveAttribute('aria-busy','false');
    release();
    await expect.poll(()=>page.locator('[data-gallery-media]').nth(1).getAttribute('data-failed')).toBe('true');
    await expect(page.locator('.gallery-main .gallery-media-feedback')).toBeHidden();
    await page.locator('[data-gallery-prev]').click();
    await expect(page.getByRole('button',{name:'Retry image',exact:true})).toBeVisible();
    fail=false;await page.getByRole('button',{name:'Retry image',exact:true}).click();
    await expect(page.locator('[data-gallery-zoom]')).toBeEnabled();
    await expect(page.locator('[data-gallery-count]')).toHaveText('2 / 12');
    await page.locator('[data-gallery-next]').click();await page.locator('[data-gallery-prev]').click();
    await expect(page.locator('.gallery-main .gallery-media-feedback')).toBeHidden();
    await page.locator('[data-gallery-select]').first().press('Home');
    await expect(page.locator('[data-gallery-zoom]')).toBeEnabled();
    const original=await page.locator('[data-gallery-media]').first().evaluate(image=>image.src);
    let zoomRelease;
    const zoomReady=new Promise(resolve=>{zoomRelease=resolve;});
    await page.route(original,async route=>{await zoomReady;await route.fulfill({status:503,body:'unavailable'});});
    await page.locator('[data-gallery-zoom]').press('Enter');
    await expect(page.locator('.media-lightbox [role=status]')).toHaveText('Loading enlarged image…');
    zoomRelease();await expect(page.locator('.media-lightbox').getByRole('button',{name:'Retry image'})).toBeVisible();
    await page.route(original,route=>route.fulfill({path:'web/assets/images/customer/saree-maroon.webp',contentType:'image/webp'}));
    await page.locator('.media-lightbox').getByRole('button',{name:'Retry image'}).click();
    await expect(page.locator('.media-lightbox .gallery-media-feedback')).toBeHidden();
    await page.keyboard.press('Escape');await expect(page.locator('[data-gallery-zoom]')).toBeFocused();
  } finally {release();}
});
