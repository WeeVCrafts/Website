const { chromium } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

// Dedicated QA only. Captures do not seed/reset the active local application.
async function main() {
  const target = process.argv[2] || 'http://127.0.0.1:18081';
  const label = process.argv[3] || 'baseline';
  if (!/^http:\/\/127\.0\.0\.1:1808[01]$/.test(target)) throw new Error('Use the isolated UI QA endpoint.');
  const {output} = require('./ui-ux-qa-paths.cjs');
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL: target, viewport: {width:390,height:844} });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  try {
    await page.goto('/login');
    await page.locator('#page input[name="email"]').fill('demo.customer01@weevcrafts.local');
    await page.locator('#page input[name="password"]').fill('WeeVCrafts-Demo-2026!');
    await page.getByRole('button', {name:'Sign in',exact:true}).click();
    await page.waitForURL(url=>url.pathname==='/products');
    await page.addInitScript(()=>{
      window.uiMeasurements={lcp:0,cls:0,longTasks:0,longTaskMs:0,eventMaxMs:0};
      new PerformanceObserver(list=>{for(const entry of list.getEntries()){window.uiMeasurements.lcp=entry.startTime;window.uiMeasurements.lcpElement=entry.element?.outerHTML?.slice(0,300);window.uiMeasurements.lcpURL=entry.url;}}).observe({type:'largest-contentful-paint',buffered:true});
      new PerformanceObserver(list=>{for(const entry of list.getEntries())if(!entry.hadRecentInput)window.uiMeasurements.cls+=entry.value;}).observe({type:'layout-shift',buffered:true});
      new PerformanceObserver(list=>{for(const entry of list.getEntries()){window.uiMeasurements.longTasks++;window.uiMeasurements.longTaskMs+=entry.duration;}}).observe({type:'longtask',buffered:true});
      new PerformanceObserver(list=>{for(const entry of list.getEntries())window.uiMeasurements.eventMaxMs=Math.max(window.uiMeasurements.eventMaxMs,entry.duration);}).observe({type:'event',buffered:true,durationThreshold:16});
    });
    const client=await context.newCDPSession(page);
    await client.send('Network.enable');
    await client.send('Network.emulateNetworkConditions',{offline:false,latency:150,downloadThroughput:200000,uploadThroughput:93750});
    await client.send('Emulation.setCPUThrottlingRate',{rate:4});
    const measures=[];
    for(const route of ['/', '/products', '/products/blue-tote', '/cart', '/checkout']) for(const cache of ['cold','warm']) {
      if(cache==='cold')await client.send('Network.clearBrowserCache');
      await page.goto(route,{waitUntil:'networkidle'});
      await page.evaluate(()=>document.fonts.ready);
      const metric=await page.evaluate(()=>({
        ...window.uiMeasurements,
        ttfb:performance.getEntriesByType('navigation')[0].responseStart,
        bytes:performance.getEntriesByType('resource').reduce((total,entry)=>total+entry.transferSize,0),
        resources:performance.getEntriesByType('resource').length,
        overflow:document.documentElement.scrollWidth>innerWidth+2,
        font:getComputedStyle(document.body).fontFamily
        ,resourceDetails:performance.getEntriesByType('resource').map(entry=>({url:entry.name,bytes:entry.transferSize,start:entry.startTime,end:entry.responseEnd}))
      }));
      await page.getByRole('button',{name:'Open profile menu',exact:true}).click();await page.keyboard.press('Escape');
      const interaction=await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(window.uiMeasurements.eventMaxMs)))));
      measures.push({route,cache,...metric,eventMaxMs:interaction});
      const name=route==='/'?'home':route.split('/').filter(Boolean).join('-');
      if(cache==='cold')await page.screenshot({path:path.join(output,`${label}-${name}-390.png`)});
    }
    await client.send('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1});
    await client.send('Emulation.setCPUThrottlingRate',{rate:1});
    await page.setViewportSize({width:1440,height:1000});
    for(const route of ['/', '/products', '/products/blue-tote', '/checkout']) {
      await page.goto(route,{waitUntil:'networkidle'});
      const name=route==='/'?'home':route.split('/').filter(Boolean).join('-');
      await page.screenshot({path:path.join(output,`${label}-${name}-1440.png`)});
    }
    await fs.writeFile(path.join(output,`${label}-measurements.json`),JSON.stringify({profile:'390x844, 150ms latency, 1.6Mbps down, 4x CPU; single lab navigation, not field p75/INP',target,measures,errors},null,2));
    console.log(JSON.stringify({label,measures:measures.map(({resourceDetails,lcpElement,...metric})=>metric),errors}));
  } finally {await browser.close();}
}
main().catch(error=>{console.error(error.message);process.exitCode=1;});
