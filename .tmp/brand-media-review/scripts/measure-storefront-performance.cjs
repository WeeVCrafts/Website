// Reproducible local lab measurements. Never a field p75 or an INP claim.
const {chromium} = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const {output} = require('./ui-ux-qa-paths.cjs');
const target = 'http://127.0.0.1:18080';
const label = process.argv[2] || 'performance';
if (!/^[a-z0-9-]+$/.test(label)) throw new Error('Use a simple artifact label.');

async function main() {
  await fs.mkdir(output, {recursive:true});
  const browser = await chromium.launch();
  try {
    const context = await browser.newContext({viewport:{width:390,height:844},deviceScaleFactor:1});
    await context.addInitScript(() => {
      const metric = window.storefrontMetrics = {lcp:0,cls:0,longTaskMs:0,eventMaxMs:0};
      let sessionStart=0, sessionEnd=0, sessionValue=0;
      new PerformanceObserver(list => {for (const entry of list.getEntries()) {
        metric.lcp=entry.startTime; metric.lcpURL=entry.url;
      }}).observe({type:'largest-contentful-paint',buffered:true});
      new PerformanceObserver(list => {for (const entry of list.getEntries()) {
        if (entry.hadRecentInput) continue;
        if (entry.startTime-sessionEnd>1000 || entry.startTime-sessionStart>5000) {
          sessionStart=entry.startTime;sessionValue=0;
        }
        sessionEnd=entry.startTime;sessionValue+=entry.value;
        metric.cls=Math.max(metric.cls,sessionValue);
      }}).observe({type:'layout-shift',buffered:true});
      new PerformanceObserver(list => {for (const entry of list.getEntries()) metric.longTaskMs+=entry.duration;}).observe({type:'longtask',buffered:true});
      new PerformanceObserver(list => {for (const entry of list.getEntries()) metric.eventMaxMs=Math.max(metric.eventMaxMs,entry.duration);}).observe({type:'event',buffered:true,durationThreshold:16});
    });
    const page = await context.newPage();
    const errors=[];page.on('pageerror',error=>errors.push(error.message));
    const client=await context.newCDPSession(page);
    await client.send('Network.enable');
    await client.send('Network.emulateNetworkConditions',{offline:false,latency:150,downloadThroughput:200000,uploadThroughput:93750});
    await client.send('Emulation.setCPUThrottlingRate',{rate:4});
    const navigations=[];
    for (const route of ['/', '/products', '/products/blue-tote']) {
      for (let run=1;run<=3;run++) {
        await client.send('Network.clearBrowserCache');
        await page.goto(target+route,{waitUntil:'networkidle'});
        await page.evaluate(()=>document.fonts.ready);
        const metric=await page.evaluate(()=>({
          ...window.storefrontMetrics,
          ttfb:performance.getEntriesByType('navigation')[0].responseStart,
          bytes:performance.getEntriesByType('resource').reduce((sum,entry)=>sum+entry.transferSize,0),
          resources:performance.getEntriesByType('resource').map(entry=>({url:new URL(entry.name).pathname,bytes:entry.transferSize})),
          images:Array.from(document.images).filter(image=>image.currentSrc).map(image=>({url:new URL(image.currentSrc).pathname,width:Math.round(image.getBoundingClientRect().width),loading:image.loading,priority:image.fetchPriority})),
          overflow:document.documentElement.scrollWidth>innerWidth+2
        }));
        navigations.push({route,run,...metric});
        if (run===1) await page.screenshot({path:path.join(output,`${label}-${route==='/'?'home':route.slice(1).replaceAll('/','-')}-390.png`)});
      }
      console.log(route+' measured');
      await fs.writeFile(path.join(output,label+'.json'),JSON.stringify({navigations,errors},null,2));
    }
    await page.goto(target+'/',{waitUntil:'networkidle'});
    await page.getByRole('button',{name:'Next collection',exact:true}).click();
    await page.waitForFunction(()=>{const image=document.querySelectorAll('[data-hero-slide] img')[1];return image?.currentSrc && image.complete && image.naturalWidth>0;});
    await page.waitForLoadState('networkidle');
    const secondHero=await page.locator('[data-hero-slide]').nth(1).locator('img').evaluate(image=>({url:new URL(image.currentSrc).pathname,width:Math.round(image.getBoundingClientRect().width),bytes:performance.getEntriesByName(image.currentSrc).at(-1)?.transferSize}));
    await page.goto(target+'/products',{waitUntil:'networkidle'});
    await page.evaluate(()=>{
      window.originalMain=document.getElementById('page');
      window.originalIntro=document.querySelector('.listing-intro');
      window.originalBanner=document.querySelector('.domestic-delivery-banner');
      document.addEventListener('htmx:afterSettle',event=>{window.filterTarget=event.detail.target.id;},{once:true});
    });
    const responsePromise=page.waitForResponse(response=>new URL(response.url()).searchParams.get('sort')==='price-asc');
    const started=Date.now();
    await page.locator('#listing-sort').selectOption('price-asc');
    const response=await responsePromise;
    const html=await response.body();
    await page.waitForFunction(()=>window.filterTarget);
    const filter=await page.evaluate(()=>({target:window.filterTarget,mainPreserved:window.originalMain===document.getElementById('page'),introPreserved:window.originalIntro===document.querySelector('.listing-intro'),bannerPreserved:window.originalBanner===document.querySelector('.domestic-delivery-banner'),eventMaxMs:window.storefrontMetrics.eventMaxMs}));
    filter.responseBytes=html.length;filter.elapsedMs=Date.now()-started;
    await client.send('Network.emulateNetworkConditions',{offline:false,latency:0,downloadThroughput:-1,uploadThroughput:-1});
    await client.send('Emulation.setCPUThrottlingRate',{rate:1});
    await page.setViewportSize({width:1440,height:1000});
    for(const route of ['/', '/products', '/products/blue-tote']) {
      await page.goto(target+route,{waitUntil:'networkidle'});
      await page.screenshot({path:path.join(output,`${label}-${route==='/'?'home':route.slice(1).replaceAll('/','-')}-1440.png`)});
    }
    const result={profile:'Chromium, cold cache, 390x844, DPR 1, 150ms latency, 1.6Mbps down, 4x CPU; 3 navigations per route. Local lab only; eventMaxMs is not INP.',navigations,secondHero,filter,errors};
    await fs.writeFile(path.join(output,label+'.json'),JSON.stringify(result,null,2));
    console.log(JSON.stringify({label,secondHero,filter,errors}));
  } finally {await browser.close();}
}
main().catch(error=>{console.error(error);process.exitCode=1;});
