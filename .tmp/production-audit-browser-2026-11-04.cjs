const {chromium}=require('playwright');
const fs=require('node:fs');
const path=require('node:path');
(async()=>{
 const out=path.resolve('.tmp/production-audit-2026-11-04');fs.mkdirSync(out,{recursive:true});
 const browser=await chromium.launch({headless:true});const result=[];
 for(const width of [390,1440]){
  const context=await browser.newContext({viewport:{width,height:900},reducedMotion:'reduce'});
  for(const route of ['/','/products','/categories','/login']){
   const page=await context.newPage();const errors=[];const failed=[];
   page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(r.status()>=400)failed.push({url:r.url().replace(/\?.*$/,''),status:r.status()});});
   const response=await page.goto('http://127.0.0.1:8080'+route,{waitUntil:'networkidle',timeout:30000});
   await page.addScriptTag({path:require.resolve('axe-core')});
   const data=await page.evaluate(async()=>({title:document.title,h1:[...document.querySelectorAll('h1')].map(e=>e.innerText),overflow:document.documentElement.scrollWidth>innerWidth+2,viewport:innerWidth,documentWidth:document.documentElement.scrollWidth,brokenImages:[...document.images].filter(i=>i.complete&&i.currentSrc&&i.naturalWidth===0).map(i=>i.currentSrc),images:[...document.images].slice(0,8).map(i=>({src:i.currentSrc,width:i.naturalWidth,renderWidth:Math.round(i.getBoundingClientRect().width),srcset:!!i.srcset,loading:i.loading})),violations:(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations.map(v=>({id:v.id,impact:v.impact,count:v.nodes.length,examples:v.nodes.slice(0,2).map(n=>({target:n.target,summary:n.failureSummary}))})),nav:performance.getEntriesByType('navigation').map(n=>({ttfb:n.responseStart,domContentLoaded:n.domContentLoadedEventEnd,load:n.loadEventEnd})),resources:performance.getEntriesByType('resource').filter(r=>r.name.includes('/assets/')).map(r=>({url:r.name.replace(location.origin,''),bytes:r.transferSize,duration:r.duration}))}));
   await page.keyboard.press('Tab');data.firstTab=await page.evaluate(()=>({tag:document.activeElement.tagName,text:document.activeElement.textContent.trim().slice(0,80),href:document.activeElement.getAttribute('href')}));
   const file=width+'-'+(route==='/'?'home':route.slice(1))+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});
   result.push({route,width,status:response.status(),headers:response.headers(),errors,failed,...data,screenshot:file});console.log(JSON.stringify({route,width,status:response.status(),overflow:data.overflow,violations:data.violations.map(v=>({id:v.id,count:v.count})),errors,failed}));await page.close();
  }
  await context.close();
 }
 await browser.close();fs.writeFileSync(path.join(out,'browser-results.json'),JSON.stringify(result,null,2));
})().catch(e=>{console.error(e);process.exitCode=1});
